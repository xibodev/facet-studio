package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	copilotauth "github.com/xibodev/llm-provider-auth/copilot"
	"github.com/xibodev/facet-studio/pkg/auth"
	"github.com/xibodev/facet-studio/pkg/logger"
	"github.com/xibodev/facet-studio/pkg/providers/openai_compat"
)

// GitHubCopilotHTTPProvider implements LLMProvider and StreamingProvider by exchanging
// GitHub OAuth credentials for dynamic Copilot session tokens and proxying OpenAI-compatible
// requests to the official Copilot endpoint (supporting individual, business, and enterprise).
type GitHubCopilotHTTPProvider struct {
	oauthToken   string
	customBase   string
	defaultModel string
	mu           sync.Mutex
	session      *copilotauth.Session
	subProvider  *openai_compat.Provider
}

// NewGitHubCopilotHTTPProvider creates a new HTTP-based Copilot provider.
func NewGitHubCopilotHTTPProvider(oauthToken, customBase, defaultModel string) *GitHubCopilotHTTPProvider {
	if defaultModel == "" {
		defaultModel = "gpt-4o"
	}
	return &GitHubCopilotHTTPProvider{
		oauthToken:   oauthToken,
		customBase:   customBase,
		defaultModel: defaultModel,
	}
}

func (p *GitHubCopilotHTTPProvider) resolveSession(ctx context.Context, force bool) (*copilotauth.Session, *openai_compat.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().Unix()
	if !force && p.session != nil && p.session.ExpiresAt-now > 60 && p.subProvider != nil {
		return p.session, p.subProvider, nil
	}

	token := strings.TrimSpace(p.oauthToken)
	if token == "" {
		if cred, err := auth.GetCredential("github-copilot"); err == nil && cred != nil {
			token = strings.TrimSpace(cred.AccessToken)
		}
	}
	if token == "" {
		var err error
		token, err = copilotauth.ResolveOAuthToken()
		if err != nil {
			return nil, nil, fmt.Errorf("resolve GitHub Copilot token: %w", err)
		}
	}

	var session *copilotauth.Session
	var err error
	if token != "" {
		session, err = copilotauth.GetSessionForOAuth(token, force)
	} else {
		session, err = copilotauth.GetSession(force)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("exchange Copilot session token: %w", err)
	}

	p.session = session
	baseURL := strings.TrimRight(session.ChatBaseURL, "/")
	if p.customBase != "" {
		baseURL = strings.TrimRight(p.customBase, "/")
	}

	headers := map[string]string{
		"Editor-Version":         "vscode/1.96.2",
		"Editor-Plugin-Version":  "copilot-chat/0.24.1",
		"Copilot-Integration-Id": "vscode-chat",
		"OpenAI-Intent":          "conversation-panel",
		"User-Agent":             "GithubCopilotChat/facet-studio",
	}

	p.subProvider = openai_compat.NewProvider(
		session.Token,
		baseURL,
		"",
		openai_compat.WithProviderName("github-copilot"),
		openai_compat.WithCustomHeaders(headers),
	)

	return p.session, p.subProvider, nil
}

func (p *GitHubCopilotHTTPProvider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	_, sub, err := p.resolveSession(ctx, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}

	resp, err := sub.Chat(ctx, messages, tools, model, options)
	if err != nil && isAuthExpired(err) {
		logger.InfoC("copilot", "Copilot token expired or unauthorized, refreshing session token...")
		_, sub, refreshErr := p.resolveSession(ctx, true)
		if refreshErr == nil {
			return sub.Chat(ctx, messages, tools, model, options)
		}
	}
	return resp, err
}

func (p *GitHubCopilotHTTPProvider) ChatStream(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
	onChunk func(accumulated string),
) (*LLMResponse, error) {
	_, sub, err := p.resolveSession(ctx, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}

	resp, err := sub.ChatStream(ctx, messages, tools, model, options, onChunk)
	if err != nil && isAuthExpired(err) {
		logger.InfoC("copilot", "Copilot token expired or unauthorized during stream, refreshing session token...")
		_, sub, refreshErr := p.resolveSession(ctx, true)
		if refreshErr == nil {
			return sub.ChatStream(ctx, messages, tools, model, options, onChunk)
		}
	}
	return resp, err
}

func (p *GitHubCopilotHTTPProvider) ChatStreamEvents(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
	onChunk func(StreamChunk),
) (*LLMResponse, error) {
	_, sub, err := p.resolveSession(ctx, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(model) == "" {
		model = p.defaultModel
	}

	resp, err := sub.ChatStreamEvents(ctx, messages, tools, model, options, onChunk)
	if err != nil && isAuthExpired(err) {
		logger.InfoC("copilot", "Copilot token expired or unauthorized during stream, refreshing session token...")
		_, sub, refreshErr := p.resolveSession(ctx, true)
		if refreshErr == nil {
			return sub.ChatStreamEvents(ctx, messages, tools, model, options, onChunk)
		}
	}
	return resp, err
}

func (p *GitHubCopilotHTTPProvider) GetDefaultModel() string {
	if p.defaultModel != "" {
		return p.defaultModel
	}
	return "gpt-4o"
}

func isAuthExpired(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "401") || strings.Contains(msg, "expired") || strings.Contains(msg, "unauthorized")
}
