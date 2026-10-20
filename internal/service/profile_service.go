package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hafio/gosplit/internal/store"
)

var allowedImageExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// UpdateAvatar saves an uploaded avatar (if present) to the upload dir and sets
// the user's Image path. Files are size-limited by UPLOAD_MAX_FILE_SIZE_MB.
// (Crop/compression is deferred; the raw image is stored behind the same path.)
func (s *Service) UpdateAvatar(ctx context.Context, u *store.User, r *http.Request) error {
	_ = ctx
	maxBytes := int64(s.Config.UploadMaxFileSizeMB) << 20
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return nil // no multipart body / nothing to do
	}
	file, hdr, err := r.FormFile("avatar")
	if err != nil {
		return nil // no file uploaded
	}
	defer func() { _ = file.Close() }()
	if hdr.Size > maxBytes {
		return fmt.Errorf("avatar exceeds %d MB", s.Config.UploadMaxFileSizeMB)
	}
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if !allowedImageExt[ext] {
		return fmt.Errorf("unsupported image type %q", ext)
	}
	if err := os.MkdirAll(s.Config.UploadDir, 0o755); err != nil { // cov:ignore env: needs a read-only or full UPLOAD_DIR volume
		return err
	}
	name := store.NewUUID() + ext
	dst := filepath.Join(s.Config.UploadDir, name)
	out, err := os.Create(dst)
	if err != nil { // cov:ignore env: needs a read-only or full UPLOAD_DIR volume
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, io.LimitReader(file, maxBytes)); err != nil { // cov:ignore env: needs a read-only or full UPLOAD_DIR volume
		return err
	}
	u.Image = sql.NullString{String: "/uploads/" + name, Valid: true}
	return nil
}

// userExport is the caller's own account as exported. Fields are listed
// explicitly: store.User has no json tags and carries credentials and internal
// state (PasswordHash, BankingID, HiddenFriendIDs) that must never leave the
// server, so marshalling it whole leaked them.
type userExport struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	Image              *string `json:"image"`
	Currency           string  `json:"currency"`
	DefaultCurrency    string  `json:"default_currency"`
	PreferredLanguage  string  `json:"preferred_language"`
	ThemeColor         string  `json:"theme_color"`
	Role               string  `json:"role"`
	CreatedAt          string  `json:"created_at"`
	EmailExpenseNotify bool    `json:"email_expense_notify"`
}

// friendExport is another account as it appears in the caller's export: enough
// to tell who a balance is with, nothing about their account.
type friendExport struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// groupExport leaves out PublicID, the group's join capability (anyone holding
// it can join), and the creator and Splitwise ids, which only mean something
// inside this server.
type groupExport struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	DefaultCurrency string  `json:"default_currency"`
	SimplifyDebts   bool    `json:"simplify_debts"`
	ArchivedAt      *string `json:"archived_at"`
	CreatedAt       string  `json:"created_at"`
}

// nullableString maps a NULL column to JSON null rather than to sql.NullString's
// {"String":...,"Valid":...} shape.
func nullableString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// ExportUserData returns a JSON blob of the user's account, friends, groups and
// expenses (account data export/download).
func (s *Service) ExportUserData(ctx context.Context, userID int64) ([]byte, error) {
	u, err := s.Store.GetUser(ctx, userID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	// Every read is checked: a takeout that silently lacked a section would
	// look complete to the person relying on it.
	friends, err := s.Store.ListFriends(ctx, userID)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	groups, err := s.Store.ListGroupsForUser(ctx, userID, true)
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}
	// Archived groups' expenses too: the export is everything that is yours,
	// not only what the Activity page shows by default.
	expenses, err := s.Store.ListActivity(ctx, userID, store.ExpenseFilter{IncludeArchived: true})
	if err != nil { // cov:ignore store-fault: needs a failing database
		return nil, err
	}

	type expenseExport struct {
		Expense      *store.Expense              `json:"expense"`
		Participants []store.ExpenseParticipant  `json:"participants"`
	}
	exps := make([]expenseExport, 0, len(expenses))
	for _, e := range expenses {
		parts, err := s.Store.GetParticipants(ctx, e.ID)
		if err != nil { // cov:ignore store-fault: needs a failing database
			return nil, err
		}
		exps = append(exps, expenseExport{Expense: e, Participants: parts})
	}

	friendsOut := make([]friendExport, 0, len(friends))
	for _, f := range friends {
		friendsOut = append(friendsOut, friendExport{ID: f.ID, Name: f.Name, Email: f.Email})
	}
	groupsOut := make([]groupExport, 0, len(groups))
	for _, g := range groups {
		groupsOut = append(groupsOut, groupExport{
			ID: g.ID, Name: g.Name, DefaultCurrency: g.DefaultCurrency,
			SimplifyDebts: g.SimplifyDebts, ArchivedAt: nullableString(g.ArchivedAt), CreatedAt: g.CreatedAt,
		})
	}

	payload := map[string]any{
		"user": userExport{
			ID: u.ID, Name: u.Name, Email: u.Email, Image: nullableString(u.Image),
			Currency: u.Currency, DefaultCurrency: u.DefaultCurrency,
			PreferredLanguage: u.PreferredLanguage, ThemeColor: u.ThemeColor, Role: u.Role,
			CreatedAt: u.CreatedAt, EmailExpenseNotify: u.EmailExpenseNotify,
		},
		"friends":  friendsOut,
		"groups":   groupsOut,
		"expenses": exps,
	}
	return json.MarshalIndent(payload, "", "  ")
}
