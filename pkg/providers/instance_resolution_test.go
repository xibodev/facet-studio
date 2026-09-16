package providers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/config"
)

type instanceResolutionProvider struct {
	instanceID string
	endpoint   string
	secret     string
	headers    map[string]string
}

func (p *instanceResolutionProvider) Chat(context.Context, []Message, []ToolDefinition, string, map[string]any) (*LLMResponse, error) {
	return nil, errors.New("not called")
}

func (p *instanceResolutionProvider) GetDefaultModel() string { return "" }

func instanceResolutionFixture(id, endpoint string) *config.ProviderInstanceConfig {
	return &config.ProviderInstanceConfig{
		ID:                id,
		ProviderKind:      "openai",
		Adapter:           "openai-compatible",
		Protocol:          "openai",
		Endpoint:          endpoint,
		AuthConnectionRef: "credential:" + id,
		Headers:           map[string]string{"X-Instance": id},
		State:             config.ProviderInstanceStateEnabled,
	}
}

func recordingInstanceFactory(calls *[]*instanceResolutionProvider) InstanceProviderFactory {
	return func(instance *config.ProviderInstanceConfig, modelID, secret string) (LLMProvider, error) {
		provider := &instanceResolutionProvider{
			instanceID: instance.ID,
			endpoint:   instance.Endpoint,
			secret:     secret,
			headers:    cloneStringMap(instance.Headers),
		}
		*calls = append(*calls, provider)
		return provider, nil
	}
}

func fixtureCredentialResolver(ref string) (string, error) {
	return strings.TrimPrefix(ref, "credential:") + "-secret", nil
}

func TestResolveInstanceTargetDirect(t *testing.T) {
	instance := instanceResolutionFixture("primary", "https://primary.example.test/v1")
	cfg := &config.Config{ProviderInstances: []*config.ProviderInstanceConfig{instance}}
	catalogs := map[string]InstanceCatalog{"primary": {InstanceID: "primary", Models: []string{"org/model"}}}
	var calls []*instanceResolutionProvider

	resolved, err := ResolveInstanceTargetOrRoute(
		cfg,
		catalogs,
		"primary/org/model",
		fixtureCredentialResolver,
		recordingInstanceFactory(&calls),
	)
	if err != nil {
		t.Fatalf("ResolveInstanceTargetOrRoute() error = %v", err)
	}
	if len(resolved.Candidates) != 1 {
		t.Fatalf("candidates = %#v", resolved.Candidates)
	}
	candidate := resolved.Candidates[0]
	if candidate.Provider != "openai" || candidate.Model != "org/model" || candidate.IdentityKey != "provider_instance:primary" {
		t.Fatalf("candidate = %#v", candidate)
	}
	provider, err := resolved.ProviderForCandidate(candidate)
	if err != nil || provider != calls[0] {
		t.Fatal("candidate provider did not retain exact instance ownership")
	}
}

func TestResolveInstanceRoutePreservesOrderAndOwnership(t *testing.T) {
	first := instanceResolutionFixture("first", "https://first.example.test/v1")
	second := instanceResolutionFixture("second", "https://second.example.test/v1")
	cfg := &config.Config{
		ProviderInstances: []*config.ProviderInstanceConfig{first, second},
		ModelRoutes: []*config.ModelRouteConfig{{
			Name:    "chat-route",
			Targets: []string{"second/shared-model", "first/shared-model"},
		}},
	}
	catalogs := map[string]InstanceCatalog{
		"first":  {InstanceID: "first", Models: []string{"shared-model"}},
		"second": {InstanceID: "second", Models: []string{"shared-model"}},
	}
	var calls []*instanceResolutionProvider

	resolved, err := ResolveInstanceTargetOrRoute(
		cfg,
		catalogs,
		"chat-route",
		fixtureCredentialResolver,
		recordingInstanceFactory(&calls),
	)
	if err != nil {
		t.Fatalf("ResolveInstanceTargetOrRoute() error = %v", err)
	}
	if len(resolved.Candidates) != 2 || resolved.Candidates[0].DisplayName != "second/shared-model" || resolved.Candidates[1].DisplayName != "first/shared-model" {
		t.Fatalf("ordered candidates = %#v", resolved.Candidates)
	}
	if resolved.Candidates[0].StableKey() == resolved.Candidates[1].StableKey() {
		t.Fatalf("same-kind identical models share stable key: %#v", resolved.Candidates)
	}
	firstResolved, err := resolved.ProviderForCandidate(resolved.Candidates[0])
	if err != nil {
		t.Fatal(err)
	}
	secondResolved, err := resolved.ProviderForCandidate(resolved.Candidates[1])
	if err != nil {
		t.Fatal(err)
	}
	firstProvider := firstResolved.(*instanceResolutionProvider)
	secondProvider := secondResolved.(*instanceResolutionProvider)
	if firstProvider.instanceID != "second" || firstProvider.endpoint != second.Endpoint || firstProvider.secret != "second-secret" || firstProvider.headers["X-Instance"] != "second" {
		t.Fatalf("first route provider borrowed ownership: %#v", firstProvider)
	}
	if secondProvider.instanceID != "first" || secondProvider.endpoint != first.Endpoint || secondProvider.secret != "first-secret" || secondProvider.headers["X-Instance"] != "first" {
		t.Fatalf("second route provider borrowed ownership: %#v", secondProvider)
	}
}

