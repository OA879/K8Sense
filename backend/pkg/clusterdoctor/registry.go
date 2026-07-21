package clusterdoctor

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// CheckFunc inspects the cluster for one specific problem (e.g. nodes stuck
// NotReady) and returns one RawFinding per affected resource. It must never
// panic and should return a non-nil error only when the check itself
// couldn't run (e.g. RBAC forbids listing the resource) — the scanner turns
// that into a skipped-check count rather than failing the whole scan.
type CheckFunc func(ctx context.Context, clientset kubernetes.Interface) ([]RawFinding, error)

//nolint:gochecknoglobals // process-wide registry, populated by checks package init()s
var checkRegistry = map[string]CheckFunc{}

// RegisterCheck makes a check function available to rules whose check_fn
// field matches name. Called from init() in the checks sub-package.
func RegisterCheck(name string, fn CheckFunc) {
	checkRegistry[name] = fn
}

// GetCheck looks up a previously registered check function by name.
func GetCheck(name string) (CheckFunc, bool) {
	fn, ok := checkRegistry[name]
	return fn, ok
}
