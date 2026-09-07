package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fvv1alpha1 "github.com/FerrLabs/FerrVault/api/ferrvault/v1alpha1"
)

const stallThresholdForTest = 15 * time.Minute

func connectionFixture() *fvv1alpha1.FerrVaultConnection {
	return &fvv1alpha1.FerrVaultConnection{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "ferrvault-dev"},
	}
}

func checkerFor(t *testing.T, hb *Heartbeat, objs ...client.Object) error {
	t.Helper()
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(objs...).Build()
	return StallChecker(c, hb, stallThresholdForTest)(&http.Request{})
}

func TestABeatingOperatorIsLive(t *testing.T) {
	hb := NewHeartbeat(time.Now().Add(-stallThresholdForTest + time.Minute))
	if err := checkerFor(t, hb, connectionFixture()); err != nil {
		t.Fatalf("reported dead while still reconciling: %v", err)
	}
}

func TestAStalledOperatorWithWorkIsNotLive(t *testing.T) {
	hb := NewHeartbeat(time.Now().Add(-stallThresholdForTest - time.Minute))
	if err := checkerFor(t, hb, connectionFixture()); err == nil {
		t.Fatal("reported live after reconciling nothing for longer than the threshold")
	}
}

func TestAnIdleClusterIsNotRestarted(t *testing.T) {
	hb := NewHeartbeat(time.Now().Add(-24 * time.Hour))
	if err := checkerFor(t, hb); err != nil {
		t.Fatalf("reported dead with no FerrVault resources to reconcile: %v", err)
	}
}

func TestAnUnreadableCacheIsNotRestarted(t *testing.T) {
	c := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return errors.New("cache not synced")
			},
		}).
		Build()

	hb := NewHeartbeat(time.Now().Add(-24 * time.Hour))
	if err := StallChecker(c, hb, stallThresholdForTest)(&http.Request{}); err != nil {
		t.Fatalf("reported dead because the cache was unreadable: %v", err)
	}
}

func TestBeatNeverMovesBackwards(t *testing.T) {
	now := time.Now()
	hb := NewHeartbeat(now)
	hb.Beat(now.Add(-time.Hour))
	if idle := hb.Idle(now); idle != 0 {
		t.Fatalf("an older beat moved the heartbeat back by %s", idle)
	}
}

func TestANilHeartbeatIsInert(t *testing.T) {
	var hb *Heartbeat
	hb.Beat(time.Now())
	if idle := hb.Idle(time.Now()); idle != 0 {
		t.Fatalf("a nil heartbeat reported %s idle", idle)
	}
}
