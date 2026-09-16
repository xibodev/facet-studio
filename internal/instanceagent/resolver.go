package instanceagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xibodev/facet-studio/pkg/agent"
	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/providers"
)

type catalogFile struct {
	Entries map[string]struct {
		InstanceID string `json:"instance_id"`
		Models     []struct {
			ID string `json:"id"`
		} `json:"models"`
	} `json:"entries"`
}

// NewResolver loads Studio-owned instance catalogs and credentials on each
// explicit selection so the kernel receives a current immutable resolution.
func NewResolver(configPath, home string) agent.InstanceSelectionResolver {
	return func(ctx context.Context, selection string) (*providers.InstanceResolution, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("load instance config: %w", err)
		}
		catalogs, err := loadCatalogs(filepath.Join(home, "model_catalogs.json"))
		if err != nil {
			return nil, err
		}
		return providers.ResolveInstanceTargetOrRoute(
			cfg,
			catalogs,
			selection,
			resolveCredential,
			providers.CreateProviderFromInstance,
		)
	}
}

func loadCatalogs(path string) (map[string]providers.InstanceCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load instance catalogs: %w", err)
	}
	var stored catalogFile
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode instance catalogs: %w", err)
	}
	catalogs := make(map[string]providers.InstanceCatalog, len(stored.Entries))
	for key, entry := range stored.Entries {
		if entry.InstanceID == "" || entry.InstanceID != key {
			continue
		}
		catalog := providers.InstanceCatalog{InstanceID: entry.InstanceID, Models: make([]string, 0, len(entry.Models))}
		for _, model := range entry.Models {
			catalog.Models = append(catalog.Models, model.ID)
		}
		catalogs[key] = catalog
	}
	return catalogs, nil
}

func resolveCredential(ref string) (string, error) {
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
