package api

import (
	"net/http"

	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/modelservice"
)

type providerRosterResponse = modelservice.ProviderRosterItem

func (h *Handler) handleListProviderRoster(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.LoadConfig(h.configPath)
	roster := modelservice.ListRoster(cfg)
	writeJSON(w, http.StatusOK, map[string]any{"providers": roster, "total": len(roster)})
}

func compatibleRosterAdapter(provider string) (string, bool) {
	return modelservice.CompatibleRosterAdapter(provider)
}
