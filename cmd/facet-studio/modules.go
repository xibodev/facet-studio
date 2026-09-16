// Module lifecycle commands.
//
// Facet Studio discovers installed module binaries, describes them, and invokes
// their capabilities as bounded detached processes. Modules are never imported
// as Go packages: they are separate programs speaking a JSON-over-CLI protocol,
// so a module can be upgraded, removed, or written in another language without
// touching the host.
//
//	facet-studio modules
//	facet-studio module-invoke <module> <capability> [json]
//	facet-studio handoff
package main

import (
	"errors"

	"github.com/spf13/cobra"

	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xibodev/facet-studio/internal/module"
	"github.com/xibodev/facet-studio/internal/moduletools"
	"github.com/xibodev/facet-studio/pkg/config"
	"github.com/xibodev/facet-studio/pkg/modproto"
)

// Home is the host's state root for the CLI.
//
// IT DELEGATES. There used to be a second fallback implementation here, and it
// disagreed with the host's whenever FACET_STUDIO_HOME was unset: this side
// resolved to an executable-anchored `.local`, the agent to `~/.facet-studio`.
// So `modules-add` installed somewhere the browser agent never looked, the CLI
// listed the module happily, and THE BROWSER SAW NOTHING -- silently, because
// discovery finding no modules looks exactly like none being installed.
//
// Masked throughout development because every launch set the variable.
//
// The dev-checkout intent is preserved and is now the host's own explicitly
// named fallback rather than a private one: a developer build still keeps its
// state beside the binary instead of writing into a user profile by surprise.
// What is gone is the SECOND ANSWER to the same question.
//
// Which one applies is decided by IsDevBuild, so the CLI and the host agree by
// construction rather than by both being careful.
func Home() string {
	if config.IsDevBuild() {
		return absOrSelf(config.DevHome())
	}
	return absOrSelf(config.GetHome())
}

