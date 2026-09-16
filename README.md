# Facet Studio

<p align="center">
  <strong>Local-First Agent Cockpit &amp; In-Process Embeddable Runtime Kernel</strong>
</p>

<p align="center">
  <a href="https://github.com/xibodev/facet-studio/releases/latest"><img src="https://img.shields.io/github/v/release/xibodev/facet-studio?color=38bdf8&label=release" alt="Release"></a>
  <a href="https://github.com/xibodev/facet-studio/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://xibodev.github.io/facet-studio/"><img src="https://img.shields.io/badge/docs-website-7c3aed.svg" alt="Documentation"></a>
</p>

---

Facet Studio is one complete, local-first agent application with a headed web experience and an embeddable in-process Go kernel. It owns conversation, model access, provider authentication, model selection, tool execution, skills, sessions, and capability modules.

- **Zero-Key Free Chat:** Ready to chat on turn 1 with verified anonymous models (OpenCode Zen, Pollinations). No credit card, no sign-ups.
- **Provider Hub & Device Auth:** First-class device sign-in for GitHub Copilot (individual & enterprise), OpenAI Codex, and 50+ OpenAI/Anthropic compatible providers.
- **Dual Binary Architecture:** Ships as `facet-studio` (headed web cockpit) supervising `facet-studio-kernel` (agentic runtime engine).
- **In-Process Go Embedding:** Downstream applications like Midden and Facet embed `pkg/agent` and `pkg/modelservice` in memory with zero CLI shell-outs.

---

## ⚡ Quick Install

### Windows (PowerShell)
```powershell
irm https://xibodev.github.io/facet-studio/install.ps1 | iex
```

### macOS &amp; Linux (Terminal)
```bash
curl -fsSL https://xibodev.github.io/facet-studio/install.sh | bash
```

The installer verifies your environment, sets your local admin password securely, auto-wires verified free models, and opens your browser directly into the dashboard (`http://localhost:18800`).

---

## 🧠 Architecture

Facet Studio is organized into three clean layers:

```
┌─────────────────────────────────────────────────────────┐
│  Tier 3: Presentation Frontends                         │
│  - Web Cockpit (facet-studio browser UI)                │
│  - Headless CLI (facet-studio-kernel agent/gateway)     │
│  - Host Applications (Facet, Midden)                    │
└────────────────────────────┬────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────┐
│  Tier 2: Model & Provider Service (pkg/modelservice)    │
│  - Curated Provider Registry (github.com/xibodev/llmgw-core)
│  - Live Endpoint Reachability & Latency Probing (ping)  │
│  - Live Catalog Sync (/models) & Free Model Filtering   │
│  - Zero-Key Anonymous Auto-Connect                      │
└────────────────────────────┬────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────┐
│  Tier 1: Core Kernel Engine (pkg/agent, pkg/providers)  │
│  - Multi-turn Agent Loop & Streaming Execution          │
│  - Native GitHub Copilot HTTP Driver (Token Refresh)    │
│  - OpenAI-Compatible & Anthropic Native Protocols       │
│  - Tool Calling, Memory Cairns, and Fallback Resilience │
└─────────────────────────────────────────────────────────┘
```

---

## 💻 Headless Kernel CLI Usage

If you run the standalone runtime `facet-studio-kernel` without a browser:

```bash
# Initialize workspace and auto-wire free models
facet-studio-kernel onboard

# Inspect all 50+ curated providers and local status
facet-studio-kernel model roster

# Ping provider endpoints and measure latency
facet-studio-kernel model ping opencode-zen

# Auto-discover and connect working free models
facet-studio-kernel model auto-free

# Run an agent turn directly in the terminal
facet-studio-kernel agent "Summarize the files in this directory"
```

---

## 📦 In-Process Go Embedding (Midden &amp; Facet)

Downstream applications embed the agent runtime directly in Go without running external processes:

```go
package main

import (
    "context"
    "fmt"

    "github.com/xibodev/facet-studio/pkg/agent"
    "github.com/xibodev/facet-studio/pkg/config"
    "github.com/xibodev/facet-studio/pkg/modelservice"
    "github.com/xibodev/facet-studio/pkg/providers"
)

func main() {
    ctx := context.Background()

    // 1. Load config (or initialize fresh defaults with free models pre-seeded)
    cfg, err := config.LoadConfig("~/.facet-studio/config.json")
    if err != nil {
        cfg = config.DefaultConfig()
    }

    // 2. (Optional) Provide native host domain tools directly in memory:
    hostTools := []providers.ToolDefinition{
        // Custom Go tools
    }

    // 3. Create the agent instance
    loop, err := agent.NewAgentLoop(cfg, hostTools)
    if err != nil {
        panic(err)
    }

    // 4. Run turns with native Go streaming callbacks:
    _, err = loop.ProcessTurn(ctx, "session-1", "Hello agent!", func(chunk string) {
        fmt.Print(chunk) // Stream directly to your UI
    })
}
```

---

## 🔨 Building From Source

### Prerequisites
* Go 1.24+
* Node.js 20+ & pnpm (only for rebuilding the React frontend)

### Build Binaries
```bash
# Build the headed web shell
go build -tags goolm,stdjson -ldflags "-s -w" -o bin/facet-studio ./web/backend

# Build the headless agent kernel
go build -tags goolm,stdjson -ldflags "-s -w" -o bin/facet-studio-kernel ./cmd/facet-studio
```

---

## 📄 License

Licensed under the MIT License. See [LICENSE](LICENSE) for details.
