package httpapp

import (
	"fmt"
	"net/http"
)

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "import", "title.import", nil)
}

func (s *Server) handleImportSplitwise(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	me := s.currentUser(r)
	res, err := s.Svc.ImportFromSplitwise(ctx, me, r.FormValue("api_key"))
	if err != nil {
		s.renderErr(w, r, "import", "title.import", nil, http.StatusBadRequest, err.Error())
		return
	}
	msg := fmt.Sprintf(s.tr(r, "flash.imported"), res.Friends, res.Groups, res.Skipped)
	s.redirectFlash(w, r, "/friends", msg)
}
