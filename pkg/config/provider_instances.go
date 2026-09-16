package config

import (
	"fmt"
	"regexp"
	"strings"
)

type ProviderInstanceState string

const (
	ProviderInstanceStateEnabled  ProviderInstanceState = "enabled"
	ProviderInstanceStateDisabled ProviderInstanceState = "disabled"
)

var providerInstanceIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)

// ProviderInstanceConfig owns one provider adapter connection independently of
// any model selected from its catalog.
type ProviderInstanceConfig struct {
	ID                string                `json:"id"`
	ProviderKind      string                `json:"provider_kind"`
	Adapter           string                `json:"adapter"`
	Protocol          string                `json:"protocol"`
	Endpoint          string                `json:"endpoint,omitempty"`
	AuthConnectionRef string                `json:"auth_connection_ref,omitempty"`
	Headers           map[string]string     `json:"headers,omitempty"`
	Settings          map[string]any        `json:"settings,omitempty"`
	State             ProviderInstanceState `json:"state"`
}

func (c *ProviderInstanceConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("provider instance is required")
	}
	if !providerInstanceIDPattern.MatchString(c.ID) {
		return fmt.Errorf("id must be a stable lowercase identifier using letters, numbers, '.', '_', or '-'")
	}
	if strings.TrimSpace(c.ProviderKind) == "" {
		return fmt.Errorf("provider_kind is required")
	}
	if strings.TrimSpace(c.Adapter) == "" {
		return fmt.Errorf("adapter is required")
	}
	if strings.TrimSpace(c.Protocol) == "" {
		return fmt.Errorf("protocol is required")
	}
	switch c.State {
	case ProviderInstanceStateEnabled, ProviderInstanceStateDisabled:
	default:
		return fmt.Errorf("state must be %q or %q", ProviderInstanceStateEnabled, ProviderInstanceStateDisabled)
	}
	for name := range c.Headers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("header name must not be empty")
		}
	}
	return nil
}

// ExactModelTarget identifies one model owned by one configured provider
// instance. ModelID may contain additional slashes.
type ExactModelTarget struct {
	InstanceID string
	ModelID    string
}

func ParseExactModelTarget(raw string) (ExactModelTarget, error) {
	raw = strings.TrimSpace(raw)
	instanceID, modelID, found := strings.Cut(raw, "/")
	if !found || !providerInstanceIDPattern.MatchString(instanceID) {
		return ExactModelTarget{}, fmt.Errorf("target must be instance-id/model-id")
	}
	if modelID == "" || strings.TrimSpace(modelID) != modelID || strings.ContainsAny(modelID, "\t\n\r ") || strings.Contains(modelID, "//") {
		return ExactModelTarget{}, fmt.Errorf("target model-id is invalid")
	}
	return ExactModelTarget{InstanceID: instanceID, ModelID: modelID}, nil
}

func (t ExactModelTarget) String() string {
	return t.InstanceID + "/" + t.ModelID
}

// ModelRouteConfig is an ordered failover route of exact instance-owned
// targets. Ordering is significant and duplicates are invalid.
type ModelRouteConfig struct {
	Name    string   `json:"name"`
	Targets []string `json:"targets"`
}

func (c *Config) ValidateProviderInstances() error {
	instances := make(map[string]*ProviderInstanceConfig, len(c.ProviderInstances))
	for i, instance := range c.ProviderInstances {
		if err := instance.Validate(); err != nil {
			return fmt.Errorf("provider_instances[%d]: %w", i, err)
		}
		if _, exists := instances[instance.ID]; exists {
			return fmt.Errorf("provider_instances[%d]: duplicate id %q", i, instance.ID)
		}
		instances[instance.ID] = instance
	}

	routes := make(map[string]struct{}, len(c.ModelRoutes))
	for i, route := range c.ModelRoutes {
		if err := validateModelRoute(route, instances); err != nil {
			return fmt.Errorf("model_routes[%d]: %w", i, err)
		}
		if _, exists := routes[route.Name]; exists {
			return fmt.Errorf("model_routes[%d]: duplicate name %q", i, route.Name)
		}
		routes[route.Name] = struct{}{}
	}
	return nil
}

func validateModelRoute(route *ModelRouteConfig, instances map[string]*ProviderInstanceConfig) error {
	if route == nil {
		return fmt.Errorf("route is required")
	}
	if !providerInstanceIDPattern.MatchString(route.Name) {
		return fmt.Errorf("name must be a stable lowercase identifier using letters, numbers, '.', '_', or '-'")
	}
	if len(route.Targets) == 0 {
		return fmt.Errorf("targets must contain at least one exact target")
	}
	seen := make(map[string]struct{}, len(route.Targets))
	for i, raw := range route.Targets {
		target, err := ParseExactModelTarget(raw)
		if err != nil {
			return fmt.Errorf("targets[%d]: %w", i, err)
		}
		instance, exists := instances[target.InstanceID]
		if !exists {
			return fmt.Errorf("targets[%d]: provider instance %q not found", i, target.InstanceID)
		}
		if instance.State == ProviderInstanceStateDisabled {
			return fmt.Errorf("targets[%d]: provider instance %q is disabled", i, target.InstanceID)
		}
		key := target.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("targets[%d]: duplicate target %q", i, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}
