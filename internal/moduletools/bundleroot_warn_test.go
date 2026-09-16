package moduletools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xibodev/facet-studio/pkg/modproto"
)

func declaringBundleRoot(name string) *modproto.Descriptor {
	return &modproto.Descriptor{
		Module:      "xibodev.facet",
		Permissions: modproto.Permissions{FilesystemRead: []string{name}},
	}
}

// A declared bundle root the host CANNOT SUPPLY is reported.
//
// The silence was the defect, not the behaviour. moduleBundleDir returns "" for
// a missing directory and the root is then simply not granted, which is right --
// handing over a path that does not exist is worse. But the module describes
// fine, lists capabilities fine, and fails at INVOKE time for a reason invisible
// from discovery.
func TestAnUnresolvableBundleRootIsReported(t *testing.T) {
	home := t.TempDir() // no modules directory at all

	w := bundleRootWarnings(home, declaringBundleRoot("facet_bundle"))
	if len(w) == 0 {
		t.Fatal("a module declaring a bundle root the host cannot resolve" +
			" produced no warning, so the root is silently not supplied and the" +
			" failure surfaces only at invocation")
	}
	if !strings.Contains(w[0], "facet_bundle") {
		t.Errorf("the warning does not name the root: %q", w[0])
	}
	// PIN THE SPECIFIC BRANCH, not merely that a warning exists.
	//
	// My first version asserted only "some warning naming facet_bundle", and a
	// mutation disabling this branch PASSED: with dir == "" the code falls
	// through to os.ReadDir(""), which fails, and the UNREADABLE case fires
	// instead -- a different message, still a warning, still naming the root.
	//
	// Two distinct causes producing one observable, caught only because the
	// mutant landed and the test stayed green. The remedies differ: "not
	// installed" versus "installed and unreadable".
	if !strings.Contains(w[0], "could not be resolved") {
		t.Errorf("the warning does not identify the UNRESOLVABLE case"+
			" specifically, so it cannot be told from an unreadable one: %q", w[0])
	}
}

// An EMPTY bundle directory is reported too, and this is the subtler case.
//
// It RESOLVES, so the root IS granted -- and a granted root containing nothing
// is indistinguishable at invoke time from one whose content was never
// installed. That is a sibling lane's finding exactly: their build script
// silently failed to copy a renderer's package.json, and their module still
// described itself correctly and still passed their own bundle-current check,
// because neither looks at runtime content.
func TestAnEmptyBundleDirectoryIsReported(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(ModulesDir(home), "xibodev.facet")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	w := bundleRootWarnings(home, declaringBundleRoot("facet_bundle"))
	if len(w) == 0 {
		t.Fatal("an EMPTY bundle directory produced no warning. It resolves, so" +
			" the root is granted and contains nothing -- a capability depending" +
			" on bundled content fails with no indication the content was never" +
			" installed")
	}
	if !strings.Contains(w[0], "EMPTY") {
		t.Errorf("the warning does not distinguish empty from unresolvable,"+
			" which need different fixes: %q", w[0])
	}
}

// A populated bundle produces NOTHING.
//
// A warning on every healthy module would train people to ignore the panel that
// also carries real findings -- the same reason staleContentWarnings stays
// silent when content matches.
func TestAPopulatedBundleIsQuiet(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(ModulesDir(home), "xibodev.facet")
	if err := os.MkdirAll(filepath.Join(dir, "remotion-composer"), 0o755); err != nil {
		t.Fatal(err)
	}

	if w := bundleRootWarnings(home, declaringBundleRoot("facet_bundle")); len(w) != 0 {
		t.Errorf("a populated bundle produced warnings: %v", w)
	}
}

// A module declaring NO bundle root is never warned about.
//
// Midden declares only source stores (claude_store and friends) and has no
// bundle at all. Reporting anything for it would be a warning about something it
// never asked for.
func TestAModuleWithNoBundleRootIsNeverWarned(t *testing.T) {
	home := t.TempDir()
	d := &modproto.Descriptor{
		Module: "midden",
		Permissions: modproto.Permissions{
			FilesystemRead: []string{"claude_store", "copilot_store"},
		},
	}
	if w := bundleRootWarnings(home, d); len(w) != 0 {
		t.Errorf("a module declaring no bundle root was warned about: %v", w)
	}
}
