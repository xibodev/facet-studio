package providers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/xibodev/facet-studio/pkg/config"
)

// InstanceCatalog is the model identity snapshot owned by one configured
// provider instance. Callers load this from their catalog persistence layer.
type InstanceCatalog struct {
	InstanceID string
	Models     []string
}

// InstanceProviderFactory creates a runtime provider from one exact target's
// owning instance. secret is resolved from that instance's auth reference.
type InstanceProviderFactory func(
	instance *config.ProviderInstanceConfig,
	modelID string,
	secret string,
) (LLMProvider, error)

// InstanceCredentialResolver resolves an instance auth connection reference.
type InstanceCredentialResolver func(ref string) (string, error)

// InstanceResolution contains ordered fallback candidates and their exact
// instance-owned providers, keyed by FallbackCandidate.StableKey().
type InstanceResolution struct {
	Candidates        []FallbackCandidate
	bindings          map[string]instanceTargetBinding
	resolveCredential InstanceCredentialResolver
	createProvider    InstanceProviderFactory
	mu                sync.Mutex
	providers         map[string]LLMProvider
}

type instanceTargetBinding struct {
	instance *config.ProviderInstanceConfig
	modelID  string
}

func (r *InstanceResolution) ProviderForCandidate(candidate FallbackCandidate) (LLMProvider, error) {
	if r == nil {
		return nil, fmt.Errorf("instance resolution is required")
	}
	key := candidate.StableKey()
	r.mu.Lock()
	defer r.mu.Unlock()
	if provider := r.providers[key]; provider != nil {
		return provider, nil
	}
	binding, ok := r.bindings[key]
	if !ok {
		return nil, fmt.Errorf("instance binding missing for %s", key)
	}
	secret := ""
	if ref := strings.TrimSpace(binding.instance.AuthConnectionRef); ref != "" {
		if r.resolveCredential == nil {
			return nil, fmt.Errorf("credential resolver is required")
		}
		var err error
		secret, err = r.resolveCredential(ref)
		if err != nil {
			return nil, fmt.Errorf("resolve provider instance credential: %w", err)
		}
	}
	provider, err := r.createProvider(cloneProviderInstance(binding.instance), binding.modelID, secret)
	if err != nil {
		return nil, err
	}
	r.providers[key] = provider
	return provider, nil
}

func (r *InstanceResolution) ModelConfigForCandidate(candidate FallbackCandidate) (*config.ModelConfig, error) {
	if r == nil {
		return nil, fmt.Errorf("instance resolution is required")
	}
	binding, ok := r.bindings[candidate.StableKey()]
	if !ok {
		return nil, fmt.Errorf("instance binding missing for %s", candidate.StableKey())
	}
	return modelConfigFromInstance(binding.instance, binding.modelID, ""), nil
}

// ResolveInstanceTargetOrRoute resolves either an exact instance-id/model-id
// target or a named route. It never consults legacy ModelConfig templates.
func ResolveInstanceTargetOrRoute(
	cfg *config.Config,
	catalogs map[string]InstanceCatalog,
	selection string,
	resolveCredential InstanceCredentialResolver,
	createProvider InstanceProviderFactory,
) (*InstanceResolution, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return nil, fmt.Errorf("target or route is required")
	}
	if createProvider == nil {
		createProvider = CreateProviderFromInstance
	}

	targets := []string{selection}
	if _, err := config.ParseExactModelTarget(selection); err != nil {
		route := findModelRoute(cfg.ModelRoutes, selection)
		if route == nil {
			return nil, fmt.Errorf("target %q is invalid and route was not found", selection)
		}
		targets = route.Targets
	}

	instances := make(map[string]*config.ProviderInstanceConfig, len(cfg.ProviderInstances))
	for _, instance := range cfg.ProviderInstances {
		if instance != nil {
			instances[instance.ID] = instance
		}
	}
	resolution := &InstanceResolution{
		Candidates:        make([]FallbackCandidate, 0, len(targets)),
		bindings:          make(map[string]instanceTargetBinding, len(targets)),
		providers:         make(map[string]LLMProvider, len(targets)),
		resolveCredential: resolveCredential,
		createProvider:    createProvider,
	}
	for index, raw := range targets {
		target, err := config.ParseExactModelTarget(raw)
		if err != nil {
			return nil, fmt.Errorf("target[%d]: %w", index, err)
		}
		instance := instances[target.InstanceID]
		if instance == nil {
			return nil, fmt.Errorf("target[%d]: provider instance %q not found", index, target.InstanceID)
		}
		if instance.State != config.ProviderInstanceStateEnabled {
			return nil, fmt.Errorf("target[%d]: provider instance %q is disabled", index, target.InstanceID)
		}
		catalog, ok := catalogs[target.InstanceID]
		if !ok || catalog.InstanceID != target.InstanceID {
			return nil, fmt.Errorf("target[%d]: catalog for provider instance %q not found", index, target.InstanceID)
		}
		if !catalogContainsModel(catalog, target.ModelID) {
			return nil, fmt.Errorf("target[%d]: model %q not found in provider instance %q catalog", index, target.ModelID, target.InstanceID)
		}

		candidate := FallbackCandidate{
			Provider:    strings.TrimSpace(instance.Protocol),
			Model:       target.ModelID,
			DisplayName: raw,
			IdentityKey: "provider_instance:" + target.InstanceID,
			ConfigKey:   "instance_target:" + target.String(),
		}
		resolution.Candidates = append(resolution.Candidates, candidate)
		resolution.bindings[candidate.StableKey()] = instanceTargetBinding{
			instance: cloneProviderInstance(instance), modelID: target.ModelID,
		}
	}
	return resolution, nil
}

