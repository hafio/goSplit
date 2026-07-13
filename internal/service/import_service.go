package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/store"
)

// ImportResult summarizes what a Splitwise import created.
type ImportResult struct {
	Friends int
	Groups  int
	Skipped int // entries skipped (e.g. members without an email)
}

// splitwiseBase is the Splitwise REST API v3.0 root (overridable in tests).
var splitwiseBase = "https://secure.splitwise.com/api/v3.0"

// ImportFromSplitwise imports the caller's Splitwise friends and groups
// (partial: expenses are NOT imported — mirrors the reference app). The user
// supplies their own Splitwise API key, so no server-side credentials are
// needed. Idempotent per Splitwise group id.
func (s *Service) ImportFromSplitwise(ctx context.Context, actor *store.User, apiKey string) (*ImportResult, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("a Splitwise API key is required")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	res := &ImportResult{}

	// Friends.
	var friendsResp struct {
		Friends []struct {
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"friends"`
	}
	if err := swGet(ctx, client, apiKey, "/get_friends", &friendsResp); err != nil {
		return nil, err
	}
	for _, f := range friendsResp.Friends {
		if f.Email == "" {
			res.Skipped++
			continue
		}
		u, err := s.importUser(ctx, f.Email, strings.TrimSpace(f.FirstName+" "+f.LastName))
		if err != nil {
			return nil, err
		}
		if err := s.Store.AddFriend(ctx, actor.ID, u.ID); err != nil {
			return nil, err
		}
		res.Friends++
	}

	// Groups (+ members).
	var groupsResp struct {
		Groups []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Members []struct {
				Email     string `json:"email"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"members"`
		} `json:"groups"`
	}
	if err := swGet(ctx, client, apiKey, "/get_groups", &groupsResp); err != nil {
		return nil, err
	}
	for _, g := range groupsResp.Groups {
		if g.Name == "" || strings.EqualFold(g.Name, "Non-group expenses") {
			continue
		}
		swID := strconv.FormatInt(g.ID, 10)
		existing, err := s.Store.GetGroupBySplitwiseID(ctx, swID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		var group *store.Group
		if existing != nil {
			group = existing
		} else {
			group, err = s.Store.CreateGroup(ctx, &store.Group{
				Name: g.Name, CreatedBy: actor.ID, DefaultCurrency: actor.DefaultCurrency,
				SplitwiseGroupID: sql.NullString{String: swID, Valid: true},
			})
			if err != nil {
				return nil, err
			}
			res.Groups++
		}
		_ = s.Store.AddGroupMember(ctx, group.ID, actor.ID)
		for _, m := range g.Members {
			if m.Email == "" {
				res.Skipped++
				continue
			}
			u, err := s.importUser(ctx, m.Email, strings.TrimSpace(m.FirstName+" "+m.LastName))
			if err != nil {
				return nil, err
			}
			_ = s.Store.AddGroupMember(ctx, group.ID, u.ID)
		}
	}
	return res, nil
}

// importUser finds or unconditionally creates a user by email (imports bypass
// the ENABLE_SENDING_INVITES gate — the user is populating their own data).
func (s *Service) importUser(ctx context.Context, email, name string) (*store.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if u, err := s.Store.GetUserByEmail(ctx, email); err == nil {
		return u, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	role := "USER"
	if s.Config.IsAdminEmail(email) {
		role = "ADMIN"
	}
	return s.Store.CreateUser(ctx, &store.User{Email: email, Name: name, Role: role})
}

// swGet performs an authenticated Splitwise GET and decodes into out.
func swGet(ctx context.Context, c *http.Client, apiKey, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, splitwiseBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("splitwise: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("splitwise: invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("splitwise: unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
