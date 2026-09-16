package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	llmgwproviders "github.com/xibodev/llmgw-core/providers"
	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/providers"
)

// ProviderCatalogSyncInput is the complete server-owned connection description
// supplied to a catalog driver. It is never populated from request fields.
type ProviderCatalogSyncInput struct {
	InstanceID        string
	ProviderKind      string
	Adapter           string
	Protocol          string
	Endpoint          string
	AuthConnectionRef string
	Headers           map[string]string
	Settings          map[string]any
	secret            string
}

type providerInstanceResponse struct {
	ID             string                       `json:"id"`
	ProviderKind   string                       `json:"provider_kind"`
	Adapter        string                       `json:"adapter"`
	Protocol       string                       `json:"protocol"`
	Endpoint       string                       `json:"endpoint,omitempty"`
	AuthConfigured bool                         `json:"auth_configured"`
	HeaderNames    []string                     `json:"header_names"`
	SettingNames   []string                     `json:"setting_names"`
	State          config.ProviderInstanceState `json:"state"`
}

type providerInstanceWriteOnly struct {
	AuthConnectionRef *string            `json:"auth_connection_ref,omitempty"`
	Headers           *map[string]string `json:"headers,omitempty"`
	Settings          *map[string]any    `json:"settings,omitempty"`
}

type providerInstanceRequestBody struct {
	ID            string                       `json:"id"`
	ProviderKind  string                       `json:"provider_kind"`
	Adapter       string                       `json:"adapter"`
	Protocol      string                       `json:"protocol"`
	Endpoint      string                       `json:"endpoint,omitempty"`
	State         config.ProviderInstanceState `json:"state"`
	WriteOnly     *providerInstanceWriteOnly   `json:"write_only,omitempty"`
	ClearAuth     bool                         `json:"clear_auth_connection,omitempty"`
	ClearHeaders  bool                         `json:"clear_headers,omitempty"`
	ClearSettings bool                         `json:"clear_settings,omitempty"`
}

func (h *Handler) registerProviderInstanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/provider-instances", h.handleListProviderInstances)
	mux.HandleFunc("POST /api/provider-instances", h.handleCreateProviderInstance)
	mux.HandleFunc("POST /api/provider-instances/auto-connect-free", h.handleAutoConnectFreeProviders)
	mux.HandleFunc("PUT /api/provider-instances/{id}", h.handleUpdateProviderInstance)
	mux.HandleFunc("DELETE /api/provider-instances/{id}", h.handleDeleteProviderInstance)
	mux.HandleFunc("POST /api/provider-instances/{id}/catalog/sync", h.handleSyncProviderInstanceCatalog)
	mux.HandleFunc("POST /api/provider-instances/{id}/ping", h.handlePingProviderInstance)
	mux.HandleFunc("GET /api/provider-instances/catalogs", h.handleListProviderInstanceCatalogs)
	mux.HandleFunc("GET /api/provider-targets", h.handleListProviderTargets)
	mux.HandleFunc("GET /api/provider-roster", h.handleListProviderRoster)
	h.registerProviderRouteRoutes(mux)
}

func decodeStrictJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request must contain one JSON value")
		}
		return err
	}
	return nil
}

func (h *Handler) handleListProviderInstances(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	instances := make([]providerInstanceResponse, 0, len(cfg.ProviderInstances))
	for _, instance := range cfg.ProviderInstances {
		if instance != nil {
			instances = append(instances, safeProviderInstanceResponse(instance))
		}
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].ID < instances[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"instances": instances, "total": len(instances)})
}

func (h *Handler) handleCreateProviderInstance(w http.ResponseWriter, r *http.Request) {
	var request providerInstanceRequestBody
	if err := decodeStrictJSON(r, &request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	instance, err := providerInstanceFromCreateRequest(request)
	if err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	for _, existing := range cfg.ProviderInstances {
		if existing != nil && existing.ID == instance.ID {
			http.Error(w, "provider instance already exists", http.StatusConflict)
			return
		}
	}
	cfg.ProviderInstances = append(cfg.ProviderInstances, instance)
	if err := cfg.ValidateProviderInstances(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "instance": safeProviderInstanceResponse(instance)})
}

