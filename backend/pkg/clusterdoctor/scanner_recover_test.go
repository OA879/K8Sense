package clusterdoctor

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"
)

func TestRunCheckSafely_RecoversPanic(t *testing.T) {
	panicky := func(context.Context, kubernetes.Interface) ([]RawFinding, error) {
		panic("boom")
	}

	raws, err := runCheckSafely(context.Background(), nil, panicky)
	if err == nil {
		t.Fatal("expected an error from a panicking check, got nil")
	}
	if raws != nil {
		t.Fatalf("expected no findings from a panicking check, got %d", len(raws))
	}
}

func TestRunCheckSafely_PassesThrough(t *testing.T) {
	ok := func(context.Context, kubernetes.Interface) ([]RawFinding, error) {
		return []RawFinding{{ResourceKind: "Pod", ResourceName: "x"}}, nil
	}

	raws, err := runCheckSafely(context.Background(), nil, ok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raws) != 1 {
		t.Fatalf("expected 1 finding passed through, got %d", len(raws))
	}
}
