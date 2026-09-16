package moduletools_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xibodev/facet-studio/internal/moduletools"
)

// Discovery against a REAL v2-declaring module binary.
//
// WHY A REAL BINARY AND NOT A FIXTURE. Every other test in this package builds
// its descriptor in Go or as a string literal I wrote -- which means they all
// test my understanding of the shape, not the shape a shipped module actually
// emits. A sibling lane probed this host's gate with a hand-built input, got
// the wrong answer, and nearly reported interoperation that was an artifact of
// their fixture. The only defence is running the real thing.
//
// SKIPS when the binary is absent rather than failing: the sibling's build
// lives in their tree and is not a dependency of this repo. A skip is honest
// about not having run; a pass would not be.
func TestDiscoveryAgainstARealV2Module(t *testing.T) {
	src := filepath.Join("..", "..", "..", "facet", "dist", "xibodev.facet.exe")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("no sibling v2 binary at %s; this test only runs where one is"+
			" built (it is not a dependency of this repo)", src)
	}

	// Install it the way the host does: <home>/modules/<id>/<id>.exe.
	home := t.TempDir()
	dir := filepath.Join(moduletools.ModulesDir(home), "xibodev.facet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	blob, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xibodev.facet.exe"), blob, 0o755); err != nil {
		t.Fatal(err)
	}

	found := moduletools.Discover(context.Background(), home)
	if len(found) != 1 {
		t.Fatalf("discovered %d modules, want 1", len(found))
	}
	in := found[0]
	if in.Err != nil {
		t.Fatalf("a real v2-declaring module failed discovery: %v.\n"+
			"That matters beyond this test: it would mean declaring"+
			" contract_version breaks the v1 read path, and every module"+
			" adopting v2 would vanish from the Modules page", in.Err)
	}

	// THE WIRE FORMAT IS STILL v1 AND MUST STAY SO. A v2 module declares
	// protocol_versions ["xibodev.module/v1"] and passes v1 descriptor
	// validation; contract_version is a different axis. If this ever fails,
	// the two axes have been conflated somewhere.
	if in.Descriptor == nil || len(in.Descriptor.Capabilities) == 0 {
		t.Fatal("the v1 descriptor did not decode, so the wire path broke while" +
			" adding the behavioural one")
	}

	// AND THE BEHAVIOURAL CONTRACT IS READ.
	if !in.V2.Decision.MayRelyOnV2() {
		t.Fatalf("a module publishing contract_version xibodev.module/v2 did not"+
			" pass the pin during discovery: outcome=%v reason=%q",
			in.V2.Decision.Pin.Outcome, in.V2.Decision.Pin.Reason)
	}

	// Report conformance rather than asserting it passes. The sibling has
	// disclosed their v2 payload is not built yet -- they publish the
	// declaration with no operations -- so a failure here is THEIR work in
	// progress, not a defect in this host, and asserting it would make this
	// test fail for something it does not own.
	// ASSERT WHICH FINDING, do not merely log that there was one.
	//
	// This test previously logged Refusal() and passed. The finding text was
	// right there in the output, and I read the summary line instead -- then
	// told the sibling lane their build reported v2_payload_half_published. It
	// reports no_operations_declared: they publish 21 v1 artifact_schemas and
	// ZERO v2 artifact_kinds, so the discriminator I had just built never fires
	// for them. They re-measured through this validator and corrected me.
	//
	// A LOG LINE IS NOT AN ASSERTION. The test observed the truth and could not
	// fail on it, which makes it evidence only for a reader who looks -- and I
	// was the reader who did not look.
	if in.V2.Conformance == nil {
		t.Fatal("conformance did not run against a v2 module")
	}
	var got []string
	for _, f := range in.V2.Conformance.Findings {
		got = append(got, f.Code)
	}
	t.Logf("real module findings: %v", got)

	hasKinds := len(in.V2.Decision.V2.Descriptor().ArtifactKinds) > 0
	for _, f := range in.V2.Conformance.Findings {
		if f.Code == "v2_payload_half_published" && !hasKinds {
			t.Error("classified as half-published with NO artifact kinds. That" +
				" finding is only decidable when kinds are present; without them" +
				" the two readings are genuinely indistinguishable")
		}
		if f.Code == "no_operations_declared" && hasKinds {
			t.Error("offered the 'genuinely has no Operations' reading while" +
				" declaring artifact kinds, which cannot be true")
		}
	}
	t.Logf("real v2 module: pin=%v capabilities=%d operations=%d",
		in.V2.Decision.Pin.Outcome, len(in.Descriptor.Capabilities),
		len(in.V2.Decision.V2.Descriptor().Operations))
}