// CreateProviderFromInstance adapts one provider instance to Studio's existing
// provider factory without consulting another instance or legacy templates.
func CreateProviderFromInstance(
	instance *config.ProviderInstanceConfig,
	modelID string,
	secret string,
) (LLMProvider, error) {
	if instance == nil {
		return nil, fmt.Errorf("provider instance is required")
	}
	modelCfg := modelConfigFromInstance(instance, modelID, secret)
	provider, resolvedModel, err := CreateProviderFromConfig(modelCfg)
	if err != nil {
		return nil, err
	}
	if resolvedModel != modelID {
		return nil, fmt.Errorf("provider factory resolved model %q, want %q", resolvedModel, modelID)
	}
	return provider, nil
}

func modelConfigFromInstance(instance *config.ProviderInstanceConfig, modelID, secret string) *config.ModelConfig {
	modelCfg := &config.ModelConfig{
		ModelName:     instance.ID + "/" + modelID,
		Provider:      strings.TrimSpace(instance.Protocol),
		Model:         modelID,
		APIBase:       strings.TrimSpace(instance.Endpoint),
		CustomHeaders: cloneStringMap(instance.Headers),
	}
	if secret != "" {
		modelCfg.SetAPIKey(secret)
	}
	applyProviderInstanceRuntimeSettings(modelCfg, instance.Settings)
	return modelCfg
}

func findModelRoute(routes []*config.ModelRouteConfig, name string) *config.ModelRouteConfig {
	for _, route := range routes {
		if route != nil && route.Name == name {
			return route
		}
	}
	return nil
}

func catalogContainsModel(catalog InstanceCatalog, modelID string) bool {
	for _, candidate := range catalog.Models {
		if candidate == modelID {
			return true
		}
	}
	return false
}

func cloneProviderInstance(instance *config.ProviderInstanceConfig) *config.ProviderInstanceConfig {
	clone := *instance
	clone.Headers = cloneStringMap(instance.Headers)
	clone.Settings = make(map[string]any, len(instance.Settings))
	for name, value := range instance.Settings {
		clone.Settings[name] = value
	}
	return &clone
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	clone := make(map[string]string, len(source))
	for name, value := range source {
		clone[name] = value
	}
	return clone
}

func applyProviderInstanceRuntimeSettings(modelCfg *config.ModelConfig, settings map[string]any) {
	if streaming, ok := settings["streaming"].(bool); ok {
		modelCfg.Streaming.Enabled = streaming
	}
	if streaming, ok := settings["streaming"].(map[string]any); ok {
		if enabled, ok := streaming["enabled"].(bool); ok {
			modelCfg.Streaming.Enabled = enabled
		}
	}
	if proxy, ok := settings["proxy"].(string); ok {
		modelCfg.Proxy = proxy
	}
	if field, ok := settings["max_tokens_field"].(string); ok {
		modelCfg.MaxTokensField = field
	}
	if transform, ok := settings["tool_schema_transform"].(string); ok {
		modelCfg.ToolSchemaTransform = transform
	}
	if timeout, ok := instanceSettingInt(settings["request_timeout"]); ok {
		modelCfg.RequestTimeout = timeout
	}
	if extraBody, ok := settings["extra_body"].(map[string]any); ok {
		modelCfg.ExtraBody = extraBody
	}
}

func instanceSettingInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	}
	return 0, false
}
