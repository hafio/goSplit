package push

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/hafio/gosplit/internal/config"
)

func TestNewDisabled(t *testing.T) {
	s := New(&config.Config{})
	if s.Enabled() {
		t.Fatal("push should be disabled without VAPID keys")
	}
	if s.PublicKey() != "" {
		t.Fatalf("PublicKey = %q, want empty when disabled", s.PublicKey())
	}
}

func TestNewEnabledFromEmailFallback(t *testing.T) {
	s := New(&config.Config{WebPushPublicKey: "pub", WebPushPrivateKey: "priv", FromEmail: "from@x.test"})
	if !s.Enabled() {
		t.Fatal("push should be enabled with both VAPID keys")
	}
	if s.PublicKey() != "pub" {
		t.Fatalf("PublicKey = %q, want pub", s.PublicKey())
	}
	if s.subscriber != "from@x.test" {
		t.Fatalf("subscriber = %q, want from@x.test (FromEmail fallback; webpush-go adds mailto:)", s.subscriber)
	}
}

func TestNewEnabledExplicitEmail(t *testing.T) {
	s := New(&config.Config{
		WebPushPublicKey: "pub", WebPushPrivateKey: "priv",
		WebPushEmail: "push@x.test", FromEmail: "from@x.test",
	})
	if s.subscriber != "push@x.test" {
		t.Fatalf("subscriber = %q, want push@x.test (explicit WebPushEmail; webpush-go adds mailto:)", s.subscriber)
	}
}

func TestSendDisabledIsNoop(t *testing.T) {
	s := New(&config.Config{}) // disabled
	gone, err := s.Send(`{"endpoint":"https://example.test"}`, Payload{Title: "t"})
	if gone || err != nil {
		t.Fatalf("disabled Send = (%v, %v), want (false, nil)", gone, err)
	}
}

func TestSendInvalidSubscriptionJSON(t *testing.T) {
	s := New(&config.Config{WebPushPublicKey: "pub", WebPushPrivateKey: "priv", FromEmail: "f@x.test"})
	gone, err := s.Send("not-json", Payload{Title: "t"})
	if err == nil {
		t.Fatal("expected an error for malformed subscription JSON")
	}
	if gone {
		t.Fatal("gone should be false on an unmarshal error")
	}
}

func TestSendEnabledDeliveryError(t *testing.T) {
	s := New(&config.Config{WebPushPublicKey: "pub", WebPushPrivateKey: "priv", FromEmail: "f@x.test"})
	// Well-formed subscription JSON, but invalid VAPID/subscription keys and an
	// unreachable loopback endpoint: SendNotification fails before/at delivery,
	// so we get (gone=false, err!=nil) without touching the network meaningfully.
	sub := `{"endpoint":"http://127.0.0.1:1/push","keys":{"p256dh":"AAAA","auth":"AAAA"}}`
	gone, err := s.Send(sub, Payload{Title: "t", Body: "b", URL: "/x"})
	if err == nil {
		t.Fatal("expected a delivery error with invalid keys/endpoint")
	}
	if gone {
		t.Fatal("gone should be false when delivery errored (not a 404/410 response)")
	}
}

// testSubscription is browser-style subscription JSON for endpoint, with a real
// P-256 key so webpush-go can encrypt to it.
func testSubscription(t *testing.T, endpoint string) string {
	t.Helper()
	_, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	auth := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef"))
	return fmt.Sprintf(`{"endpoint":%q,"keys":{"p256dh":%q,"auth":%q}}`, endpoint, pub, auth)
}

// vapidSubject decodes the "sub" claim from a "vapid t=<jwt>, k=<key>" header.
func vapidSubject(t *testing.T, header string) string {
	t.Helper()
	tok, _, _ := strings.Cut(strings.TrimPrefix(header, "vapid t="), ",")
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("no JWT in Authorization %q", header)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("JWT payload: %v", err)
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("JWT claims: %v", err)
	}
	return claims.Sub
}

// TestSendDeliversAndReportsGone: against a push service, 201 is a delivery,
// and 404 or 410 means the browser dropped the subscription -- gone, so the
// caller prunes it. The VAPID token names the contact with a single mailto:,
// even when WEB_PUSH_EMAIL is configured with one.
func TestSendDeliversAndReportsGone(t *testing.T) {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	s := New(&config.Config{WebPushPublicKey: pub, WebPushPrivateKey: priv, WebPushEmail: "mailto:push@x.test"})
	for _, tc := range []struct {
		status int
		gone   bool
	}{{http.StatusCreated, false}, {http.StatusNotFound, true}, {http.StatusGone, true}} {
		auth := make(chan string, 1)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth <- r.Header.Get("Authorization")
			w.WriteHeader(tc.status)
		}))
		gone, err := s.Send(testSubscription(t, srv.URL+"/push"), Payload{Title: "t", Body: "b"})
		srv.Close()
		if err != nil || gone != tc.gone {
			t.Errorf("push service answering %d: Send = (%v, %v), want (%v, nil)", tc.status, gone, err, tc.gone)
		}
		if sub := vapidSubject(t, <-auth); sub != "mailto:push@x.test" {
			t.Errorf("VAPID subject = %q, want mailto:push@x.test", sub)
		}
	}
}
