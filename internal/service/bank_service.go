package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/hafio/gosplit/internal/bank"
	"github.com/hafio/gosplit/internal/store"
)

// bankData is what we cache per user in cached_bank_data.
type bankData struct {
	AccessToken  string             `json:"accessToken"`
	Transactions []bank.Transaction `json:"transactions"`
	SyncedAt     string             `json:"syncedAt"`
}

// BankEnabled reports whether bank sync is configured.
func (s *Service) BankEnabled() bool { return s.Bank != nil && s.Bank.Enabled() }

// CreateBankLinkToken returns a Plaid Link token for the client flow.
func (s *Service) CreateBankLinkToken(ctx context.Context, user *store.User) (string, error) {
	return s.Bank.CreateLinkToken(ctx, itoa(user.ID))
}

// ConnectBank exchanges a public token and stores the access token.
func (s *Service) ConnectBank(ctx context.Context, user *store.User, publicToken string) error {
	token, err := s.Bank.ExchangePublicToken(ctx, publicToken)
	if err != nil {
		return err
	}
	data := s.loadBankData(ctx, user.ID)
	data.AccessToken = token
	return s.saveBankData(ctx, user.ID, data)
}

// SyncBankTransactions fetches recent transactions and caches them.
func (s *Service) SyncBankTransactions(ctx context.Context, user *store.User) ([]bank.Transaction, error) {
	data := s.loadBankData(ctx, user.ID)
	if data.AccessToken == "" {
		return nil, errors.New("no bank account connected yet")
	}
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -s.Config.PlaidIntervalInDays)
	txns, err := s.Bank.FetchTransactions(ctx, data.AccessToken, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	data.Transactions = txns
	data.SyncedAt = toISO(end)
	if err := s.saveBankData(ctx, user.ID, data); err != nil {
		return nil, err
	}
	return txns, nil
}

// CachedTransactions returns the last-synced transactions (may be empty).
func (s *Service) CachedTransactions(ctx context.Context, userID int64) []bank.Transaction {
	return s.loadBankData(ctx, userID).Transactions
}

// FindTransaction returns a cached transaction by id.
func (s *Service) FindTransaction(ctx context.Context, userID int64, txID string) (bank.Transaction, bool) {
	for _, t := range s.loadBankData(ctx, userID).Transactions {
		if t.ID == txID {
			return t, true
		}
	}
	return bank.Transaction{}, false
}

func (s *Service) loadBankData(ctx context.Context, userID int64) bankData {
	var d bankData
	if raw, err := s.Store.GetBankData(ctx, userID); err == nil {
		_ = json.Unmarshal([]byte(raw), &d)
	}
	return d
}

func (s *Service) saveBankData(ctx context.Context, userID int64, d bankData) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.Store.PutBankData(ctx, userID, string(b))
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
