// Package httpapp wires the chi router, middleware, and HTTP handlers to the
// service layer and the templ-free html/template renderer. Handlers stay thin:
// parse input, call a service method, render a page.
package httpapp

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/hafio/gosplit/internal/auth"
	"github.com/hafio/gosplit/internal/config"
	"github.com/hafio/gosplit/internal/service"
	"github.com/hafio/gosplit/internal/store"
	"github.com/hafio/gosplit/internal/web"
)

// Server holds the shared dependencies for all handlers.
type Server struct {
	Cfg      *config.Config
	Store    *store.Store
	Svc      *service.Service
	Auth     *auth.Manager
	Renderer *web.Renderer
}

// New builds a Server.
func New(cfg *config.Config, st *store.Store, svc *service.Service, am *auth.Manager, r *web.Renderer) *Server {
	return &Server{Cfg: cfg, Store: st, Svc: svc, Auth: am, Renderer: r}
}

// Router assembles the full route tree.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Public assets + health: no session lookup, no CSRF, cacheable.
	r.Handle("/static/*", s.Renderer.AssetsHandler())
	r.Handle("/uploads/*", cacheControl("public, max-age=3600",
		http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.Cfg.UploadDir)))))
	r.Get("/healthz", s.handleHealth)
	r.Get("/manifest.webmanifest", s.serveAsset("manifest.webmanifest", "application/manifest+json", "public, max-age=3600"))
	r.Get("/sw.js", s.serveAsset("sw.js", "application/javascript", "no-cache"))

	// Everything else needs the session/CSRF context.
	r.Group(func(r chi.Router) {
		r.Use(s.Auth.Authenticate)
		r.Use(s.Auth.VerifyCSRF)

		r.Get("/offline", s.handleOffline)

		// Auth (public).
		r.Get("/login", s.handleLoginPage)
		r.Post("/login", s.handleLogin)
		r.Get("/register", s.handleRegisterPage)
		r.Post("/register", s.handleRegister)
		r.Get("/forgot-password", s.handleForgotPage)
		r.Post("/forgot-password", s.handleForgot)
		r.Get("/reset-password", s.handleResetPage)
		r.Post("/reset-password", s.handleReset)
		r.Post("/auth/magic", s.handleMagicRequest)
		r.Get("/auth/magic", s.handleMagicConsume)
		r.Post("/logout", s.handleLogout)

		r.Get("/", s.handleHome)

		// Authenticated area.
		r.Group(func(r chi.Router) {
			r.Use(s.Auth.RequireUser)

			r.Get("/balances", s.handleBalances)

			r.Get("/friends", s.handleFriends)
			r.Post("/friends/add", s.handleFriendAdd)
			r.Get("/friends/{id}", s.handleFriendDetail)
			r.Post("/friends/{id}/hide", s.handleFriendHide)
			r.Post("/friends/{id}/delete", s.handleFriendDelete)
			r.Get("/friends/{id}/settle", s.handleSettlePage)
			r.Post("/friends/{id}/settle", s.handleSettle)
			r.Get("/friends/{id}/convert", s.handleConvertPage)
			r.Post("/friends/{id}/convert", s.handleConvert)
			r.Get("/rates", s.handleRate)
			r.Get("/friends/{id}/collapse", s.handleFriendCollapsePage)
			r.Post("/friends/{id}/collapse", s.handleFriendCollapse)

			r.Get("/groups", s.handleGroups)
			r.Post("/groups/create", s.handleGroupCreate)
			r.Get("/groups/{id}", s.handleGroupDetail)
			r.Post("/groups/{id}/archive", s.handleGroupArchive)
			r.Get("/groups/{id}/collapse", s.handleGroupCollapsePage)
			r.Post("/groups/{id}/collapse", s.handleGroupCollapse)
			r.Post("/groups/{id}/simplify", s.handleGroupSimplify)
			r.Get("/groups/{id}/settle", s.handleGroupSettleAllPage)
			r.Post("/groups/{id}/settle", s.handleGroupSettleAll)
			r.Get("/groups/{id}/settle/{to}", s.handleGroupSettlePage)
			r.Post("/groups/{id}/settle/{to}", s.handleGroupSettle)
			r.Post("/groups/{id}/invite", s.handleGroupInvite)
			r.Post("/groups/join/{publicId}", s.handleGroupJoin)
			r.Get("/g/{publicId}", s.handleGroupJoinPage)

			r.Get("/expenses/new", s.handleExpenseNew)
			r.Post("/expenses", s.handleExpenseCreate)
			r.Get("/expenses/{id}", s.handleExpenseDetail)
			r.Post("/expenses/{id}/delete", s.handleExpenseDelete)
			r.Get("/expenses/{id}/move", s.handleExpenseMovePage)
			r.Post("/expenses/{id}/move", s.handleExpenseMove)

			r.Get("/activity", s.handleActivity)

			r.Get("/recurring", s.handleRecurringList)
			r.Post("/recurring", s.handleRecurringCreate)
			r.Post("/recurring/{id}/delete", s.handleRecurringDelete)

			r.Get("/push/public-key", s.handlePushPublicKey)
			r.Post("/push/subscribe", s.handlePushSubscribe)
			r.Post("/push/unsubscribe", s.handlePushUnsubscribe)
			r.Post("/push/test", s.handlePushTest)

			r.Get("/import", s.handleImportPage)
			r.Post("/import/splitwise", s.handleImportSplitwise)

			r.Get("/bank", s.handleBankPage)
			r.Post("/bank/link-token", s.handleBankLinkToken)
			r.Post("/bank/exchange", s.handleBankExchange)
			r.Post("/bank/sync", s.handleBankSync)
			r.Get("/bank/tx/{txid}/convert", s.handleBankConvert)

			r.Get("/profile", s.handleProfile)
			r.Post("/profile", s.handleProfileUpdate)
			r.Post("/profile/password", s.handlePasswordChange)
			r.Get("/profile/export", s.handleExport)
		})

		// Admin area.
		r.Group(func(r chi.Router) {
			r.Use(s.Auth.RequireUser, s.Auth.RequireAdmin)
			r.Get("/admin", s.handleAdmin)
			r.Post("/admin/users/create", s.handleAdminCreate)
			r.Post("/admin/users/{id}", s.handleAdminUpdate)
			r.Post("/admin/users/{id}/password", s.handleAdminSetPassword)
			r.Post("/admin/users/{id}/toggle", s.handleAdminToggle)
			r.Post("/admin/users/{id}/magic", s.handleAdminMagic)
		})
	})

	return r
}

