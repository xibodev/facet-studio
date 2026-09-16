package modelservice

import (
	"context"
	"fmt"
	"net/http"

	"github.com/xibodev/facet-studio/pkg/config"
	llmgwproviders "github.com/xibodev/llmgw-core/providers"
)

// AutoConnectFree connects all verified anonymous free providers to config, queries their catalogs,
// and activates working free models.
func AutoConnectFree(ctx context.Context, cfg *config.Config, client *http.Client, syncFunc CatalogSyncFunc) (*AutoConnectResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if syncFunc == nil {
		syncFunc = func(c context.Context, in ProviderCatalogSyncInput) ([]CatalogModel, error) {
			return SyncCompatibleCatalog(c, in, client)
		}
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
		var instance *config.ProviderInstanceConfig
		if !existingIDs[instID] {
			instance = &config.ProviderInstanceConfig{
				ID:           instID,
				ProviderKind: profile.RegistryID,
				Adapter:      "openai-compatible",
				Protocol:     "openai",
				Endpoint:     profile.BaseURL,
				State:        config.ProviderInstanceStateEnabled,
			}
			cfg.ProviderInstances = append(cfg.ProviderInstances, instance)
			existingIDs[instID] = true
			connected++
			connectedInstances = append(connectedInstances, instID)
		} else {
			for _, inst := range cfg.ProviderInstances {
				if inst != nil && inst.ID == instID {
					instance = inst
					break
				}
			}
		}

		if instance != nil {
			input := CatalogSyncInputFromInstance(instance)
			models, syncErr := syncFunc(ctx, input)
			if syncErr == nil && len(models) > 0 {
				freeModels := FilterAnonymousFreeModels(profile.RegistryID, models)
				_ = SaveProviderInstanceCatalog(instance, freeModels)
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
					// If the default model is unset, set it to the first verified free model
					if cfg.Agents.Defaults.ModelName == "" {
						cfg.Agents.Defaults.ModelName = exact
					}
				}
			}
		}
	}

	return &AutoConnectResult{
		OK:        true,
		Total:     len(profiles),
		Connected: connected,
		Verified:  verified,
		Instances: connectedInstances,
	}, nil
}

// AutoConnectFreeAndSave connects free providers and saves the updated configuration to disk.
func AutoConnectFreeAndSave(ctx context.Context, configPath string, client *http.Client, syncFunc CatalogSyncFunc) (*AutoConnectResult, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	res, err := AutoConnectFree(ctx, cfg, client, syncFunc)
	if err != nil {
		return nil, err
	}

	if res.Connected > 0 || res.Verified > 0 {
		if err := config.SaveConfig(configPath, cfg); err != nil {
			return nil, fmt.Errorf("save config: %w", err)
		}
	}

	return res, nil
}
