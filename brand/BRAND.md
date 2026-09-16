# Facet Studio Brand

## Brand idea

Facet Studio makes agentic AI local-first, modular, and embeddable. The geometric facet crystal mark shows multiple facets refracting intelligence: the headed shell, the headless kernel, the modular capabilities, and the embeddable in-process engine. The core represents the operator's retained authority and local-first sovereignty. The identity must feel crisp, engineered, and dependable—never magical, nebulous, or cloud-dependent.

## Audience

Facet Studio is built for developers, operators, and downstream system consumers:
- Operators who want a private, local-first web cockpit to chat with models, run tools, and supervise agents.
- Terminal power users who run automated or headless agent turns through `facet-studio-kernel`.
- Downstream software engineers (Facet, Midden) who embed the agent loop directly into Go binaries without subprocess shell-outs.

Brand communication assumes technical literacy without requiring prior knowledge of Facet Studio.

## Positioning

Facet Studio is one complete, local-first agent application with one headed web experience and an embeddable in-process Go kernel. It owns conversation, model access, provider authentication, model selection, tool execution, skills, sessions, and capability modules.

Facet Studio is not a hosted SaaS platform, a cloud broker, or an opaque multi-tenant server.

Approved short descriptor:

> Local-first agent cockpit and in-process embeddable runtime kernel.

Approved full descriptor:

> A complete local-first agent application with a headed web cockpit and an embeddable in-process Go kernel.

## Canonical naming

| Context | Use | Notes |
|---|---|---|
| Product name and public prose | **Facet Studio** | Use title case. This is the complete product identity. |
| Compact visual wordmark | **facet.studio** | Use only as artwork or a compact signature. It is not a replacement for the product name in prose. |
| Headed web distribution & binary | `facet-studio` | Lowercase with hyphen. Supervises the agent kernel. |
| Headless runtime engine binary | `facet-studio-kernel` | Lowercase with hyphen. Standalone agent and daemon. |
| Go module namespace | `github.com/xibodev/facet-studio` | Canonical Go import path. |
| Local state directory | `~/.facet-studio` | The user's configuration, auth store, and databases. |

## Logo system

### Master Geometry

The master mark is an eight-facet precision crystal diamond drawn on `viewBox="0 0 64 64"`:

```text
Upper apex: 32, 4
Right apex: 58, 22
Lower apex: 32, 60
Left apex:  6,  22
Center intersection: 32, 26
Facet polygons:
  [32,4 58,22 32,60 6,22] fill primary gradient #7CC4FF -> #3E5DB9
  [32,4 58,22 32,26]      fill top-right facet opacity 0.85
  [32,4 6,22 32,26]       fill top-left facet #8FD0FF opacity 0.55
  [6,22 32,26 32,60]      fill bottom-left facet opacity 0.70
  [58,22 32,26 32,60]     fill bottom-right facet #1E2F6B opacity 0.55
```

### Variants

- **Primary mark (`logos/mark.svg`)**: The facet crystal inside a rounded squircle tile (`fill="#0F172A"`).
- **Inverse mark (`logos/mark-inverse.svg`)**: Transparent background for light and dark fields.
- **Horizontal lockup (`logos/lockup.svg`)**: Mark paired with `facet.studio` typography.
- **Monochrome (`logos/mono-black.svg` & `logos/mono-white.svg`)**: Solid black or white for single-color print or stamps.

### Clearspace and Minimum Size

- **Clearspace**: Minimum 25% of the mark width on all four sides.
- **Digital minimum**: 16 px (browser favicon), 24 px (standalone logo mark).
- **Horizontal lockup digital minimum**: 120 px wide.

## Color

### Core Palette

| Token | Hex / OKLCH | Role |
|---|---|---|
| Facet Blue | `#0071E3` / `oklch(0.52 0.19 250)` | Primary brand accent, interactive controls, and links |
| Studio Navy | `#0F172A` / `oklch(0.20 0.03 260)` | Primary dark field and logo tile |
| Studio Canvas | `#F8FAFC` / `oklch(0.975 0.008 250)` | Primary light background canvas |
| Studio Surface | `#FFFFFF` / `oklch(0.995 0.003 250)` | High-contrast card surface |
| Studio Ink | `#0F172A` / `oklch(0.24 0.025 250)` | High-contrast text on light surfaces |
| Studio Muted | `#64748B` / `oklch(0.52 0.025 250)` | Secondary text and structural labels |
| Studio Line | `#E2E8F0` / `oklch(0.83 0.018 250)` | Hairline borders and structural dividers |

## Typography

The identity uses the modern platform system UI font stack:

```css
-apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Inter", system-ui, sans-serif
```

For identifiers, commands, exit codes, and hashes:

```css
"JetBrains Mono", ui-monospace, "SFMono-Regular", Menlo, Monaco, Consolas, monospace
```

Use weight 700 for display headings, 600 for card titles, 400 for prose, and 500-600 for buttons.

## Voice

The Facet Studio voice is clear, technical, bounded, and direct:
- State what the system does and how it is structured.
- Distinguish the headed web application from the headless kernel and in-process embedding.
- No conversational filler, artificial hype, or emojis in product prose.
- Truth in capabilities: zero-key free models are verified live; in-process consumers make zero subprocess shell-outs.
