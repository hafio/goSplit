// Package push sends Web Push notifications (VAPID) using webpush-go. It is a
// no-op when VAPID keys are not configured, so the feature is cleanly optional.
package push

import (
	"encoding/json"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/hafio/gosplit/internal/config"
)

// Sender delivers push payloads. Disabled senders silently drop.
type Sender struct {
	publicKey  string
	privateKey string
	subscriber string
	enabled    bool
}

// New builds a Sender from config. Push is enabled only when both VAPID keys
// are present.
func New(cfg *config.Config) *Sender {
	enabled := cfg.WebPushPublicKey != "" && cfg.WebPushPrivateKey != ""
	if !enabled {
		slog.Info("push: VAPID keys not set, web push disabled")
	}
	sub := cfg.WebPushEmail
	if sub == "" {
		sub = cfg.FromEmail
	}
	return &Sender{
		publicKey:  cfg.WebPushPublicKey,
		privateKey: cfg.WebPushPrivateKey,
		subscriber: "mailto:" + sub,
		enabled:    enabled,
	}
}

// Enabled reports whether push is configured.
func (s *Sender) Enabled() bool { return s.enabled }

// PublicKey returns the VAPID public key for the client subscribe flow.
func (s *Sender) PublicKey() string { return s.publicKey }

// Payload is the JSON delivered to the service worker.
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
}

// Send delivers payload to one subscription (raw browser JSON). It returns
// (gone=true) when the subscription is expired/invalid (HTTP 404/410) so the
// caller can prune it.
func (s *Sender) Send(subscriptionJSON string, p Payload) (gone bool, err error) {
	if !s.enabled {
		return false, nil
	}
	var sub webpush.Subscription
	if err := json.Unmarshal([]byte(subscriptionJSON), &sub); err != nil {
		return false, err
	}
	body, _ := json.Marshal(p)
	resp, err := webpush.SendNotification(body, &sub, &webpush.Options{
		Subscriber:      s.subscriber,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		TTL:             30,
	})
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return true, nil
	}
	return false, nil
}
