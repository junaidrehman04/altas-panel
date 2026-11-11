package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/junaidrehman04/altas/apps/api/internal/analytics"
	"github.com/junaidrehman04/altas/apps/api/internal/auth"
	"github.com/junaidrehman04/altas/apps/api/internal/config"
	"github.com/junaidrehman04/altas/apps/api/internal/geoip"
	"github.com/junaidrehman04/altas/apps/api/internal/inspect"
	"github.com/junaidrehman04/altas/apps/api/internal/metrics"
	"github.com/junaidrehman04/altas/apps/api/internal/stacks"
	"github.com/junaidrehman04/altas/apps/api/internal/update"
	"github.com/junaidrehman04/altas/apps/api/internal/version"
	"github.com/junaidrehman04/altas/apps/api/internal/webui"
	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

const sessionCookie = "altas_session"
const trustCookie = "altas_trust"

const registrationClosedError = "Public registration is closed. The first account is the panel admin; further accounts will be invite-only."

type handler struct {
	cfg       config.Config
	svc       *auth.Service
	log       *log.Logger
	metrics   *metrics.Collector
	hostStore *metrics.Store
	traffic   *analytics.Recorder
	geo       *analytics.Reloader
	reader    *inspect.Reader
	probe     stacks.Probe
	updates   *update.Service
	routes    http.Handler
}

// API is the panel HTTP service. RunMaintenance starts the slow update loop.
type API struct {
	h *handler
}

// ServeHTTP dispatches a panel request.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.h.routes.ServeHTTP(w, r)
}

// RunMaintenance checks for an official release on the configured interval.
// It does nothing until an admin turns auto-update on.
func (a *API) RunMaintenance(ctx context.Context) {
	a.h.updates.Run(ctx)
}

// RunGeoIP downloads GeoLite2-Country when a license key is set and the database
// file is missing, then refreshes it weekly. No key leaves the current reader as it is.
func (a *API) RunGeoIP(ctx context.Context) {
	geoip.Run(ctx, geoip.Options{
		Path:   a.h.cfg.GeoDBPath(),
		Key:    a.h.cfg.MaxMindLicenseKey,
		Logger: a.h.log,
		Reload: a.h.geo.Reload,
	})
}

// Locator is the shared country reader, including the access-log tail.
func (a *API) Locator() analytics.Locator {
	return a.h.geo
}

// New builds the HTTP API. logger receives the development password-reset URL.
func New(cfg config.Config, sqlDB *sql.DB, logger *log.Logger) *API {
	if logger == nil {
		logger = log.Default()
	}
	svc, err := auth.NewService(sqlDB, auth.Params{
		Memory:  cfg.ArgonMemory,
		Time:    cfg.ArgonTime,
		Threads: cfg.ArgonThreads,
	}, cfg.SessionTTL, cfg.ResetTTL)
	if err != nil {
		panic(err)
	}
	geo := analytics.NewReloader(cfg.GeoDBPath(), logger)
	h := &handler{
		cfg:       cfg,
		svc:       svc,
		log:       logger,
		metrics:   metrics.NewCollector(cfg.HostProc, cfg.HostRoot),
		hostStore: metrics.NewStore(sqlDB),
		traffic:   analytics.NewRecorder(sqlDB, geo),
		geo:       geo,
		reader:    inspect.NewReader(cfg.HostProc, cfg.HostRoot),
		probe:     stacks.NewProbe(cfg.DockerProbe, cfg.DockerSocket),
		updates: update.New(sqlDB, update.Options{
			Version:    version.Version,
			Owner:      cfg.UpdateOwner,
			Repo:       cfg.UpdateRepo,
			BaseURL:    cfg.UpdateAPIBase,
			Interval:   cfg.UpdateInterval,
			Unit:       cfg.UpdateUnit,
			Restart:    cfg.UpdateRestart,
			BinaryPath: cfg.UpdateBinary,
			PublicKey:  cfg.UpdatePubKey,
			OS:         cfg.UpdateOS,
			Arch:       cfg.UpdateArch,
		}),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(trustForwardedFromProxy)
	r.Use(h.track)
	r.Use(middleware.Recoverer)

	r.Get("/api/health", h.health)
	r.Get("/api/version", h.version)
	r.Get("/api/auth/registration", h.registration)
	r.Post("/api/auth/register", h.register)
	r.Post("/api/auth/login", h.login)
	r.Post("/api/auth/totp", h.totp)
	r.Post("/api/auth/recovery", h.recovery)
	r.Post("/api/auth/logout", h.logout)
	r.Post("/api/auth/forgot-password", h.forgot)
	r.Post("/api/auth/reset-password", h.reset)
	r.Get("/api/me", h.me)
	r.Post("/api/me/totp/setup", h.totpSetup)
	r.Post("/api/me/totp/confirm", h.totpConfirm)
	r.Post("/api/me/totp/disable", h.totpDisable)
	r.Get("/api/panel/bootstrap", h.bootstrap)
	r.Get("/api/panel/edge", h.panelEdge)
	r.Get("/api/panel/apm", h.panelAPM)
	r.Get("/api/panel/hardware", h.panelHardware)
	r.Get("/api/alerts", h.alerts)
	r.Post("/api/alerts/read", h.alertsRead)
	r.Post("/api/alerts/dismiss", h.alertsDismiss)
	r.Get("/api/metrics/host", h.hostMetrics)
	r.Get("/api/metrics/host/series", h.hostSeries)
	r.Get("/api/analytics/summary", h.analyticsSummary)
	r.Get("/api/stacks", h.stackCatalog)
	r.Get("/api/stacks/compose", h.composeProjects)
	r.Get("/api/settings", h.getSettings)
	r.Patch("/api/settings", h.patchSettings)
	r.Post("/api/settings/updates/check", h.checkUpdates)
	r.Post("/api/settings/updates/apply", h.applyUpdate)
	r.Post("/api/settings/updates/rollback", h.rollbackUpdate)
	r.NotFound(h.ui)
	h.routes = r
	return &API{h: h}
}

func (h *handler) ui(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	webui.Serve(w, r)
}

// trustForwardedFromProxy rewrites RemoteAddr from X-Forwarded-For only when
// the immediate peer is loopback or a private address. A client that connects
// directly cannot pick its own address by sending the header. When a trusted
// proxy appends the peer it saw, the last entry is the one that is kept.
func trustForwardedFromProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
			port = "0"
		}
		peer := net.ParseIP(host)
		if peer != nil && (peer.IsLoopback() || peer.IsPrivate()) {
			if client := forwardedClient(r); client != "" {
				r.RemoteAddr = net.JoinHostPort(client, port)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func forwardedClient(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := strings.TrimSpace(parts[i])
			if ip := net.ParseIP(candidate); ip != nil {
				return ip.String()
			}
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		if ip := net.ParseIP(real); ip != nil {
			return ip.String()
		}
	}
	return ""
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version.Version})
}

