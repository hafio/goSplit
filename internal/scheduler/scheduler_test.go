package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
)

// newSchedService builds a real service over a throwaway SQLite database so the
// scheduler's maintenance passes run against genuine store methods.
func newSchedService(t *testing.T) *service.Service {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "sched.db")
	st, err := store.Open(context.Background(), &config.Config{
		DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := &config.Config{BaseURL: "http://test.local", CacheRetentionInterval: 24 * time.Hour}
	svc := service.New(st, &mail.LogMailer{}, cfg)
	svc.SetSynchronousEmail() // no lingering mail goroutines during tests
	return svc
}

func TestNew(t *testing.T) {
	svc := newSchedService(t)
	s := New(svc, "holder-1")
	if s.Svc != svc || s.Holder != "holder-1" || s.Interval != time.Minute {
		t.Fatalf("New = %+v, want Svc set, holder-1, 1m", s)
	}
}

func TestTickAsLeader(t *testing.T) {
	svc := newSchedService(t)
	s := New(svc, "leader")
	// Fresh lock table: this holder becomes leader and runs each cleanup pass
	// without error against an empty database.
	s.tick(context.Background())
	// The lock is now ours (same holder can renew it).
	leader, err := svc.Store.AcquireLock(context.Background(), "cleanup", "leader", time.Minute)
	if err != nil || !leader {
		t.Fatalf("expected to still hold the lock: leader=%v err=%v", leader, err)
	}
}

func TestTickNonLeader(t *testing.T) {
	svc := newSchedService(t)
	// Another instance holds the lock with a long TTL, so we are not the leader.
	if ok, err := svc.Store.AcquireLock(context.Background(), "cleanup", "other", time.Hour); err != nil || !ok {
		t.Fatalf("pre-acquire as other: ok=%v err=%v", ok, err)
	}
	s := New(svc, "me")
	s.tick(context.Background()) // not leader -> early return, no side effects, no panic
}

func TestRunStopsOnCancel(t *testing.T) {
	svc := newSchedService(t)
	s := New(svc, "runner")
	s.Interval = 5 * time.Millisecond // exercise the ticker branch quickly
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	time.Sleep(25 * time.Millisecond) // let a few ticks fire
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
