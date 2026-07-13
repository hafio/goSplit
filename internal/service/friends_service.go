package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/store"
)

// findOrCreateUser returns an existing user by email, or creates a pending one
// (name-less, password-less) so they can be invited. Creating pending users is
// gated by ENABLE_SENDING_INVITES.
func (s *Service) findOrCreateUser(ctx context.Context, email string) (*store.User, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, false, errors.New("email is required")
	}
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err == nil {
		return u, false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, false, err
	}
	if !s.Config.EnableSendingInvites {
		return nil, false, errors.New("that person doesn't have an account and invites are disabled")
	}
	role := "USER"
	if s.Config.IsAdminEmail(email) {
		role = "ADMIN"
	}
	created, err := s.Store.CreateUser(ctx, &store.User{Email: email, Role: role})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// AddFriendByEmail links the current user to another (creating + inviting a
// pending user when enabled).
func (s *Service) AddFriendByEmail(ctx context.Context, ownerID int64, email string) error {
	friend, created, err := s.findOrCreateUser(ctx, email)
	if err != nil {
		return err
	}
	if friend.ID == ownerID {
		return errors.New("you can't add yourself")
	}
	if err := s.Store.AddFriend(ctx, ownerID, friend.ID); err != nil {
		return err
	}
	if created {
		s.sendInvite(ctx, email, "A friend added you on GoSplit. Sign in with this email to see your shared expenses:\n\n"+s.Config.BaseURL+"/login")
	}
	return nil
}

// ToggleHiddenFriend flips a friend's hidden flag on the user's hidden list.
func (s *Service) ToggleHiddenFriend(ctx context.Context, u *store.User, friendID int64) error {
	out := make([]int64, 0, len(u.HiddenFriendIDs)+1)
	found := false
	for _, id := range u.HiddenFriendIDs {
		if id == friendID {
			found = true
			continue
		}
		out = append(out, id)
	}
	if !found {
		out = append(out, friendID)
	}
	u.HiddenFriendIDs = out
	return s.Store.SetHiddenFriends(ctx, u.ID, out)
}

// InviteToGroup adds a user (existing or newly-invited) to a group.
func (s *Service) InviteToGroup(ctx context.Context, actor *store.User, groupID int64, email string) error {
	member, err := s.Store.IsGroupMember(ctx, groupID, actor.ID)
	if err != nil {
		return err
	}
	if !member {
		return errors.New("only members can invite others")
	}
	invitee, created, err := s.findOrCreateUser(ctx, email)
	if err != nil {
		return err
	}
	if err := s.Store.AddGroupMember(ctx, groupID, invitee.ID); err != nil {
		return err
	}
	if created {
		g, _ := s.Store.GetGroup(ctx, groupID)
		name := "a group"
		if g != nil {
			name = g.Name
		}
		s.sendInvite(ctx, email, fmt.Sprintf("You've been added to %q on GoSplit. Sign in with this email:\n\n%s/login", name, s.Config.BaseURL))
	}
	return nil
}

// sendInvite emails an invitation without blocking the caller. Sending runs in
// its own goroutine with a bounded context so a slow or unreachable SMTP server
// never stalls the HTTP request that triggered it.
func (s *Service) sendInvite(_ context.Context, email, body string) {
	if !s.Config.EnableSendingInvites {
		return
	}
	s.sendMailAsync(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Mail.Send(ctx, email, "You've been invited to GoSplit", body); err != nil {
			slog.Warn("invite email failed", "to", email, "err", err)
		}
	})
}
