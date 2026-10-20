package service

import (
	"context"
	"errors"

	"github.com/hafio/gosplit/internal/store"
)

// Errors surfaced to group handlers.
var (
	// ErrLimitedUser is returned to a LIMITED account for anything that creates a
	// group: creating one directly and running a Splitwise import.
	ErrLimitedUser = errors.New("limited accounts cannot create groups or import from Splitwise; ask an admin to make this account a full user")
)

// CreateGroup creates g on behalf of actor, who becomes its creator and first
// member. It is the one gate in front of Store.CreateGroup: an actor who may not
// create groups (store.User.CanCreateGroups) is refused before anything is
// written.
func (s *Service) CreateGroup(ctx context.Context, actor *store.User, g *store.Group) (*store.Group, error) {
	if !actor.CanCreateGroups() {
		return nil, ErrLimitedUser
	}
	g.CreatedBy = actor.ID
	return s.Store.CreateGroup(ctx, g)
}
