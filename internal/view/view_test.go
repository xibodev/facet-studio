package view

import "testing"

// TestMediaTypeClaimIsNotTrustedBlindly records a defect found in production by
// the Facet lane: Google Flow declares image/png for JPEG bytes.
//
// The host cannot detect the truth itself -- it has a media type string, not
// the file -- so the property that matters is that a wrong claim degrades to a
// wrong-but-harmless renderer rather than to a broken cockpit. Both map to
// Image, so a lying provider costs nothing here. Modules detect from file
// signatures at their own boundary, which is where the bytes are.
func TestMediaTypeClaimIsNotTrustedBlindly(t *testing.T) {
	if fromMediaType("image/png") != fromMediaType("image/jpeg") {
		t.Error("a mislabelled image must still resolve to an image primitive")
	}
}

func TestResolveByMediaType(t *testing.T) {
	r := NewRegistry()
	cases := map[string]Primitive{
		"video/mp4":                Video,
		"audio/mpeg":               Audio,
		"image/jpeg":               Image,
		"application/json":         JSON,
		"application/vnd.x+json":   JSON,
		"text/markdown":            Markdown,
		"text/plain; charset=utf8": Text,
		"application/pdf":          Document,
		"image/svg+xml":            Diagram,
		"text/csv":                 Table,
		"application/octet-stream": Download,
		"":                         JSON,
	}
	for mt, want := range cases {
		if got := r.Resolve(mt, ""); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", mt, got, want)
		}
	}
}

// TestPresentationHintWins covers the composition rule: a module declares which
// primitive suits its artefact, and that beats the media type, because a JSON
// document may be a timeline and only the module knows.
func TestPresentationHintWins(t *testing.T) {
	r := NewRegistry()

	if got := r.Resolve("application/json", string(Timeline)); got != Timeline {
		t.Errorf("declared hint ignored: got %q, want %q", got, Timeline)
	}
	// An unknown hint is not honoured: a module cannot invent a primitive, and
	// asking for one must degrade rather than break the card.
	if got := r.Resolve("application/json", "holographic"); got != JSON {
		t.Errorf("unknown primitive was honoured: got %q", got)
	}
}

// TestDisablingIsAPreferenceNotAFailure is the deactivation rule: turning a
// primitive off changes presentation, never validity. The artefact must still
// reach the user.
func TestDisablingIsAPreferenceNotAFailure(t *testing.T) {
	r := NewRegistry()
	r.SetEnabled(Video, false)

	got := r.Resolve("video/mp4", "")
	if got == Video {
		t.Fatal("a disabled primitive was still used")
	}
	if got != Download {
		t.Errorf("got %q; a disabled rendering must fall back so the artefact is still reachable", got)
	}

	// And re-enabling restores it, so the toggle is a preference rather than a
	// one-way door.
	r.SetEnabled(Video, true)
	if r.Resolve("video/mp4", "") != Video {
		t.Error("re-enabling a primitive did not restore it")
	}
}

// TestEveryPrimitiveIsExplained guards the settings surface: a user turning a
// renderer off should know what they are turning off.
func TestEveryPrimitiveIsExplained(t *testing.T) {
	for _, p := range All() {
		if Describe(p) == "" {
			t.Errorf("primitive %q has no description", p)
		}
		if !known(p) {
			t.Errorf("primitive %q is listed but not recognised by known()", p)
		}
	}
}

// TestMiddenOutputTypesAreRenderable guards the correction that widened this
// vocabulary.
//
// The first version was frozen against a picture in which Facet was the rich
// module and Midden's artefacts were "manifests and prose, any table primitive
// covers them". Both lanes said so; both were wrong. Midden's workbench
// declares type-aware previews over a library of PDF, DOCX, EPUB, HTML and SVG
// outputs -- it is multimedia-BROAD where Facet is video-FIRST.
//
// Under the narrow vocabulary every one of those fell to download, including
// PDF -- which is the operator's own deduplication test case, and so could not
// have been satisfied at all.
func TestMiddenOutputTypesAreRenderable(t *testing.T) {
	r := NewRegistry()
	for mt, want := range map[string]Primitive{
		"application/pdf":      Document,
		"application/epub+zip": Document,
		"image/svg+xml":        Diagram,
		"text/csv":             Table,
		"text/html":            Text,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   Document,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": Slides,
	} {
		if got := r.Resolve(mt, ""); got != want {
			t.Errorf("Resolve(%q) = %q, want %q; a broad-output module must not fall to download", mt, got, want)
		}
	}
}
