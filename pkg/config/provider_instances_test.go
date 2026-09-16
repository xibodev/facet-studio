package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func validProviderInstance(id string) *ProviderInstanceConfig {
	return &ProviderInstanceConfig{
		ID:                id,
		ProviderKind:      "openai",
		Adapter:           "openai-compatible",
		Protocol:          "openai",
		Endpoint:          "https://api.example.test/v1",
		AuthConnectionRef: "credential:test",
		Headers:           map[string]string{"X-Test": "fixture"},
		Settings:          map[string]any{"organization": "example"},
		State:             ProviderInstanceStateEnabled,
	}
}

func TestProviderInstanceConfigStableIDRoundTrip(t *testing.T) {
	want := validProviderInstance("openai.primary-1")
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got ProviderInstanceConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.ID != want.ID {
		t.Fatalf("ID = %q, want stable ID %q", got.ID, want.ID)
	}
	if got.ProviderKind != want.ProviderKind || got.Adapter != want.Adapter || got.Protocol != want.Protocol {
		t.Fatalf("adapter identity changed after round trip: got %#v, want %#v", got, *want)
	}
	if got.AuthConnectionRef != want.AuthConnectionRef || got.Headers["X-Test"] != "fixture" {
		t.Fatalf("connection ownership changed after round trip: got %#v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProviderInstanceFoundationJSONRoundTrip(t *testing.T) {
	want := Config{
		ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("openai-primary")},
		ModelRoutes: []*ModelRouteConfig{{
			Name:    "chat-default",
			Targets: []string{"openai-primary/openai/gpt-5"},
		}},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(got.ProviderInstances) != 1 || got.ProviderInstances[0].ID != "openai-primary" {
		t.Fatalf("provider_instances = %#v", got.ProviderInstances)
	}
	if len(got.ModelRoutes) != 1 || got.ModelRoutes[0].Targets[0] != "openai-primary/openai/gpt-5" {
		t.Fatalf("model_routes = %#v", got.ModelRoutes)
	}
	if err := got.ValidateProviderInstances(); err != nil {
		t.Fatalf("ValidateProviderInstances() error = %v", err)
	}
}

func TestProviderInstanceConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ProviderInstanceConfig)
		wantErr string
	}{
		{name: "valid"},
		{name: "disabled is a valid lifecycle state", mutate: func(c *ProviderInstanceConfig) { c.State = ProviderInstanceStateDisabled }},
		{name: "missing id", mutate: func(c *ProviderInstanceConfig) { c.ID = "" }, wantErr: "id must be"},
		{name: "unstable uppercase id", mutate: func(c *ProviderInstanceConfig) { c.ID = "OpenAI" }, wantErr: "id must be"},
		{name: "missing provider kind", mutate: func(c *ProviderInstanceConfig) { c.ProviderKind = " " }, wantErr: "provider_kind is required"},
		{name: "missing adapter", mutate: func(c *ProviderInstanceConfig) { c.Adapter = "" }, wantErr: "adapter is required"},
		{name: "missing protocol", mutate: func(c *ProviderInstanceConfig) { c.Protocol = "" }, wantErr: "protocol is required"},
		{name: "missing state", mutate: func(c *ProviderInstanceConfig) { c.State = "" }, wantErr: "state must be"},
		{name: "unknown state", mutate: func(c *ProviderInstanceConfig) { c.State = "paused" }, wantErr: "state must be"},
		{name: "empty header", mutate: func(c *ProviderInstanceConfig) { c.Headers = map[string]string{" ": "value"} }, wantErr: "header name must not be empty"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instance := validProviderInstance("fixture")
			if tc.mutate != nil {
				tc.mutate(instance)
			}
			err := instance.Validate()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseExactModelTarget(t *testing.T) {
	target, err := ParseExactModelTarget("openrouter-main/anthropic/claude-sonnet")
	if err != nil {
		t.Fatalf("ParseExactModelTarget() error = %v", err)
	}
	if target.InstanceID != "openrouter-main" || target.ModelID != "anthropic/claude-sonnet" {
		t.Fatalf("target = %#v", target)
	}
	if target.String() != "openrouter-main/anthropic/claude-sonnet" {
		t.Fatalf("String() = %q", target.String())
	}

	for _, raw := range []string{"", "instance", "/model", "Instance/model", "instance/", "instance/model id", "instance/model//variant"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseExactModelTarget(raw); err == nil {
				t.Fatalf("ParseExactModelTarget(%q) expected error", raw)
			}
		})
	}
}

func TestConfigValidateProviderInstancesAndRoutes(t *testing.T) {
	openAI := validProviderInstance("openai-main")
	anthropic := validProviderInstance("anthropic-backup")
	anthropic.ProviderKind = "anthropic"
	anthropic.Adapter = "anthropic-messages"
	anthropic.Protocol = "anthropic"

	cfg := &Config{
		ProviderInstances: []*ProviderInstanceConfig{openAI, anthropic},
		ModelRoutes: []*ModelRouteConfig{{
			Name:    "primary-chat",
			Targets: []string{"openai-main/shared-model", "anthropic-backup/shared-model"},
		}},
	}
	if err := cfg.ValidateProviderInstances(); err != nil {
		t.Fatalf("ValidateProviderInstances() error = %v", err)
	}
	if got := cfg.ModelRoutes[0].Targets; got[0] != "openai-main/shared-model" || got[1] != "anthropic-backup/shared-model" {
		t.Fatalf("route order changed: %#v", got)
	}
}

func TestConfigValidateProviderInstancesRejectsInvalidOwnership(t *testing.T) {
	tests := []struct {
		name    string
		build   func() *Config
		wantErr string
	}{
		{
			name: "duplicate instance ids",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("same"), validProviderInstance("same")}}
			},
			wantErr: "duplicate id",
		},
		{
			name: "disabled instance cannot be routed",
			build: func() *Config {
				instance := validProviderInstance("disabled-one")
				instance.State = ProviderInstanceStateDisabled
				return &Config{ProviderInstances: []*ProviderInstanceConfig{instance}, ModelRoutes: []*ModelRouteConfig{{Name: "chat", Targets: []string{"disabled-one/model"}}}}
			},
			wantErr: "is disabled",
		},
		{
			name: "malformed target",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("one")}, ModelRoutes: []*ModelRouteConfig{{Name: "chat", Targets: []string{"one"}}}}
			},
			wantErr: "instance-id/model-id",
		},
		{
			name: "duplicate target",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("one")}, ModelRoutes: []*ModelRouteConfig{{Name: "chat", Targets: []string{"one/model", "one/model"}}}}
			},
			wantErr: "duplicate target",
		},
		{
			name: "missing instance is not borrowed from another instance",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("configured")}, ModelRoutes: []*ModelRouteConfig{{Name: "chat", Targets: []string{"missing/model"}}}}
			},
			wantErr: `provider instance "missing" not found`,
		},
		{
			name: "empty route",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("one")}, ModelRoutes: []*ModelRouteConfig{{Name: "chat"}}}
			},
			wantErr: "targets must contain",
		},
		{
			name: "duplicate route name",
			build: func() *Config {
				return &Config{ProviderInstances: []*ProviderInstanceConfig{validProviderInstance("one")}, ModelRoutes: []*ModelRouteConfig{{Name: "chat", Targets: []string{"one/a"}}, {Name: "chat", Targets: []string{"one/b"}}}}
			},
			wantErr: "duplicate name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build().ValidateProviderInstances()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateProviderInstances() error = %v, want error containing %q", err, tc.wantErr)
			}
		})
	}
}
