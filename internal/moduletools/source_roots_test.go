package moduletools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xibodev/facet-studio/pkg/modproto"
)

func sourceRootDescriptor() *modproto.Descriptor {
	return &modproto.Descriptor{
		Module: "source.fixture",
		Permissions: modproto.Permissions{
			FilesystemRead: []string{"claude_store", "copilot_store", "opencode_store"},
		},
	}
}

func TestGrantRootsWithSourceOverridesUsesOnlyExplicitAbsoluteDirectories(t *testing.T) {
	home := t.TempDir()
	claude := filepath.Join(t.TempDir(), "claude")
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	opencode := filepath.Join(opencodeDir, "opencode.db")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opencode, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := GrantRootsWithSourceOverrides(sourceRootDescriptor(), home, "", map[string]string{
		"claude_store":   claude,
		"copilot_store":  "relative/copilot",
		"opencode_store": opencode,
	})
	for name, want := range map[string]string{"claude_store": claude, "opencode_store": opencode} {
		root, ok := roots[name]
		if !ok || root.Path != filepath.Clean(want) || root.Mode != "ro" {
			t.Fatalf("root %s = %#v", name, root)
		}
	}
	if _, ok := roots["copilot_store"]; ok {
		t.Fatal("relative explicit root was granted")
	}
}

func TestGrantRootsWithSourceOverridesNormalizesOpenCodeDirectoryToDatabase(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "opencode.db")
	if err := os.WriteFile(database, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := GrantRootsWithSourceOverrides(sourceRootDescriptor(), t.TempDir(), "", map[string]string{
		"opencode_store": dir,
	})
	if root := roots["opencode_store"]; root.Path != database || root.Mode != "ro" {
		t.Fatalf("opencode root = %#v, want %q read-only", root, database)
	}
}

func TestNormalizeExplicitSourceRootRejectsWrongShapes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-opencode.db")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{
		"claude_store":   file,
		"copilot_store":  file,
		"opencode_store": file,
	} {
		if got, ok := NormalizeExplicitSourceRoot(name, path); ok {
			t.Fatalf("NormalizeExplicitSourceRoot(%q, %q) = %q, want rejection", name, path, got)
		}
	}
}

func TestGrantRootsWithExplicitOverridesNeverFallsBackToAmbientHome(t *testing.T) {
	ambient := t.TempDir()
	t.Setenv("HOME", ambient)
	t.Setenv("USERPROFILE", ambient)
	for _, path := range []string{".claude", ".copilot", filepath.Join(".local", "share", "opencode")} {
		if err := os.MkdirAll(filepath.Join(ambient, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roots := GrantRootsWithSourceOverrides(sourceRootDescriptor(), t.TempDir(), "", map[string]string{
		"claude_store": filepath.Join(ambient, "missing"),
	})
	for _, name := range []string{"claude_store", "copilot_store", "opencode_store"} {
		if _, ok := roots[name]; ok {
			t.Fatalf("%s fell back to ambient home", name)
		}
	}
}

func TestGrantRootsDefaultBehaviorStillUsesAmbientStores(t *testing.T) {
	ambient := t.TempDir()
	t.Setenv("HOME", ambient)
	t.Setenv("USERPROFILE", ambient)
	claude := filepath.Join(ambient, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	roots := GrantRoots(sourceRootDescriptor(), t.TempDir(), "")
	root, ok := roots["claude_store"]
	if !ok || root.Path != claude || root.Mode != "ro" {
		t.Fatalf("default claude root = %#v", root)
	}
}
