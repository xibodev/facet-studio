package modelservice

import (
	"context"

	"github.com/xibodev/facet-studio/pkg/config"
)

// CatalogModel represents a single model entry in a saved catalog.
type CatalogModel struct {
	ID      string         `json:"id"`
	OwnedBy string         `json:"owned_by,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

// CatalogEntry is a saved list of upstream models fetched for a specific provider+key combination.
type CatalogEntry struct {
	ID         string         `json:"id"`
	InstanceID string         `json:"instance_id,omitempty"`
	Provider   string         `json:"provider"`
	APIBase    string         `json:"api_base"`
	APIKeyMask string         `json:"api_key_mask"`
	Models     []CatalogModel `json:"models"`
	FetchedAt  string         `json:"fetched_at"`
}

// CatalogStore holds all saved model catalogs.
type CatalogStore struct {
	Entries map[string]*CatalogEntry `json:"entries"`
}

// ProviderRosterItem describes a provider from the curated registry and its local configuration status.
type ProviderRosterItem struct {
	ID                  string   `json:"id"`
	DisplayName         string   `json:"display_name"`
	Label               string   `json:"label,omitempty"`
	Description         string   `json:"description,omitempty"`
	Categories          []string `json:"categories,omitempty"`
	Adapter             string   `json:"adapter,omitempty"`
	Protocol            string   `json:"protocol,omitempty"`
	DefaultEndpoint     string   `json:"default_endpoint,omitempty"`
	Compatibility       string   `json:"compatibility"`
	AuthMethods         []string `json:"auth_methods,omitempty"`
	RequiresAPIKey      bool     `json:"requires_api_key"`
	RequiresBaseURL     bool     `json:"requires_base_url"`
	AnonymousAutomation bool     `json:"anonymous_automation"`
	OnboardingFields    []string `json:"onboarding_fields,omitempty"`
	Configured          bool     `json:"configured"`
	InstanceCount       int      `json:"instance_count"`
	ConfiguredInstances []string `json:"configured_instances,omitempty"`
}

// ProviderCatalogSyncInput defines parameters for syncing models from an upstream provider.
type ProviderCatalogSyncInput struct {
	InstanceID        string
	ProviderKind      string
	Adapter           string
	Protocol          string
	Endpoint          string
	AuthConnectionRef string
	Headers           map[string]string
	Settings          map[string]any
	Secret            string
}

// CatalogSyncFunc queries models for a provider instance.
type CatalogSyncFunc func(ctx context.Context, input ProviderCatalogSyncInput) ([]CatalogModel, error)

// PingResult describes the outcome of probing an endpoint's reachability.
type PingResult struct {
	OK         bool   `json:"ok"`
	InstanceID string `json:"instance_id"`
	LatencyMS  int64  `json:"latency_ms"`
	ModelCount int    `json:"model_count,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// AutoConnectResult reports the results of auto-connecting anonymous free providers.
type AutoConnectResult struct {
	OK        bool     `json:"ok"`
	Total     int      `json:"total"`
	Connected int      `json:"connected"`
	Verified  int      `json:"verified"`
	Instances []string `json:"instances"`
}

// CatalogSyncInputFromInstance converts a config.ProviderInstanceConfig to ProviderCatalogSyncInput.
func CatalogSyncInputFromInstance(instance *config.ProviderInstanceConfig) ProviderCatalogSyncInput {
	if instance == nil {
		return ProviderCatalogSyncInput{}
	}
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
