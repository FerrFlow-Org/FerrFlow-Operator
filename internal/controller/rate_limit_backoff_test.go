package controller

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"
)

func TestBackoffGrowsAndCaps(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: rateLimitBackoffBase},
		{attempt: 1, want: 10 * time.Second},
		{attempt: 2, want: 20 * time.Second},
		{attempt: 5, want: 160 * time.Second},
		{attempt: 6, want: rateLimitBackoffMax},
		{attempt: 99, want: rateLimitBackoffMax},
		{attempt: -1, want: rateLimitBackoffBase},
	}
	for _, tc := range cases {
		if got := backoffFor(tc.attempt); got != tc.want {
			t.Errorf("backoffFor(%d) = %s, want %s", tc.attempt, got, tc.want)
		}
	}
}

func TestConsecutiveRateLimitsOutrunAFixedWait(t *testing.T) {
	const (
		resources  = 29
		refillPerS = 1.0
		fixedWait  = 20 * time.Second
	)

	offered := func(wait time.Duration) float64 {
		return resources / wait.Seconds()
	}

	if offered(fixedWait) <= refillPerS {
		t.Fatalf("the fixed wait this replaces was not actually saturating, "+
			"offered %.2f/s against %.2f/s refill", offered(fixedWait), refillPerS)
	}

	b := newRateLimitBackoff()
	key := types.NamespacedName{Namespace: "default", Name: "app-secrets"}
	var last time.Duration
	for round := 0; round < 8; round++ {
		last = b.next(key)
		if offered(last) <= refillPerS {
			return
		}
	}
	t.Fatalf("still offering %.2f/s after eight rounds, last wait %s",
		offered(last), last)
}

func TestJitterStaysWithinHalfTheDelay(t *testing.T) {
	b := newRateLimitBackoff()
	key := types.NamespacedName{Namespace: "default", Name: "app-secrets"}
	for i := 0; i < 200; i++ {
		b.forget(key)
		got := b.next(key)
		if got < rateLimitBackoffBase/2 || got >= rateLimitBackoffBase {
			t.Fatalf("jittered base = %s, want within [%s, %s)",
				got, rateLimitBackoffBase/2, rateLimitBackoffBase)
		}
	}
}

func TestForgetResetsAndKeysAreIndependent(t *testing.T) {
	b := newRateLimitBackoff()
	one := types.NamespacedName{Namespace: "default", Name: "one"}
	two := types.NamespacedName{Namespace: "default", Name: "two"}

	for i := 0; i < 4; i++ {
		b.next(one)
	}

	if got := b.next(two); got >= rateLimitBackoffBase {
		t.Errorf("a second key started at %s, want a base-length wait", got)
	}

	b.forget(one)
	if got := b.next(one); got >= rateLimitBackoffBase {
		t.Errorf("after forget the wait was %s, want a base-length wait", got)
	}
}
