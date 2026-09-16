package modelservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/providers"
)

const maxProviderCatalogResponseSize = 4 << 20

type compatibleCatalogModel struct {
	ID      string         `json:"id"`
	OwnedBy string         `json:"owned_by"`
	Extra   map[string]any `json:"extra"`
}

// InspectGitHubCopilotFunc points to the discovery function for GitHub Copilot.
// It can be overridden in tests.
var InspectGitHubCopilotFunc = providers.InspectGitHubCopilot

// AnonymousFreeModelIDs lists known working free models for anonymous providers.
var AnonymousFreeModelIDs = map[string][]string{
	"opencode_zen":     {"ling-3.0-flash-fin-free", "muse-spark-1.2-contributor-free", "nemotron-3.5-lightning-free"},
	"kilo_code":        {"kilo-auto/free", "liquid/lfm-2.5-2.6b:free", "cohere/north-mini-code:free"},
	"llm7":             {"codestral-latest", "mistral-Nemo-Instruct-2407", "minimax-m2.7"},
	"ovh_ai_endpoints": {"Qwen3.8-27B", "Mistral-Nemo-Instruct-2407", "gpt-oss-20b"},
	"pollinations":     {"openai-fast", "deepseek-reasoner"},
}

// SyncCompatibleCatalog queries an upstream provider's /models endpoint and parses the results.
func SyncCompatibleCatalog(ctx context.Context, input ProviderCatalogSyncInput, client *http.Client) ([]CatalogModel, error) {
	if strings.EqualFold(strings.TrimSpace(input.Adapter), "github-copilot-native") {
		return SyncNativeGitHubCopilotCatalog(ctx)
	}
	if client == nil {
		client = http.DefaultClient
	}
	path, err := ProviderCatalogPath(input.Settings)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(input.Endpoint), "/")
	if endpoint == "" {
		return nil, fmt.Errorf("provider endpoint is required")
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint+path,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create catalog request: %w", err)
	}
	for name, value := range input.Headers {
		request.Header.Set(name, value)
	}

	secret := strings.TrimSpace(input.Secret)
	switch strings.ToLower(strings.TrimSpace(input.Adapter)) {
	case "openai-compatible":
		if secret != "" {
			request.Header.Set("Authorization", "Bearer "+secret)
		}
	case "anthropic-compatible":
		if secret != "" {
			request.Header.Set("X-Api-Key", secret)
		}
		if request.Header.Get("Anthropic-Version") == "" {
			version := "2023-06-01"
			if configured, ok := input.Settings["anthropic_version"].(string); ok && strings.TrimSpace(configured) != "" {
				version = strings.TrimSpace(configured)
			}
			request.Header.Set("Anthropic-Version", version)
		}
	default:
		return nil, fmt.Errorf("adapter does not support compatible catalog sync")
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("catalog request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog endpoint returned status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderCatalogResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read catalog response: %w", err)
	}
	if len(body) > maxProviderCatalogResponseSize {
		return nil, fmt.Errorf("catalog response exceeds %d bytes", maxProviderCatalogResponseSize)
	}
	return ParseCompatibleCatalogModels(body)
}

// SyncNativeGitHubCopilotCatalog queries the native GitHub Copilot model info.
func SyncNativeGitHubCopilotCatalog(ctx context.Context) ([]CatalogModel, error) {
	_, modelInfos, err := InspectGitHubCopilotFunc(ctx)
	if err != nil {
		return nil, err
	}
	models := make([]CatalogModel, 0, len(modelInfos))
	for _, info := range modelInfos {
		if strings.TrimSpace(info.ID) == "" {
			continue
		}
		extra := map[string]any{
			"name":                        info.Name,
			"native_adapter":              true,
			"text_input":                  true,
			"text_output":                 true,
			"studio_tools":                false,
			"copilot_builtin_tools":       false,
			"reasoning_effort":            info.Capabilities.Supports.ReasoningEffort,
			"supported_reasoning_efforts": info.SupportedReasoningEfforts,
		}
		if info.Capabilities.Limits.MaxPromptTokens != nil {
			extra["max_prompt_tokens"] = *info.Capabilities.Limits.MaxPromptTokens
		}
		if info.Capabilities.Limits.MaxContextWindowTokens != nil {
			extra["context_window_tokens"] = *info.Capabilities.Limits.MaxContextWindowTokens
		}
		if info.Policy != nil {
			extra["policy_state"] = info.Policy.State
		}
		if info.Billing != nil && info.Billing.Multiplier != nil {
			extra["billing_multiplier"] = *info.Billing.Multiplier
		}
		models = append(models, CatalogModel{ID: info.ID, OwnedBy: "github-copilot", Extra: extra})
	}
	return models, nil
}

// ProviderCatalogPath determines the catalog URL path from settings or defaults to "/models".
func ProviderCatalogPath(settings map[string]any) (string, error) {
	path := "/models"
	if configured, ok := settings["catalog_path"]; ok {
		value, ok := configured.(string)
		if !ok || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
			return "", fmt.Errorf("catalog_path must be an absolute URL path")
		}
		path = value
	}
	return path, nil
}

// ParseCompatibleCatalogModels decodes JSON array or { "data": [...] } response.
func ParseCompatibleCatalogModels(body []byte) ([]CatalogModel, error) {
	var envelope struct {
		Data []compatibleCatalogModel `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		return CatalogModelsFromItems(envelope.Data), nil
	}
	var items []compatibleCatalogModel
	if err := json.Unmarshal(body, &items); err == nil && items != nil {
		return CatalogModelsFromItems(items), nil
	}
	return nil, fmt.Errorf("decode catalog response: unsupported model list shape")
}

// CatalogModelsFromItems translates compatible items to CatalogModel list.
func CatalogModelsFromItems(items []compatibleCatalogModel) []CatalogModel {
	models := make([]CatalogModel, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		models = append(models, CatalogModel{ID: item.ID, OwnedBy: item.OwnedBy, Extra: item.Extra})
	}
	return models
}

// FilterAnonymousFreeModels narrows a catalog to verified free or community models.
func FilterAnonymousFreeModels(providerKind string, models []CatalogModel) []CatalogModel {
	preferred, hasPreferred := AnonymousFreeModelIDs[providerKind]
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

// ResolveProviderCredentialReference loads an access token from the auth store.
func ResolveProviderCredentialReference(ref string) (string, error) {
	kind, key, found := strings.Cut(strings.TrimSpace(ref), ":")
	if !found || kind != "credential" || strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("auth_connection_ref must use credential:<store-key>")
	}
	cred, err := auth.GetCredential(strings.TrimSpace(key))
	if err != nil {
		return "", fmt.Errorf("load credential reference: %w", err)
	}
	if cred == nil || strings.TrimSpace(cred.AccessToken) == "" {
		return "", fmt.Errorf("credential %q has no access token", key)
	}
	return strings.TrimSpace(cred.AccessToken), nil
}
