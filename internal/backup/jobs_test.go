package backup

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestJobTrackerLifecycle covers a job from start to completion.
func TestJobTrackerLifecycle(t *testing.T) {
	tr := NewJobTracker()
	release := make(chan struct{})
	done := make(chan struct{})

	token, err := tr.Start(7, func(context.Context) (JobStatus, error) {
		<-release
		close(done)
		return JobStatus{State: JobDone, ResultPath: "/tmp/x.gsbak"}, nil
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	st, ok := tr.Status(token)
	if !ok || st.State != JobRunning {
		t.Fatalf("state = %q (found %v), want running", st.State, ok)
	}
	if st.ActingUserID != 7 {
		t.Errorf("acting user = %d, want 7", st.ActingUserID)
	}

	close(release)
	<-done
	waitFor(t, func() bool {
		s, _ := tr.Status(token)
		return s.State == JobDone
	}, "job did not reach done")

	st, _ = tr.Status(token)
	if st.ResultPath != "/tmp/x.gsbak" {
		t.Errorf("result path = %q", st.ResultPath)
	}
	// The acting user survives the transition, since the restore poll needs it
	// to re-establish the session.
	if st.ActingUserID != 7 {
		t.Errorf("acting user lost across completion: %d", st.ActingUserID)
	}
	if st.FinishedAt.IsZero() {
		t.Error("FinishedAt was not stamped")
	}
}

// TestJobTrackerRecordsFailure asserts a failing job lands in the error state
// with a sanitized message.
func TestJobTrackerRecordsFailure(t *testing.T) {
	tr := NewJobTracker()
	token, err := tr.Start(1, func(context.Context) (JobStatus, error) {
		return JobStatus{}, errors.New("host=db user=u password=hunter2 dbname=app: boom")
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, func() bool {
		s, _ := tr.Status(token)
		return s.State == JobError
	}, "job did not reach error")

	st, _ := tr.Status(token)
	if strings.Contains(st.Err, "hunter2") {
		t.Errorf("job error leaked a DSN password: %q", st.Err)
	}
	if !strings.Contains(st.Err, "boom") {
		t.Errorf("job error lost the useful part: %q", st.Err)
	}
}

// TestJobTrackerUnknownToken asserts an unknown token is not found, rather
// than yielding a zero-value job that would read as running.
func TestJobTrackerUnknownToken(t *testing.T) {
	tr := NewJobTracker()
	if _, ok := tr.Status("nope"); ok {
		t.Error("an unknown token resolved")
	}
	if _, _, _, ok := tr.TakePending("nope"); ok {
		t.Error("an unknown pending token resolved")
	}
	if _, ok := tr.PeekPending("nope"); ok {
		t.Error("an unknown pending token peeked")
	}
}

// TestTokensAreUnguessable asserts tokens are long and distinct. The restore
// status route is gated by the token alone, so predictability would be an
// authorization hole.
func TestTokensAreUnguessable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, err := newToken()
		if err != nil {
			t.Fatalf("newToken: %v", err)
		}
		if len(tok) < 40 {
			t.Fatalf("token is only %d chars: %q", len(tok), tok)
		}
		if seen[tok] {
			t.Fatalf("token collision after %d draws", i)
		}
		seen[tok] = true
	}
}

// TestTakePendingIsSingleUse asserts a token cannot be replayed to apply the
// same archive twice.
func TestTakePendingIsSingleUse(t *testing.T) {
	tr := NewJobTracker()
	v := &Validated{Manifest: Manifest{CreatedAt: "2026-09-04T08:00:00.000Z"}}

	token, err := tr.StagePending(v, "", 3)
	if err != nil {
		t.Fatalf("StagePending: %v", err)
	}
	if man, ok := tr.PeekPending(token); !ok || man.CreatedAt == "" {
		t.Fatal("PeekPending did not return the staged manifest")
	}
	// Peek must not consume.
	if _, ok := tr.PeekPending(token); !ok {
		t.Fatal("PeekPending consumed the pending restore")
	}

	got, _, actingID, ok := tr.TakePending(token)
	if !ok || got != v {
		t.Fatal("TakePending did not return the staged archive")
	}
	if actingID != 3 {
		t.Errorf("acting user = %d, want 3", actingID)
	}
	if _, _, _, ok := tr.TakePending(token); ok {
		t.Error("a pending token was usable twice; a restore could be replayed")
	}
}

// TestPruneDropsExpiredJobsAndPending asserts the TTL sweep runs, so an
// unconfirmed upload does not sit on the data volume forever.
func TestPruneDropsExpiredJobsAndPending(t *testing.T) {
	tr := NewJobTracker()

	token, err := tr.Start(1, func(context.Context) (JobStatus, error) {
		return JobStatus{State: JobDone}, nil
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, func() bool {
		s, _ := tr.Status(token)
		return s.State == JobDone
	}, "job did not finish")

	pendingToken, err := tr.StagePending(&Validated{}, "", 1)
	if err != nil {
		t.Fatalf("StagePending: %v", err)
	}

	// Nothing expires yet.
	tr.Prune(time.Now())
	if _, ok := tr.Status(token); !ok {
		t.Error("a fresh finished job was pruned")
	}
	if _, ok := tr.PeekPending(pendingToken); !ok {
		t.Error("a fresh pending restore was pruned")
	}

	// Well past the TTL, both go.
	tr.Prune(time.Now().Add(2 * jobTTL))
	if _, ok := tr.Status(token); ok {
		t.Error("an expired job survived the sweep")
	}
	if _, ok := tr.PeekPending(pendingToken); ok {
		t.Error("an expired pending restore survived the sweep")
	}
}

// TestPruneKeepsRunningJobs asserts a long-running job is never swept out from
// under its own poll.
func TestPruneKeepsRunningJobs(t *testing.T) {
	tr := NewJobTracker()
	release := make(chan struct{})
	defer close(release)

	token, err := tr.Start(1, func(context.Context) (JobStatus, error) {
		<-release
		return JobStatus{State: JobDone}, nil
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	tr.Prune(time.Now().Add(100 * jobTTL))
	if _, ok := tr.Status(token); !ok {
		t.Error("a still-running job was pruned")
	}
}

// TestDiscardPendingReleases asserts an abandoned upload is dropped on demand.
func TestDiscardPendingReleases(t *testing.T) {
	tr := NewJobTracker()
	token, err := tr.StagePending(&Validated{}, "", 1)
	if err != nil {
		t.Fatalf("StagePending: %v", err)
	}
	tr.DiscardPending(token)
	if _, ok := tr.PeekPending(token); ok {
		t.Error("a discarded pending restore is still present")
	}
	// Discarding twice must not panic.
	tr.DiscardPending(token)
}

// TestSanitizeErr covers the secret-stripping cases that matter: a Postgres
// DSN carries its password, and a constraint violation can echo the offending
// row -- which in this schema could be a session token or a password hash.
func TestSanitizeErr(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		mustNotHave []string
		mustHave    []string
	}{
		{
			name:        "libpq keyword dsn",
			err:         errors.New(`store: open: host=db user=app password=s3cr3t dbname=gosplit sslmode=disable`),
			mustNotHave: []string{"s3cr3t"},
			mustHave:    []string{"host=db", "redacted"},
		},
		{
			name:        "quoted keyword password",
			err:         errors.New(`host=db password='p@ss w0rd' dbname=x`),
			mustNotHave: []string{"p@ss w0rd"},
			mustHave:    []string{"redacted"},
		},
		{
			name:        "url dsn",
			err:         errors.New(`failed to connect to postgres://app:topsecret@db:5432/gosplit`),
			mustNotHave: []string{"topsecret"},
			mustHave:    []string{"redacted"},
		},
		{
			name:        "constraint detail",
			err:         errors.New("ERROR: duplicate key value violates unique constraint \"sessions_pkey\"\nDETAIL: Key (token)=(abc123secret) already exists."),
			mustNotHave: []string{"abc123secret"},
			mustHave:    []string{"sessions_pkey", "redacted"},
		},
		{
			name:     "ordinary error passes through",
			err:      errors.New("backup: archive is missing table \"users\""),
			mustHave: []string{"missing table"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeErr(tc.err)
			for _, bad := range tc.mustNotHave {
				if strings.Contains(got, bad) {
					t.Errorf("sanitized error still contains %q:\n %s", bad, got)
				}
			}
			for _, want := range tc.mustHave {
				if !strings.Contains(got, want) {
					t.Errorf("sanitized error lost %q:\n %s", want, got)
				}
			}
		})
	}
	if sanitizeErr(nil) != "" {
		t.Error("sanitizeErr(nil) should be empty")
	}
}

// TestSanitizeErrTruncates bounds a hostile message before it reaches a log.
func TestSanitizeErrTruncates(t *testing.T) {
	got := sanitizeErr(errors.New(strings.Repeat("x", 5000)))
	if len(got) > 900 {
		t.Errorf("sanitized error is %d bytes, want it bounded", len(got))
	}
	if !strings.HasSuffix(got, "(truncated)") {
		t.Error("a truncated error should say so")
	}
}

// waitFor polls cond until it holds, failing the test on timeout. Background
// jobs finish asynchronously, so a poll is the honest way to observe them.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal(msg)
}
