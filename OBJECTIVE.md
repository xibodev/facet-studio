# Facet Studio Objective

## What This App Must Be

Facet Studio is one complete local-first agent application with one headed web
experience. It owns conversation, model access, provider authentication, model
selection, tool execution, skills, sessions, runtime events, approvals,
artifacts, and installable capability modules.

Its agent runtime must also be a supported in-process kernel that Facet and
Midden can embed. The kernel is a subsystem of Facet Studio, not a second
product. Full Studio adds the headed shell and module host around that same
runtime.

GitHub Copilot support is ordinary provider support inside the existing
provider system. It must use normal authentication, model, routing, history,
and approval paths rather than create a separate architecture.

## Required Order

1. Inspect the current source and tests to establish what actually works.
2. Complete and verify Studio's provider, authentication, model, session,
   approval, module-lifecycle, and artifact paths.
3. Define the smallest supported kernel surface required by real Facet and
   Midden consumers.
4. Prove the headed product and both external consumers from clean checkouts.
5. Tag and release Facet Studio and its kernel on one version train.

Facet and Midden standalone releases remain blocked until step 5. Their domain,
bundle, and module work may proceed independently.

## Start Clean

1. Read only this file for intent. Do not reconstruct plans from deleted
   documentation.
2. Run `git status` and preserve all existing work unless the user explicitly
   asks to replace it.
3. Inspect entry points, `go.mod`, build files, source boundaries, and tests.
4. Run the smallest relevant baseline checks before changing code.
5. Classify behavior as implemented only when source and executable tests prove
   it. Treat uncommitted code, skipped tests, fixtures, and comments as evidence
   to verify, not product truth.
6. Make the smallest dependency-ordered change, then test it end to end.
7. Do not create new Markdown plans, status logs, prompts, handoffs, or
   architecture essays. Put durable contracts in code and tests. Use Git history
   only when provenance is necessary.

## Boundaries

- Do not import Facet or Midden product code into Studio.
- Do not make standalone products host themselves through the detached module
  protocol; they bind domain tools natively.
- Do not add a second provider system, agent loop, or approval engine for a
  particular integration.
- Do not claim an OS sandbox when modules run as the current user.
- Do not claim release support before clean-install, consumer, and real-turn
  evidence exists.
- Do not commit, tag, push, publish, or release without explicit user approval.
