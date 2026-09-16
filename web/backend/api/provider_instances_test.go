package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/config"
)

func providerInstanceFixture(id, endpoint string) *config.ProviderInstanceConfig {
	return &config.ProviderInstanceConfig{
		ID:                id,
		ProviderKind:      "openai",
		Adapter:           "openai-compatible",
		Protocol:          "openai",
		Endpoint:          endpoint,
		AuthConnectionRef: "credential:" + id,
		Headers:           map[string]string{"X-Instance": id},
		Settings:          map[string]any{"tenant": id},
		State:             config.ProviderInstanceStateEnabled,
	}
}

func providerInstanceTestHandler(t *testing.T, instances ...*config.ProviderInstanceConfig) (*Handler, *http.ServeMux, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FACET_STUDIO_HOME", home)
	configPath := filepath.Join(home, "config.json")
	cfg := config.DefaultConfig()
	cfg.ProviderInstances = instances
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, mux, configPath
}

func providerInstanceRequest(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)
	return recorder
}

func providerInstanceAdminBody(t *testing.T, instance *config.ProviderInstanceConfig, includeSensitive bool) string {
	t.Helper()
	request := providerInstanceRequestBody{
		ID: instance.ID, ProviderKind: instance.ProviderKind, Adapter: instance.Adapter,
		Protocol: instance.Protocol, Endpoint: instance.Endpoint, State: instance.State,
	}
	if includeSensitive {
		authRef := instance.AuthConnectionRef
		headers := cloneStringValues(instance.Headers)
		settings := cloneAnyValues(instance.Settings)
		request.WriteOnly = &providerInstanceWriteOnly{
			AuthConnectionRef: &authRef, Headers: &headers, Settings: &settings,
		}
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return string(body)
}

func TestProviderInstanceCRUD(t *testing.T) {
	_, mux, configPath := providerInstanceTestHandler(t)
	created := providerInstanceFixture("openai-main", "https://one.example.test/v1")
	body := providerInstanceAdminBody(t, created, true)

	recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances", string(body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(cfg.ProviderInstances) != 1 || cfg.ProviderInstances[0].ID != "openai-main" {
		t.Fatalf("provider instances = %#v", cfg.ProviderInstances)
	}

	recorder = providerInstanceRequest(t, mux, http.MethodGet, "/api/provider-instances", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"openai-main"`) {
		t.Fatalf("list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	created.Endpoint = "https://updated.example.test/v1"
	created.State = config.ProviderInstanceStateDisabled
	body = providerInstanceAdminBody(t, created, false)
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/provider-instances/openai-main", string(body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() after update error = %v", err)
	}
	if cfg.ProviderInstances[0].Endpoint != created.Endpoint || cfg.ProviderInstances[0].State != config.ProviderInstanceStateDisabled {
		t.Fatalf("updated instance = %#v", cfg.ProviderInstances[0])
	}

	recorder = providerInstanceRequest(t, mux, http.MethodDelete, "/api/provider-instances/openai-main", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() after delete error = %v", err)
	}
	if len(cfg.ProviderInstances) != 0 {
		t.Fatalf("provider instances after delete = %#v", cfg.ProviderInstances)
	}
}

func TestProviderInstanceCRUDRejectsIdentityChangesAndDuplicates(t *testing.T) {
	_, mux, _ := providerInstanceTestHandler(t, providerInstanceFixture("existing", "https://existing.example.test/v1"))
	duplicate := providerInstanceAdminBody(t, providerInstanceFixture("existing", "https://other.example.test/v1"), true)
	recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances", duplicate)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	renamed := providerInstanceAdminBody(t, providerInstanceFixture("renamed", "https://other.example.test/v1"), false)
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/provider-instances/existing", renamed)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "immutable") {
		t.Fatalf("rename status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderInstanceCatalogSyncUsesPersistedInstanceOwnership(t *testing.T) {
	first := providerInstanceFixture("first", "https://first.example.test/v1")
	second := providerInstanceFixture("second", "https://second.example.test/v1")
	h, mux, _ := providerInstanceTestHandler(t, first, second)
	var inputs []ProviderCatalogSyncInput
	h.providerCredentialResolver = func(ref string) (string, error) {
		return "fixture-secret", nil
	}
	h.providerCatalogSync = func(_ context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error) {
		inputs = append(inputs, input)
		return []CatalogModel{{ID: input.InstanceID + "-model"}}, nil
	}

	for _, id := range []string{"first", "second"} {
		recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/"+id+"/catalog/sync", `{}`)
		if recorder.Code != http.StatusOK {
			t.Fatalf("sync %s status = %d, body = %s", id, recorder.Code, recorder.Body.String())
		}
	}
	if len(inputs) != 2 {
		t.Fatalf("sync inputs = %#v", inputs)
	}
	if inputs[0].Endpoint != first.Endpoint || inputs[0].AuthConnectionRef != first.AuthConnectionRef || inputs[0].Headers["X-Instance"] != "first" || inputs[0].Settings["tenant"] != "first" {
		t.Fatalf("first sync input = %#v", inputs[0])
	}
	if inputs[1].Endpoint != second.Endpoint || inputs[1].AuthConnectionRef != second.AuthConnectionRef {
		t.Fatalf("second sync input = %#v", inputs[1])
	}

	store, err := loadCatalogs()
	if err != nil {
		t.Fatalf("loadCatalogs() error = %v", err)
	}
	if len(store.Entries) != 2 || store.Entries["first"].Models[0].ID != "first-model" || store.Entries["second"].Models[0].ID != "second-model" {
		t.Fatalf("instance-owned catalogs = %#v", store.Entries)
	}
}

func TestProviderInstanceCatalogSyncRejectsBrowserOverrides(t *testing.T) {
	h, mux, _ := providerInstanceTestHandler(t, providerInstanceFixture("owned", "https://owned.example.test/v1"))
	called := false
	h.providerCatalogSync = func(context.Context, ProviderCatalogSyncInput) ([]CatalogModel, error) {
		called = true
		return nil, nil
	}

	for _, body := range []string{
		`{"endpoint":"http://localhost:11434/v1"}`,
		`{"api_key":"browser-secret"}`,
		`{"provider":"ollama"}`,
	} {
		recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/owned/catalog/sync", body)
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "unknown field") {
			t.Fatalf("override %s status = %d, body = %s", body, recorder.Code, recorder.Body.String())
		}
	}
	if called {
		t.Fatal("catalog sync driver called for browser-provided overrides")
	}
}

func TestProviderInstanceCatalogSyncRefusesDisabledAndNonCompatibleInstances(t *testing.T) {
	disabled := providerInstanceFixture("disabled", "https://disabled.example.test/v1")
	disabled.State = config.ProviderInstanceStateDisabled
	copilot := providerInstanceFixture("copilot", "")
	copilot.ProviderKind = "github-copilot"
	copilot.Adapter = "unsupported-copilot-adapter"
	copilot.Protocol = "copilot"
	missingEndpoint := providerInstanceFixture("missing-endpoint", "")
	h, mux, _ := providerInstanceTestHandler(t, disabled, copilot, missingEndpoint)
	called := false
	h.providerCatalogSync = func(context.Context, ProviderCatalogSyncInput) ([]CatalogModel, error) {
		called = true
		return nil, nil
	}

	tests := []struct {
		id       string
		status   int
		contains string
	}{
		{id: "disabled", status: http.StatusConflict, contains: "disabled"},
		{id: "copilot", status: http.StatusBadRequest, contains: "does not support"},
		{id: "missing-endpoint", status: http.StatusBadRequest, contains: "endpoint is required"},
	}
	for _, tc := range tests {
		recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/"+tc.id+"/catalog/sync", `{}`)
		if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.contains) {
			t.Fatalf("sync %s status = %d, body = %s", tc.id, recorder.Code, recorder.Body.String())
		}
	}
	if called {
		t.Fatal("catalog sync driver called for disabled, Copilot, or endpoint-less instance")
	}
}

func TestCompatibleProviderCatalogSyncUsesResolvedCredentialAndPersistedConnection(t *testing.T) {
	tests := []struct {
		name             string
		adapter          string
		settings         map[string]any
		response         string
		wantPath         string
		wantAuthHeader   string
		wantAPIKeyHeader string
		wantVersion      string
		wantModel        CatalogModel
	}{
		{
			name:           "openai envelope",
			adapter:        "openai-compatible",
			settings:       map[string]any{"catalog_path": "/v1/models"},
			response:       `{"data":[{"id":"gpt-fixture","owned_by":"fixture"}]}`,
			wantPath:       "/v1/models",
			wantAuthHeader: "Bearer resolved-secret",
			wantModel:      CatalogModel{ID: "gpt-fixture", OwnedBy: "fixture"},
		},
		{
			name:             "anthropic bare list",
			adapter:          "anthropic-compatible",
			settings:         map[string]any{"catalog_path": "/models", "anthropic_version": "2024-01-01"},
			response:         `[{"id":"claude-fixture","extra":{"tier":"test"}}]`,
			wantPath:         "/models",
			wantAPIKeyHeader: "resolved-secret",
			wantVersion:      "2024-01-01",
			wantModel:        CatalogModel{ID: "claude-fixture", Extra: map[string]any{"tier": "test"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instance := providerInstanceFixture("owned", "https://owned.example.test/base")
			instance.Adapter = tc.adapter
			instance.Settings = tc.settings
			instance.Headers = map[string]string{"X-Persisted": "yes"}
			h, mux, _ := providerInstanceTestHandler(t, instance)
			var requests int
			h.providerCredentialResolver = func(ref string) (string, error) {
				if ref != "credential:owned" {
					t.Fatalf("credential reference = %q", ref)
				}
				return "resolved-secret", nil
			}
			h.providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requests++
				if request.URL.String() != "https://owned.example.test/base"+tc.wantPath {
					t.Fatalf("request URL = %q", request.URL.String())
				}
				if request.Header.Get("X-Persisted") != "yes" {
					t.Fatalf("persisted header = %q", request.Header.Get("X-Persisted"))
				}
				if request.Header.Get("Authorization") != tc.wantAuthHeader {
					t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
				}
				if request.Header.Get("X-Api-Key") != tc.wantAPIKeyHeader {
					t.Fatalf("X-Api-Key = %q", request.Header.Get("X-Api-Key"))
				}
				if request.Header.Get("Anthropic-Version") != tc.wantVersion {
					t.Fatalf("Anthropic-Version = %q", request.Header.Get("Anthropic-Version"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.response)),
					Request:    request,
				}, nil
			})}

			recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/owned/catalog/sync", `{}`)
			if recorder.Code != http.StatusOK {
				t.Fatalf("sync status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
			store, err := loadCatalogs()
			if err != nil {
				t.Fatalf("loadCatalogs() error = %v", err)
			}
			entry := store.Entries["owned"]
			if entry == nil || entry.InstanceID != "owned" || len(entry.Models) != 1 {
				t.Fatalf("catalog entry = %#v", entry)
			}
			got := entry.Models[0]
			if got.ID != tc.wantModel.ID || got.OwnedBy != tc.wantModel.OwnedBy {
				t.Fatalf("catalog model = %#v, want %#v", got, tc.wantModel)
			}
			if tc.wantModel.Extra != nil && got.Extra["tier"] != tc.wantModel.Extra["tier"] {
				t.Fatalf("catalog model extra = %#v", got.Extra)
			}
		})
	}
}

func TestProviderInstanceCatalogSyncResolverFailureMakesNoRequest(t *testing.T) {
	h, mux, _ := providerInstanceTestHandler(t, providerInstanceFixture("owned", "https://owned.example.test/v1"))
	h.providerCredentialResolver = func(ref string) (string, error) {
		if ref != "credential:owned" {
			t.Fatalf("credential reference = %q", ref)
		}
		return "", errors.New("fixture resolver unavailable")
	}
	requests := 0
	h.providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("unexpected request")
	})}

	recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/owned/catalog/sync", `{}`)
	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "fixture resolver unavailable") {
		t.Fatalf("sync status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestProviderInstanceCatalogSyncSameKindHTTPIsolation(t *testing.T) {
	first := providerInstanceFixture("first", "https://first.example.test/v1")
	second := providerInstanceFixture("second", "https://second.example.test/v1")
	h, mux, _ := providerInstanceTestHandler(t, first, second)
	h.providerCredentialResolver = func(ref string) (string, error) {
		return strings.TrimPrefix(ref, "credential:") + "-secret", nil
	}
	h.providerCatalogHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		host := request.URL.Hostname()
		id := strings.TrimSuffix(host, ".example.test")
		if request.Header.Get("Authorization") != "Bearer "+id+"-secret" {
			t.Fatalf("%s Authorization = %q", id, request.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"` + id + `-model"}]}`)),
			Request:    request,
		}, nil
	})}

	for _, id := range []string{"first", "second"} {
		recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/"+id+"/catalog/sync", `{}`)
		if recorder.Code != http.StatusOK {
			t.Fatalf("sync %s status = %d, body = %s", id, recorder.Code, recorder.Body.String())
		}
	}
	store, err := loadCatalogs()
	if err != nil {
		t.Fatalf("loadCatalogs() error = %v", err)
	}
	if store.Entries["first"].Models[0].ID != "first-model" || store.Entries["second"].Models[0].ID != "second-model" {
		t.Fatalf("catalog entries = %#v", store.Entries)
	}
}

