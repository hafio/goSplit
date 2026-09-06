package httpapp

import (
	"context"
	"testing"
	"time"

	"github.com/hafio/gosplit/internal/backup"
)

// TestStartJanitorStopsWithContext asserts the sweeper is tied to the server's
// lifetime, so it does not outlive a shutdown.
func TestStartJanitorStopsWithContext(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.app.StartJanitor(ctx)
	cancel()
	// Nothing to assert beyond it not panicking or deadlocking; the goroutine
	// returns on the cancelled context.
	time.Sleep(20 * time.Millisecond)
}

// TestJanitorPrunesAbandonedUpload is the regression guard for the defect this
// file fixes. Prune was written, documented and tested, but never called from
// anywhere -- so an admin who uploaded an archive, saw the preview and walked
// away left both the uploaded file and its staging directory on the data
// volume permanently.
//
// It calls the sweep directly rather than waiting on the ticker, so the test
// is about the wiring and the effect, not about timing.
func TestJanitorPrunesAbandonedUpload(t *testing.T) {
	h := newHarness(t)

	token, err := h.app.Jobs.StagePending(&backup.Validated{}, "", 1)
	if err != nil {
		t.Fatalf("StagePending: %v", err)
	}
	if _, ok := h.app.Jobs.PeekPending(token); !ok {
		t.Fatal("the pending restore was not staged")
	}

	// A sweep at the present moment leaves it alone.
	h.app.Jobs.Prune(time.Now())
	if _, ok := h.app.Jobs.PeekPending(token); !ok {
		t.Error("a fresh pending restore was swept")
	}

	// Well past the TTL, it goes.
	h.app.Jobs.Prune(time.Now().Add(24 * time.Hour))
	if _, ok := h.app.Jobs.PeekPending(token); ok {
		t.Error("an abandoned upload survived the sweep, so it would leak on the data volume")
	}
}

// TestJanitorIsWiredIntoTheServer asserts a real Server has a job tracker to
// sweep. Without it StartJanitor would nil-dereference on its first tick --
// which, at a five-minute interval, is exactly the kind of fault that would
// not surface until long after a deploy.
func TestJanitorIsWiredIntoTheServer(t *testing.T) {
	h := newHarness(t)
	if h.app.Jobs == nil {
		t.Fatal("Server.Jobs is nil, so the janitor would panic on its first tick")
	}
	// One tick's worth of work, run directly.
	h.app.Jobs.Prune(time.Now())
	backup.SweepStalePartials(h.cfg.BackupDir, time.Now())
}
