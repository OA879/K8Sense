package api

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func deployWithLabels(ns, name string, podLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
			},
		},
	}
}

func TestBuildNetworkMap_PolicyExposureAndEdges(t *testing.T) {
	frontend := deployWithLabels("shop", "frontend", map[string]string{"app": "frontend"})
	backend := deployWithLabels("shop", "backend", map[string]string{"app": "backend"})

	// Only "backend" is protected, and only "frontend" may reach it.
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "backend-allow-frontend"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
				}},
			}},
		},
	}

	cs := k8sfake.NewSimpleClientset(frontend, backend, policy)

	m, err := buildNetworkMap(context.Background(), cs, "")
	if err != nil {
		t.Fatal(err)
	}

	exposure := map[string]string{}
	for _, n := range m.Nodes {
		exposure[n.Name] = n.Exposure
	}

	if exposure["frontend"] != "open" {
		t.Errorf("frontend exposure = %q, want open (no policy selects it)", exposure["frontend"])
	}

	if exposure["backend"] != "restricted" {
		t.Errorf("backend exposure = %q, want restricted", exposure["backend"])
	}

	if len(m.Edges) != 1 {
		t.Fatalf("got %d policy edges, want 1 (frontend->backend)", len(m.Edges))
	}

	if m.Edges[0].Source != "Deployment/shop/frontend" || m.Edges[0].Target != "Deployment/shop/backend" {
		t.Errorf("edge = %s -> %s, want frontend -> backend", m.Edges[0].Source, m.Edges[0].Target)
	}

	// No mesh Prometheus in a fake cluster → overlay absent, request still fine.
	if m.Mesh.Enabled {
		t.Errorf("mesh should be disabled without a reachable Prometheus")
	}
}

func TestBuildNetworkMap_IsolatedAndDefaultAllow(t *testing.T) {
	// A default-deny policy (selects all, no ingress rules) isolates the workload.
	denyAll := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: "locked", Name: "default-deny"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{}, // empty = all pods
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		},
	}
	locked := deployWithLabels("locked", "vault", map[string]string{"app": "vault"})
	wideOpen := deployWithLabels("public", "web", map[string]string{"app": "web"})

	cs := k8sfake.NewSimpleClientset(denyAll, locked, wideOpen)

	m, err := buildNetworkMap(context.Background(), cs, "")
	if err != nil {
		t.Fatal(err)
	}

	exposure := map[string]string{}
	for _, n := range m.Nodes {
		exposure[n.Name] = n.Exposure
	}

	if exposure["vault"] != "isolated" {
		t.Errorf("vault exposure = %q, want isolated (default-deny)", exposure["vault"])
	}

	if exposure["web"] != "open" {
		t.Errorf("web exposure = %q, want open (no policy in its namespace)", exposure["web"])
	}
}

func TestClassifyDB(t *testing.T) {
	cases := []struct {
		name       string
		containers []corev1.Container
		want       string
	}{
		{"postgres image", []corev1.Container{{Image: "postgres:16"}}, "postgres"},
		{"mariadb image -> mysql", []corev1.Container{{Image: "docker.io/mariadb:11"}}, "mysql"},
		{"redis by port", []corev1.Container{{Image: "some/app", Ports: []corev1.ContainerPort{{ContainerPort: 6379}}}}, "redis"},
		{"exporter is not a db", []corev1.Container{{Image: "prometheuscommunity/postgres-exporter"}}, ""},
		{"plain app", []corev1.Container{{Image: "nginx:alpine", Ports: []corev1.ContainerPort{{ContainerPort: 80}}}}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyDB(c.containers); got != c.want {
				t.Fatalf("classifyDB = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuildNetworkMap_DetectsDatabase(t *testing.T) {
	db := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "orders-db"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "postgres:16"}}},
			},
		},
	}
	app := deployWithLabels("data", "orders", map[string]string{"app": "orders"})

	m, err := buildNetworkMap(context.Background(), k8sfake.NewSimpleClientset(db, app), "")
	if err != nil {
		t.Fatal(err)
	}

	var dbNode, appNode *netNode
	for i := range m.Nodes {
		switch m.Nodes[i].Name {
		case "orders-db":
			dbNode = &m.Nodes[i]
		case "orders":
			appNode = &m.Nodes[i]
		}
	}

	if dbNode == nil || !dbNode.Database || dbNode.DBEngine != "postgres" {
		t.Fatalf("orders-db node = %+v, want database postgres", dbNode)
	}
	if appNode == nil || appNode.Database {
		t.Fatalf("orders app should not be flagged as a database: %+v", appNode)
	}
}

func TestExternalDBNodes(t *testing.T) {
	extName := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "rds-postgres"},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "prod.abc123.eu-west-1.rds.amazonaws.com",
			Ports:        []corev1.ServicePort{{Port: 5432}},
		},
	}
	selectorless := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "external-redis"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Port: 6379}},
		},
	}

	nodes, err := externalDBNodes(context.Background(),
		k8sfake.NewSimpleClientset(extName, selectorless), "")
	if err != nil {
		t.Fatal(err)
	}

	engines := map[string]string{}
	for _, n := range nodes {
		if n.Kind != "External" || !n.Database {
			t.Errorf("node %s should be an external database: %+v", n.ID, n)
		}
		engines[n.ID] = n.DBEngine
	}

	if engines["External/data/rds-postgres"] != "postgres" {
		t.Errorf("rds-postgres engine = %q, want postgres", engines["External/data/rds-postgres"])
	}
	if engines["External/data/external-redis"] != "redis" {
		t.Errorf("external-redis engine = %q, want redis", engines["External/data/external-redis"])
	}
}

func TestReferencesService(t *testing.T) {
	tok := serviceRefTokens("orders-db", "shop")
	cases := []struct {
		text string
		want bool
	}{
		{"postgres://orders-db:5432/orders", true},   // bare name, host-like
		{"host: orders-db.shop.svc.cluster.local", true}, // fqdn
		{"myorders-dbx=1", false},                    // substring, not a token
		{"orders-db-replica:5432", false},            // different service (dash after)
		{"nothing relevant here", false},
	}
	for _, c := range cases {
		if got := referencesService(strings.ToLower(c.text), "orders-db", tok); got != c.want {
			t.Errorf("referencesService(%q) = %v, want %v", c.text, got, c.want)
		}
	}

	// Short names are not matched bare (avoid noise); only FQDN would.
	if referencesService("db db db", "db", serviceRefTokens("db", "x")) {
		t.Error("short bare name 'db' should not match")
	}
}

func TestInferredConnections_AppToDB(t *testing.T) {
	ordersTmpl := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "orders"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Image: "orders:1",
			Env:   []corev1.EnvVar{{Name: "DATABASE_URL", Value: "postgres://orders-db:5432/orders"}},
		}}},
	}
	dbTmpl := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "orders-db"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "postgres:16"}}},
	}

	orders := makeWorkload("shop", "orders", "Deployment", ordersTmpl)
	db := makeWorkload("shop", "orders-db", "StatefulSet", dbTmpl)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "orders-db"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "orders-db"}},
	}

	edges := inferredConnections(context.Background(), k8sfake.NewSimpleClientset(svc),
		[]workload{orders, db}, "")

	if len(edges) != 1 {
		t.Fatalf("got %d inferred edges, want 1: %+v", len(edges), edges)
	}
	e := edges[0]
	if e.Source != "Deployment/shop/orders" || e.Target != "StatefulSet/shop/orders-db" || e.Via != "orders-db" {
		t.Fatalf("edge = %+v, want orders -> orders-db via orders-db", e)
	}
}
