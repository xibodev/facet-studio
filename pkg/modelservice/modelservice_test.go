package modelservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xibodev/facet-studio/pkg/config"
)

func TestListRoster(t *testing.T) {
	cfg := &config.Config{
		ProviderInstances: []*config.ProviderInstanceConfig{
			{
				ID:           "opencode-zen",
				ProviderKind: "opencode_zen",
				Adapter:      "openai-compatible",
			},
		},
	}
	roster := ListRoster(cfg)
	if len(roster) == 0 {
		t.Fatal("expected non-empty roster")
	}

	foundZen := false
	for _, item := range roster {
		if item.ID == "opencode_zen" || item.ID == "opencode-zen" {
			foundZen = true
			if !item.Configured {
				t.Fatalf("expected opencode_zen to be marked configured, got %+v", item)
			}
		}
	}
	if !foundZen {
		t.Fatal("expected opencode_zen in roster")
	}
}

func TestMaskAPIKeyValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "****"},
		{"12345678", "****"},
		{"123456789", "123****89"},
		{"123456789012", "123****12"},
		{"sk-1234567890abcd", "sk-****abcd"},
	}
	for _, tc := range tests {
		got := MaskAPIKeyValue(tc.input)
		if got != tc.want {
			t.Errorf("MaskAPIKeyValue(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFilterAnonymousFreeModels(t *testing.T) {
	models := []CatalogModel{
		{ID: "gpt-4o"},
		{ID: "ling-3.0-flash-fin-free"},
		{ID: "claude-3-5-sonnet"},
	}
	filtered := FilterAnonymousFreeModels("opencode_zen", models)
	if len(filtered) != 1 || filtered[0].ID != "ling-3.0-flash-fin-free" {
		t.Fatalf("unexpected filtered models: %+v", filtered)
	}
}

func TestSyncCompatibleCatalog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"test-model","owned_by":"test"}]}`))
	}))
	defer ts.Close()

	input := ProviderCatalogSyncInput{
		Endpoint: ts.URL,
		Adapter:  "openai-compatible",
	}

	models, err := SyncCompatibleCatalog(context.Background(), input, ts.Client())
	if err != nil {
		t.Fatalf("SyncCompatibleCatalog failed: %v", err)
	}
	if len(models) != 1 || models[0].ID != "test-model" {
		t.Fatalf("unexpected models: %+v", models)
	}
}
