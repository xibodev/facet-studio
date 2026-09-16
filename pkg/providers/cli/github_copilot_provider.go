package cliprovider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	copilot "github.com/github/copilot-sdk/go"
	"github.com/github/copilot-sdk/go/rpc"
	"github.com/xibodev/facet-studio/pkg/config"
)

type GitHubCopilotProvider struct {
	client *copilot.Client
	model  string
	mu     sync.Mutex
}

func NewGitHubCopilotProvider(uri string, connectMode string, model string) (*GitHubCopilotProvider, error) {
	if connectMode == "" {
		connectMode = "stdio"
	}

	switch connectMode {
	case "stdio":
		client := newNativeGitHubCopilotClient(copilot.ModeEmpty)
		if err := client.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("start GitHub Copilot CLI: %w", err)
		}
		return &GitHubCopilotProvider{client: client, model: model}, nil
	case "grpc":
		client := copilot.NewClient(&copilot.ClientOptions{
			Connection: copilot.URIConnection{URL: uri},
		})
		if err := client.Start(context.Background()); err != nil {
			return nil, fmt.Errorf(
				"can't connect to Github Copilot: %w; `https://github.com/github/copilot-sdk/blob/main/docs/getting-started.md#connecting-to-an-external-cli-server` for details",
				err,
			)
		}

		return &GitHubCopilotProvider{client: client, model: model}, nil
	default:
		return nil, fmt.Errorf("unknown connect mode: %s", connectMode)
	}
}

func (p *GitHubCopilotProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		p.client.Stop()
		p.client = nil
	}
}

func (p *GitHubCopilotProvider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	if len(tools) != 0 {
		return nil, fmt.Errorf("GitHub Copilot native adapter does not yet support Studio tool calls")
	}
	if len(messages) != 1 || strings.ToLower(strings.TrimSpace(messages[0].Role)) != "user" {
		return nil, fmt.Errorf("GitHub Copilot native adapter currently supports one text-only user message per turn")
	}
	if strings.TrimSpace(model) == "" {
		model = p.model
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil, fmt.Errorf("provider closed")
	}
	session, err := p.client.CreateSession(ctx, githubCopilotTextSessionConfig(model))
	if err != nil {
		return nil, fmt.Errorf("create text-only Copilot session: %w", err)
	}
	defer session.Disconnect()
	resp, err := session.SendAndWait(ctx, copilot.MessageOptions{
		Prompt: messages[0].Content,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message to copilot: %w", err)
	}

	if resp == nil {
		return nil, fmt.Errorf("empty response from copilot")
	}
	data, ok := resp.Data.(*copilot.AssistantMessageData)
	if !ok {
		return nil, fmt.Errorf("no content in copilot response")
	}
	if len(data.ToolRequests) != 0 {
		return nil, fmt.Errorf("GitHub Copilot returned unsupported tool requests")
	}
	content := data.Content

	return &LLMResponse{
		FinishReason: "stop",
		Content:      content,
	}, nil
}

func (p *GitHubCopilotProvider) GetDefaultModel() string {
	return "gpt-4.1"
}

func newNativeGitHubCopilotClient(mode copilot.ClientMode) *copilot.Client {
	return newGitHubCopilotClient(copilot.StdioConnection{Path: "copilot"}, mode)
}

func newGitHubCopilotClient(connection copilot.RuntimeConnection, mode copilot.ClientMode) *copilot.Client {
	useLoggedInUser := true
	options := &copilot.ClientOptions{
		Connection:      connection,
		UseLoggedInUser: &useLoggedInUser,
		Env:             copilotLoggedInUserEnvironment(),
		Mode:            mode,
	}
	if mode == copilot.ModeEmpty {
		options.BaseDirectory = filepath.Join(config.GetHome(), "providers", "github-copilot")
	}
	return copilot.NewClient(options)
}

func githubCopilotTextSessionConfig(model string) *copilot.SessionConfig {
	return &copilot.SessionConfig{
		Model:          model,
		AvailableTools: []string{},
		OnPermissionRequest: func(_ copilot.PermissionRequest, _ copilot.PermissionInvocation) (rpc.PermissionDecision, error) {
			return &rpc.PermissionDecisionReject{}, nil
		},
	}
}

func copilotLoggedInUserEnvironment() []string {
	blocked := map[string]struct{}{
		"COPILOT_GITHUB_TOKEN": {},
		"GH_TOKEN":             {},
		"GITHUB_TOKEN":         {},
	}
	environment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, skip := blocked[strings.ToUpper(name)]; skip {
				continue
			}
		}
		environment = append(environment, entry)
	}
	return environment
}

func InspectGitHubCopilot(ctx context.Context) (*copilot.GetAuthStatusResponse, []copilot.ModelInfo, error) {
	client := newNativeGitHubCopilotClient(copilot.ModeCopilotCli)
	if err := client.Start(ctx); err != nil {
		return nil, nil, fmt.Errorf("start GitHub Copilot CLI: %w", err)
	}
	defer client.Stop()
	authStatus, err := client.GetAuthStatus(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get GitHub Copilot authentication status: %w", err)
	}
	if !authStatus.IsAuthenticated {
		return authStatus, nil, fmt.Errorf("GitHub Copilot CLI is not authenticated")
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		return authStatus, nil, fmt.Errorf("list GitHub Copilot models: %w", err)
	}
	return authStatus, models, nil
}
