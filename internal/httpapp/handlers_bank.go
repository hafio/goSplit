package httpapp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/money"
)

type bankTxRow struct {
	ID       string
	Name     string
	Amount   string
	Currency string
	Date     string
}

func (s *Server) handleBankPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	txns := s.Svc.CachedTransactions(ctx, me.ID)
	rows := make([]bankTxRow, 0, len(txns))
	for _, t := range txns {
		rows = append(rows, bankTxRow{
			ID: t.ID, Name: t.Name, Amount: money.Format(t.AmountMinor, t.Currency),
			Currency: t.Currency, Date: t.Date,
		})
	}
	s.render(w, r, "bank", "title.bank", map[string]any{
		"Enabled":      s.Svc.BankEnabled(),
		"Transactions": rows,
	})
}

// handleBankLinkToken returns a Plaid Link token (JSON) for the client flow.
func (s *Server) handleBankLinkToken(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	token, err := s.Svc.CreateBankLinkToken(ctx, me)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"linkToken": token})
}

// handleBankExchange exchanges a Plaid public token for an access token.
func (s *Server) handleBankExchange(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	var body struct {
		PublicToken string `json:"public_token"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
	if err := s.Svc.ConnectBank(ctx, me, body.PublicToken); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBankSync fetches + caches recent transactions.
func (s *Server) handleBankSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	if _, err := s.Svc.SyncBankTransactions(ctx, me); err != nil {
		s.renderErr(w, r, "message", "msg.sync_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	s.redirectFlash(w, r, "/bank", "flash.tx_synced")
}

// handleBankConvert redirects a bank transaction into the prefilled add-expense
// form (spec: "convert a transaction into an expense").
func (s *Server) handleBankConvert(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	tx, ok := s.Svc.FindTransaction(ctx, me.ID, chi.URLParam(r, "txid"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := url.Values{}
	q.Set("name", tx.Name)
	q.Set("amount", money.Format(tx.AmountMinor, tx.Currency))
	q.Set("currency", tx.Currency)
	q.Set("date", tx.Date)
	http.Redirect(w, r, "/expenses/new?"+q.Encode(), http.StatusSeeOther)
}
