# Cerne Brand System

## Brand idea

**Cerne** is Portuguese for the core or heartwood: the durable center that
remains when surrounding layers change. The identity translates that idea into
an open C around a distinct core. The opening suggests an adaptable shell; the
held square signals a stable local runtime and retained human authority.

The system should feel precise, grounded, capable, and calm. It should never
look mystical, cloud-dependent, or self-directing.

## Audience

Cerne speaks to:

- people who want a headed, local-first cockpit for working with agents;
- developers who embed an in-process Go runtime in another application;
- technical operators who need visible control over models, providers,
  authentication, tools, skills, sessions, and modules;
- evaluators who need to understand the headed application and embedded runtime
  as parts of one product.

Assume technical literacy, but do not require familiarity with the repository
or its compatibility identifiers.

## Positioning

Approved positioning:

> The local agent core that stays useful when the shell changes.

Approved descriptor:

> Local-first agent cockpit and embeddable runtime.

Approved full descriptor:

> A complete local-first agent application with a headed web cockpit and an
> embeddable in-process Go runtime.

Product truth is bounded: Cerne is a local-first headed agent application and
an embeddable runtime. Human authority is central. Product communication may
describe its model, provider, authentication, tool, skill, session, and module
surfaces. Do not describe Cerne as hosted SaaS, promise autonomous outcomes,
claim an operating-system sandbox, or state that Facet or Midden consumption has
been proven.

## Naming and compatibility

| Context | Name in this candidate | Rule |
|---|---|---|
| Product and public prose | **Cerne** | Capital C. Use for the product identity. |
| Visual wordmark | **cerne** | Lowercase. Use as artwork, not as sentence-case prose. |
| Meaning | Portuguese for **core** or **heartwood** | Explain when useful; do not turn it into a second tagline. |
| Repository | `facet-studio` | Compatibility identifier remains unchanged. |
| Go module | `github.com/xibodev/facet-studio` | Compatibility identifier remains unchanged. |
| Headed binary | `facet-studio` | Compatibility identifier remains unchanged. |
| Runtime binary | `facet-studio-kernel` | Compatibility identifier remains unchanged. |
| Local state | `~/.facet-studio` | Compatibility identifier remains unchanged. |

This is documentation, not a code or state migration. Do not invent `cerne`
binary, module, repository, environment, or state names until a separately
approved migration defines them.

## Mark

### Idea and construction

The mark is a fixed open C enclosing a compact core. It uses
`viewBox="0 0 100 100"`. Do not redraw, round, stroke, rotate, skew, or close the
opening.

Outer open C:

```text
M72 20 H36 C27.16 20 20 27.16 20 36 V64 C20 72.84 27.16 80 36 80 H72 V68 H38 C34.69 68 32 65.31 32 62 V38 C32 34.69 34.69 32 38 32 H72 Z
```

Inner form, always at `opacity=".48"`:

```text
M66 38 H46 C41.58 38 38 41.58 38 46 V54 C38 58.42 41.58 62 46 62 H66 V53 H48 C46.9 53 46 52.1 46 51 V49 C46 47.9 46.9 47 48 47 H66 Z
```

Core:

```text
rect x="72" y="44" width="12" height="12" rx="2.5"
```

No variant may use gradients, filters, glows, shadows, or extra geometry inside
the mark.

### Variants

- **Primary mark**: Cobalt outer form, Deep inner form, Deep core; use on Canvas
  or Surface.
- **Inverse mark**: Dark-surface Text outer form, Soft inner form, Soft core; use
  on Core Night.
- **Primary lockup**: primary mark with the lowercase Ink wordmark.
- **Inverse lockup**: inverse mark with the lowercase Soft wordmark.
- **Wordmark**: lowercase `cerne`, without an attached descriptor.
- **Monochrome**: all forms use black or white; the inner form retains `.48`
  opacity so the construction remains legible.
- **App icons**: inverse mark on a Core Night tile. Maskable artwork uses a
  full-bleed tile and keeps the mark within the central safe zone.

