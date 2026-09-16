package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/xibodev/facet-studio/pkg/modelservice"
)

var inspectGitHubCopilotFunc = modelservice.InspectGitHubCopilotFunc

func (h *Handler) syncCompatibleProviderCatalog(ctx context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error) {
	if strings.EqualFold(strings.TrimSpace(input.Adapter), "github-copilot-native") {
		return syncNativeGitHubCopilotCatalog(ctx)
	}
	client := h.providerCatalogHTTPClient
	if client == nil {
		return nil, fmt.Errorf("catalog HTTP client is not configured")
	}
	return modelservice.SyncCompatibleCatalog(ctx, input, client)
}

func syncNativeGitHubCopilotCatalog(ctx context.Context) ([]CatalogModel, error) {
	modelservice.InspectGitHubCopilotFunc = inspectGitHubCopilotFunc
	return modelservice.SyncNativeGitHubCopilotCatalog(ctx)
}

func providerCatalogPath(settings map[string]any) (string, error) {
	return modelservice.ProviderCatalogPath(settings)
}

func parseCompatibleCatalogModels(body []byte) ([]CatalogModel, error) {
	return modelservice.ParseCompatibleCatalogModels(body)
}
