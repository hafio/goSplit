package httpapp

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/hafio/gosplit/internal/push"
)

// handlePushPublicKey returns the VAPID public key (empty when push is off).
func (s *Server) handlePushPublicKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":   s.Svc.Push.Enabled(),
		"publicKey": s.Svc.Push.PublicKey(),
	})
}

// handlePushSubscribe stores the browser's push subscription for the user.
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var sub struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(body, &sub); err != nil || sub.Endpoint == "" {
		http.Error(w, "invalid subscription", http.StatusBadRequest)
		return
	}
	me := s.currentUser(r)
	if err := s.Store.SavePushSubscription(ctx, me.ID, sub.Endpoint, string(body)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handlePushUnsubscribe removes a stored subscription.
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
	me := s.currentUser(r)
	_ = s.Store.DeletePushSubscription(ctx, me.ID, body.Endpoint)
	w.WriteHeader(http.StatusNoContent)
}

// handlePushTest sends a test notification to the user's subscriptions.
func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	if !s.Svc.Push.Enabled() {
		http.Error(w, "push is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	subs, err := s.Store.ListPushSubscriptions(ctx, []int64{me.ID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := push.Payload{Title: "GoSplit", Body: "Test notification 🎉", URL: "/balances"}
	for _, sub := range subs {
		if gone, _ := s.Svc.Push.Send(sub.Subscription, payload); gone {
			_ = s.Store.DeletePushSubscription(ctx, sub.UserID, sub.Endpoint)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
