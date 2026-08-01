package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Live grounding: alongside the last scan's findings, the Copilot is given a
// real-time snapshot of the cluster gathered from the API at question time —
// which pods are unhealthy right now, the most recent Warning events, and any
// node problems. This is what lets it answer "what is crashing?" accurately
// instead of only reasoning over a scan that may be minutes or hours old. It is
// strictly read-only (list calls) and best-effort: any gather error degrades to
// findings-only grounding rather than failing the chat.

const (
	maxLivePods   = 25
	maxLiveEvents = 15
)

type podProblem struct {
	namespace string
	name      string
	reason    string
	restarts  int32
}

type eventNote struct {
	namespace string
	object    string
	reason    string
	message   string
	count     int32
}

// liveSnapshot is the current cluster health as seen right now.
type liveSnapshot struct {
	problemPods []podProblem
	warnings    []eventNote
	nodeIssues  []string
}

func (l *liveSnapshot) empty() bool {
	return l == nil || (len(l.problemPods) == 0 && len(l.warnings) == 0 && len(l.nodeIssues) == 0)
}

// gatherLiveSnapshot reads the current unhealthy pods, recent Warning events and
// node problems. Returns nil if nothing useful could be read. namespace ""
// snapshots the whole cluster.
func gatherLiveSnapshot(ctx context.Context, clientset kubernetes.Interface, namespace string) *liveSnapshot {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	snap := &liveSnapshot{}
	snap.problemPods = gatherProblemPods(ctx, clientset, ns)
	snap.warnings = gatherWarningEvents(ctx, clientset, ns)
	snap.nodeIssues = gatherNodeIssues(ctx, clientset)

	if snap.empty() {
		return nil
	}

	return snap
}

func gatherProblemPods(ctx context.Context, clientset kubernetes.Interface, ns string) []podProblem {
	pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var out []podProblem

	for i := range pods.Items {
		if p := podHealth(&pods.Items[i]); p != nil {
			out = append(out, *p)
		}
	}

	// Most restarts first — the loudest problems lead.
	sort.SliceStable(out, func(i, j int) bool { return out[i].restarts > out[j].restarts })

	if len(out) > maxLivePods {
		out = out[:maxLivePods]
	}

	return out
}

// podHealth returns a problem record if the pod is unhealthy, else nil.
func podHealth(p *corev1.Pod) *podProblem {
	var restarts int32
	for _, cs := range p.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}

	// Pod-level phase problems.
	switch p.Status.Phase {
	case corev1.PodPending, corev1.PodFailed, corev1.PodUnknown:
		reason := string(p.Status.Phase)
		if p.Status.Reason != "" {
			reason = p.Status.Reason
		}

		return &podProblem{p.Namespace, p.Name, reason, restarts}
	}

	// Running/Succeeded: flag containers stuck waiting or repeatedly terminated.
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			continue
		}

		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" && cs.State.Waiting.Reason != "ContainerCreating" {
			return &podProblem{p.Namespace, p.Name, cs.State.Waiting.Reason, restarts}
		}

		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" && cs.State.Terminated.Reason != "Completed" {
			return &podProblem{p.Namespace, p.Name, cs.State.Terminated.Reason, restarts}
		}
	}

	return nil
}

func gatherWarningEvents(ctx context.Context, clientset kubernetes.Interface, ns string) []eventNote {
	events, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	warnings := make([]corev1.Event, 0, len(events.Items))

	for _, e := range events.Items {
		if e.Type == corev1.EventTypeWarning {
			warnings = append(warnings, e)
		}
	}

	// Most recent first.
	sort.SliceStable(warnings, func(i, j int) bool {
		return eventTime(warnings[i]).After(eventTime(warnings[j]))
	})

	var out []eventNote

	for _, e := range warnings {
		out = append(out, eventNote{
			namespace: e.Namespace,
			object:    e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
			reason:    e.Reason,
			message:   e.Message,
			count:     e.Count,
		})

		if len(out) >= maxLiveEvents {
			break
		}
	}

	return out
}

func eventTime(e corev1.Event) time.Time {
	if !e.LastTimestamp.IsZero() {
		return e.LastTimestamp.Time
	}

	return e.EventTime.Time
}

func gatherNodeIssues(ctx context.Context, clientset kubernetes.Interface) []string {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var out []string

	for i := range nodes.Items {
		node := &nodes.Items[i]

		for _, c := range node.Status.Conditions {
			switch {
			case c.Type == corev1.NodeReady && c.Status != corev1.ConditionTrue:
				out = append(out, fmt.Sprintf("%s is NotReady", node.Name))
			case c.Type != corev1.NodeReady && c.Status == corev1.ConditionTrue &&
				strings.HasSuffix(string(c.Type), "Pressure"):
				out = append(out, fmt.Sprintf("%s reports %s", node.Name, c.Type))
			}
		}

		if node.Spec.Unschedulable {
			out = append(out, fmt.Sprintf("%s is cordoned (unschedulable)", node.Name))
		}
	}

	return out
}

// renderLiveSnapshot appends the live-state section to the system prompt.
func renderLiveSnapshot(b *strings.Builder, live *liveSnapshot) {
	if live.empty() {
		b.WriteString("\nLIVE CLUSTER STATE (as of now): no unhealthy pods, warning events, or node problems detected.\n")
		return
	}

	b.WriteString("\nLIVE CLUSTER STATE (as of now):\n")

	if len(live.problemPods) > 0 {
		b.WriteString("Unhealthy pods:\n")

		for _, p := range live.problemPods {
			fmt.Fprintf(b, "- %s/%s: %s", strings.TrimPrefix(p.namespace+"/", "/"), p.name, p.reason)
			if p.restarts > 0 {
				fmt.Fprintf(b, " (%d restarts)", p.restarts)
			}

			b.WriteByte('\n')
		}
	}

	if len(live.warnings) > 0 {
		b.WriteString("Recent Warning events (newest first):\n")

		for _, e := range live.warnings {
			ns := strings.TrimSuffix(e.namespace+"/", "/")
			fmt.Fprintf(b, "- %s%s [%s]: %s", ns, e.object, e.reason, strings.TrimSpace(e.message))

			if e.count > 1 {
				fmt.Fprintf(b, " (x%d)", e.count)
			}

			b.WriteByte('\n')
		}
	}

	if len(live.nodeIssues) > 0 {
		b.WriteString("Node problems:\n")

		for _, n := range live.nodeIssues {
			fmt.Fprintf(b, "- %s\n", n)
		}
	}
}