// absOrSelf makes a path absolute, returning it unchanged when it cannot.
func absOrSelf(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func modulesDir() string { return filepath.Join(Home(), "modules") }

// discover finds installed module binaries.
//
// Installation is just a file on disk: the host asks each binary to describe
// itself rather than reading a registry, so there is no metadata that can drift
// out of sync with the executable.
// discover finds installed modules using the SAME implementation the cockpit
// uses.
//
// It previously had its own copy that scanned only flat files, so a module
// installed into its own directory by `modules add` was invisible to the CLI
// while the cockpit listed it. Two implementations of "what is installed"
// disagreeing is the class of bug this project keeps finding; there is now one.
func discover() ([]*module.Runner, error) {
	var runners []*module.Runner
	for _, in := range moduletools.Discover(context.Background(), Home()) {
		runners = append(runners, in.Runner)
	}
	return runners, nil
}

func cmdModules() error {
	runners, err := discover()
	if err != nil {
		return err
	}
	if len(runners) == 0 {
		fmt.Printf("no modules installed in %s\n", modulesDir())
		return nil
	}

	for _, r := range runners {
		d, res, err := r.Describe(context.Background())
		if err != nil {
			// A broken module is reported, never fatal: one bad module must not
			// take down discovery of the others.
			fmt.Printf("%-24s UNAVAILABLE  %v\n", filepath.Base(r.Binary), err)
			continue
		}

		status := "enabled"
		if dir, ok := moduletools.ModuleDir(Home(), r.Binary); ok && moduletools.Disabled(dir) {
			status = "disabled"
		}
		fmt.Printf("\n%s  v%s  (%s)  [%s]\n", d.Module, d.Version, d.Name, status)
		for _, c := range d.Capabilities {
			fmt.Printf("  %-30s %s\n", c.ID, c.Summary)
			fmt.Printf("  %-30s %s\n", "", effectLine(c.Effects))
		}
		if n := len(d.AgentOverlays); n > 0 {
			fmt.Printf("  overlays: %d   skills: %d\n", n, len(d.Skills))
		}
		if len(d.Permissions.Subprocess) > 0 {
			resolved, missing := module.ResolveBinaries(d.Permissions.Subprocess)
			fmt.Printf("  binaries: %d/%d resolved", len(resolved), len(d.Permissions.Subprocess))
			if len(missing) > 0 {
				fmt.Printf("   MISSING: %s", strings.Join(missing, " "))
			}
			fmt.Println()
		}
		for _, w := range res.Envelope.Warnings {
			fmt.Printf("  warning: %s\n", w)
		}
	}
	return nil
}

// effectLine renders declared effects the way the cockpit must: a cost that is
// unknown is shown as unknown, never as free.
func effectLine(e modproto.Effects) string {
	var parts []string
	if e.Network {
		parts = append(parts, "network")
	}
	if e.ExternalWrites {
		parts = append(parts, "writes")
	}
	if e.CostKnown {
		parts = append(parts, "cost known")
	} else {
		parts = append(parts, "COST UNKNOWN - approval required")
	}
	return strings.Join(parts, ", ")
}

func cmdInvoke(moduleID, capability, input string) (*module.Result, error) {
	return cmdInvokeWithSourceRoots(moduleID, capability, input, nil)
}

func cmdInvokeWithSourceRoots(
	moduleID,
	capability,
	input string,
	sourceRoots map[string]string,
) (*module.Result, error) {
	for name, path := range sourceRoots {
		if _, ok := moduletools.NormalizeExplicitSourceRoot(name, path); !ok {
			return nil, fmt.Errorf("invalid explicit %s source root %q: expected %s", name, path, explicitSourceRootShape(name))
		}
	}
	runners, err := discover()
	if err != nil {
		return nil, err
	}

	for _, r := range runners {
		d, _, err := r.Describe(context.Background())
		if err != nil || d.Module != moduleID {
			continue
		}
		if dir, ok := moduletools.ModuleDir(Home(), r.Binary); ok && moduletools.Disabled(dir) {
			return nil, fmt.Errorf("module %q is disabled; enable it before invoking capabilities", moduleID)
		}

		// Bind identity now that the descriptor has named the module, so the
		// invocation is checked against the module it claims to be.
		r.ModuleID = d.Module

		workspace, err := filepath.Abs(filepath.Join(Home(), "workspace"))
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			return nil, err
		}

		req := &modproto.Request{
			Capability: capability,
			Roots:      grantRootsWithSourceOverrides(d, workspace, sourceRoots),
			DeadlineMS: module.DefaultInvokeDeadlineMS,
		}
		// The CLI takes one JSON blob and normally sends it as Input, which is
		// what a capability's RequestSchema describes. Some modules also read
		// root-level fields of their own alongside it, so any key the caller
		// supplies that is NOT part of the host's fixed envelope is passed
		// through at the root as well.
		//
		// This is a v1 convenience for driving real modules by hand, not a
		// protocol rule: Input remains the contract, and the host's own fields
		// always win so a module cannot capture them.
		if err := passThroughRootFields(req, input); err != nil {
			return nil, err
		}
		// Per-invocation authority: only the binaries this module declared.
		//
		// Two steps, not one. GrantBinaries resolves NAMES to absolute paths
		// (modules run with no PATH to search); ApplyGrants records what the
		// host actually authorizes. A module reads the grant to decide whether
		// it MAY shell out and the path to know what to run, so supplying only
		// the path left it holding an executable it was not permitted to use.
		//
		// The CLI called only the first. The agent and the cockpit called both,
		// which is why this failed here and nowhere else.
		if missing := module.GrantBinaries(d, req); len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "note: unresolved binaries: %s\n", strings.Join(missing, " "))
		}
		module.ApplyGrants(d, req, module.GrantAll())

		res, err := r.Invoke(context.Background(), d, req)
		if res != nil && res.Stderr != "" {
			fmt.Fprintf(os.Stderr, "--- module diagnostics ---\n%s", res.Stderr)
		}
		if err != nil {
			return res, err
		}
		lastRequest = req
		return res, render(res)
	}
	return nil, fmt.Errorf("module %q is not installed in %s", moduleID, modulesDir())
}

func explicitSourceRootShape(name string) string {
	if name == "opencode_store" {
		return "an existing absolute opencode.db file or a directory containing opencode.db"
	}
	return "an existing absolute directory"
}

// lastRequest is the request behind the most recent invocation, kept so the
// handoff demo can resolve artifact roots. A real cockpit tracks this per
// invocation; v1 keeps one.
var lastRequest *modproto.Request

