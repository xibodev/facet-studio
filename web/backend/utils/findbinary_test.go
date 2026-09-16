package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The launcher must find the binary the BUILD ACTUALLY PRODUCES.
//
// This is the defect that made the gateway unstartable: the Makefile builds
// BINARY_NAME=facet-studio, so `facet-studio.exe` lands on disk, and this
// lookup searched only for `facetstudio.exe` -- no hyphen. The stat missed, the
// bare name was returned, exec failed, and the launcher logged "Gateway process
// exited: exit status 1" five times without once naming the path it tried.
//
// A user-visible feature was dead and the log said nothing usable. That is why
// the fallback now warns with the directory and the names attempted.
func TestBothBinarySpellingsAreFound(t *testing.T) {
	for _, name := range spellingsForOS() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
				t.Fatal(err)
			}

			// EnvBinary is the explicit override and takes precedence, which is
			// the mechanism this test can use to point the lookup at a
			// controlled directory without moving the test executable.
			t.Setenv("FACET_STUDIO_BINARY", path)

			got := FindFacetStudioBinary()
			if got != path {
				t.Fatalf("FindFacetStudioBinary() = %q, want %q. The build"+
					" produces facet-studio(.exe); a lookup that misses it"+
					" leaves the gateway unstartable", got, path)
			}
		})
	}
}

// The HYPHENATED name is preferred, because that is what the Makefile builds.
//
// Pinned as an ordering rather than a set: if both are present, picking the
// hyphenless one would run whatever stale copy an older install left behind.
func TestTheHyphenatedNameIsPreferredWhenBothExist(t *testing.T) {
	names := spellingsForOS()
	if len(names) < 2 {
		t.Skip("platform has a single spelling")
	}
	if !strings.Contains(names[0], "-") {
		t.Fatalf("the first candidate %q is not the hyphenated form, so a stale"+
			" hyphenless install would win over the built binary", names[0])
	}
}

// An absent binary returns the hyphenated name rather than an empty string.
//
// Returning "" would make exec fail with a message about an empty path, which
// is less useful than a name a person can search for.
func TestAnAbsentBinaryFallsBackToAName(t *testing.T) {
	t.Setenv("FACET_STUDIO_BINARY", "")
	got := FindFacetStudioBinary()
	if got == "" {
		t.Fatal("returned an empty path; exec would then fail with a message" +
			" naming nothing at all")
	}
}

func spellingsForOS() []string {
	if runtime.GOOS == "windows" {
		return []string{"facet-studio.exe", "facetstudio.exe"}
	}
	return []string{"facet-studio", "facetstudio"}
}
