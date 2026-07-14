package httpapp

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/mail"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
	"github.com/hafio/gosplit/internal/web"
)

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	st     *store.Store
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "http.db")
	cfg := &config.Config{
		BaseURL: "http://test.local", DatabaseURL: "file:" + dbPath, Engine: config.EngineSQLite,
		EnableSendingInvites: true, DefaultHomepage: "/balances",
		AdminEmails: []string{"admin@example.com"},
	}
	st, err := store.Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := service.New(st, &mail.LogMailer{}, cfg)
	svc.SetSynchronousEmail() // no lingering mail goroutines during tests
	am := auth.NewManager(st, false)
	renderer, err := web.NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	app := New(cfg, st, svc, am, renderer)
	srv := httptest.NewServer(app.Router())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	return &harness{t: t, srv: srv, client: &http.Client{Jar: jar}, st: st}
}

func (h *harness) csrf() string {
	u, _ := url.Parse(h.srv.URL)
	for _, c := range h.client.Jar.Cookies(u) {
		if c.Name == auth.CSRFCookie {
			return c.Value
		}
	}
	return ""
}

func (h *harness) get(path string) *http.Response {
	h.t.Helper()
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (h *harness) post(path string, form url.Values) *http.Response {
	h.t.Helper()
	form.Set("csrf_token", h.csrf())
	resp, err := h.client.PostForm(h.srv.URL+path, form)
	if err != nil {
		h.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func (h *harness) register(name, email, password string) {
	h.t.Helper()
	_ = body(h.t, h.get("/login")) // seed CSRF cookie
	resp := h.post("/register", url.Values{"name": {name}, "email": {email}, "password": {password}})
	if resp.StatusCode != http.StatusOK { // followed redirect -> 200 homepage
		h.t.Fatalf("register status %d", resp.StatusCode)
	}
	_ = body(h.t, resp)
}

func TestHealth(t *testing.T) {
	h := newHarness(t)
	resp := h.get("/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status %d", resp.StatusCode)
	}
	if !strings.Contains(body(t, resp), "ok") {
		t.Fatal("health body missing ok")
	}
}

func TestStaticCacheHeaders(t *testing.T) {
	h := newHarness(t)

	resp := h.get("/static/app.css?v=x")
	_ = body(t, resp)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("fingerprinted asset Cache-Control = %q, want immutable", cc)
	}

	resp = h.get("/static/app.css")
	_ = body(t, resp)
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age=3600") {
		t.Fatalf("unversioned asset Cache-Control = %q, want max-age=3600", cc)
	}

	resp = h.get("/sw.js")
	_ = body(t, resp)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("sw.js Cache-Control = %q, want no-cache", cc)
	}
}

func TestLoginRequiredRedirect(t *testing.T) {
	h := newHarness(t)
	// Do not follow redirects so we can see the 303.
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.get("/balances")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("want redirect to login, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/login") {
		t.Fatalf("redirect location = %q", loc)
	}
}

func TestCSRFRejected(t *testing.T) {
	h := newHarness(t)
	_ = body(t, h.get("/login")) // get a csrf cookie
	// POST without csrf_token / header should be forbidden.
	resp, err := h.client.PostForm(h.srv.URL+"/register", url.Values{"email": {"x@example.com"}, "password": {"password12"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 without CSRF, got %d", resp.StatusCode)
	}
}

func TestFullFlow(t *testing.T) {
	h := newHarness(t)
	h.register("Alice", "alice@example.com", "password123")

	// Balances page loads.
	if resp := h.get("/balances"); resp.StatusCode != http.StatusOK {
		t.Fatalf("balances status %d", resp.StatusCode)
	} else {
		_ = body(t, resp)
	}

	// Add a friend (creates a pending user + friendship).
	h.post("/friends/add", url.Values{"email": {"bob@example.com"}})
	friendsBody := body(t, h.get("/friends"))
	if !strings.Contains(friendsBody, "bob@example.com") {
		t.Fatalf("friends page missing bob: %s", friendsBody)
	}

	// Look up bob's id from the store via a fresh expense: create a group first.
	h.post("/groups/create", url.Values{"name": {"Trip"}, "currency": {"USD"}})
	groupsBody := body(t, h.get("/groups"))
	if !strings.Contains(groupsBody, "Trip") {
		t.Fatalf("groups page missing Trip: %s", groupsBody)
	}

	// Add a direct expense between alice(1) and bob(2), equal split.
	h.post("/expenses", url.Values{
		"name": {"Dinner"}, "amount": {"40.00"}, "currency": {"USD"},
		"date": {"2025-01-02"}, "method": {"EQUAL"}, "paid_by": {"1"},
		"include_1": {"1"}, "value_1": {""},
		"include_2": {"1"}, "value_2": {""},
	})

	// Activity should show the expense; balances should be non-empty.
	if !strings.Contains(body(t, h.get("/activity")), "Dinner") {
		t.Fatal("activity missing Dinner")
	}
	balBody := body(t, h.get("/balances"))
	if !strings.Contains(balBody, "bob@example.com") && !strings.Contains(balBody, "owes") && !strings.Contains(balBody, "owe") {
		t.Fatalf("balances page unexpected: %s", balBody)
	}

	// Filtered friend history (description filter) still renders.
	if resp := h.get("/friends/2?q=Dinner"); resp.StatusCode != http.StatusOK {
		t.Fatalf("friend filter status %d", resp.StatusCode)
	} else if !strings.Contains(body(t, resp), "Dinner") {
		t.Fatal("filtered friend history missing Dinner")
	}

	// Logout.
	h.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp := h.post("/logout", url.Values{})
	_ = body(t, resp)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout status %d", resp.StatusCode)
	}
}
