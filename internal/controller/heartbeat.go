package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	fvv1alpha1 "github.com/FerrLabs/FerrVault/api/ferrvault/v1alpha1"
)

type Heartbeat struct {
	mu   sync.Mutex
	last time.Time
}

func NewHeartbeat(now time.Time) *Heartbeat {
	return &Heartbeat{last: now}
}

func (h *Heartbeat) Beat(now time.Time) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if now.After(h.last) {
		h.last = now
	}
}

func (h *Heartbeat) Idle(now time.Time) time.Duration {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return now.Sub(h.last)
}

func StallChecker(c client.Client, hb *Heartbeat, threshold time.Duration) healthz.Checker {
	return func(req *http.Request) error {
		idle := hb.Idle(time.Now())
		if idle <= threshold {
			return nil
		}
		watching, err := hasWatchedResources(req.Context(), c)
		if err != nil || !watching {
			return nil
		}
		return fmt.Errorf("no reconcile completed for %s", idle.Truncate(time.Second))
	}
}

func hasWatchedResources(ctx context.Context, c client.Client) (bool, error) {
	var conns fvv1alpha1.FerrVaultConnectionList
	if err := c.List(ctx, &conns, client.Limit(1)); err != nil {
		return false, err
	}
	if len(conns.Items) > 0 {
		return true, nil
	}
	var secrets fvv1alpha1.FerrVaultSecretList
	if err := c.List(ctx, &secrets, client.Limit(1)); err != nil {
		return false, err
	}
	return len(secrets.Items) > 0, nil
}
