package push

import (
	"testing"

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
	if s.subscriber != "mailto:from@x.test" {
		t.Fatalf("subscriber = %q, want mailto:from@x.test (FromEmail fallback)", s.subscriber)
	}
}

func TestNewEnabledExplicitEmail(t *testing.T) {
	s := New(&config.Config{
		WebPushPublicKey: "pub", WebPushPrivateKey: "priv",
		WebPushEmail: "push@x.test", FromEmail: "from@x.test",
	})
	if s.subscriber != "mailto:push@x.test" {
		t.Fatalf("subscriber = %q, want mailto:push@x.test (explicit WebPushEmail)", s.subscriber)
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
