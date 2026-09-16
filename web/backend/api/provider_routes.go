package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/xibodev/facet-studio/pkg/config"
)

type providerInstanceCatalogResponse struct {
	InstanceID   string         `json:"instance_id"`
	ProviderKind string         `json:"provider_kind"`
	Models       []CatalogModel `json:"models"`
	FetchedAt    string         `json:"fetched_at"`
}

type providerTargetResponse struct {
	Target       string `json:"target"`
	InstanceID   string `json:"instance_id"`
	ModelID      string `json:"model_id"`
	ProviderKind string `json:"provider_kind"`
	OwnedBy      string `json:"owned_by,omitempty"`
	FetchedAt    string `json:"fetched_at"`
}

func (h *Handler) registerProviderRouteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/model-routes", h.handleListProviderRoutes)
	mux.HandleFunc("POST /api/model-routes", h.handleCreateProviderRoute)
	mux.HandleFunc("PUT /api/model-routes/{name}", h.handleUpdateProviderRoute)
	mux.HandleFunc("DELETE /api/model-routes/{name}", h.handleDeleteProviderRoute)
	mux.HandleFunc("GET /api/active-models", h.handleListActiveModels)
	mux.HandleFunc("POST /api/active-models", h.handleSetActiveModels)
	mux.HandleFunc("POST /api/active-models/add", h.handleAddActiveModel)
	mux.HandleFunc("POST /api/active-models/remove", h.handleRemoveActiveModel)
}

func (h *Handler) handleListProviderInstanceCatalogs(w http.ResponseWriter, r *http.Request) {
	cfg, store, err := h.loadProviderProjectionState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	catalogs := instanceCatalogResponses(cfg, store)
	writeJSON(w, http.StatusOK, map[string]any{"catalogs": catalogs, "total": len(catalogs)})
}

func (h *Handler) handleListProviderTargets(w http.ResponseWriter, r *http.Request) {
	cfg, store, err := h.loadProviderProjectionState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allTargets := providerTargetResponses(cfg, store)

	if r.URL.Query().Get("all") == "true" {
		writeJSON(w, http.StatusOK, map[string]any{"targets": allTargets, "total": len(allTargets)})
		return
	}

	if len(cfg.ActiveModels) > 0 {
		activeSet := make(map[string]bool, len(cfg.ActiveModels))
		for _, m := range cfg.ActiveModels {
			activeSet[strings.TrimSpace(m)] = true
		}
		filtered := make([]providerTargetResponse, 0, len(cfg.ActiveModels))
		for _, target := range allTargets {
			if activeSet[target.Target] {
				filtered = append(filtered, target)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"targets": filtered, "total": len(filtered)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"targets": allTargets, "total": len(allTargets)})
}

func (h *Handler) handleListActiveModels(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	active := append([]string(nil), cfg.ActiveModels...)
	writeJSON(w, http.StatusOK, map[string]any{"active_models": active, "total": len(active)})
}

func (h *Handler) handleSetActiveModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Models []string `json:"models"`
	}
	if err := decodeStrictJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg.ActiveModels = req.Models
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active_models": cfg.ActiveModels, "status": "ok"})
}

func (h *Handler) handleAddActiveModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := decodeStrictJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		http.Error(w, "target is required", http.StatusBadRequest)
		return
	}
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, m := range cfg.ActiveModels {
		if m == target {
			writeJSON(w, http.StatusOK, map[string]any{"active_models": cfg.ActiveModels, "status": "ok"})
			return
		}
	}
	cfg.ActiveModels = append(cfg.ActiveModels, target)
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active_models": cfg.ActiveModels, "status": "ok"})
}

func (h *Handler) handleRemoveActiveModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := decodeStrictJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	target := strings.TrimSpace(req.Target)
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated := make([]string, 0, len(cfg.ActiveModels))
	for _, m := range cfg.ActiveModels {
		if m != target {
			updated = append(updated, m)
		}
	}
	cfg.ActiveModels = updated
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"active_models": cfg.ActiveModels, "status": "ok"})
}

func (h *Handler) loadProviderProjectionState() (*config.Config, *CatalogStore, error) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to load config: %w", err)
	}
	store, err := loadCatalogs()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to load catalogs: %w", err)
	}
	return cfg, store, nil
}

func instanceCatalogResponses(cfg *config.Config, store *CatalogStore) []providerInstanceCatalogResponse {
	instances := providerInstancesByID(cfg)
	responses := make([]providerInstanceCatalogResponse, 0)
	for key, entry := range store.Entries {
		instance := instances[key]
		if !validInstanceCatalog(key, entry, instance) {
			continue
		}
		models := append([]CatalogModel(nil), entry.Models...)
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
		responses = append(responses, providerInstanceCatalogResponse{
			InstanceID:   instance.ID,
			ProviderKind: instance.ProviderKind,
			Models:       models,
			FetchedAt:    entry.FetchedAt,
		})
	}
	sort.Slice(responses, func(i, j int) bool { return responses[i].InstanceID < responses[j].InstanceID })
	return responses
}

