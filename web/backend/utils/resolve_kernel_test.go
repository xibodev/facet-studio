package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The shell must resolve a kernel named facet-studio-kernel sitting beside it.
func TestResolvesTheKernelBesideTheShell(t *testing.T) {
	dir := t.TempDir()
	name := "facet-studio-kernel"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FACET_STUDIO_BINARY", filepath.Join(dir, name))

	got := FindFacetStudioBinary()
	if !strings.Contains(got, "facet-studio-kernel") {
		t.Fatalf("resolved %q, which is not the kernel beside the shell", got)
	}
}
