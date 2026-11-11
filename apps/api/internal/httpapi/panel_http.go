package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/analytics"
	"github.com/junaidrehman04/altas/apps/api/internal/auth"
	"github.com/junaidrehman04/altas/apps/api/internal/panelview"
	"github.com/junaidrehman04/altas/apps/api/internal/version"
	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

func (h *handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	user, err := h.currentUser(r)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	src, err := h.sources(r, "1h")
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	id := panelview.BuildHardware(src).Identity
	org := strings.TrimSpace(h.cfg.Org)
	if org == "" {
		org = id.FQDN
	}
	if org == "" {
		org = "This host"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": version.Version,
		"env":     h.cfg.Env,
		"org":     org,
		"user":    userJSON(user)["user"],
		"host":    id,
		"agent": map[string]any{
			"connected":   true,
			"cpu_percent": src.Host.AgentCPU,
		},
	})
}

func (h *handler) panelEdge(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	src, err := h.sources(r, "")
	if err != nil {
		h.writePanelError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, panelview.BuildEdge(src))
}

func (h *handler) panelAPM(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	src, err := h.sources(r, "")
	if err != nil {
		h.writePanelError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, panelview.BuildAPM(src))
}

func (h *handler) panelHardware(w http.ResponseWriter, r *http.Request) {
	if _, err := h.currentUser(r); err != nil {
		h.writeAuthError(w, err)
		return
	}
	src, err := h.sources(r, "")
	if err != nil {
		h.writePanelError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, panelview.BuildHardware(src))
}

func (h *handler) sources(r *http.Request, fallback string) (panelview.Sources, error) {
	name := strings.TrimSpace(r.URL.Query().Get("range"))
	if name == "" {
		name = fallback
	}
	if name == "" {
		name = "24h"
	}
	span, err := window.Parse(name, time.Now())
	if err != nil {
		return panelview.Sources{}, err
	}
	ctx := r.Context()
	snap, snapErr := h.metrics.Snapshot()
	if snapErr != nil {
		snap.Hostname = ""
	}
	series, err := h.hostStore.Series(ctx, span)
	if err != nil {
		return panelview.Sources{}, err
	}
	detail, err := h.hostStore.DetailSeries(ctx, span)
	if err != nil {
		return panelview.Sources{}, err
	}
	cores, err := h.hostStore.CoreSeries(ctx, span)
	if err != nil {
		return panelview.Sources{}, err
	}
	project := ""
	if raw := strings.TrimSpace(r.URL.Query().Get("project")); raw != "" {
		project = analytics.SanitizeProject(raw)
		if project == "" {
			return panelview.Sources{}, errBadProject
		}
	}
	summary, err := h.traffic.Summary(ctx, span, "")
	if err != nil {
		return panelview.Sources{}, err
	}
	events, err := h.svc.RecentAuthEvents(ctx, 20)
	if err != nil {
		events = nil
	}
	containers, _ := h.probe.Containers(ctx)
	host := h.reader.Report()
	return panelview.Sources{
		Now:        time.Now(),
		Range:      span.Name,
		Snap:       snap,
		Series:     series,
		Detail:     detail,
		Cores:      cores,
		Summary:    summary,
		Events:     events,
		Host:       host,
		Containers: containers,
		AccessLog:  strings.TrimSpace(h.cfg.AccessLog) != "",
		Geo:        h.geo != nil && h.geo.Enabled(),
		Project:    project,
	}, nil
}

func (h *handler) decorateAlerts(userID string, src panelview.Sources, flags map[string]auth.AlertFlag) []panelview.Alert {
	_ = userID
	raw := panelview.BuildAlerts(src)
	out := make([]panelview.Alert, 0, len(raw))
	for _, alert := range raw {
		flag := flags[alert.ID]
		if flag.Dismissed {
			continue
		}
		alert.Read = flag.Read
		out = append(out, alert)
	}
	return out
}

var errBadProject = errors.New("bad project")

func (h *handler) writePanelError(w http.ResponseWriter, err error) {
	if errors.Is(err, errBadProject) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project id is invalid"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "range must be 1h, 6h, live, 24h, 7d, 30d, or month"})
}