Do not place the primary mark directly on Cobalt or Deep. Do not recolor
individual forms beyond the supplied variants.

### Clear space and minimum sizes

Use the core width, `12%` of the master viewBox, as the minimum clear space on
all sides of a mark or lockup. Nothing visually active may enter that zone.

| Asset | Minimum digital size | Minimum print size |
|---|---:|---:|
| Standalone mark | 16 px | 5 mm |
| App icon | 32 px | 8 mm |
| Horizontal lockup | 112 px wide | 28 mm wide |
| Standalone wordmark | 72 px wide | 20 mm wide |

At 16 px, use the supplied SVG without adding detail or sharpening effects. If
the inner form cannot reproduce cleanly in a constrained production process,
use the supplied monochrome mark at a larger size rather than simplifying it.

## Color

### Palette

| Token | Hex | Role |
|---|---|---|
| Cobalt | `#3D63D8` | Primary identity and selected actions |
| Deep | `#2749AD` | Strong accent, core, and primary hover state |
| Dark-surface Text | `#9CABFF` | Brand and readable secondary text on Core Night |
| Soft | `#E8ECFF` | Inverse wordmark and high-emphasis content on Core Night |
| Core Night | `#111215` | Dark canvas and icon tile |
| Canvas | `#F7F5EF` | Warm primary light canvas |
| Surface | `#FFFFFF` | Raised light surface |
| Ink | `#171719` | Primary text on light surfaces |
| Muted | `#666976` | Secondary text on light surfaces |
| Line | `#D9D7D0` | Light-surface borders and dividers |

Use Core Night, Canvas, and Surface as fields. Cobalt is an accent, not a page
background by default. Dark-surface Text is specifically tuned for Core Night;
do not substitute Cobalt for small text on dark surfaces.

### Verified contrast pairs

Ratios use WCAG relative luminance and are rounded to two decimals.

| Foreground / background | Ratio | Text use |
|---|---:|---|
| Ink / Canvas | 16.42:1 | AAA normal text |
| Ink / Surface | 17.90:1 | AAA normal text |
| Muted / Canvas | 5.01:1 | AA normal text |
| Muted / Surface | 5.46:1 | AA normal text |
| Deep / Surface | 7.95:1 | AAA normal text |
| Cobalt / Surface | 5.28:1 | AA normal text |
| Surface / Cobalt | 5.28:1 | AA normal text |
| Dark-surface Text / Core Night | 8.64:1 | AAA normal text |
| Soft / Core Night | 15.94:1 | AAA normal text |

Contrast approval applies to the listed pairs, not to arbitrary opacity,
blending, or nearby colors. The `.48` inner mark is decorative geometry and is
not a text color.

## Typography

Use Inter when already available and a system sans fallback otherwise. The
brand kit includes no font binaries and requires no remote resource.

```css
Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif
```

Use a platform monospace stack for identifiers, commands, paths, model names,
and runtime values:

```css
ui-monospace, "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace
```

Use weight 650-700 for concise display headings, 600 for labels, and 400-500 for
prose. Prefer sentence case. Keep measures compact and spacing deliberate; do
not imitate a terminal for general marketing copy.

The SVG wordmark contains live text with the approved fallback stack. Convert
that text to outlines only for a controlled export where typography must be
frozen; retain the editable text master in this kit.

## Voice

Cerne sounds direct, technically literate, and bounded. Lead with what the
person can do, name where authority resides, and distinguish current behavior
from plans.

Approved examples:

- "Choose a model, review the tools, then start the session."
- "Your approval is required before the tool continues."
- "Embed the runtime in-process and keep the host application in control."
- "Local-first agent cockpit and embeddable runtime."
- "Facet and Midden are planned consumers; this candidate does not prove their
  integration."

Prohibited examples:

- "Set it loose and let it run your business." This promises unsupervised
  autonomy.
