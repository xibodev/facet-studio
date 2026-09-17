# Cerne Brand Kit

This directory is the complete editable Cerne identity candidate. It is
self-contained for review and works offline with system font fallbacks. Read
[`BRAND.md`](BRAND.md) before adapting an asset.

This candidate changes brand files only. Every consumer listed below is a
planned projection target, not evidence that the current product, website, or
release surfaces use Cerne.

## Inventory

| File | Purpose | Editable master |
|---|---|---|
| `BRAND.md` | Identity, naming, geometry, accessibility, voice, and use rules | Yes |
| `README.md` | Inventory, export, and validation instructions | Yes |
| `LICENSES.md` | Brand-art ownership and software-license boundary | Yes |
| `provenance.json` | Operator-selected basis, inventory, and planned projections | Yes |
| `tokens.json` | Tool-neutral palette, semantics, typography, and fixed geometry | Yes |
| `tokens.css` | CSS projection of the same token system | Yes |
| `preview.html` | Offline decision board and illustrative application mockups | Yes |
| `check.mjs` | Dependency-free structural and scope-aware brand checker | Yes |
| `logos/mark.svg` | Primary open-C mark for light surfaces | Yes |
| `logos/mark-inverse.svg` | Mark for Core Night and other approved dark fields | Yes |
| `logos/lockup.svg` | Primary mark and lowercase wordmark | Yes |
| `logos/lockup-inverse.svg` | Inverse mark and lowercase wordmark | Yes |
| `logos/wordmark.svg` | Lowercase wordmark for light surfaces | Yes |
| `logos/wordmark-inverse.svg` | Lowercase wordmark for dark surfaces | Yes |
| `logos/mono-black.svg` | Black single-color mark | Yes |
| `logos/mono-white.svg` | White single-color mark | Yes |
| `icons/favicon.svg` | 64-unit browser favicon master on a dark tile | Yes |
| `icons/app-icon.svg` | 512-unit application icon master | Yes |
| `icons/icon-192.svg` | 192-unit manifest icon master | Yes |
| `icons/icon-512.svg` | 512-unit manifest icon master | Yes |
| `icons/maskable.svg` | 512-unit full-bleed maskable icon with safe-zone artwork | Yes |
| `og/og-default.svg` | 1200 x 630 social-preview master | Yes |
| `og/og-default.png` | Deterministic 1200 x 630 raster social preview | No, generated |

SVG, JSON, CSS, HTML, and Markdown files are the editable sources. Do not edit
the PNG as a master. The kit intentionally includes no font files, remote
resources, asset hashes, lock manifests, or generated dependency directories.

## Planned consumer projections

| Planned target | Candidate sources | Adoption work still required |
|---|---|---|
| Public website | Lockups, mark, favicon, OG image, palette, typography | Update and verify website code in a separately scoped change. |
| Headed cockpit | Lockups, marks, icons, semantic theme tokens | Map tokens into the existing UI and test both themes separately. |
| App/PWA packaging | App, 192, 512, and maskable icons | Export platform rasters and update manifests/package resources separately. |
| Repository/package pages | Lockup, mark, descriptor, app icon | Update metadata and public prose after compatibility review. |
| Release/social media | OG PNG and approved descriptors | Adopt through the owning release and publishing process. |

The repository remains `facet-studio`; the Go module remains
`github.com/xibodev/facet-studio`; the binaries remain `facet-studio` and
`facet-studio-kernel`; and the state directory remains `~/.facet-studio`. This
kit does not migrate those compatibility identifiers.

## Review locally

From the repository root:

```powershell
node brand/check.mjs
Start-Process (Resolve-Path -LiteralPath 'brand/preview.html')
```

The preview uses only relative local assets. No server, install, product binary,
or network request is required.

## Regenerate the OG PNG

The raster is generated from `og/og-default.svg` with exact
`@resvg/resvg-js@2.6.2`. Keep all temporary package output and the npm cache
inside an explicitly guarded approved build directory. Run from the repository
root in PowerShell:

```powershell
$ApprovedPrefix = 'C:\Users\gafar\AppData\Local\Temp\opencode\cerne-brand-build-'
$BuildRoot = 'C:\Users\gafar\AppData\Local\Temp\opencode\cerne-brand-build-resvg'
if (-not $BuildRoot.StartsWith($ApprovedPrefix, [StringComparison]::OrdinalIgnoreCase)) {
  throw 'Refusing output outside the approved Cerne brand build prefix.'
}
$RepoRoot = (git rev-parse --show-toplevel).Trim()
if (Test-Path -LiteralPath $BuildRoot) { Remove-Item -LiteralPath $BuildRoot -Recurse -Force }
New-Item -ItemType Directory -Path $BuildRoot | Out-Null
npm install --prefix $BuildRoot --cache "$BuildRoot\npm-cache" --no-audit --no-fund --save-exact '@resvg/resvg-js@2.6.2'
$env:CERNE_OG_SOURCE = Join-Path $RepoRoot 'brand\og\og-default.svg'
$env:CERNE_OG_TARGET = Join-Path $RepoRoot 'brand\og\og-default.png'
Push-Location $BuildRoot
node --input-type=module -e "import fs from 'node:fs'; import { Resvg } from '@resvg/resvg-js'; const svg = fs.readFileSync(process.env.CERNE_OG_SOURCE, 'utf8'); const png = new Resvg(svg, { fitTo: { mode: 'width', value: 1200 }, font: { loadSystemFonts: true, defaultFontFamily: 'Arial' } }).render().asPng(); fs.writeFileSync(process.env.CERNE_OG_TARGET, png);"
Pop-Location
node brand/check.mjs
```

Do not add the temporary package files, npm cache, `node_modules`, or lockfile to
the repository.

## Manual exports

- Export SVGs in sRGB without gradients, filters, glow, drop shadow, or added
  metadata.
- Preserve each supplied viewBox and the exact mark paths and core rectangle.
- Keep transparent logo backgrounds transparent. Only icon files include the
  Core Night tile.
- For a typography-frozen vendor handoff, convert wordmark text to outlines in a
  copy and retain the live-text SVG here as the master.
- Rasterize the OG image at exactly 1200 x 630. Rasterize icons at their named
  dimensions; inspect 16, 32, and 64 px favicon use separately.
- Do not upscale a raster to create another deliverable. Render again from SVG.

## Validation

Run these checks from the repository root:

```powershell
node brand/check.mjs
python -c "from pathlib import Path; import xml.etree.ElementTree as ET; files=list(Path('brand').rglob('*.svg')); [ET.parse(p) for p in files]; print(f'parsed {len(files)} SVG files')"
node -e "const fs=require('node:fs'); for(const p of ['brand/tokens.json','brand/provenance.json']) JSON.parse(fs.readFileSync(p,'utf8')); console.log('parsed brand JSON')"
git diff --check
git status --porcelain
git diff --name-only
git diff --quiet HEAD -- . ':(exclude)brand/**'
```

The final command must return success. Status and diff output must contain only
`brand/**`. The root `LICENSE` and `NOTICE` remain the authority for product
software and PicoClaw-derived software attribution.
