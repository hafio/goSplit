package httpapp

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/hafio/gosplit/internal/service"
)

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	// An import creates groups, so a LIMITED account gets the explanation instead
	// of a form whose submit the service would refuse anyway.
	if !s.currentUser(r).CanCreateGroups() {
		s.renderLimited(w, r)
		return
	}
	s.render(w, r, "import", "title.import", nil)
}

func (s *Server) handleImportSplitwise(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	res, err := s.Svc.ImportFromSplitwise(ctx, me, r.FormValue("api_key"))
	if errors.Is(err, service.ErrLimitedUser) {
		s.renderLimited(w, r)
		return
	}
	if err != nil {
		s.renderErr(w, r, "import", "title.import", nil, http.StatusBadRequest, err.Error())
		return
	}
	msg := fmt.Sprintf(s.tr(r, "flash.imported"), res.Friends, res.Groups, res.Skipped) // cov:ignore env: needs the live Splitwise API
	s.redirectFlash(w, r, "/friends", msg)
}
