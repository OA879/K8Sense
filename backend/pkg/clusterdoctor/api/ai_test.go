package api

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/OA879/K8Sense/backend/pkg/clusterdoctor"
)

func TestRenderGroundingPrompt_SortsAndSummarises(t *testing.T) {
	findings := []clusterdoctor.Finding{
		{Severity: "medium", RuleName: "cpu-noise", ResourceKind: "Deployment", Namespace: "app", ResourceName: "api", Description: "no cpu limit"},
		{Severity: "critical", RuleName: "priv-pod", ResourceKind: "Pod", Namespace: "app", ResourceName: "root-shell", Description: "runs privileged", Remediation: "drop privileged"},
	}

	prompt := renderGroundingPrompt("prod", findings, nil)

	if !strings.Contains(prompt, "K8sense Copilot") || !strings.Contains(prompt, "CLUSTER: prod") {
		t.Error("missing role framing / cluster name")
	}
	if !strings.Contains(prompt, "1 critical, 1 medium") {
		t.Errorf("bad severity summary in:\n%s", prompt)
	}
	crit := strings.Index(prompt, "root-shell")
	med := strings.Index(prompt, "api")
	if crit == -1 || med == -1 || crit > med {
		t.Errorf("critical finding should precede medium; crit=%d med=%d", crit, med)
	}
	if !strings.Contains(prompt, "drop privileged") {
		t.Error("remediation not included")
	}
}

func TestRenderGroundingPrompt_NoContext(t *testing.T) {
	if !strings.Contains(renderGroundingPrompt("", nil, nil), "No specific cluster") {
		t.Error("empty cluster should be stated")
	}
	if !strings.Contains(renderGroundingPrompt("prod", nil, nil), "none available") {
		t.Error("no-findings case should be stated")
	}
}

func TestRenderGroundingPrompt_IncludesLiveState(t *testing.T) {
	live := &liveSnapshot{
		problemPods: []podProblem{{namespace: "app", name: "web-0", reason: "CrashLoopBackOff", restarts: 7}},
		warnings:    []eventNote{{namespace: "app", object: "Pod/web-0", reason: "BackOff", message: "back-off restarting", count: 5}},
		nodeIssues:  []string{"node-2 is NotReady"},
	}

	prompt := renderGroundingPrompt("prod", nil, live)

	for _, want := range []string{"LIVE CLUSTER STATE", "CrashLoopBackOff", "7 restarts", "BackOff", "node-2 is NotReady"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("live prompt missing %q in:\n%s", want, prompt)
		}
	}
}

func TestGatherLiveSnapshot_FlagsUnhealthyPodsAndNodes(t *testing.T) {
	crashing := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "bad"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Ready:        false,
				RestartCount: 4,
				State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "good"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}
	badNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
			{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
		}},
	}

	snap := gatherLiveSnapshot(context.Background(), k8sfake.NewSimpleClientset(crashing, healthy, badNode), "")
	if snap == nil {
		t.Fatal("expected a snapshot")
	}
	if len(snap.problemPods) != 1 || snap.problemPods[0].name != "bad" || snap.problemPods[0].reason != "CrashLoopBackOff" {
		t.Errorf("problem pods = %+v", snap.problemPods)
	}
	if snap.problemPods[0].restarts != 4 {
		t.Errorf("restarts = %d", snap.problemPods[0].restarts)
	}
	if len(snap.nodeIssues) != 1 || !strings.Contains(snap.nodeIssues[0], "NotReady") {
		t.Errorf("node issues = %v", snap.nodeIssues)
	}
}

func TestGatherLiveSnapshot_HealthyClusterReturnsNil(t *testing.T) {
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app", Name: "good"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}
	if snap := gatherLiveSnapshot(context.Background(), k8sfake.NewSimpleClientset(healthy), ""); snap != nil {
		t.Errorf("healthy cluster should yield nil snapshot, got %+v", snap)
	}
}