// cmdHandoff runs the cross-module path end to end: Midden produces a seed,
// the host stages it with digest verification, and Facet consumes it from a
// read-only root without ever learning Midden's layout.
func cmdHandoff() error {
	fmt.Println("== 1. midden seed.create ==")
	res, err := cmdInvoke("midden", "seed.create",
		`{"tool":"claude","days":2,"max_sessions":2,"goal":"explain what facet-studio does","name":"handoff"}`)
	if err != nil {
		return err
	}
	if len(res.Envelope.Execution.Artifacts) == 0 {
		return fmt.Errorf("seed.create produced no artifact")
	}
	seed := res.Envelope.Execution.Artifacts[0]

	fmt.Println("\n== 2. host stages the artifact, verifying its digest ==")
	store, err := filepath.Abs(filepath.Join(Home(), "artifacts"))
	if err != nil {
		return err
	}
	staged, err := module.StageArtifact(lastRequest, seed, store)
	if err != nil {
		return fmt.Errorf("staging refused the artifact: %w", err)
	}
	fmt.Printf("   verified %s\n   staged   %s\n", seed.Digest, staged)

	fmt.Println("\n== 3. facet consumes the staged seed from a read-only root ==")
	fmt.Printf("   host supplies roots{seed_in: %s, mode: ro}\n", staged)
	fmt.Println("   Midden's layout is never exposed; Facet reads by path + digest.")

	// This step used to print its intent and return nil -- so `handoff` exited
	// 0 having done two of three steps, which reads as SUCCESS to anything
	// scripting it. A module author found it by running the command and asking
	// where step 3 went. A pipeline that stops must say which step and why.
	consumer, err := seedConsumingCapability("xibodev.facet")
	if err != nil {
		return fmt.Errorf("step 3 cannot run: %w", err)
	}

	// The seed is sent in the shape the consumer DECLARES: an object of
	// {schema, path, digest}, which is what its request schema says.
	//
	// This used to send flat seed_path/seed_digest fields -- a guess, and the
	// same guess that made the resolver miss the capability. Unknown fields did
	// not simply get ignored: they pushed the consumer down a path where it
	// minted its own request_id, and the host refused the envelope for not
	// echoing the one it sent. A wrong request shape surfaced as a protocol
	// violation naming something unrelated.
	// creative.tools.run is a RENDER capability that also accepts a seed, not a
	// seed-consumer with an optional job. It needs the work described alongside
	// the provenance, so the handoff asks for the smallest real render the seed
	// can carry -- otherwise it proves the plumbing and produces nothing.
	request, err := json.Marshal(map[string]any{
		"seed": map[string]string{
			"schema": seed.Kind,
			"path":   staged,
			"digest": seed.Digest,
		},
		"tool": "video_compose",
		"input": map[string]any{
			"operation": "remotion_render",
			"output":    "renders/handoff.mp4",
			"width":     640,
			"height":    360,
			"scenes": []map[string]any{{
				"id": "s1", "type": "text_card",
				"start_seconds": 0, "end_seconds": 3,
				"text":     seedTitle(res),
				"fontSize": 48, "backgroundColor": "#0b1220", "color": "#f8fafc",
			}},
		},
	})
	if err != nil {
		return err
	}
	if _, err := cmdInvoke("xibodev.facet", consumer, string(request)); err != nil {
		return fmt.Errorf("step 3 (%s) failed: %w", consumer, err)
	}
	return nil
}

// seedConsumingCapability finds the capability that accepts a staged seed, or
// explains precisely why the handoff cannot complete.
//
// The explanation matters more than the lookup: when no such capability exists,
// the honest report is that the CONSUMER has not exposed one -- not that the
// handoff is broken, and not silence.
func seedConsumingCapability(moduleID string) (string, error) {
	runners, err := discover()
	if err != nil {
		return "", err
	}
	for _, r := range runners {
		d, _, describeErr := r.Describe(context.Background())
		if describeErr != nil || d.Module != moduleID {
			continue
		}
		// A capability accepts a seed when it SAYS SO in its request schema.
		//
		// This used to look for an id containing "seed" or a "seed_path" field
		// -- both guesses, and both wrong. The consumer publishes `seed` as an
		// object of {schema, path, digest} on several existing capabilities
		// rather than as a capability of its own, so the resolver found nothing
		// and reported the consumer as not having published its half. It had.
		//
		// Matching the declared field rather than a naming convention means the
		// consumer can put it wherever it likes and the handoff still finds it.
		if id := seedCapabilityIn(d); id != "" {
			return id, nil
		}
		ids := make([]string, 0, len(d.Capabilities))
		for _, c := range d.Capabilities {
			ids = append(ids, c.ID)
		}
		return "", fmt.Errorf(
			"%s exposes no capability that accepts a seed; it declares %s."+
				"\n   The seed IS staged and digest-verified, so the handoff"+
				" contract is met on the producer side -- the consumer has"+
				" not published the other half",
			moduleID, strings.Join(ids, ", "))
	}
	return "", fmt.Errorf("module %q is not installed in %s", moduleID, modulesDir())
}

