package httpapp

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hafio/gosplit/internal/money"
	"github.com/hafio/gosplit/internal/store"
)

const (
	// menuLimit is how many notifications the topbar dropdown shows before
	// deferring to the full page.
	menuLimit = 6
	// notifPath is the list page, and the fallback for anything that cannot be
	// resolved to an entry.
	notifPath = "/notifications"
	// notifPage is the page name in the template and fragment registries.
	notifPage = "notifications"
	// notifTitle is the page title key.
	notifTitle = "title.notifications"
)

// notificationRow is the presentation shape of one notification: the message is
// already rendered in the viewer's language, so the template never has to know
// which kind it is looking at.
type notificationRow struct {
	ID        int64
	Message   string
	ShareLine string
	Href      string
	Read      bool
	DateDay   string
	DateMonth string
}

// entityHref resolves a notification's link. entity_id carries no foreign key by
// design, so the target can be gone; the fallback keeps a stale entry clickable
// instead of sending the reader to a dead end. This is the one place a new
// entity type needs registering.
func entityHref(n *store.Notification) string {
	switch n.EntityType {
	case "expense":
		return "/expenses/" + n.EntityID
	default:
		return notifPath
	}
}

// buildNotificationRows renders each notification into display form. The kind
// string selects the locale entry and the arguments are always (actor, title),
// mirroring service.renderKind so both channels read identically.
func (s *Server) buildNotificationRows(r *http.Request, nc *nameCache, ns []*store.Notification) []notificationRow {
	ctx := r.Context()
	rows := make([]notificationRow, 0, len(ns))
	for _, n := range ns {
		day, mon := feedDateParts(dateOnly(n.CreatedAt))
		row := notificationRow{
			ID:        n.ID,
			Message:   fmt.Sprintf(s.tr(r, "notif."+n.Kind), nc.user(ctx, n.ActorID), n.Title),
			Href:      entityHref(n),
			Read:      n.IsRead(),
			DateDay:   day,
			DateMonth: mon,
		}
		switch {
		case n.Amount < 0:
			row.ShareLine = fmt.Sprintf(s.tr(r, "notif.you_owe"), money.FormatWithCode(-n.Amount, n.Currency))
		case n.Amount > 0:
			row.ShareLine = fmt.Sprintf(s.tr(r, "notif.you_are_owed"), money.FormatWithCode(n.Amount, n.Currency))
		}
		rows = append(rows, row)
	}
	return rows
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	ns, err := s.Store.ListNotifications(ctx, me.ID, feedLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]any{"Items": s.buildNotificationRows(r, s.newNameCache(), ns)}
	s.render(w, r, notifPage, notifTitle, data)
}

// handleNotificationsMenu answers the topbar dropdown. It renders the block
// directly rather than going through s.render, because the dropdown is chrome
// the layout owns, not a swap target any page registers.
func (s *Server) handleNotificationsMenu(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	ns, err := s.Store.ListNotifications(ctx, me.ID, menuLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vd := s.vd(r, notifTitle,
		map[string]any{"Items": s.buildNotificationRows(r, s.newNameCache(), ns)})
	vd.Page = notifPage
	s.Renderer.RenderFragment(w, r, http.StatusOK, notifPage, "frag_notif_menu", vd)
}

// handleNotificationsBadge answers the bell's own poller. The badge lives
// outside #content, so the page freshness poller can never refresh it.
func (s *Server) handleNotificationsBadge(w http.ResponseWriter, r *http.Request) {
	vd := s.vd(r, notifTitle, nil)
	vd.Page = notifPage
	s.Renderer.RenderFragment(w, r, http.StatusOK, notifPage, "frag_notif_badge", vd)
}

// handleNotificationOpen marks one notification read and forwards to its entry.
// The store scopes the lookup by owner, so somebody else's id behaves exactly
// like one that does not exist -- it is never marked read, and the redirect
// reveals nothing about whether it was there.
func (s *Server) handleNotificationOpen(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	id, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.Redirect(w, r, notifPath, http.StatusSeeOther)
		return
	}
	n, err := s.Store.GetNotification(ctx, id, me.ID)
	if err != nil {
		http.Redirect(w, r, notifPath, http.StatusSeeOther)
		return
	}
	if err := s.Store.MarkNotificationRead(ctx, id, me.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, entityHref(n), http.StatusSeeOther)
}

func (s *Server) handleNotificationsReadAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	if err := s.Store.MarkAllNotificationsRead(ctx, me.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.redirectFlash(w, r, notifPath, "flash.notifications_read")
}
