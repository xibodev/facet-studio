package instanceagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/config"
)

func TestResolverLoadsPersistedInstanceCatalogAndBuildsProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FACET_STUDIO_HOME", home)
	configPath := filepath.Join(home, "config.json")
	cfg := config.DefaultConfig()
	cfg.ProviderInstances = []*config.ProviderInstanceConfig{{
		ID:                "fixture",
		ProviderKind:      "openai",
		Adapter:           "openai-compatible",
		Protocol:          "openai",
		Endpoint:          "https://fixture.example.test/v1",
		AuthConnectionRef: "credential:fixture-auth",
		Headers:           map[string]string{"X-Fixture": "owned"},
		State:             config.ProviderInstanceStateEnabled,
	}}
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if err := auth.SetCredential("fixture-auth", &auth.AuthCredential{
		AccessToken: "fixture-secret",
		Provider:    "fixture-auth",
		AuthMethod:  "token",
	}); err != nil {
		t.Fatalf("SetCredential() error = %v", err)
	}
	catalog := catalogFile{Entries: map[string]struct {
		InstanceID string `json:"instance_id"`
		Models     []struct {
			ID string `json:"id"`
		} `json:"models"`
	}{
		"fixture": {
			InstanceID: "fixture",
			Models: []struct {
				ID string `json:"id"`
			}{{ID: "org/model"}},
		},
	}}
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, "model_catalogs.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := NewResolver(configPath, home)(context.Background(), "fixture/org/model")
	if err != nil {
		t.Fatalf("resolver error = %v", err)
	}
	if len(resolved.Candidates) != 1 || resolved.Candidates[0].DisplayName != "fixture/org/model" {
		t.Fatalf("candidates = %#v", resolved.Candidates)
	}
	provider, err := resolved.ProviderForCandidate(resolved.Candidates[0])
	if err != nil || provider == nil {
		t.Fatal("provider was not constructed from persisted instance")
	}
}
