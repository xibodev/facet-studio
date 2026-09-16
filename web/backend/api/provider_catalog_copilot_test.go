package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	copilot "github.com/github/copilot-sdk/go"
)

func TestNativeGitHubCopilotCatalogMapsCapabilities(t *testing.T) {
	original := inspectGitHubCopilotFunc
	t.Cleanup(func() { inspectGitHubCopilotFunc = original })
	maxPrompt := 64000
	maxContext := 128000
	billingMultiplier := 0.25
	inspectGitHubCopilotFunc = func(context.Context) (*copilot.GetAuthStatusResponse, []copilot.ModelInfo, error) {
		return &copilot.GetAuthStatusResponse{IsAuthenticated: true}, []copilot.ModelInfo{{
			ID:   "gpt-fixture",
			Name: "GPT Fixture",
			Capabilities: copilot.ModelCapabilities{
				Supports: copilot.ModelSupports{ReasoningEffort: true},
				Limits:   copilot.ModelLimits{MaxPromptTokens: &maxPrompt, MaxContextWindowTokens: &maxContext},
			},
			Policy:                    &copilot.ModelPolicy{State: "enabled"},
			Billing:                   &copilot.ModelBilling{Multiplier: &billingMultiplier},
			SupportedReasoningEfforts: []string{"low", "high"},
		}}, nil
	}

	models, err := syncNativeGitHubCopilotCatalog(t.Context())
	if err != nil {
		t.Fatalf("syncNativeGitHubCopilotCatalog() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-fixture" || models[0].OwnedBy != "github-copilot" {
		t.Fatalf("models = %#v", models)
	}
	extra := models[0].Extra
	if extra["native_adapter"] != true || extra["studio_tools"] != false || extra["copilot_builtin_tools"] != false || extra["context_window_tokens"] != 128000 || extra["max_prompt_tokens"] != 64000 {
		t.Fatalf("capability labels = %#v", extra)
	}
}

func TestNativeGitHubCopilotInstanceSyncNeedsNoEndpointOrCredential(t *testing.T) {
	instance := providerInstanceFixture("copilot", "")
	instance.ProviderKind = "github-copilot"
	instance.Adapter = "github-copilot-native"
	instance.Protocol = "github-copilot"
	instance.AuthConnectionRef = ""
	h, mux, _ := providerInstanceTestHandler(t, instance)
	h.providerCredentialResolver = func(string) (string, error) {
		t.Fatal("native Copilot sync must not resolve or copy a credential")
		return "", nil
	}
	h.providerCatalogSync = func(_ context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error) {
		if input.Endpoint != "" || input.AuthConnectionRef != "" || input.Secret != "" {
			t.Fatalf("native input contains endpoint or credential: %#v", input)
		}
		return []CatalogModel{{ID: "gpt-fixture", Extra: map[string]any{"native_adapter": true}}}, nil
	}

	recorder := providerInstanceRequest(t, mux, "POST", "/api/provider-instances/copilot/catalog/sync", `{}`)
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Models []CatalogModel `json:"models"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || len(response.Models) != 1 {
		t.Fatalf("response = %s, error = %v", recorder.Body.String(), err)
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), "token") {
		t.Fatalf("response unexpectedly contains credential terminology: %s", recorder.Body.String())
	}
}

func TestNativeGitHubCopilotInstanceRejectsExternalConnectionAuthority(t *testing.T) {
	instance := providerInstanceFixture("copilot", "https://example.test")
	instance.ProviderKind = "github-copilot"
	instance.Adapter = "github-copilot-native"
	instance.Protocol = "github-copilot"
	if err := validateProviderInstanceForAPI(instance); err == nil || !strings.Contains(err.Error(), "does not accept an endpoint") {
		t.Fatalf("validateProviderInstanceForAPI() error = %v", err)
	}

	instance.Endpoint = ""
	instance.AuthConnectionRef = ""
	instance.Headers = nil
	instance.ProviderKind = "openai"
	if err := validateProviderInstanceForAPI(instance); err == nil || !strings.Contains(err.Error(), "requires github-copilot") {
		t.Fatalf("validateProviderInstanceForAPI() protocol error = %v", err)
	}

	instance.ProviderKind = "github-copilot"
	instance.Protocol = "github-copilot"
	instance.AuthConnectionRef = "credential:copilot"
	if err := validateProviderInstanceForAPI(instance); err == nil || !strings.Contains(err.Error(), "does not accept an auth connection") {
		t.Fatalf("validateProviderInstanceForAPI() auth error = %v", err)
	}

	instance.AuthConnectionRef = ""
	instance.Headers = map[string]string{"Authorization": "must-not-be-used"}
	if err := validateProviderInstanceForAPI(instance); err == nil || !strings.Contains(err.Error(), "does not accept headers") {
		t.Fatalf("validateProviderInstanceForAPI() headers error = %v", err)
	}
}