// --- render helpers -------------------------------------------------------

func (s *Server) vd(r *http.Request, title string, data any) web.ViewData {
	u := auth.UserFrom(r.Context())
	pref, theme := "", ""
	if u != nil {
		pref = u.PreferredLanguage
		theme = u.ThemeColor
	}
	lang := s.Renderer.DetectLang(pref, r.Header.Get("Accept-Language"))
	return web.ViewData{
		Title: s.Renderer.T(lang, title),
		User:  u,
		CSRF:  auth.CSRFFrom(r.Context()),
		Flash: s.Renderer.T(lang, r.URL.Query().Get("flash")),
		Lang:  lang,
		Theme: theme,
		Nav:   navSlug(r.URL.Path),
		Data:  data,
	}
}

// tr translates key for the request's resolved language (user preference,
// then Accept-Language, then default) — for server-side text (titles, flash
// messages, error strings) that isn't rendered through a template's .T.
func (s *Server) tr(r *http.Request, key string) string {
	u := auth.UserFrom(r.Context())
	pref := ""
	if u != nil {
		pref = u.PreferredLanguage
	}
	lang := s.Renderer.DetectLang(pref, r.Header.Get("Accept-Language"))
	return s.Renderer.T(lang, key)
}

// navSlug maps the request path to the active primary-nav item so the layout
// can highlight the current tab.
func navSlug(path string) string {
	switch {
	case strings.HasPrefix(path, "/balances"):
		return "balances"
	case strings.HasPrefix(path, "/friends"):
		return "friends"
	case strings.HasPrefix(path, "/groups"):
		return "groups"
	case strings.HasPrefix(path, "/activity"):
		return "activity"
	}
	return ""
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page, title string, data any) {
	s.Renderer.Render(w, http.StatusOK, page, s.vd(r, title, data))
}

func (s *Server) renderErr(w http.ResponseWriter, r *http.Request, page, title string, data any, status int, errMsg string) {
	vd := s.vd(r, title, data)
	vd.Error = errMsg
	s.Renderer.Render(w, status, page, vd)
}

func (s *Server) currentUser(r *http.Request) *store.User { return auth.UserFrom(r.Context()) }

// --- small helpers --------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DB.PingContext(r.Context()); err != nil {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if s.currentUser(r) == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	dest := s.Cfg.DefaultHomepage
	if dest == "" {
		dest = "/balances"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (s *Server) serveAsset(name, contentType, cache string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := web.Asset(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", cache)
		_, _ = w.Write(b)
	}
}

// cacheControl sets a Cache-Control header before delegating to next.
func cacheControl(v string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", v)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleOffline(w http.ResponseWriter, r *http.Request) {
	vd := s.vd(r, "msg.offline_title", s.tr(r, "msg.offline"))
	s.Renderer.Render(w, http.StatusOK, "message", vd)
}

func atoi64(s string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n, err == nil
}

// requestLogger logs each request with slog (method, path, status, duration).
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"status", ww.Status(), "bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds())
	})
}

// ctxTimeout returns a short request context (guards slow DB calls).
func ctxTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 10*time.Second)
}