// passThroughRootFields places one JSON blob from the command line into the
// request.
//
// A capability's RequestSchema describes Request.Input, so an explicit "input"
// key is used as Input verbatim and any sibling keys are passed through at the
// request root. A blob with no "input" key is treated as Input in its entirety,
// which is the common case.
//
// The passthrough exists because real modules already read some arguments
// beside Input rather than inside it, and a v1 that cannot drive them is not a
// working v1. Host-owned fields are never overwritten, so a module cannot
// capture protocol, roots, grants, binaries, or the bounds by naming them.
func passThroughRootFields(req *modproto.Request, input string) error {
	var supplied map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &supplied); err != nil {
		return fmt.Errorf("input must be a JSON object: %w", err)
	}

	nested, hasNested := supplied["input"]
	if hasNested {
		req.Input = nested
	} else {
		// No explicit "input" key: the blob IS the input, which is the common
		// case. It is also still passed through at the root, because a module
		// may read some arguments there instead -- and duplicating a value the
		// caller typed once is harmless, while dropping it is not.
		req.Input = json.RawMessage(input)
	}

	reserved := map[string]bool{
		"protocol": true, "capability": true, "request_id": true,
		"input": true, "roots": true, "grants": true, "binaries": true,
		"deadline_ms": true, "max_output_bytes": true,
	}

	req.Extra = make(map[string]json.RawMessage, len(supplied))
	for k, v := range supplied {
		if reserved[k] {
			continue
		}
		req.Extra[k] = v
	}
	return nil
}

// grantRoots maps the logical root NAMES a module declared onto real absolute
// paths, for this invocation only.
//
// This is where "installing a module grants nothing" becomes concrete: a module
// declares the names it needs, and the host decides what -- if anything -- each
// one points at. A name the host does not recognise is simply not supplied, and
// the module fails closed rather than reaching for it another way.
//
// Source stores are supplied READ-ONLY. That guarantee is the host's, enforced
// on every path the module returns, and does not depend on the module behaving.
// grantRoots maps the logical root NAMES a module declared onto real absolute
// paths, for this invocation only.
//
// It delegates to moduletools.GrantRoots -- the SAME implementation the agent
// uses. It used to be a second copy, and the copy did not grant a module its
// own bundle root, so a module could not find the runtime it ships with. The
// same module rendered video fine through the agent and failed through the CLI
// with "Remotion Composer runtime not found", which reads as a broken install
// rather than a missing grant.
//
// Two implementations of "what is this module allowed to see" is the same
// duplicated-decision shape that has produced several bugs here. There is now
// one.
func grantRoots(d *modproto.Descriptor, workspace string) map[string]modproto.Root {
	return moduletools.GrantRoots(d, Home(), workspace)
}

func grantRootsWithSourceOverrides(
	d *modproto.Descriptor,
	workspace string,
	sourceRoots map[string]string,
) map[string]modproto.Root {
	if sourceRoots == nil {
		return grantRoots(d, workspace)
	}
	return moduletools.GrantRootsWithSourceOverrides(d, Home(), workspace, sourceRoots)
}

