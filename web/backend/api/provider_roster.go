package api

import (
	"net/http"
	"sort"
	"strings"

	llmgwproviders "github.com/xibodev/llmgw-core/providers"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/providers"
)

type providerRosterResponse struct {
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

func (h *Handler) handleListProviderRoster(w http.ResponseWriter, r *http.Request) {
	cfg, _ := config.LoadConfig(h.configPath)
	configuredByKind := make(map[string][]string)
	if cfg != nil {
		for _, inst := range cfg.ProviderInstances {
			if inst == nil {
				continue
			}
			normKind := providers.NormalizeProvider(inst.ProviderKind)
			configuredByKind[normKind] = append(configuredByKind[normKind], inst.ID)
			configuredByKind[inst.ProviderKind] = append(configuredByKind[inst.ProviderKind], inst.ID)
			configuredByKind[inst.ID] = append(configuredByKind[inst.ID], inst.ID)
		}
	}

	rosterMap := make(map[string]providerRosterResponse)

	// 1. Populate from curated llmgw-core ProviderRegistry
	for _, reg := range llmgwproviders.ProviderRegistry() {
		entry := providerRosterResponse{
			ID:                  reg.ID,
			DisplayName:         reg.Label,
			Label:               reg.Label,
			Description:         reg.Description,
			Categories:          append([]string(nil), reg.Categories...),
			DefaultEndpoint:     reg.DefaultBaseURL,
			RequiresAPIKey:      reg.RequiresAPIKey,
			RequiresBaseURL:     reg.RequiresBaseURL,
			AnonymousAutomation: reg.AnonymousAutomation,
			AuthMethods:         append([]string(nil), reg.AuthMethods...),
			OnboardingFields:    append([]string(nil), reg.OnboardingFields...),
			Compatibility:       "discovery_only",
		}

		if reg.Protocol != "" {
			entry.Protocol = reg.Protocol
		} else {
			entry.Protocol = reg.RuntimeType
		}

		normID := providers.NormalizeProvider(reg.ID)
		if normID == "github-copilot" {
			entry.Adapter = "github-copilot-native"
			entry.Protocol = "github-copilot"
			entry.Compatibility = "native"
		} else if strings.EqualFold(reg.Protocol, "anthropic") || strings.EqualFold(reg.RuntimeType, "anthropic") {
			entry.Adapter = "anthropic-compatible"
			entry.Compatibility = "compatible"
		} else if adapter, ok := compatibleRosterAdapter(reg.ID); ok {
			entry.Adapter = adapter
			entry.Compatibility = "compatible"
		} else if strings.EqualFold(reg.Protocol, "openai") || strings.EqualFold(reg.RuntimeType, "openai") {
			entry.Adapter = "openai-compatible"
			entry.Compatibility = "compatible"
		}

		insts := configuredByKind[reg.ID]
		if len(insts) == 0 {
			insts = configuredByKind[normID]
		}
		if len(insts) > 0 {
			entry.Configured = true
			entry.InstanceCount = len(insts)
			entry.ConfiguredInstances = insts
		}

		rosterMap[reg.ID] = entry
	}

	// 2. Overlay Studio-native options if missing
	for _, option := range providers.ModelProviderOptions() {
		normID := providers.NormalizeProvider(option.ID)
		if existing, ok := rosterMap[option.ID]; ok {
			if existing.DefaultEndpoint == "" && option.DefaultAPIBase != "" {
				existing.DefaultEndpoint = option.DefaultAPIBase
				rosterMap[option.ID] = existing
			}
			continue
		}
		if _, ok := rosterMap[normID]; ok {
			continue
		}

		entry := providerRosterResponse{
			ID:              option.ID,
			DisplayName:     option.DisplayName,
			Label:           option.DisplayName,
			DefaultEndpoint: option.DefaultAPIBase,
			Compatibility:   "discovery_only",
			RequiresAPIKey:  true,
		}
		if normID == "github-copilot" {
			entry.Adapter = "github-copilot-native"
			entry.Protocol = "github-copilot"
			entry.Compatibility = "native"
		} else if adapter, ok := compatibleRosterAdapter(option.ID); ok {
			entry.Adapter = adapter
			entry.Protocol = option.ID
			entry.Compatibility = "compatible"
		}

		insts := configuredByKind[option.ID]
		if len(insts) == 0 {
			insts = configuredByKind[normID]
		}
		if len(insts) > 0 {
			entry.Configured = true
			entry.InstanceCount = len(insts)
			entry.ConfiguredInstances = insts
		}
		rosterMap[option.ID] = entry
	}

	roster := make([]providerRosterResponse, 0, len(rosterMap))
	for _, entry := range rosterMap {
		roster = append(roster, entry)
	}

	sort.Slice(roster, func(i, j int) bool { return roster[i].ID < roster[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"providers": roster, "total": len(roster)})
}

func compatibleRosterAdapter(provider string) (string, bool) {
	switch providers.NormalizeProvider(provider) {
	case "openai", "litellm", "lmstudio", "gpt4free", "openrouter", "groq", "zhipu", "nvidia", "venice",
		"nearai", "ollama", "moonshot", "shengsuanyun", "siliconflow", "deepseek", "cerebras",
		"vivgrid", "volcengine", "vllm", "qwen-portal", "qwen-intl", "qwen-us", "mistral",
		"avian", "longcat", "modelscope", "novita", "alibaba-coding", "zai", "mimo", "minimax",
		"pollinations", "kilo_code", "kilo-code", "llm7", "ovh_ai_endpoints", "ovh-ai", "opencode_zen", "opencode-zen":
		return "openai-compatible", true
	case "anthropic-messages", "alibaba-coding-anthropic", "anthropic":
		return "anthropic-compatible", true
	default:
		return "", false
	}
}
