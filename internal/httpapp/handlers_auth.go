package httpapp

import (
	"net/http"
	"strings"
	"unicode"
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
	if err := s.Auth.SetSession(w, r, u.ID); err != nil { // cov:ignore store-fault: creating the session row fails only on a broken database
		s.renderErr(w, r, "login", "title.sign_in", nil, http.StatusInternalServerError, s.tr(r, "err.session_start"))
		return
	}
	http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.DisableEmailSignup { // cov:ignore pass-through: the service refusal is tested in internal/service
		vd := s.vdPage(w, r, "msg.signup_disabled_title", s.tr(r, "msg.signup_disabled"))
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
	if err := s.Auth.SetSession(w, r, u.ID); err != nil { // cov:ignore store-fault: creating the session row fails only on a broken database
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
	vd := s.vdPage(w, r, "msg.check_email_title", s.tr(r, "msg.reset_sent"))
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
	vd := s.vdPage(w, r, "msg.password_updated_title", s.tr(r, "msg.password_updated"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleMagicRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	if err := s.Svc.RequestMagicLink(ctx, r.FormValue("email")); err != nil {
		s.renderErr(w, r, "login", "title.sign_in", nil, http.StatusBadRequest, err.Error())
		return
	}
	vd := s.vdPage(w, r, "msg.check_email_title", s.tr(r, "msg.magic_sent"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func (s *Server) handleMagicConsume(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := ctxTimeout(r)
	defer cancel()
	u, err := s.Svc.ConsumeMagicLink(ctx, r.URL.Query().Get("token"))
	if err != nil {
		vd := s.vdPage(w, r, "msg.signin_failed_title", err.Error())
		s.Renderer.Render(w, http.StatusBadRequest, "message", vd)
		return
	}
	if err := s.Auth.SetSession(w, r, u.ID); err != nil { // cov:ignore store-fault: creating the session row fails only on a broken database
		http.Error(w, "could not start session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Auth.ClearSession(w, r)
	// Drop what this browser kept of the account: the service worker's offline
	// copies of its pages, and the worker itself, which the next page load
	// registers again. On a shared device the next person would otherwise see
	// them offline.
	w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// safeNext returns next only if it is a local path (prevents open redirects).
// Browsers read a backslash as a slash, so "/\evil.example" would leave the
// site just like "//evil.example"; control characters are refused as well.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsAny(next, `\`) || strings.IndexFunc(next, unicode.IsControl) >= 0 {
		return "/"
	}
	return next
}
