package httpapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/money"
)

// currencyRe is the validation gate for currency codes that reach the rate
// provider URL (golden rule §3): exactly three ASCII letters.
var currencyRe = regexp.MustCompile(`^[A-Za-z]{3}$`)

// handleRate returns the exchange rate for from->to (optionally on a date) as
// JSON: {"rate":"0.78"}. Inputs are validated before touching the provider.
func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	from := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("from")))
	to := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("to")))
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if !currencyRe.MatchString(from) || !currencyRe.MatchString(to) {
		http.Error(w, "currencies must be 3-letter codes", http.StatusBadRequest)
		return
	}
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			http.Error(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	}
	rate, err := s.Svc.GetRate(ctx, from, to, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"rate": rate})
}

func (s *Server) handleConvertPage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	friend, err := s.Store.GetUser(ctx, fid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	bals, _ := s.Store.FriendBalance(ctx, me.ID, fid)

	// Preselect from the first nonzero balance: its currency + abs amount, and
	// the direction (owes you / you owe).
	fromCur := me.DefaultCurrency
	if fromCur == "" {
		fromCur = "USD"
	}
	fromAmt, direction := "", "owed"
	for _, b := range bals {
		if b.Amount != 0 {
			fromCur = b.Currency
			fromAmt = money.Format(absInt64(b.Amount), b.Currency)
			if b.Amount < 0 {
				direction = "owe"
			}
			break
		}
	}
	toCur := "USD"
	if fromCur == "USD" {
		toCur = "EUR"
	}
	s.render(w, r, "convert", "title.convert", map[string]any{
		"Friend": friend, "FriendID": fid, "Balances": bals,
		"FromCurrency": fromCur, "ToCurrency": toCur, "FromAmount": fromAmt,
		"Direction": direction, "Date": time.Now().Format("2006-01-02"),
	})
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	fromCur := strings.ToUpper(r.FormValue("from_currency"))
	toCur := strings.ToUpper(r.FormValue("to_currency"))
	fromAmt, err := money.Parse(r.FormValue("from_amount"), fromCur)
	if err != nil {
		s.renderErr(w, r, "message", "msg.conversion_failed", s.tr(r, "err.invalid_from_amount"), http.StatusBadRequest, err.Error())
		return
	}
	toAmt, err := money.Parse(r.FormValue("to_amount"), toCur)
	if err != nil {
		s.renderErr(w, r, "message", "msg.conversion_failed", s.tr(r, "err.invalid_to_amount"), http.StatusBadRequest, err.Error())
		return
	}
	// Move the balance between currencies: the from-leg must cancel the existing
	// balance, so its payer is the current creditor. "owed" = friend owes you, so
	// the friend is that creditor (sender=friend); "owe" = you owe the friend
	// (sender=me). The to-leg then re-creates the balance in the target currency.
	sender, receiver := fid, me.ID
	switch r.FormValue("direction") {
	case "", "owed":
		sender, receiver = fid, me.ID
	case "owe":
		sender, receiver = me.ID, fid
	default:
		s.renderErr(w, r, "message", "msg.conversion_failed", s.tr(r, "err.invalid_direction"), http.StatusBadRequest, "invalid direction")
		return
	}
	if _, err := s.Svc.CreateConversionExact(ctx, me.ID, sender, receiver, fromAmt, toAmt, fromCur, toCur, r.FormValue("date"), nil); err != nil {
		s.renderErr(w, r, "message", "msg.conversion_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	flash := fmt.Sprintf(s.tr(r, "flash.converted"),
		money.Format(fromAmt, fromCur), fromCur, money.Format(toAmt, toCur), toCur)
	s.redirectFlash(w, r, "/friends/"+chi.URLParam(r, "id"), flash)
}
