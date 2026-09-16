package modelservice

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/fileutil"
	"github.com/xibodev/facet-studio/pkg/providers"
)

// CatalogFilePath returns the absolute path to the local model catalogs cache file.
func CatalogFilePath() string {
	return filepath.Join(config.GetHome(), "model_catalogs.json")
}

// LoadCatalogs reads the model catalogs cache from disk.
func LoadCatalogs() (*CatalogStore, error) {
	path := CatalogFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CatalogStore{Entries: make(map[string]*CatalogEntry)}, nil
		}
		return nil, fmt.Errorf("read model catalogs: %w", err)
	}
	var store CatalogStore
	if err := json.Unmarshal(data, &store); err != nil {
		return &CatalogStore{Entries: make(map[string]*CatalogEntry)}, nil
	}
	if store.Entries == nil {
		store.Entries = make(map[string]*CatalogEntry)
	}
	return &store, nil
}

// SaveCatalogs atomically writes the model catalogs cache to disk.
func SaveCatalogs(store *CatalogStore) error {
	path := CatalogFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create catalog directory: %w", err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal catalog: %w", err)
	}
	return fileutil.WriteFileAtomic(path, data, 0600)
}

// SaveProviderInstanceCatalog saves catalog models for a specific instance.
func SaveProviderInstanceCatalog(instance *config.ProviderInstanceConfig, models []CatalogModel) error {
	store, err := LoadCatalogs()
	if err != nil {
		return err
	}
	store.Entries[instance.ID] = &CatalogEntry{
		ID:         instance.ID,
		InstanceID: instance.ID,
		Provider:   instance.ProviderKind,
		APIBase:    strings.TrimRight(strings.TrimSpace(instance.Endpoint), "/"),
		Models:     models,
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	return SaveCatalogs(store)
}

// DeleteProviderInstanceCatalog removes catalog models for a specific instance.
func DeleteProviderInstanceCatalog(instanceID string) error {
	store, err := LoadCatalogs()
	if err != nil {
		return err
	}
	delete(store.Entries, instanceID)
	return SaveCatalogs(store)
}

// GenerateCatalogKey creates a deterministic key for a provider+base+key combination.
func GenerateCatalogKey(provider, apiBase, apiKey string) string {
	provider = providers.NormalizeProvider(provider)
	apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
	hash := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%s|%s|%x", provider, apiBase, hash[:6])
}

// MaskAPIKeyValue masks an API key for display.
func MaskAPIKeyValue(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	if len(key) <= 12 {
		return key[:3] + "****" + key[len(key)-2:]
	}
	return key[:3] + "****" + key[len(key)-4:]
}