// render is the normalized event: one shape for every module, carrying the
// facts the host is responsible for surfacing honestly.
func render(res *module.Result) error {
	e := res.Envelope

	status := "OK"
	if !e.OK {
		status = "FAILED"
	}
	fmt.Printf("[%s] %s / %s  (%s)\n", status, e.Module, e.Operation, res.Duration.Round(1e6))

	if !e.OK {
		fmt.Printf("  error   %s: %s\n", e.Error.Code, e.Error.Message)
		if e.Error.Retryable {
			fmt.Println("  retryable: yes")
		}
	}

	x := e.Execution
	fmt.Printf("  effects local=%v network=%v writes=%v provider=%q\n",
		x.Local, x.Network, x.ExternalWrites, x.Provider)
	fmt.Printf("  cost    estimated=%s actual=%s\n", cost(x.EstimatedCost), cost(x.ActualCost))

	for _, w := range e.Warnings {
		fmt.Printf("  warning %s\n", w)
	}
	// Host-side findings are labelled separately from the module's own, so a
	// reader can tell who is complaining about what.
	for _, w := range res.Warnings {
		fmt.Printf("  HOST    %s\n", w)
	}

	// Artifact cards: pointers, never inline payloads.
	for _, a := range x.Artifacts {
		fmt.Printf("  artifact %s\n", a.ID)
		fmt.Printf("    kind   %s\n", a.Kind)
		fmt.Printf("    path   %s (root %s)\n", a.Path, a.Root)
		fmt.Printf("    bytes  %d\n", a.Bytes)
		fmt.Printf("    digest %s\n", a.Digest)
	}

	if e.OK && len(e.Result) > 0 {
		var pretty any
		if err := json.Unmarshal(e.Result, &pretty); err == nil {
			blob, _ := json.MarshalIndent(pretty, "  ", "  ")
			if len(blob) > 1200 {
				blob = append(blob[:1200], []byte("\n  ... (truncated for display)")...)
			}
			fmt.Printf("  result  %s\n", blob)
		}
	}
	return nil
}

// cost renders the distinction the consent model rests on: null is unknown and
// must never be displayed as free.
func cost(c *float64) string {
	if c == nil {
		return "UNKNOWN"
	}
	return fmt.Sprintf("%.4f", *c)
}

// NewModulesCommand lists installed modules and their capabilities.
func NewModulesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "modules",
		Short: "List installed modules and their capabilities",
		RunE: func(_ *cobra.Command, _ []string) error {
			return cmdModules()
		},
	}
}

// NewModuleInvokeCommand runs one capability of one installed module.
func NewModuleInvokeCommand() *cobra.Command {
	var claudeSourceRoot string
	var copilotSourceRoot string
	var opencodeSourceRoot string
	cmd := &cobra.Command{
		Use:   "module-invoke <module> <capability> [json]",
		Short: "Invoke a module capability as a bounded detached process",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := "{}"
			if len(args) > 2 {
				input = args[2]
			}
			var sourceRoots map[string]string
			if cmd.Flags().Changed("claude-source-root") || cmd.Flags().Changed("copilot-source-root") || cmd.Flags().Changed("opencode-source-root") {
				sourceRoots = map[string]string{}
				if cmd.Flags().Changed("claude-source-root") {
					sourceRoots["claude_store"] = claudeSourceRoot
				}
				if cmd.Flags().Changed("copilot-source-root") {
					sourceRoots["copilot_store"] = copilotSourceRoot
				}
				if cmd.Flags().Changed("opencode-source-root") {
					sourceRoots["opencode_store"] = opencodeSourceRoot
				}
			}
			_, err := cmdInvokeWithSourceRoots(args[0], args[1], input, sourceRoots)
			return err
		},
	}
	cmd.Flags().StringVar(&claudeSourceRoot, "claude-source-root", "", "absolute read-only Claude source store for this invocation")
	cmd.Flags().StringVar(&copilotSourceRoot, "copilot-source-root", "", "absolute read-only Copilot source store for this invocation")
	cmd.Flags().StringVar(&opencodeSourceRoot, "opencode-source-root", "", "absolute read-only OpenCode source store for this invocation")
	return cmd
}

// NewHandoffCommand demonstrates the cross-module path: one module produces an
// artifact, the host stages it with digest verification, and another module
// consumes it from a read-only root.
func NewHandoffCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "handoff",
		Short: "Run the cross-module seed handoff end to end",
		RunE: func(_ *cobra.Command, _ []string) error {
			return cmdHandoff()
		},
	}
}

// NewModuleAddCommand installs a module binary into this host.
//
// This is the command a module's own installer shells out to. The host
// validates, chooses the destination, and derives identity from the
// descriptor, so a module never writes into host state nor needs to know the
// host's layout.
func NewModuleAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "modules-add <path-to-module-binary>",
		Short: "Install a detached module from a local binary",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			id, err := module.Install(context.Background(), Home(), args[0])
			if id != "" {
				fmt.Printf("installed module %s\n", id)
			}
			var partial *module.PartialInstall
			if errors.As(err, &partial) {
				// The module works; only its documentation is missing. Say so
				// rather than failing, because the consequence is otherwise
				// invisible: the agent behaves as if it documented nothing.
				for _, w := range partial.Warnings {
					fmt.Fprintf(os.Stderr, "  warning: %s\n", w)
				}
				return nil
			}
			return err
		},
	}
}

