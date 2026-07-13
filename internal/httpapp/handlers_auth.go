package httpapp

import (
	"net/http"
	"strings"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, r, "login", "title.sign_in", nil)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	email := r.FormValue("email")
	password := r.FormValue("password")
	u, err := s.Svc.LoginPassword(ctx, email, password)
	if err != nil {
		s.renderErr(w, r, "login", "title.sign_in", nil, http.StatusUnauthorized, err.Error())
		return
	}
	if err := s.Auth.SetSession(w, r, u.ID); err != nil {
		s.renderErr(w, r, "login", "title.sign_in", nil, http.StatusInternalServerError, s.tr(r, "err.session_start"))
		return
	}
	http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.DisableEmailSignup {
		vd := s.vd(r, "msg.signup_disabled_title", s.tr(r, "msg.signup_disabled"))
		s.Renderer.Render(w, http.StatusForbidden, "message", vd)
		return
	}
	s.render(w, r, "register", "title.create_account", nil)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u, err := s.Svc.Register(ctx, r.FormValue("name"), r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		s.renderErr(w, r, "register", "title.create_account", nil, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Auth.SetSession(w, r, u.ID); err != nil {
		s.renderErr(w, r, "register", "title.create_account", nil, http.StatusInternalServerError, s.tr(r, "err.session_start"))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleForgotPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "forgot", "title.forgot_password", nil)
}

func (s *Server) handleForgot(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	_ = s.Svc.ForgotPassword(ctx, r.FormValue("email"))
	vd := s.vd(r, "msg.check_email_title", s.tr(r, "msg.reset_sent"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleResetPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	s.render(w, r, "reset", "title.reset_password", map[string]any{"Token": token})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	token := r.FormValue("token")
	if err := s.Svc.ResetPassword(ctx, token, r.FormValue("password")); err != nil {
		s.renderErr(w, r, "reset", "title.reset_password", map[string]any{"Token": token}, http.StatusBadRequest, err.Error())
		return
	}
	vd := s.vd(r, "msg.password_updated_title", s.tr(r, "msg.password_updated"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleMagicRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	if err := s.Svc.RequestMagicLink(ctx, r.FormValue("email")); err != nil {
		s.renderErr(w, r, "login", "title.sign_in", nil, http.StatusBadRequest, err.Error())
		return
	}
	vd := s.vd(r, "msg.check_email_title", s.tr(r, "msg.magic_sent"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleMagicConsume(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u, err := s.Svc.ConsumeMagicLink(ctx, r.URL.Query().Get("token"))
	if err != nil {
		vd := s.vd(r, "msg.signin_failed_title", err.Error())
		s.Renderer.Render(w, http.StatusBadRequest, "message", vd)
		return
	}
	if err := s.Auth.SetSession(w, r, u.ID); err != nil {
		http.Error(w, "could not start session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.ClearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// safeNext returns next only if it is a local path (prevents open redirects).
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return "/"
	}
	return next
}
