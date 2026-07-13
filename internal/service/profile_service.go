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
	if err := os.MkdirAll(s.Config.UploadDir, 0o755); err != nil {
		return err
	}
	name := store.NewUUID() + ext
	dst := filepath.Join(s.Config.UploadDir, name)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, io.LimitReader(file, maxBytes)); err != nil {
		return err
	}
	u.Image = sql.NullString{String: "/uploads/" + name, Valid: true}
	return nil
}

// ExportUserData returns a JSON blob of the user's account, friends, groups and
// expenses (account data export/download).
func (s *Service) ExportUserData(ctx context.Context, userID int64) ([]byte, error) {
	u, err := s.Store.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	friends, _ := s.Store.ListFriends(ctx, userID)
	groups, _ := s.Store.ListGroupsForUser(ctx, userID, true)
	expenses, _ := s.Store.ListActivity(ctx, userID, store.ExpenseFilter{})

	type expenseExport struct {
		Expense      *store.Expense              `json:"expense"`
		Participants []store.ExpenseParticipant  `json:"participants"`
	}
	exps := make([]expenseExport, 0, len(expenses))
	for _, e := range expenses {
		parts, _ := s.Store.GetParticipants(ctx, e.ID)
		exps = append(exps, expenseExport{Expense: e, Participants: parts})
	}

	payload := map[string]any{
		"user":     u,
		"friends":  friends,
		"groups":   groups,
		"expenses": exps,
	}
	return json.MarshalIndent(payload, "", "  ")
}
