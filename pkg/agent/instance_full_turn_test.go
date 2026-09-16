package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/providers"
)

type fullTurnInstanceProvider struct {
	mu        sync.Mutex
	calls     int
	closed    atomic.Int32
	block     bool
	failFirst bool
	direct    bool
}

func (p *fullTurnInstanceProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	p.mu.Lock()
	p.calls++
	call := p.calls
	p.mu.Unlock()
	if p.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if p.failFirst && call == 1 {
		return nil, &providers.FailoverError{Reason: providers.FailoverNetwork, Wrapped: errors.New("fixture unavailable")}
	}
	if p.direct {
		return &providers.LLMResponse{Content: "selected final"}, nil
	}
	if call == 1 {
		return &providers.LLMResponse{ToolCalls: []providers.ToolCall{{
			ID: "call-1", Type: "function", Name: "mock_custom", Arguments: map[string]any{},
		}}}, nil
	}
	return &providers.LLMResponse{Content: "selected final"}, nil
}

func TestInstanceSelectionChangesWithinOneSession(t *testing.T) {
	providerByTarget := map[string]*fullTurnInstanceProvider{
		"first/model":  {direct: true},
		"second/model": {direct: true},
	}
	resolver := func(_ context.Context, selection string) (*providers.InstanceResolution, error) {
		return fullTurnResolution(t, selection, []string{selection}, func(instance *config.ProviderInstanceConfig, model string, _ string) (providers.LLMProvider, error) {
			return providerByTarget[instance.ID+"/"+model], nil
		}), nil
	}
	al, _ := newFullTurnSelectionLoop(t, resolver)
	for _, selection := range []string{"first/model", "second/model"} {
		if _, err := al.processMessage(context.Background(), selectedPicoMessage("same-session", selection, selection)); err != nil {
			t.Fatal(err)
		}
	}
	history := al.registry.GetDefaultAgent().Sessions.GetHistory("same-session")
	if len(history) != 4 || history[0].RequestedSelection != "first/model" || history[1].ServedTarget != "first/model" || history[2].RequestedSelection != "second/model" || history[3].ServedTarget != "second/model" {
		t.Fatalf("history selection continuity = %#v", history)
	}
}

func TestInstanceSelectionRejectsHookRewriteAndMediaReroute(t *testing.T) {
	provider := &fullTurnInstanceProvider{direct: true}
	selection := "first/model"
	resolution := fullTurnResolution(t, selection, []string{selection}, func(*config.ProviderInstanceConfig, string, string) (providers.LLMProvider, error) {
		return provider, nil
	})
	al, _ := newFullTurnSelectionLoop(t, func(context.Context, string) (*providers.InstanceResolution, error) { return resolution, nil })
	if err := al.MountHook(NamedHook("instance-rewrite", modelRewriteHook{model: "legacy-other"})); err != nil {
		t.Fatal(err)
	}
	_, err := al.processMessage(context.Background(), selectedPicoMessage("hook-session", selection, "rewrite"))
	if err == nil || !strings.Contains(err.Error(), "hook model rewrites are not supported for instance-selected turns") {
		t.Fatalf("hook rewrite error = %v", err)
	}

	al.hooks = NewHookManager(al.runtimeEvents.Channel())
	mediaMessage := selectedPicoMessage("media-session", selection, "inspect")
	mediaMessage.Media = []string{"data:image/png;base64,iVBORw0KGgo="}
	_, err = al.processMessage(context.Background(), mediaMessage)
	if err == nil || !strings.Contains(err.Error(), "media model rerouting is not supported for instance-selected turns") {
		t.Fatalf("media reroute error = %v", err)
	}
}

func (p *fullTurnInstanceProvider) GetDefaultModel() string { return "fixture" }
func (p *fullTurnInstanceProvider) Close()                  { p.closed.Add(1) }