- "Securely sandboxed by default." This claims a sandbox without evidence.
- "The cloud agent platform for every team." This misstates the local-first
  product as hosted SaaS.
- "Already powering Facet and Midden." This claims unproven consumption.
- "Magic intelligence at the heart of everything." This is vague and
  anthropomorphic.

Avoid superlatives, inevitability, sentience, and guarantees. Prefer "can" or a
specific present-tense fact over "always," "never fails," or "fully autonomous."

## Legal and attribution boundary

The Cerne name, mark, wordmark, lockups, icons, preview, and social artwork are
Cerne brand artwork governed by `brand/LICENSES.md`. Software licensing and
upstream attribution are separate. PicoClaw software attribution remains in the
root `NOTICE`, alongside the root `LICENSE`; do not copy, abbreviate, or replace
that software notice with brand language.

Brand applications must not imply affiliation, endorsement, or ownership of an
upstream project. Legal-entity wording is outside this kit unless separately
approved.

## Imagery and motion

Use quiet, structural imagery: close material detail, concentric growth,
layered interfaces, precise crops, and real product captures. Favor warm neutral
fields with one cobalt focal point. Do not use generic humanoid robots, glowing
brains, cosmic clouds, fake terminal noise, or decorative network meshes.

Motion should explain the relationship between shell and core:

- reveal the open C before the core, or keep the core steady while surrounding
  interface layers change;
- use 160-240 ms transitions for interface applications;
- prefer opacity and short position shifts with standard easing;
- honor reduced-motion preferences;
- do not pulse continuously, spin the mark, add glow trails, or imply that the
  product acts without a person.

## Do and don't

| Do | Don't |
|---|---|
| Use the supplied fixed geometry. | Reconstruct the C with a font glyph. |
| Keep the core visually distinct. | Merge the core into the outer C. |
| Use approved contrast pairs. | Set Cobalt body text on Core Night. |
| Pair warm Canvas with crisp Surface. | Fill every surface with brand blue. |
| Describe human review and control. | Promise autonomous guarantees. |
| Label planned applications as illustrative. | Present mockups as shipped UI. |
| Preserve compatibility identifiers verbatim. | Rename binaries or state in a brand-only change. |
| Keep exports flat and unfiltered. | Add gradients, glows, bevels, or shadows to the mark. |

## Touchpoints

| Touchpoint | Preferred asset | Color mode | Status in this candidate |
|---|---|---|---|
| Public website header | `logos/lockup.svg` | Canvas or Surface | Planned target only |
| Public website dark footer | `logos/lockup-inverse.svg` | Core Night | Planned target only |
| Website metadata | `icons/favicon.svg`, `og/og-default.png` | Supplied | Planned target only |
| Cockpit header | `logos/lockup.svg` or inverse | Match application theme | Planned target only |
| Cockpit collapsed sidebar | `logos/mark.svg` or inverse | Match application theme | Planned target only |
| App and PWA manifests | `icons/icon-192.svg`, `icons/icon-512.svg`, `icons/maskable.svg` | Core Night tile | Planned target only |
| Desktop application icon | `icons/app-icon.svg` | Core Night tile | Planned target only |
| Repository and package surfaces | `logos/lockup.svg`, `icons/app-icon.svg` | Surface | Planned target only |
| Release and social cards | `og/og-default.png` | Core Night | Planned target only |
| Single-color print or engraving | `logos/mono-black.svg` or `logos/mono-white.svg` | One ink | Ready artwork |

## Implementation and migration note

This is a strict brand-only candidate. Production code, public website code,
cockpit code, CLI strings, package metadata, repository naming, binaries,
installers, workflows, workspace seeds, user state, and legal notices remain
unchanged. The mockups in `preview.html` show visual application decisions only.

A future adoption change must map each planned consumer deliberately, preserve
compatibility identifiers until a separate migration is approved, update legal
and release surfaces through their normal review paths, and verify the resulting
product rather than treating this brand kit as runtime evidence.