func TestProviderInstanceListRedactsSensitiveConfigurationAndSorts(t *testing.T) {
	second := providerInstanceFixture("second", "https://second.example.test/v1")
	second.Headers = map[string]string{"X-Zeta": "header-secret", "Authorization": "secret-token"}
	second.Settings = map[string]any{"zeta": "setting-secret", "api_key": "secret-key"}
	first := providerInstanceFixture("first", "https://first.example.test/v1")
	_, mux, _ := providerInstanceTestHandler(t, second, first)

	recorder := providerInstanceRequest(t, mux, http.MethodGet, "/api/provider-instances", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, secret := range []string{"credential:first", "credential:second", "header-secret", "secret-token", "setting-secret", "secret-key"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response exposed %q: %s", secret, body)
		}
	}
	var response struct {
		Instances []providerInstanceResponse `json:"instances"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(response.Instances) != 2 || response.Instances[0].ID != "first" || response.Instances[1].ID != "second" {
		t.Fatalf("instances = %#v", response.Instances)
	}
	if !response.Instances[1].AuthConfigured || strings.Join(response.Instances[1].HeaderNames, ",") != "Authorization,X-Zeta" || strings.Join(response.Instances[1].SettingNames, ",") != "api_key,zeta" {
		t.Fatalf("safe instance DTO = %#v", response.Instances[1])
	}
}

func TestProviderInstanceCatalogAndTargetProjectionsExcludeLegacyDisabledAndInvalid(t *testing.T) {
	first := providerInstanceFixture("first", "https://first.example.test/v1")
	second := providerInstanceFixture("second", "https://second.example.test/v1")
	disabled := providerInstanceFixture("disabled", "https://disabled.example.test/v1")
	disabled.State = config.ProviderInstanceStateDisabled
	_, mux, _ := providerInstanceTestHandler(t, second, disabled, first)
	store := &CatalogStore{Entries: map[string]*CatalogEntry{
		"first":      {ID: "first", InstanceID: "first", Provider: "openai", Models: []CatalogModel{{ID: "shared"}, {ID: "alpha"}}, FetchedAt: "2026-01-01T00:00:00Z"},
		"second":     {ID: "second", InstanceID: "second", Provider: "openai", Models: []CatalogModel{{ID: "shared"}}, FetchedAt: "2026-01-02T00:00:00Z"},
		"disabled":   {ID: "disabled", InstanceID: "disabled", Provider: "openai", Models: []CatalogModel{{ID: "hidden"}}},
		"legacy|key": {ID: "legacy|key", Provider: "openai", Models: []CatalogModel{{ID: "legacy-model"}}},
		"wrong":      {ID: "wrong", InstanceID: "other", Provider: "openai", Models: []CatalogModel{{ID: "wrong-model"}}},
	}}
	if err := saveCatalogs(store); err != nil {
		t.Fatalf("saveCatalogs() error = %v", err)
	}

	catalogRecorder := providerInstanceRequest(t, mux, http.MethodGet, "/api/provider-instances/catalogs", "")
	if catalogRecorder.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, body = %s", catalogRecorder.Code, catalogRecorder.Body.String())
	}
	var catalogs struct {
		Catalogs []providerInstanceCatalogResponse `json:"catalogs"`
	}
	if err := json.Unmarshal(catalogRecorder.Body.Bytes(), &catalogs); err != nil {
		t.Fatalf("catalog Unmarshal() error = %v", err)
	}
	if len(catalogs.Catalogs) != 3 || catalogs.Catalogs[0].InstanceID != "disabled" || catalogs.Catalogs[1].InstanceID != "first" || catalogs.Catalogs[2].InstanceID != "second" {
		t.Fatalf("catalogs = %#v", catalogs.Catalogs)
	}
	if catalogs.Catalogs[1].Models[0].ID != "alpha" {
		t.Fatalf("catalog models not sorted: %#v", catalogs.Catalogs[1].Models)
	}

	targetRecorder := providerInstanceRequest(t, mux, http.MethodGet, "/api/provider-targets", "")
	var targets struct {
		Targets []providerTargetResponse `json:"targets"`
	}
	if err := json.Unmarshal(targetRecorder.Body.Bytes(), &targets); err != nil {
		t.Fatalf("target Unmarshal() error = %v", err)
	}
	want := []string{"first/alpha", "first/shared", "second/shared"}
	if len(targets.Targets) != len(want) {
		t.Fatalf("targets = %#v", targets.Targets)
	}
	for i := range want {
		if targets.Targets[i].Target != want[i] {
			t.Fatalf("targets[%d] = %q, want %q", i, targets.Targets[i].Target, want[i])
		}
	}
}

func TestProviderRouteCRUDPreservesOrderAndValidatesAuthoritativeTargets(t *testing.T) {
	first := providerInstanceFixture("first", "https://first.example.test/v1")
	second := providerInstanceFixture("second", "https://second.example.test/v1")
	_, mux, configPath := providerInstanceTestHandler(t, first, second)
	if err := saveCatalogs(&CatalogStore{Entries: map[string]*CatalogEntry{
		"first":  {ID: "first", InstanceID: "first", Provider: "openai", Models: []CatalogModel{{ID: "one"}}},
		"second": {ID: "second", InstanceID: "second", Provider: "openai", Models: []CatalogModel{{ID: "two"}}},
	}}); err != nil {
		t.Fatalf("saveCatalogs() error = %v", err)
	}

	recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/model-routes", `{"name":"route-b","targets":["second/two","first/one"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if strings.Join(cfg.ModelRoutes[0].Targets, ",") != "second/two,first/one" {
		t.Fatalf("route order = %#v", cfg.ModelRoutes[0].Targets)
	}

	recorder = providerInstanceRequest(t, mux, http.MethodPost, "/api/model-routes", `{"name":"route-a","targets":["first/missing"]}`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "enabled instance-owned catalog") {
		t.Fatalf("invalid target status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/model-routes/route-b", `{"name":"renamed","targets":["first/one"]}`)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "immutable") {
		t.Fatalf("rename status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/model-routes/route-b", `{"name":"route-b","targets":["first/one","second/two"]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = providerInstanceRequest(t, mux, http.MethodGet, "/api/model-routes", "")
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"targets":["first/one","second/two"]`) {
		t.Fatalf("list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = providerInstanceRequest(t, mux, http.MethodDelete, "/api/model-routes/route-b", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete route status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderInstanceMutationConflictsWithRouteReferences(t *testing.T) {
	instance := providerInstanceFixture("owned", "https://owned.example.test/v1")
	_, mux, configPath := providerInstanceTestHandler(t, instance)
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	cfg.ModelRoutes = []*config.ModelRouteConfig{{Name: "used-route", Targets: []string{"owned/model"}}}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	recorder := providerInstanceRequest(t, mux, http.MethodDelete, "/api/provider-instances/owned", "")
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "used-route") {
		t.Fatalf("delete status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	instance.State = config.ProviderInstanceStateDisabled
	body := providerInstanceAdminBody(t, instance, false)
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/provider-instances/owned", body)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "used-route") {
		t.Fatalf("disable status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestProviderInstanceAdminWriteOnlyCreatePreserveAndClear(t *testing.T) {
	_, mux, configPath := providerInstanceTestHandler(t)
	instance := providerInstanceFixture("secure", "https://secure.example.test/v1")
	createBody := providerInstanceAdminBody(t, instance, true)
	recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances", createBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, secret := range []string{instance.AuthConnectionRef, "secure", instance.Headers["X-Instance"]} {
		if secret != "secure" && strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("create response exposed %q: %s", secret, recorder.Body.String())
		}
	}
	if strings.Contains(recorder.Body.String(), "auth_connection_ref") || strings.Contains(recorder.Body.String(), `"headers"`) || strings.Contains(recorder.Body.String(), `"settings"`) {
		t.Fatalf("create response exposed write-only fields: %s", recorder.Body.String())
	}

	instance.Endpoint = "https://changed.example.test/v1"
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/provider-instances/secure", providerInstanceAdminBody(t, instance, false))
	if recorder.Code != http.StatusOK {
		t.Fatalf("preserve update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	stored := cfg.ProviderInstances[0]
	if stored.AuthConnectionRef != "credential:secure" || stored.Headers["X-Instance"] != "secure" || stored.Settings["tenant"] != "secure" {
		t.Fatalf("omitted sensitive values were not preserved: %#v", stored)
	}

	clearBody := providerInstanceRequestBody{
		ID: "secure", ProviderKind: "openai", Adapter: "openai-compatible", Protocol: "openai",
		Endpoint: instance.Endpoint, State: config.ProviderInstanceStateEnabled,
		ClearAuth: true, ClearHeaders: true, ClearSettings: true,
	}
	data, _ := json.Marshal(clearBody)
	recorder = providerInstanceRequest(t, mux, http.MethodPut, "/api/provider-instances/secure", string(data))
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear update status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	cfg, err = config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() after clear error = %v", err)
	}
	stored = cfg.ProviderInstances[0]
	if stored.AuthConnectionRef != "" || stored.Headers != nil || stored.Settings != nil {
		t.Fatalf("sensitive values not cleared: %#v", stored)
	}
	if strings.Contains(recorder.Body.String(), "credential:secure") || strings.Contains(recorder.Body.String(), "X-Instance\":\"secure") {
		t.Fatalf("clear response exposed secret data: %s", recorder.Body.String())
	}
}

func TestProviderInstanceAdminRejectsCredentialBearingEndpoint(t *testing.T) {
	_, mux, _ := providerInstanceTestHandler(t)
	for _, endpoint := range []string{
		"https://user:secret@example.test/v1",
		"https://example.test/v1?api_key=secret",
		"example.test/v1",
	} {
		instance := providerInstanceFixture("unsafe", endpoint)
		recorder := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances", providerInstanceAdminBody(t, instance, true))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("endpoint %q status = %d, body = %s", endpoint, recorder.Code, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), "secret") {
			t.Fatalf("endpoint error echoed credential: %s", recorder.Body.String())
		}
	}
}

func TestProviderRosterSeparatesCompatibleAndDiscoveryOnlyWithoutModels(t *testing.T) {
	_, mux, _ := providerInstanceTestHandler(t)
	recorder := providerInstanceRequest(t, mux, http.MethodGet, "/api/provider-roster", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "common_models") || strings.Contains(body, "gpt-5.4-mini") {
		t.Fatalf("roster exposed executable template models: %s", body)
	}
	var response struct {
		Providers []providerRosterResponse `json:"providers"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	entries := make(map[string]providerRosterResponse, len(response.Providers))
	for index, entry := range response.Providers {
		entries[entry.ID] = entry
		if index > 0 && response.Providers[index-1].ID > entry.ID {
			t.Fatalf("roster not sorted: %#v", response.Providers)
		}
	}
	if entries["openai"].Compatibility != "compatible" || entries["openai"].Adapter != "openai-compatible" {
		t.Fatalf("openai roster entry = %#v", entries["openai"])
	}
	if entries["github-copilot"].Compatibility != "native" || entries["github-copilot"].Adapter != "github-copilot-native" || entries["github-copilot"].Protocol != "github-copilot" {
		t.Fatalf("copilot roster entry = %#v", entries["github-copilot"])
	}
}

func TestProviderInstancePingAndAutoConnectFree(t *testing.T) {
	h, mux, _ := providerInstanceTestHandler(t)
	h.providerCatalogSync = func(_ context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error) {
		if input.InstanceID == "test-reachable" {
			return []CatalogModel{{ID: "m1"}, {ID: "m2"}}, nil
		}
		return nil, errors.New("upstream unreachable")
	}

	// 1. Auto connect free providers
	rec := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/auto-connect-free", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("auto-connect status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var autoRes struct {
		OK        bool     `json:"ok"`
		Total     int      `json:"total"`
		Connected int      `json:"connected"`
		Instances []string `json:"instances"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &autoRes); err != nil {
		t.Fatalf("Unmarshal auto-connect error = %v", err)
	}
	if !autoRes.OK || autoRes.Total == 0 {
		t.Fatalf("unexpected auto-connect result: %#v", autoRes)
	}

	// 2. Ping reachable instance
	inst := providerInstanceFixture("test-reachable", "https://api.test/v1")
	createRec := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances", providerInstanceAdminBody(t, inst, true))
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRec.Code, createRec.Body.String())
	}

	pingRec := providerInstanceRequest(t, mux, http.MethodPost, "/api/provider-instances/test-reachable/ping", "")
	if pingRec.Code != http.StatusOK {
		t.Fatalf("ping status = %d, body = %s", pingRec.Code, pingRec.Body.String())
	}
	var pingRes struct {
		OK         bool   `json:"ok"`
		InstanceID string `json:"instance_id"`
		ModelCount int    `json:"model_count"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(pingRec.Body.Bytes(), &pingRes); err != nil {
		t.Fatalf("Unmarshal ping error = %v", err)
	}
	if !pingRes.OK || pingRes.Status != "reachable" || pingRes.ModelCount != 2 {
		t.Fatalf("unexpected ping result: %#v", pingRes)
	}
}