func fullTurnResolution(t *testing.T, selection string, targets []string, factory providers.InstanceProviderFactory) *providers.InstanceResolution {
	t.Helper()
	cfg := &config.Config{}
	catalogs := make(map[string]providers.InstanceCatalog, len(targets))
	for _, raw := range targets {
		target, err := config.ParseExactModelTarget(raw)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ProviderInstances = append(cfg.ProviderInstances, &config.ProviderInstanceConfig{
			ID: target.InstanceID, ProviderKind: "openai", Adapter: "fixture", Protocol: "openai",
			State: config.ProviderInstanceStateEnabled, Settings: map[string]any{"streaming": true},
		})
		catalogs[target.InstanceID] = providers.InstanceCatalog{InstanceID: target.InstanceID, Models: []string{target.ModelID}}
	}
	if len(targets) > 1 {
		cfg.ModelRoutes = []*config.ModelRouteConfig{{Name: selection, Targets: targets}}
	}
	resolved, err := providers.ResolveInstanceTargetOrRoute(cfg, catalogs, selection, nil, factory)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func newFullTurnSelectionLoop(t *testing.T, resolver InstanceSelectionResolver) (*AgentLoop, *bus.MessageBus) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Agents.Defaults.ModelName = "legacy"
	cfg.Agents.Defaults.MaxToolIterations = 4
	cfg.ModelList = []*config.ModelConfig{{ModelName: "legacy", Provider: "openai", Model: "legacy", APIBase: "http://127.0.0.1:1"}}
	msgBus := bus.NewMessageBus()
	al := NewAgentLoop(cfg, msgBus, &mockProvider{}, WithInstanceSelectionResolver(resolver))
	al.registry.GetDefaultAgent().Tools.Register(&mockCustomTool{})
	return al, msgBus
}

func selectedPicoMessage(sessionKey, selection, content string) bus.InboundMessage {
	return bus.InboundMessage{
		Context: bus.InboundContext{
			Channel: "pico", ChatID: "pico:" + sessionKey, ChatType: "direct", SenderID: "pico-user",
			Raw: map[string]string{bus.MetadataKeyModelSelection: selection},
		},
		Content: content, SessionKey: sessionKey,
	}
}

func TestInstanceSelectionPicoFullTurnToolLoopHistoryAndOutbound(t *testing.T) {
	provider := &fullTurnInstanceProvider{}
	selection := "first/model"
	resolution := fullTurnResolution(t, selection, []string{selection}, func(*config.ProviderInstanceConfig, string, string) (providers.LLMProvider, error) {
		return provider, nil
	})
	al, msgBus := newFullTurnSelectionLoop(t, func(context.Context, string) (*providers.InstanceResolution, error) { return resolution, nil })

	al.runTurnWithSteering(context.Background(), selectedPicoMessage("session-1", selection, "use the tool"))
	if provider.calls != 2 || provider.closed.Load() != 1 {
		t.Fatalf("provider calls=%d closed=%d", provider.calls, provider.closed.Load())
	}
	history := al.registry.GetDefaultAgent().Sessions.GetHistory("session-1")
	if len(history) < 4 {
		t.Fatalf("history = %#v", history)
	}
	if history[0].RequestedSelection != selection {
		t.Fatalf("user requested selection = %q", history[0].RequestedSelection)
	}
	final := history[len(history)-1]
	if final.ModelName != selection || final.RequestedSelection != selection || final.ServedTarget != selection || final.ServedIdentity == "" {
		t.Fatalf("final history identity = %#v", final)
	}

	deadline := time.After(time.Second)
	for {
		select {
		case outbound := <-msgBus.OutboundChan():
			if outbound.Content != "selected final" {
				continue
			}
			if outbound.Context.Raw[bus.MetadataKeyModelSelection] != selection || outbound.Context.Raw[bus.MetadataKeyServedTarget] != selection || outbound.Context.Raw[bus.MetadataKeyServedIdentity] == "" {
				t.Fatalf("outbound identity = %#v", outbound.Context.Raw)
			}
			return
		case <-deadline:
			t.Fatal("final Pico outbound not published")
		}
	}
}

