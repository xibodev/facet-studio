package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/modelservice"
	"github.com/xibodev/facet-studio/pkg/providers"
)

// CatalogModel represents a single model entry in a saved catalog.
type CatalogModel = modelservice.CatalogModel

// CatalogEntry is a saved list of upstream models fetched for a specific provider+key combination.
type CatalogEntry = modelservice.CatalogEntry

// CatalogStore holds all saved model catalogs.
type CatalogStore = modelservice.CatalogStore

func saveProviderInstanceCatalog(instance *config.ProviderInstanceConfig, models []CatalogModel) error {
	return modelservice.SaveProviderInstanceCatalog(instance, models)
}

func deleteProviderInstanceCatalog(instanceID string) error {
	return modelservice.DeleteProviderInstanceCatalog(instanceID)
}

func catalogFilePath() string {
	return modelservice.CatalogFilePath()
}

func generateCatalogKey(provider, apiBase, apiKey string) string {
	return modelservice.GenerateCatalogKey(provider, apiBase, apiKey)
}

func maskAPIKeyValue(key string) string {
	return modelservice.MaskAPIKeyValue(key)
}

func loadCatalogs() (*CatalogStore, error) {
	return modelservice.LoadCatalogs()
}

func saveCatalogs(store *CatalogStore) error {
	return modelservice.SaveCatalogs(store)
}

// SaveCatalog persists a fetched model list for a given provider+key combination.
// If a catalog with the same key already exists, it is updated.
func SaveCatalog(provider, apiBase, apiKey string, models []CatalogModel) error {
	store, err := loadCatalogs()
	if err != nil {
		return err
	}
	key := generateCatalogKey(provider, apiBase, apiKey)
	provider = providers.NormalizeProvider(provider)
	store.Entries[key] = &CatalogEntry{
		ID:         key,
		Provider:   provider,
		APIBase:    strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		APIKeyMask: maskAPIKeyValue(apiKey),
		Models:     models,
		FetchedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	return saveCatalogs(store)
}

// handleListCatalogs returns all saved model catalogs.
//
//	GET /api/models/catalog
func (h *Handler) handleListCatalogs(w http.ResponseWriter, r *http.Request) {
	store, err := loadCatalogs()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load catalogs: %v", err), http.StatusInternalServerError)
		return
	}

	entries := make([]*CatalogEntry, 0, len(store.Entries))
	for _, e := range store.Entries {
		entries = append(entries, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleDeleteCatalog deletes a saved model catalog by ID.
//
//	DELETE /api/models/catalog/{id}
func (h *Handler) handleDeleteCatalog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	store, err := loadCatalogs()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load catalogs: %v", err), http.StatusInternalServerError)
		return
	}

	if _, ok := store.Entries[id]; !ok {
		http.Error(w, "catalog not found", http.StatusNotFound)
		return
	}

	delete(store.Entries, id)
	if err := saveCatalogs(store); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save catalogs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
