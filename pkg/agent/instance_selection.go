package agent

import (
	"context"
	"fmt"

	"github.com/xibodev/facet-studio/pkg/providers"
)

type InstanceSelectionResult struct {
	Response       *providers.LLMResponse
	ServedIdentity string
	ExactTarget    string
}

// ProcessInstanceSelection invokes one explicit instance target or named route.
// It is intentionally separate from legacy/default turn selection.
func (al *AgentLoop) ProcessInstanceSelection(
	ctx context.Context,
	selection string,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	options map[string]any,
	onChunk func(string),
) (*InstanceSelectionResult, error) {
	if al.instanceSelectionResolver == nil {
		return nil, fmt.Errorf("instance selection resolver is not configured")
	}
	resolved, err := al.instanceSelectionResolver(ctx, selection)
	if err != nil {
		return nil, err
	}
	if resolved == nil || len(resolved.Candidates) == 0 {
		return nil, fmt.Errorf("instance selection resolved no candidates")
	}

	result, err := al.fallback.ExecuteCandidate(ctx, resolved.Candidates, func(
		ctx context.Context,
		candidate providers.FallbackCandidate,
	) (*providers.LLMResponse, error) {
		provider, providerErr := resolved.ProviderForCandidate(candidate)
		if providerErr != nil {
			return nil, providerErr
		}
		if provider == nil {
			return nil, fmt.Errorf("provider missing for %s", candidate.StableKey())
		}
		if onChunk == nil {
			return provider.Chat(ctx, messages, tools, candidate.Model, options)
		}
		streaming, ok := provider.(providers.StreamingProvider)
		if !ok {
			return provider.Chat(ctx, messages, tools, candidate.Model, options)
		}
		visible := false
		response, streamErr := streaming.ChatStream(ctx, messages, tools, candidate.Model, options, func(chunk string) {
			if chunk != "" {
				visible = true
			}
			onChunk(chunk)
		})
		if streamErr != nil && visible {
			return nil, &providers.FailoverError{
				Reason:   providers.FailoverFormat,
				Provider: candidate.Provider,
				Model:    candidate.Model,
				Wrapped:  visibleInstanceStreamError{err: streamErr},
			}
		}
		return response, streamErr
	})
	if err != nil {
		return nil, err
	}
	served := result.IdentityKey
	exactTarget := ""
	for _, candidate := range resolved.Candidates {
		if candidate.StableKey() == served {
			exactTarget = candidate.DisplayName
			break
		}
	}
	return &InstanceSelectionResult{
		Response:       result.Response,
		ServedIdentity: served,
		ExactTarget:    exactTarget,
	}, nil
}

type visibleInstanceStreamError struct{ err error }

func (e visibleInstanceStreamError) Error() string { return e.err.Error() }
func (e visibleInstanceStreamError) Unwrap() error { return e.err }