func (h *Handler) handleUpdateProviderInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var request providerInstanceRequestBody
	if err := decodeStrictJSON(r, &request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if request.ID != id {
		http.Error(w, "provider instance id is immutable and must match the request path", http.StatusBadRequest)
		return
	}
	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	index := providerInstanceIndex(cfg, id)
	if index < 0 {
		http.Error(w, "provider instance not found", http.StatusNotFound)
		return
	}
	instance, err := providerInstanceFromUpdateRequest(request, cfg.ProviderInstances[index])
	if err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	if instance.State == config.ProviderInstanceStateDisabled {
		if routes := routesReferencingInstance(cfg.ModelRoutes, id); len(routes) > 0 {
			http.Error(w, fmt.Sprintf("provider instance %q is referenced by routes: %s", id, strings.Join(routes, ", ")), http.StatusConflict)
			return
		}
	}
	cfg.ProviderInstances[index] = instance
	if err := cfg.ValidateProviderInstances(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "instance": safeProviderInstanceResponse(instance)})
}

func (h *Handler) handleDeleteProviderInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.configMu.Lock()
	defer h.configMu.Unlock()
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	index := providerInstanceIndex(cfg, id)
	if index < 0 {
		http.Error(w, "provider instance not found", http.StatusNotFound)
		return
	}
	if routes := routesReferencingInstance(cfg.ModelRoutes, id); len(routes) > 0 {
		http.Error(w, fmt.Sprintf("provider instance %q is referenced by routes: %s", id, strings.Join(routes, ", ")), http.StatusConflict)
		return
	}
	cfg.ProviderInstances = append(cfg.ProviderInstances[:index], cfg.ProviderInstances[index+1:]...)
	if err := cfg.ValidateProviderInstances(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}
	if err := config.SaveConfig(h.configPath, cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
		return
	}
	if err := deleteProviderInstanceCatalog(id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete provider catalog: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSyncProviderInstanceCatalog(w http.ResponseWriter, r *http.Request) {
	var request struct{}
	if err := decodeStrictJSON(r, &request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	index := providerInstanceIndex(cfg, r.PathValue("id"))
	if index < 0 {
		http.Error(w, "provider instance not found", http.StatusNotFound)
		return
	}
	instance := cfg.ProviderInstances[index]
	if instance.State == config.ProviderInstanceStateDisabled {
		http.Error(w, "provider instance is disabled", http.StatusConflict)
		return
	}
	if !catalogSyncAdapterSupported(instance.Adapter) {
		http.Error(w, "provider instance adapter does not support catalog sync", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(instance.Endpoint) == "" && instance.Adapter != "github-copilot-native" {
		http.Error(w, "provider instance endpoint is required for catalog sync", http.StatusBadRequest)
		return
	}
	if h.providerCatalogSync == nil {
		http.Error(w, "provider catalog sync driver is not configured", http.StatusNotImplemented)
		return
	}

	input := catalogSyncInputFromInstance(instance)
	if instance.Adapter != "github-copilot-native" {
		if h.providerCredentialResolver == nil {
			http.Error(w, "provider credential resolver is not configured", http.StatusInternalServerError)
			return
		}
		input.secret, err = h.providerCredentialResolver(instance.AuthConnectionRef)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to resolve provider credential: %v", err), http.StatusBadGateway)
			return
		}
	}
	models, err := h.providerCatalogSync(r.Context(), input)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to sync provider catalog: %v", err), http.StatusBadGateway)
		return
	}
	if err := saveProviderInstanceCatalog(instance, models); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save provider catalog: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"instance_id": instance.ID, "models": models, "total": len(models)})
}

