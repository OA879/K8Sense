package api

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Database awareness for the Network Map. Databases are inferred from the
// Kubernetes API — no mesh needed: in-cluster ones by their container image or
// listening port, external ones by ExternalName / selector-less Services that
// front an off-cluster datastore. It's a heuristic (a metrics exporter image
// can look database-ish), but a high-signal one for "where is the data?".

// dbImageSignatures maps an image-name substring to a normalised engine label.
//
//nolint:gochecknoglobals // static lookup table
var dbImageSignatures = []struct{ sub, engine string }{
	{"postgres", "postgres"}, {"timescale", "postgres"},
	{"mysql", "mysql"}, {"mariadb", "mysql"}, {"percona", "mysql"},
	{"mongo", "mongodb"},
	{"redis", "redis"}, {"valkey", "redis"},
	{"cassandra", "cassandra"}, {"scylla", "cassandra"},
	{"cockroach", "cockroachdb"},
	{"elasticsearch", "elasticsearch"}, {"opensearch", "elasticsearch"},
	{"clickhouse", "clickhouse"},
	{"memcached", "memcached"},
	{"rabbitmq", "rabbitmq"},
	{"neo4j", "neo4j"},
}

// dbPortEngines maps a listening port to an engine. Only a database *server*
// listens on these, so this is a reliable confirmation signal.
//
//nolint:gochecknoglobals // static lookup table
var dbPortEngines = map[int32]string{
	5432: "postgres", 3306: "mysql", 27017: "mongodb", 6379: "redis",
	9042: "cassandra", 26257: "cockroachdb", 9200: "elasticsearch",
	8123: "clickhouse", 11211: "memcached", 5672: "rabbitmq", 7687: "neo4j",
}

// classifyDB returns the database engine a set of containers looks like, or ""
// if it isn't a datastore. Image name is checked first, then listening ports.
func classifyDB(containers []corev1.Container) string {
	for _, c := range containers {
		img := strings.ToLower(c.Image)
		// Skip obvious sidecars that merely mention a DB (metrics exporters).
		if strings.Contains(img, "exporter") {
			continue
		}

		for _, sig := range dbImageSignatures {
			if strings.Contains(img, sig.sub) {
				return sig.engine
			}
		}
	}

	for _, c := range containers {
		for _, p := range c.Ports {
			if eng, ok := dbPortEngines[p.ContainerPort]; ok {
				return eng
			}
		}
	}

	return ""
}

// engineForServicePorts returns the engine implied by a Service's ports, if any.
func engineForServicePorts(ports []corev1.ServicePort) string {
	for _, p := range ports {
		if eng, ok := dbPortEngines[p.Port]; ok {
			return eng
		}

		if eng, ok := dbPortEngines[p.TargetPort.IntVal]; ok {
			return eng
		}
	}

	return ""
}

// externalDBNodes finds datastores that live outside the cluster but are
// reachable through it: ExternalName Services (DNS aliases to a managed DB) and
// selector-less Services on a database port (whose endpoints point off-cluster).
// They appear as "external" nodes so the map shows the cluster's data
// dependencies even though there is no in-cluster pod for them.
func externalDBNodes(ctx context.Context, clientset kubernetes.Interface, ns string) ([]netNode, error) {
	svcs, err := clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodes []netNode

	for _, svc := range svcs.Items {
		engine := engineForServicePorts(svc.Spec.Ports)

		switch {
		case svc.Spec.Type == corev1.ServiceTypeExternalName:
			// A DNS alias to an external host — always an external dependency;
			// badge it as a DB when the port looks like one.
			target := svc.Spec.ExternalName
			nodes = append(nodes, netNode{
				ID: "External/" + svc.Namespace + "/" + svc.Name, Namespace: svc.Namespace,
				Name: target, Kind: "External", Exposure: "external",
				Database: engine != "", DBEngine: engine,
			})

		case len(svc.Spec.Selector) == 0 && engine != "":
			// Selector-less Service on a DB port — an off-cluster datastore
			// fronted by manual endpoints.
			nodes = append(nodes, netNode{
				ID: "External/" + svc.Namespace + "/" + svc.Name, Namespace: svc.Namespace,
				Name: svc.Name, Kind: "External", Exposure: "external",
				Database: true, DBEngine: engine,
			})
		}
	}

	return nodes, nil
}
