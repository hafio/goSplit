package httpapp

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// collapseView is the data for the archive/collapse confirmation page.
type collapseView struct {
	Kind      string // "friend" | "group"
	Target    string // display name
	Action    string // form action (GET preview + POST apply)
	CancelURL string
	Before    string         // selected cutoff date (YYYY-MM-DD), "" until picked
	Today     string         // default for the date input
	Counts    map[string]int // per-currency count of collapsible expenses (preview)
	Total     int
}

func (s *Server) handleFriendCollapsePage(w http.ResponseWriter, r *http.Request) {
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
	action := fmt.Sprintf("/friends/%d/collapse", fid)
	vd := collapseView{Kind: "friend", Target: displayName(friend), Action: action,
		CancelURL: fmt.Sprintf("/friends/%d", fid), Before: r.URL.Query().Get("before"),
		Today: time.Now().Format("2006-01-02")}
	if vd.Before != "" {
		counts, _ := s.Svc.PreviewArchive(ctx, me.ID, &fid, nil, vd.Before)
		vd.Counts = counts
		for _, n := range counts {
			vd.Total += n
		}
	}
	s.render(w, r, "collapse", "archive.title", vd)
}

func (s *Server) handleFriendCollapse(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	fid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, err := s.Svc.ArchiveDirect(ctx, me.ID, fid, r.FormValue("before"))
	if err != nil {
		s.renderErr(w, r, "message", "msg.archive_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	s.redirectFlash(w, r, fmt.Sprintf("/friends/%d", fid), fmt.Sprintf(s.tr(r, "flash.archived_n"), n))
}

func (s *Server) handleGroupCollapsePage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	gid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	g, err := s.Store.GetGroup(ctx, gid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	action := fmt.Sprintf("/groups/%d/collapse", gid)
	vd := collapseView{Kind: "group", Target: g.Name, Action: action,
		CancelURL: fmt.Sprintf("/groups/%d", gid), Before: r.URL.Query().Get("before"),
		Today: time.Now().Format("2006-01-02")}
	if vd.Before != "" {
		counts, _ := s.Svc.PreviewArchive(ctx, me.ID, nil, &gid, vd.Before)
		vd.Counts = counts
		for _, n := range counts {
			vd.Total += n
		}
	}
	s.render(w, r, "collapse", "archive.title", vd)
}

func (s *Server) handleGroupCollapse(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	gid, ok := atoi64(chi.URLParam(r, "id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	n, err := s.Svc.ArchiveGroup(ctx, me.ID, gid, r.FormValue("before"))
	if err != nil {
		s.renderErr(w, r, "message", "msg.archive_failed", err.Error(), http.StatusBadRequest, err.Error())
		return
	}
	s.redirectFlash(w, r, fmt.Sprintf("/groups/%d", gid), fmt.Sprintf(s.tr(r, "flash.archived_n"), n))
}
