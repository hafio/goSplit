package backup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// A backup or a restore takes far longer than an HTTP request may. The server
// sets a 60s WriteTimeout and handlers a 10s context timeout, so the admin
// panel starts the work as a background job and polls for its state.
//
// The tracker also holds validated-but-unconfirmed uploads, so the panel can
// show a manifest preview and then apply exactly the archive that was
// validated -- not a re-read of the file.

// JobState is a job's lifecycle state.
type JobState string

const (
	JobRunning JobState = "running"
	JobDone    JobState = "done"
	JobError   JobState = "error"
)

// jobTTL is how long a finished job stays readable. It bounds both memory and
// the window in which a job token is useful.
const jobTTL = 15 * time.Minute

// JobStatus is a job's observable state. Err is sanitized -- see sanitizeErr.
type JobStatus struct {
	State JobState
	Err   string

	// ResultPath is set by a completed backup job.
	ResultPath string
	// Manifest is set by a completed backup or restore job.
	Manifest *Manifest
	// ActingUserID is the admin who started a restore, so the poll that
	// observes completion can re-establish their session if that user still
	// exists in the restored data.
	ActingUserID int64

	CreatedAt  time.Time
	FinishedAt time.Time
}

// pending is an archive that passed validation and is awaiting confirmation.
type pending struct {
	validated *Validated
	path      string
	actingID  int64
	createdAt time.Time
}

// JobTracker holds background jobs and pending restores for one server. It is
// per-Server rather than a package global, so tests and multiple instances do
// not share state.
type JobTracker struct {
	mu      sync.Mutex
	jobs    map[string]*JobStatus
	pending map[string]*pending
}

// NewJobTracker builds an empty tracker.
func NewJobTracker() *JobTracker {
	return &JobTracker{
		jobs:    map[string]*JobStatus{},
		pending: map[string]*pending{},
	}
}

// Start runs fn in the background and returns its token.
//
// The job is rooted in context.Background(), never the request's context:
// that one is cancelled the moment the kickoff response returns, which would
// abort the work immediately.
func (t *JobTracker) Start(actingUserID int64, fn func(context.Context) (JobStatus, error)) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	t.mu.Lock()
	t.jobs[token] = &JobStatus{State: JobRunning, ActingUserID: actingUserID, CreatedAt: now}
	t.mu.Unlock()

	go func() {
		ctx := context.Background()
		st, err := fn(ctx)
		t.mu.Lock()
		defer t.mu.Unlock()
		cur := t.jobs[token]
		if cur == nil {
			return
		}
		st.CreatedAt = cur.CreatedAt
		st.ActingUserID = cur.ActingUserID
		st.FinishedAt = time.Now()
		if err != nil {
			st.State = JobError
			st.Err = sanitizeErr(err)
			slog.Error("backup: job failed", "err", st.Err)
		} else if st.State == "" {
			st.State = JobDone
		}
		t.jobs[token] = &st
	}()
	return token, nil
}

// Status returns a job's state.
func (t *JobTracker) Status(token string) (JobStatus, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	j, ok := t.jobs[token]
	if !ok {
		return JobStatus{}, false
	}
	return *j, true
}

// StagePending records a validated archive awaiting confirmation and returns
// its token.
func (t *JobTracker) StagePending(v *Validated, path string, actingUserID int64) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending[token] = &pending{validated: v, path: path, actingID: actingUserID, createdAt: time.Now()}
	return token, nil
}

// TakePending removes and returns a pending restore. Removing it on read means
// a token cannot be replayed to apply the same archive twice.
func (t *JobTracker) TakePending(token string) (*Validated, string, int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.pending[token]
	if !ok {
		return nil, "", 0, false
	}
	delete(t.pending, token)
	return p.validated, p.path, p.actingID, true
}

// PeekPending reports a pending restore's manifest without consuming it, for
// rendering the confirmation page.
func (t *JobTracker) PeekPending(token string) (Manifest, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.pending[token]
	if !ok {
		return Manifest{}, false
	}
	return p.validated.Manifest, true
}

// Prune drops finished jobs and abandoned pending restores past their TTL,
// releasing each pending archive's staging directory and uploaded file. Without
// this an upload that is never confirmed would sit on the data volume forever.
func (t *JobTracker) Prune(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for token, j := range t.jobs {
		if j.State != JobRunning && now.Sub(j.FinishedAt) > jobTTL {
			delete(t.jobs, token)
		}
	}
	for token, p := range t.pending {
		if now.Sub(p.createdAt) <= jobTTL {
			continue
		}
		p.validated.Discard()
		removeQuietly(p.path)
		delete(t.pending, token)
		slog.Info("backup: discarded an unconfirmed restore upload past its TTL")
	}
}

// DiscardPending releases a pending restore without applying it.
func (t *JobTracker) DiscardPending(token string) {
	t.mu.Lock()
	p, ok := t.pending[token]
	if ok {
		delete(t.pending, token)
	}
	t.mu.Unlock()
	if ok {
		p.validated.Discard()
		removeQuietly(p.path)
	}
}

// newToken mints an unguessable job token. The restore status route is gated
// by this token rather than a session, because a restore replaces the users
// table and so destroys the caller's own session row mid-flight.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// dsnCredential matches the secret-bearing parts of a connection string, in
// both the URL and libpq keyword forms.
var dsnCredential = regexp.MustCompile(`(?i)(password\s*=\s*)('[^']*'|"[^"]*"|\S+)|(://[^:/@\s]+:)[^@\s]+(@)`)

// sanitizeErr strips secrets from an error before it is surfaced to an admin
// or written to a log.
//
// Two things leak here in practice: a Postgres DSN carries its password, and a
// constraint-violation message can echo the offending row's contents -- which
// for this schema could be a session token or a password hash.
func sanitizeErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	msg = dsnCredential.ReplaceAllString(msg, "${1}[redacted]${3}[redacted]${4}")

	// A driver's DETAIL line quotes the values that violated a constraint.
	if i := strings.Index(msg, "DETAIL:"); i >= 0 {
		msg = msg[:i] + "DETAIL: [redacted]"
	}
	const max = 800
	if len(msg) > max {
		msg = msg[:max] + "... (truncated)"
	}
	return msg
}

// removeQuietly deletes a path, ignoring a missing file.
func removeQuietly(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("backup: could not remove temporary file", "path", path, "err", err)
	}
}