func (h *Handler) handlePingProviderInstance(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}
	index := providerInstanceIndex(cfg, r.PathValue("id"))
	if index < 0 {
		http.Error(w, "provider instance not found", http.StatusNotFound)
		return
	}
	instance := cfg.ProviderInstances[index]

	start := time.Now()

	// If it supports catalog sync, use providerCatalogSync to test credentials + reachability
	if catalogSyncAdapterSupported(instance.Adapter) && h.providerCatalogSync != nil {
		input := catalogSyncInputFromInstance(instance)
		if instance.Adapter != "github-copilot-native" && h.providerCredentialResolver != nil && instance.AuthConnectionRef != "" {
			input.secret, _ = h.providerCredentialResolver(instance.AuthConnectionRef)
		}
		models, syncErr := h.providerCatalogSync(r.Context(), input)
		latencyMs := time.Since(start).Milliseconds()
		if syncErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":          false,
				"instance_id": instance.ID,
				"latency_ms":  latencyMs,
				"status":      "unreachable",
				"error":       syncErr.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"instance_id": instance.ID,
			"latency_ms":  latencyMs,
			"model_count": len(models),
			"status":      "reachable",
		})
		return
	}

	// For general HTTP endpoints
	if strings.TrimSpace(instance.Endpoint) != "" {
		req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, instance.Endpoint, nil)
		if reqErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":          false,
				"instance_id": instance.ID,
				"latency_ms":  0,
				"status":      "unreachable",
				"error":       reqErr.Error(),
			})
			return
		}
		client := h.providerCatalogHTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		resp, respErr := client.Do(req)
		latencyMs := time.Since(start).Milliseconds()
		if respErr != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":          false,
				"instance_id": instance.ID,
				"latency_ms":  latencyMs,
				"status":      "unreachable",
				"error":       respErr.Error(),
			})
			return
		}
		_ = resp.Body.Close()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"instance_id": instance.ID,
			"latency_ms":  latencyMs,
			"status":      "reachable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"instance_id": instance.ID,
		"latency_ms":  time.Since(start).Milliseconds(),
		"status":      "configured",
	})
}

func (h *Handler) handleAutoConnectFreeProviders(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.LoadConfig(h.configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusInternalServerError)
		return
	}

	profiles := llmgwproviders.AnonymousProviderProfiles()
	existingIDs := make(map[string]bool)
	for _, inst := range cfg.ProviderInstances {
		if inst != nil {
			existingIDs[inst.ID] = true
		}
	}

	connected := 0
	verified := 0
	var connectedInstances []string

	for _, profile := range profiles {
		instID := profile.ProviderID
		if instID == "" {
			instID = profile.RegistryID
		}
		if !existingIDs[instID] {
			newInstance := &config.ProviderInstanceConfig{
				ID:           instID,
				ProviderKind: profile.RegistryID,
				Adapter:      "openai-compatible",
				Protocol:     "openai",
				Endpoint:     profile.BaseURL,
				State:        config.ProviderInstanceStateEnabled,
			}
			cfg.ProviderInstances = append(cfg.ProviderInstances, newInstance)
			existingIDs[instID] = true
			connected++
			connectedInstances = append(connectedInstances, instID)
		}

		if h.providerCatalogSync != nil {
			idx := providerInstanceIndex(cfg, instID)
			if idx >= 0 {
				input := catalogSyncInputFromInstance(cfg.ProviderInstances[idx])
				models, syncErr := h.providerCatalogSync(r.Context(), input)
				if syncErr == nil && len(models) > 0 {
					freeModels := filterAnonymousFreeModels(profile.RegistryID, models)
					_ = saveProviderInstanceCatalog(cfg.ProviderInstances[idx], freeModels)
					verified++
					for _, m := range freeModels {
						exact := instID + "/" + m.ID
						alreadyActive := false
						for _, existing := range cfg.ActiveModels {
							if existing == exact {
								alreadyActive = true
								break
							}
						}
						if !alreadyActive {
							cfg.ActiveModels = append(cfg.ActiveModels, exact)
						}
					}
				}
			}
		}
	}

	if connected > 0 {
		if err := config.SaveConfig(h.configPath, cfg); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"total":     len(profiles),
		"connected": connected,
		"verified":  verified,
		"instances": connectedInstances,
	})
}

var anonymousFreeModelIDs = map[string][]string{
	"opencode_zen":     {"ling-3.0-flash-fin-free", "muse-spark-1.2-contributor-free", "nemotron-3.5-lightning-free"},
	"kilo_code":        {"kilo-auto/free", "liquid/lfm-2.5-2.6b:free", "cohere/north-mini-code:free"},
	"llm7":             {"codestral-latest", "mistral-Nemo-Instruct-2407", "minimax-m2.7"},
	"ovh_ai_endpoints": {"Qwen3.8-27B", "Mistral-Nemo-Instruct-2407", "gpt-oss-20b"},
	"pollinations":     {"openai-fast"},
}