func TestResolveInstanceTargetValidationFailures(t *testing.T) {
	enabled := instanceResolutionFixture("enabled", "https://enabled.example.test/v1")
	disabled := instanceResolutionFixture("disabled", "https://disabled.example.test/v1")
	disabled.State = config.ProviderInstanceStateDisabled
	cfg := &config.Config{ProviderInstances: []*config.ProviderInstanceConfig{enabled, disabled}}
	validCatalog := InstanceCatalog{InstanceID: "enabled", Models: []string{"model"}}

	tests := []struct {
		name      string
		selection string
		catalogs  map[string]InstanceCatalog
		wantErr   string
	}{
		{name: "missing instance", selection: "missing/model", catalogs: map[string]InstanceCatalog{}, wantErr: `instance "missing" not found`},
		{name: "disabled instance", selection: "disabled/model", catalogs: map[string]InstanceCatalog{"disabled": {InstanceID: "disabled", Models: []string{"model"}}}, wantErr: "is disabled"},
		{name: "missing catalog", selection: "enabled/model", catalogs: map[string]InstanceCatalog{}, wantErr: "catalog"},
		{name: "catalog belongs to another instance", selection: "enabled/model", catalogs: map[string]InstanceCatalog{"enabled": {InstanceID: "other", Models: []string{"model"}}}, wantErr: "catalog"},
		{name: "missing model", selection: "enabled/other", catalogs: map[string]InstanceCatalog{"enabled": validCatalog}, wantErr: "not found"},
		{name: "missing route", selection: "not-a-route", catalogs: map[string]InstanceCatalog{"enabled": validCatalog}, wantErr: "route was not found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := ResolveInstanceTargetOrRoute(
				cfg,
				tc.catalogs,
				tc.selection,
				fixtureCredentialResolver,
				func(*config.ProviderInstanceConfig, string, string) (LLMProvider, error) {
					called = true
					return &instanceResolutionProvider{}, nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
			if called {
				t.Fatal("provider factory called for invalid target")
			}
		})
	}
}

func TestResolveInstanceRouteDoesNotConsultLegacyModelTemplates(t *testing.T) {
	instance := instanceResolutionFixture("owned", "https://owned.example.test/v1")
	cfg := &config.Config{
		ProviderInstances: []*config.ProviderInstanceConfig{instance},
		ModelRoutes:       []*config.ModelRouteConfig{{Name: "route", Targets: []string{"owned/model"}}},
		ModelList: []*config.ModelConfig{{
			ModelName:     "legacy-template",
			Provider:      "openai",
			Model:         "model",
			APIBase:       "https://wrong.example.test/v1",
			APIKeys:       config.SimpleSecureStrings("wrong-secret"),
			CustomHeaders: map[string]string{"X-Instance": "wrong"},
		}},
	}
	var calls []*instanceResolutionProvider
	resolved, err := ResolveInstanceTargetOrRoute(
		cfg,
		map[string]InstanceCatalog{"owned": {InstanceID: "owned", Models: []string{"model"}}},
		"route",
		fixtureCredentialResolver,
		recordingInstanceFactory(&calls),
	)
	if err != nil {
		t.Fatalf("ResolveInstanceTargetOrRoute() error = %v", err)
	}
	resolvedProvider, err := resolved.ProviderForCandidate(resolved.Candidates[0])
	if err != nil {
		t.Fatal(err)
	}
	provider := resolvedProvider.(*instanceResolutionProvider)
	if provider.endpoint != instance.Endpoint || provider.secret != "owned-secret" || provider.headers["X-Instance"] != "owned" {
		t.Fatalf("provider borrowed legacy template: %#v", provider)
	}
}