func TestInstanceSelectionOrderedFallbackIsLazyAndOwned(t *testing.T) {
	primary := &fullTurnInstanceProvider{failFirst: true}
	fallback := &fullTurnInstanceProvider{calls: 1}
	created := make(map[string]int)
	resolution := fullTurnResolution(t, "route", []string{"first/model", "second/model"}, func(instance *config.ProviderInstanceConfig, _ string, _ string) (providers.LLMProvider, error) {
		created[instance.ID]++
		if instance.ID == "first" {
			return primary, nil
		}
		return fallback, nil
	})
	al, _ := newFullTurnSelectionLoop(t, func(context.Context, string) (*providers.InstanceResolution, error) { return resolution, nil })

	response, err := al.processMessage(context.Background(), selectedPicoMessage("fallback-session", "route", "answer"))
	if err != nil || response != "selected final" {
		t.Fatalf("response=%q error=%v", response, err)
	}
	if created["first"] != 1 || created["second"] != 1 || primary.closed.Load() != 1 || fallback.closed.Load() != 1 {
		t.Fatalf("created=%v closed=%d/%d", created, primary.closed.Load(), fallback.closed.Load())
	}
	history := al.registry.GetDefaultAgent().Sessions.GetHistory("fallback-session")
	final := history[len(history)-1]
	if final.RequestedSelection != "route" || final.ServedTarget != "second/model" || final.ServedIdentity == "" {
		t.Fatalf("fallback identity = %#v", final)
	}
}

func TestInstanceSelectionSuccessfulPrimaryDoesNotConstructFallback(t *testing.T) {
	primary := &fullTurnInstanceProvider{direct: true}
	created := make(map[string]int)
	resolution := fullTurnResolution(t, "route", []string{"first/model", "second/model"}, func(instance *config.ProviderInstanceConfig, _ string, _ string) (providers.LLMProvider, error) {
		created[instance.ID]++
		return primary, nil
	})
	al, _ := newFullTurnSelectionLoop(t, func(context.Context, string) (*providers.InstanceResolution, error) { return resolution, nil })
	if _, err := al.processMessage(context.Background(), selectedPicoMessage("lazy-session", "route", "answer")); err != nil {
		t.Fatal(err)
	}
	if created["first"] != 1 || created["second"] != 0 || primary.closed.Load() != 1 {
		t.Fatalf("created=%v primary closed=%d", created, primary.closed.Load())
	}
}

func TestInstanceSelectionOverlappingTurnsOwnProvidersAndCancel(t *testing.T) {
	var createdMu sync.Mutex
	var created []*fullTurnInstanceProvider
	resolver := func(context.Context, string) (*providers.InstanceResolution, error) {
		return fullTurnResolution(t, "first/model", []string{"first/model"}, func(*config.ProviderInstanceConfig, string, string) (providers.LLMProvider, error) {
			provider := &fullTurnInstanceProvider{block: true}
			createdMu.Lock()
			created = append(created, provider)
			createdMu.Unlock()
			return provider, nil
		}), nil
	}
	al, _ := newFullTurnSelectionLoop(t, resolver)
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	errs := make(chan error, 2)
	go func() {
		_, err := al.processMessage(ctx1, selectedPicoMessage("one", "first/model", "one"))
		errs <- err
	}()
	go func() {
		_, err := al.processMessage(ctx2, selectedPicoMessage("two", "first/model", "two"))
		errs <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		createdMu.Lock()
		count := len(created)
		createdMu.Unlock()
		if count == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel1()
	cancel2()
	for range 2 {
		if err := <-errs; !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	}
	createdMu.Lock()
	defer createdMu.Unlock()
	if len(created) != 2 || created[0] == created[1] || created[0].closed.Load() != 1 || created[1].closed.Load() != 1 {
		t.Fatalf("providers = %#v closed=%d/%d", created, created[0].closed.Load(), created[1].closed.Load())
	}
}
