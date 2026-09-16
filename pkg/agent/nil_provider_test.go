package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/bus"
	"github.com/xibodev/facet-studio/pkg/config"
)

func TestProcessDirectWithoutConfiguredProviderFailsCleanly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	loop := NewAgentLoop(cfg, bus.NewMessageBus(), nil)
	defer loop.Close()

	_, err := loop.ProcessDirect(context.Background(), "hello", "nil-provider")
	if err == nil || !strings.Contains(err.Error(), "no active LLM provider configured") {
		t.Fatalf("ProcessDirect() error = %v", err)
	}
}
