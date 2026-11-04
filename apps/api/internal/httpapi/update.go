package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/auth"
	"github.com/junaidrehman04/altas/apps/api/internal/update"
)

func (h *handler) getSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	status, err := h.updates.Status(r.Context())
	if err != nil {
		h.writeUpdateError(w, err)
		return
	}
	h.writeUpdateView(w, status, false, status.UpdateAvailable)
}

func (h *handler) patchSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var body struct {
		AutoUpdate *bool `json:"auto_update"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.AutoUpdate == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "auto_update is required"})
		return
	}
	if err := h.updates.SetAutoUpdate(r.Context(), *body.AutoUpdate); err != nil {
		h.writeUpdateError(w, err)
		return
	}
	status, err := h.updates.Status(r.Context())
	if err != nil {
		h.writeUpdateError(w, err)
		return
	}
	h.writeUpdateView(w, status, false, status.UpdateAvailable)
}

func (h *handler) checkUpdates(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !decodeJSON(w, r, &struct{}{}) {
		return
	}
	result, err := h.updates.Check(r.Context(), true)
	if err != nil {
		h.writeUpdateError(w, err)
		return
	}
	h.writeUpdateView(w, result.Status, result.Applied, result.UpdateAvailable)
}

func (h *handler) applyUpdate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !decodeJSON(w, r, &struct{}{}) {
		return
	}
	result, err := h.updates.Apply(r.Context())
	if err != nil {
		h.writeUpdateError(w, err)
		return
	}
	h.writeUpdateView(w, result.Status, result.Applied, result.UpdateAvailable)
}

func (h *handler) rollbackUpdate(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !decodeJSON(w, r, &struct{}{}) {
		return
	}
	if err := h.updates.Rollback(r.Context()); err != nil {
		h.writeUpdateError(w, err)
		return
	}
	status, err := h.updates.Status(r.Context())
	if err != nil {
		h.writeUpdateError(w, err)
		return
	}
	h.writeUpdateView(w, status, false, status.UpdateAvailable)
}

func (h *handler) requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return auth.User{}, false
	}
	if !user.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access is required."})
		return auth.User{}, false
	}
	return user, true
}

func (h *handler) writeUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, update.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Update checks are rate limited. Try again later."})
	case errors.Is(err, update.ErrNoBackup):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "No previous binary is available to restore."})
	case errors.Is(err, update.ErrChecksumMismatch), errors.Is(err, update.ErrChecksumFormat):
		h.log.Printf("update verify: %v", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Release checksum did not match. The binary was not replaced."})
	case errors.Is(err, update.ErrSignature):
		h.log.Printf("update signature: %v", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Release signature verification failed. The binary was not replaced."})
	case errors.Is(err, update.ErrNotBinary):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Release asset is not a binary. The file was not replaced."})
	case errors.Is(err, update.ErrNoRelease):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "No official release has been published yet."})
	case errors.Is(err, update.ErrSource), errors.Is(err, update.ErrAsset):
		h.log.Printf("update source: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Could not read the official release."})
	default:
		h.log.Printf("update: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "something went wrong"})
	}
}

func (h *handler) writeUpdateView(w http.ResponseWriter, status update.Status, applied, available bool) {
	var last *string
	if status.LastCheckSet {
		formatted := status.LastCheck.UTC().Format(time.RFC3339)
		last = &formatted
	}
	behavior := "When auto-update is off, Check now only reports the release and Apply installs it."
	if status.AutoUpdate {
		behavior = "When auto-update is on, a verified newer release is installed and the service restarts."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"auto_update":            status.AutoUpdate,
		"last_check":             last,
		"current_version":        status.CurrentVersion,
		"latest_available":       status.LatestAvailable,
		"rollback_available":     status.RollbackAvailable,
		"last_error":             status.LastError,
		"check_interval_seconds": int(status.Interval.Seconds()),
		"restart_mode":           status.RestartMode,
		"unit":                   status.Unit,
		"verification":           status.Verification,
		"repository":             status.Repository,
		"apply_behavior":         behavior,
		"applied":                applied,
		"update_available":       available,
	})
}
