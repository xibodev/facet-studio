package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleInvokeSourceRootFlagsAreLocalCommandOnly(t *testing.T) {
	cmd := NewModuleInvokeCommand()
	for _, name := range []string{"claude-source-root", "copilot-source-root", "opencode-source-root"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("missing local module-invoke flag %q", name)
		}
	}
	root := NewFacetStudioCommand()
	for _, name := range []string{"claude-source-root", "copilot-source-root", "opencode-source-root"} {
		if root.PersistentFlags().Lookup(name) != nil {
			t.Fatalf("source root flag %q leaked to global/browser-facing configuration", name)
		}
	}
}

func TestCmdInvokeWithSourceRootsRejectsInvalidRootBeforeDiscovery(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "opencode.db")
	_, err := cmdInvokeWithSourceRoots("module", "capability", "{}", map[string]string{
		"opencode_store": missing,
	})
	if err == nil || !strings.Contains(err.Error(), "existing absolute opencode.db file") {
		t.Fatalf("error = %v", err)
	}

	wrongFile := filepath.Join(t.TempDir(), "wrong.db")
	if err := os.WriteFile(wrongFile, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = cmdInvokeWithSourceRoots("module", "capability", "{}", map[string]string{
		"claude_store": wrongFile,
	})
	if err == nil || !strings.Contains(err.Error(), "existing absolute directory") {
		t.Fatalf("error = %v", err)
	}
}