func (h *handler) version(w http.ResponseWriter, _ *http.Request) {
	host := ""
	env := ""
	if h.reader != nil {
		host = h.reader.Report().FQDN
	}
	env = h.cfg.Env
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version, "hostname": host, "env": env})
}

func (h *handler) registration(w http.ResponseWriter, r *http.Request) {
	open, err := h.svc.RegistrationOpen(r.Context())
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"open": open})
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	user, sess, err := h.svc.Register(r.Context(), body.Email, body.Password)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	h.setSessionCookie(w, r, sess)
	writeJSON(w, http.StatusCreated, userJSON(user))
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Remember *bool  `json:"remember"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	trust := ""
	if c, err := r.Cookie(trustCookie); err == nil {
		trust = c.Value
	}
	outcome, err := h.svc.Login(r.Context(), body.Email, body.Password, auth.LoginOptions{
		Remember:   body.Remember,
		TrustToken: trust,
		Place:      h.signInPlace(r),
	})
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	if outcome.TOTPRequired {
		writeJSON(w, http.StatusOK, map[string]any{
			"totp_required": true,
			"challenge":     outcome.Challenge,
			"account":       outcome.User.Email,
		})
		return
	}
	h.setSessionCookie(w, r, outcome.Session)
	writeJSON(w, http.StatusOK, userJSON(outcome.User))
}

func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	raw := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		raw = c.Value
	}
	if err := h.svc.Logout(r.Context(), raw); err != nil {
		h.writeAuthError(w, err)
		return
	}
	h.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) forgot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	token, ok, err := h.svc.RequestReset(r.Context(), body.Email)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	if ok && h.cfg.Env == "development" {
		h.log.Printf("reset url: %s/reset-password?token=%s", h.cfg.WebOrigin, token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) reset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), body.Token, body.Password); err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (h *handler) hostMetrics(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	snap, err := h.metrics.Snapshot()
	if err != nil {
		h.log.Printf("metrics: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Host metrics are not available."})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (h *handler) hostSeries(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	span, err := window.Parse(r.URL.Query().Get("range"), time.Now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "range must be 1h, 6h, live, 24h, 7d, 30d, or month"})
		return
	}
	series, err := h.hostStore.Series(r.Context(), span)
	if err != nil {
		h.log.Printf("metrics series: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "something went wrong"})
		return
	}
	writeJSON(w, http.StatusOK, series)
}

func (h *handler) analyticsSummary(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	span, err := window.Parse(r.URL.Query().Get("range"), time.Now())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "range must be 1h, 6h, live, 24h, 7d, 30d, or month"})
		return
	}
	project, ok := projectFilter(r.URL.Query().Get("project"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is invalid"})
		return
	}
	summary, err := h.traffic.Summary(r.Context(), span, project)
	if err != nil {
		h.log.Printf("analytics: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "something went wrong"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func projectFilter(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	project := analytics.SanitizeProject(raw)
	if project == "" {
		return "", false
	}
	return project, true
}

func (h *handler) track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.traffic == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := h.traffic.Record(ctx, analytics.Event{
			At:         start,
			Project:    requestProject(r),
			Host:       r.Host,
			App:        requestApp(r),
			Path:       r.URL.Path,
			Status:     status,
			Bytes:      int64(wrapped.BytesWritten()),
			Latency:    time.Since(start),
			HasLatency: true,
			RemoteIP:   remoteIP(r),
			UserAgent:  r.UserAgent(),
		})
		if err != nil {
			h.log.Printf("analytics: record failed")
		}
	})
}

func requestProject(r *http.Request) string {
	if project := analytics.SanitizeProject(r.Header.Get("X-Altas-Project")); project != "" {
		return project
	}
	return "panel"
}

func requestApp(r *http.Request) string {
	if app := analytics.SanitizeProject(r.Header.Get("X-Altas-App")); app != "" {
		return app
	}
	return "panel"
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func (h *handler) stackCatalog(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects_root": h.projectsRoot(),
		"stacks":        stacks.Catalog(),
	})
}

func (h *handler) composeProjects(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	report, err := h.probe.Report(r.Context())
	if err != nil {
		h.log.Printf("docker probe: %v", err)
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *handler) currentUser(r *http.Request) (auth.User, error) {
	raw := ""
	if c, err := r.Cookie(sessionCookie); err == nil {
		raw = c.Value
	}
	return h.svc.UserFromSession(r.Context(), raw)
}

func (h *handler) projectsRoot() string {
	root := strings.TrimSpace(h.cfg.ProjectsRoot)
	if root == "" {
		return "/var/lib/altas/projects"
	}
	return root
}

func (h *handler) writeAuthError(w http.ResponseWriter, err error) {
	var validation auth.ValidationError
	var locked auth.LockedError
	var cred auth.CredentialError
	switch {
	case errors.As(err, &locked):
		secs := int(locked.RetryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "Too many failed attempts. Try again in 15 minutes.", "locked": true, "retry_after_seconds": secs,
		})
	case errors.As(err, &cred):
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": "Invalid email or password", "attempts_remaining": cred.AttemptsRemaining,
		})
	case errors.Is(err, auth.ErrInvalidCode):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "That code is invalid or has expired."})
	case errors.As(err, &validation):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validation.Message})
	case errors.Is(err, auth.ErrRegistrationClosed):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": registrationClosedError})
	case errors.Is(err, auth.ErrEmailTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with that email already exists"})
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
	case errors.Is(err, auth.ErrInvalidToken):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Reset link is invalid or expired"})
	case errors.Is(err, auth.ErrUnauthenticated):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sign in to continue"})
	default:
		h.log.Printf("error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "something went wrong"})
	}
}

func (h *handler) setSessionCookie(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	cookie := &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	}
	if sess.Persistent {
		cookie.Expires = sess.ExpiresAt
		cookie.MaxAge = int(time.Until(sess.ExpiresAt).Seconds())
		if cookie.MaxAge < 0 {
			cookie.MaxAge = 0
		}
	}
	http.SetCookie(w, cookie)
}

func (h *handler) setTrustCookie(w http.ResponseWriter, r *http.Request, raw string) {
	http.SetCookie(w, &http.Cookie{
		Name:     trustCookie,
		Value:    raw,
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *handler) signInPlace(r *http.Request) string {
	if h.geo == nil || !h.geo.Enabled() {
		return ""
	}
	truncated, ok := analytics.TruncateIP(remoteIP(r))
	if !ok {
		return ""
	}
	country, city := h.geo.Lookup(truncated)
	country = strings.TrimSpace(country)
	city = strings.TrimSpace(city)
	switch {
	case city != "" && country != "":
		return city + ", " + country
	case country != "":
		return country
	default:
		return ""
	}
}

func (h *handler) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func userJSON(user auth.User) map[string]any {
	last := ""
	if !user.LastSignInAt.IsZero() {
		last = user.LastSignInAt.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"user": map[string]any{
			"id":                 user.ID,
			"email":              user.Email,
			"is_admin":           user.IsAdmin,
			"created_at":         user.CreatedAt.UTC().Format(time.RFC3339),
			"totp_enabled":       user.TOTPEnabled,
			"last_sign_in_at":    last,
			"last_sign_in_place": user.LastSignInPlace,
			"initials":           initials(user.Email),
			"label":              accountLabel(user.Email),
		},
	}
}

func accountLabel(email string) string {
	name, _, ok := strings.Cut(email, "@")
	if !ok || name == "" {
		return email
	}
	return name
}

func initials(email string) string {
	label := accountLabel(email)
	letters := make([]rune, 0, 2)
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			letters = append(letters, r)
		}
		if len(letters) == 2 {
			break
		}
	}
	if len(letters) == 0 {
		return "A"
	}
	return strings.ToUpper(string(letters))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "content type must be application/json"})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be json"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
