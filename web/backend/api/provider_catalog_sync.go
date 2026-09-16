package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xibodev/facet-studio/pkg/providers"
)

const maxProviderCatalogResponseSize = 4 << 20

type compatibleCatalogModel struct {
	ID      string         `json:"id"`
	OwnedBy string         `json:"owned_by"`
	Extra   map[string]any `json:"extra"`
}

var inspectGitHubCopilotFunc = providers.InspectGitHubCopilot

func (h *Handler) syncCompatibleProviderCatalog(ctx context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error) {
	if strings.EqualFold(strings.TrimSpace(input.Adapter), "github-copilot-native") {
		return syncNativeGitHubCopilotCatalog(ctx)
	}
	client := h.providerCatalogHTTPClient
	if client == nil {
		return nil, fmt.Errorf("catalog HTTP client is not configured")
	}
	path, err := providerCatalogPath(input.Settings)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(input.Endpoint, "/")+path,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create catalog request: %w", err)
	}
	for name, value := range input.Headers {
		request.Header.Set(name, value)
	}

	switch strings.ToLower(strings.TrimSpace(input.Adapter)) {
	case "openai-compatible":
		request.Header.Set("Authorization", "Bearer "+input.secret)
	case "anthropic-compatible":
		request.Header.Set("X-Api-Key", input.secret)
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
	return parseCompatibleCatalogModels(body)
}

func syncNativeGitHubCopilotCatalog(ctx context.Context) ([]CatalogModel, error) {
	_, modelInfos, err := inspectGitHubCopilotFunc(ctx)
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

func providerCatalogPath(settings map[string]any) (string, error) {
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

func parseCompatibleCatalogModels(body []byte) ([]CatalogModel, error) {
	var envelope struct {
		Data []compatibleCatalogModel `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Data != nil {
		return catalogModelsFromItems(envelope.Data), nil
	}
	var items []compatibleCatalogModel
	if err := json.Unmarshal(body, &items); err == nil && items != nil {
		return catalogModelsFromItems(items), nil
	}
	return nil, fmt.Errorf("decode catalog response: unsupported model list shape")
}

func catalogModelsFromItems(items []compatibleCatalogModel) []CatalogModel {
	models := make([]CatalogModel, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		models = append(models, CatalogModel{ID: item.ID, OwnedBy: item.OwnedBy, Extra: item.Extra})
	}
	return models
}