// NewModuleRemoveCommand uninstalls a module.
func NewModuleRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "modules-remove <module-id>",
		Short: "Remove an installed module (its state is left intact)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := module.Remove(Home(), args[0]); err != nil {
				return err
			}
			fmt.Printf("removed module %s\n", args[0])
			return nil
		},
	}
}

func NewModuleEnableCommand() *cobra.Command {
	return newModuleEnabledCommand("modules-enable", true)
}

func NewModuleDisableCommand() *cobra.Command {
	return newModuleEnabledCommand("modules-disable", false)
}

func newModuleEnabledCommand(name string, enabled bool) *cobra.Command {
	verb := "Enable"
	if !enabled {
		verb = "Disable"
	}
	return &cobra.Command{
		Use:   name + " <module-id>",
		Short: verb + " an installed module without removing its files or state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			home := Home()
			for _, in := range moduletools.Discover(context.Background(), home) {
				if in.Descriptor == nil || in.Descriptor.Module != args[0] {
					continue
				}
				dir, ok := moduletools.ModuleDir(home, in.Runner.Binary)
				if !ok {
					return fmt.Errorf("module %q has no private install directory and cannot be toggled safely", args[0])
				}
				if err := moduletools.SetDisabled(dir, !enabled); err != nil {
					return err
				}
				state := "enabled"
				if !enabled {
					state = "disabled"
				}
				fmt.Printf("%s module %s\n", state, args[0])
				return nil
			}
			return fmt.Errorf("module %q is not installed in %s", args[0], modulesDir())
		},
	}
}

// seedCapabilityIn names the capability that should consume a staged seed, or
// "" when the module declares none.
//
// Several capabilities may accept one -- the current consumer declares it on
// four -- so this prefers the one that DOES the work. Taking the first match
// resolved an `estimate` capability, which reads the seed and prices a job
// rather than producing anything: the handoff would have proven the plumbing
// while producing no artifact, and reported success.
func seedCapabilityIn(d *modproto.Descriptor) string {
	var seeded []string
	for _, c := range d.Capabilities {
		if declaresSeedField(d.RequestSchemas[c.RequestSchema]) {
			seeded = append(seeded, c.ID)
		}
	}
	// Verbs that produce, in preference order. A module naming its capabilities
	// differently still resolves, just to its first seed-accepting one.
	for _, want := range []string{".run", ".produce", ".create"} {
		for _, id := range seeded {
			if strings.HasSuffix(id, want) {
				return id
			}
		}
	}
	if len(seeded) > 0 {
		return seeded[0]
	}
	return ""
}

// declaresSeedField reports whether a request schema has a top-level "seed"
// property.
//
// The argument is the schema BODY, looked up from the descriptor's
// RequestSchemas map. Capability.RequestSchema is an ID into that map, not the
// schema itself -- the protocol has always said so, and I read the ID as a
// schema and parsed "creative.tools.run.request/v1" as JSON. It failed, the
// resolver found nothing, and the host reported the consumer as not having
// published its half of the handoff. It had.
//
// This parses rather than substring-matching: "seed" appears in prose
// descriptions ("seeded", "the random seed"), and invoking the wrong capability
// because a summary used the word would be worse than finding nothing. A schema
// the host cannot parse declares nothing, by the same rule -- the host acts on
// what a module states, never on what it might have meant.
func declaresSeedField(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return false
	}
	_, ok := s.Properties["seed"]
	return ok
}

// seedTitle names what the seed is about, for the video the handoff renders.
//
// It reads the seed's own goal rather than inventing a caption: a handoff that
// renders text unrelated to the seed would look like it worked while proving
// nothing about the data actually travelling.
func seedTitle(res *module.Result) string {
	const fallback = "Recovered from past sessions"
	if res == nil || res.Envelope == nil || len(res.Envelope.Result) == 0 {
		return fallback
	}
	var out struct {
		Goal  string `json:"goal"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(res.Envelope.Result, &out); err != nil {
		return fallback
	}
	for _, s := range []string{out.Title, out.Goal} {
		if s != "" {
			return s
		}
	}
	return fallback
}