func filterAnonymousFreeModels(providerKind string, models []CatalogModel) []CatalogModel {
	preferred, hasPreferred := anonymousFreeModelIDs[providerKind]
	filtered := make([]CatalogModel, 0)
	for _, m := range models {
		idLower := strings.ToLower(m.ID)
		if strings.HasSuffix(idLower, "-free") || strings.HasSuffix(idLower, "/free") || strings.HasSuffix(idLower, ":free") {
			filtered = append(filtered, m)
			continue
		}
		if hasPreferred {
			for _, pref := range preferred {
				if strings.EqualFold(m.ID, pref) {
					filtered = append(filtered, m)
					break
				}
			}
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	if hasPreferred {
		for _, pref := range preferred {
			filtered = append(filtered, CatalogModel{ID: pref})
		}
		return filtered
	}
	return models
}

func resolveProviderCredentialReference(ref string) (string, error) {
	kind, key, found := strings.Cut(strings.TrimSpace(ref), ":")
	if !found || kind != "credential" || strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("auth_connection_ref must use credential:<store-key>")
	}
	credential, err := auth.GetCredential(strings.TrimSpace(key))
	if err != nil {
		return "", fmt.Errorf("load credential reference: %w", err)
	}
	if credential == nil || strings.TrimSpace(credential.AccessToken) == "" {
		return "", fmt.Errorf("credential reference not found")
	}
	if credential.IsExpired() {
		return "", fmt.Errorf("credential reference is expired")
	}
	return credential.AccessToken, nil
}

func providerInstanceIndex(cfg *config.Config, id string) int {
	for i, instance := range cfg.ProviderInstances {
		if instance != nil && instance.ID == id {
			return i
		}
	}
	return -1
}

func catalogSyncAdapterSupported(adapter string) bool {
	switch strings.ToLower(strings.TrimSpace(adapter)) {
	case "openai-compatible", "anthropic-compatible", "github-copilot-native":
		return true
	default:
		return false
	}
}

func catalogSyncInputFromInstance(instance *config.ProviderInstanceConfig) ProviderCatalogSyncInput {
	headers := make(map[string]string, len(instance.Headers))
	for name, value := range instance.Headers {
		headers[name] = value
	}
	settings := make(map[string]any, len(instance.Settings))
	for name, value := range instance.Settings {
		settings[name] = value
	}
	return ProviderCatalogSyncInput{
		InstanceID:        instance.ID,
		ProviderKind:      instance.ProviderKind,
		Adapter:           instance.Adapter,
		Protocol:          instance.Protocol,
		Endpoint:          instance.Endpoint,
		AuthConnectionRef: instance.AuthConnectionRef,
		Headers:           headers,
		Settings:          settings,
	}
}

func safeProviderInstanceResponse(instance *config.ProviderInstanceConfig) providerInstanceResponse {
	headerNames := make([]string, 0, len(instance.Headers))
	for name := range instance.Headers {
		headerNames = append(headerNames, name)
	}
	settingNames := make([]string, 0, len(instance.Settings))
	for name := range instance.Settings {
		settingNames = append(settingNames, name)
	}
	sort.Strings(headerNames)
	sort.Strings(settingNames)
	return providerInstanceResponse{
		ID:             instance.ID,
		ProviderKind:   instance.ProviderKind,
		Adapter:        instance.Adapter,
		Protocol:       instance.Protocol,
		Endpoint:       instance.Endpoint,
		AuthConfigured: strings.TrimSpace(instance.AuthConnectionRef) != "",
		HeaderNames:    headerNames,
		SettingNames:   settingNames,
		State:          instance.State,
	}
}

func routesReferencingInstance(routes []*config.ModelRouteConfig, instanceID string) []string {
	references := make([]string, 0)
	for _, route := range routes {
		if route == nil {
			continue
		}
		for _, raw := range route.Targets {
			target, err := config.ParseExactModelTarget(raw)
			if err == nil && target.InstanceID == instanceID {
				references = append(references, route.Name)
				break
			}
		}
	}
	sort.Strings(references)
	return references
}

func providerInstanceFromCreateRequest(request providerInstanceRequestBody) (*config.ProviderInstanceConfig, error) {
	if request.ClearAuth || request.ClearHeaders || request.ClearSettings {
		return nil, fmt.Errorf("clear fields are only valid when updating an existing provider instance")
	}
	instance := publicProviderInstanceFromRequest(request)
	if request.WriteOnly != nil {
		if request.WriteOnly.AuthConnectionRef != nil {
			instance.AuthConnectionRef = *request.WriteOnly.AuthConnectionRef
		}
		if request.WriteOnly.Headers != nil {
			instance.Headers = cloneStringValues(*request.WriteOnly.Headers)
		}
		if request.WriteOnly.Settings != nil {
			instance.Settings = cloneAnyValues(*request.WriteOnly.Settings)
		}
	}
	if err := validateProviderInstanceForAPI(instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func providerInstanceFromUpdateRequest(request providerInstanceRequestBody, existing *config.ProviderInstanceConfig) (*config.ProviderInstanceConfig, error) {
	instance := publicProviderInstanceFromRequest(request)
	instance.AuthConnectionRef = existing.AuthConnectionRef
	instance.Headers = cloneStringValues(existing.Headers)
	instance.Settings = cloneAnyValues(existing.Settings)
	if request.WriteOnly != nil {
		if request.ClearAuth && request.WriteOnly.AuthConnectionRef != nil ||
			request.ClearHeaders && request.WriteOnly.Headers != nil ||
			request.ClearSettings && request.WriteOnly.Settings != nil {
			return nil, fmt.Errorf("a sensitive field cannot be replaced and cleared in the same request")
		}
		if request.WriteOnly.AuthConnectionRef != nil {
			instance.AuthConnectionRef = *request.WriteOnly.AuthConnectionRef
		}
		if request.WriteOnly.Headers != nil {
			instance.Headers = cloneStringValues(*request.WriteOnly.Headers)
		}
		if request.WriteOnly.Settings != nil {
			instance.Settings = cloneAnyValues(*request.WriteOnly.Settings)
		}
	}
	if request.ClearAuth {
		instance.AuthConnectionRef = ""
	}
	if request.ClearHeaders {
		instance.Headers = nil
	}
	if request.ClearSettings {
		instance.Settings = nil
	}
	if err := validateProviderInstanceForAPI(instance); err != nil {
		return nil, err
	}
	return instance, nil
}

func publicProviderInstanceFromRequest(request providerInstanceRequestBody) *config.ProviderInstanceConfig {
	return &config.ProviderInstanceConfig{
		ID: request.ID, ProviderKind: request.ProviderKind, Adapter: request.Adapter,
		Protocol: request.Protocol, Endpoint: request.Endpoint, State: request.State,
	}
}

func validateProviderInstanceForAPI(instance *config.ProviderInstanceConfig) error {
	if err := instance.Validate(); err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(instance.Adapter), "github-copilot-native") {
		if providers.NormalizeProvider(instance.ProviderKind) != "github-copilot" ||
			providers.NormalizeProvider(instance.Protocol) != "github-copilot" {
			return fmt.Errorf("github-copilot-native requires github-copilot provider_kind and protocol")
		}
		if strings.TrimSpace(instance.Endpoint) != "" {
			return fmt.Errorf("github-copilot-native does not accept an endpoint")
		}
		if strings.TrimSpace(instance.AuthConnectionRef) != "" {
			return fmt.Errorf("github-copilot-native does not accept an auth connection reference")
		}
		if len(instance.Headers) != 0 {
			return fmt.Errorf("github-copilot-native does not accept headers")
		}
		return nil
	}
	endpoint := strings.TrimSpace(instance.Endpoint)
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("endpoint must be an absolute URL")
	}
	if parsed.User != nil {
		return fmt.Errorf("endpoint must not contain URL userinfo")
	}
	if parsed.RawQuery != "" || parsed.ForceQuery {
		return fmt.Errorf("endpoint must not contain a query string")
	}
	return nil
}

func cloneStringValues(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneAnyValues(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