func providerTargetResponses(cfg *config.Config, store *CatalogStore) []providerTargetResponse {
	instances := providerInstancesByID(cfg)
	targets := make([]providerTargetResponse, 0)
	for key, entry := range store.Entries {
		instance := instances[key]
		if instance == nil || instance.State != config.ProviderInstanceStateEnabled || !validInstanceCatalog(key, entry, instance) {
			continue
		}
		for _, model := range entry.Models {
			modelID := strings.TrimSpace(model.ID)
			if modelID == "" {
				continue
			}
			raw := instance.ID + "/" + modelID
			if _, err := config.ParseExactModelTarget(raw); err != nil {
				continue
			}
			targets = append(targets, providerTargetResponse{
				Target: raw, InstanceID: instance.ID, ModelID: modelID,
				ProviderKind: instance.ProviderKind, OwnedBy: model.OwnedBy, FetchedAt: entry.FetchedAt,
			})
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Target < targets[j].Target })
	return targets
}

func providerInstancesByID(cfg *config.Config) map[string]*config.ProviderInstanceConfig {
	instances := make(map[string]*config.ProviderInstanceConfig, len(cfg.ProviderInstances))
	for _, instance := range cfg.ProviderInstances {
		if instance != nil {
			instances[instance.ID] = instance
		}
	}
	return instances
}

func validInstanceCatalog(key string, entry *CatalogEntry, instance *config.ProviderInstanceConfig) bool {
	return entry != nil && instance != nil && key == instance.ID && entry.ID == instance.ID &&
		entry.InstanceID == instance.ID && entry.Provider == instance.ProviderKind
}

func (h *Handler) handleListProviderRoutes(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	routes := cloneSortedRoutes(cfg.ModelRoutes)
	writeJSON(w, http.StatusOK, map[string]any{"routes": routes, "total": len(routes)})
}

func (h *Handler) handleCreateProviderRoute(w http.ResponseWriter, r *http.Request) {
	var route config.ModelRouteConfig
	if err := decodeStrictJSON(r, &route); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, store, err := h.loadProviderProjectionState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if providerRouteIndex(cfg.ModelRoutes, route.Name) >= 0 {
		http.Error(w, "model route already exists", http.StatusConflict)
		return
	}
	if err := validateProviderRouteAgainstTargets(cfg, store, &route); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	cfg.ModelRoutes = append(cfg.ModelRoutes, &route)
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "route": &route})
}

func (h *Handler) handleUpdateProviderRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var route config.ModelRouteConfig
	if err := decodeStrictJSON(r, &route); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if route.Name != name {
		http.Error(w, "model route name is immutable and must match the request path", http.StatusBadRequest)
		return
	}
	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, store, err := h.loadProviderProjectionState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	index := providerRouteIndex(cfg.ModelRoutes, name)
	if index < 0 {
		http.Error(w, "model route not found", http.StatusNotFound)
		return
	}
	if err := validateProviderRouteAgainstTargets(cfg, store, &route); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	cfg.ModelRoutes[index] = &route
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "route": &route})
}

func (h *Handler) handleDeleteProviderRoute(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	index := providerRouteIndex(cfg.ModelRoutes, name)
	if index < 0 {
		http.Error(w, "model route not found", http.StatusNotFound)
		return
	}
	cfg.ModelRoutes = append(cfg.ModelRoutes[:index], cfg.ModelRoutes[index+1:]...)
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func validateProviderRouteAgainstTargets(cfg *config.Config, store *CatalogStore, route *config.ModelRouteConfig) error {
	clone := *cfg
	clone.ModelRoutes = []*config.ModelRouteConfig{route}
	if err := clone.ValidateProviderInstances(); err != nil {
		return err
	}
	validTargets := make(map[string]struct{})
	for _, target := range providerTargetResponses(cfg, store) {
		validTargets[target.Target] = struct{}{}
	}
	for _, target := range route.Targets {
		if _, ok := validTargets[target]; !ok {
			return fmt.Errorf("target %q is not present in an enabled instance-owned catalog", target)
		}
	}
	return nil
}

func providerRouteIndex(routes []*config.ModelRouteConfig, name string) int {
	for i, route := range routes {
		if route != nil && route.Name == name {
			return i
		}
	}
	return -1
}

func cloneSortedRoutes(routes []*config.ModelRouteConfig) []*config.ModelRouteConfig {
	clones := make([]*config.ModelRouteConfig, 0, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		clone := *route
		clone.Targets = append([]string(nil), route.Targets...)
		clones = append(clones, &clone)
	}
	sort.Slice(clones, func(i, j int) bool { return clones[i].Name < clones[j].Name })
	return clones
}
