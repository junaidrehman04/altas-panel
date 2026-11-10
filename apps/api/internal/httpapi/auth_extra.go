package httpapi

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/skip2/go-qrcode"
)

func (h *handler) totp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
		Trust     bool   `json:"trust"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	user, sess, trust, err := h.svc.CompleteTOTP(r.Context(), body.Challenge, body.Code, body.Trust)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	h.setSessionCookie(w, r, sess)
	if trust != "" {
		h.setTrustCookie(w, r, trust)
	}
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (h *handler) recovery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	user, sess, err := h.svc.CompleteRecovery(r.Context(), body.Challenge, body.Code)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	h.setSessionCookie(w, r, sess)
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (h *handler) totpSetup(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	secret, uri, err := h.svc.BeginEnrollment(r.Context(), user.ID, user.Email)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	png, err := qrcode.Encode(uri, qrcode.Medium, 220)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"secret":  secret,
		"otpauth": uri,
		"qr_png":  base64.StdEncoding.EncodeToString(png),
	})
}

func (h *handler) totpConfirm(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	codes, err := h.svc.ConfirmEnrollment(r.Context(), user.ID, body.Code)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	user.TOTPEnabled = true
	payload := userJSON(user)
	payload["recovery_codes"] = codes
	writeJSON(w, http.StatusOK, payload)
}

func (h *handler) totpDisable(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.DisableTOTP(r.Context(), user.ID, body.Password, body.Code); err != nil {
		h.writeAuthError(w, err)
		return
	}
	user.TOTPEnabled = false
	writeJSON(w, http.StatusOK, userJSON(user))
}

func (h *handler) alerts(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	src, err := h.sources(r, "24h")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "range must be 1h, 6h, live, 24h, 7d, 30d, or month"})
		return
	}
	flags, err := h.svc.AlertStates(r.Context(), user.ID)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	items := h.decorateAlerts(user.ID, src, flags)
	writeJSON(w, http.StatusOK, map[string]any{"alerts": items})
}

func (h *handler) alertsRead(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	var body struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	ids := body.IDs
	if body.All {
		src, err := h.sources(r, "24h")
		if err != nil {
			h.writeAuthError(w, err)
			return
		}
		ids = nil
		for _, alert := range h.decorateAlerts(user.ID, src, nil) {
			ids = append(ids, alert.ID)
		}
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}
	if err := h.svc.MarkAlertsRead(r.Context(), user.ID, ids); err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) alertsDismiss(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if err := h.svc.DismissAlert(r.Context(), user.ID, strings.TrimSpace(body.ID)); err != nil {
		h.writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
