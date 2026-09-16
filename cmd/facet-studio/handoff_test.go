package main

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The bug this exists for, found by a module author running the command: step 3
// printed its intent and returned nil. `handoff` exited 0 having done two of
// three steps, which reads as SUCCESS to anything scripting it.
//
// A silent exit-0 mid-pipeline is the least diagnosable failure available: no
// step named, no reason, no failure marker.
func TestAMissingConsumerIsReportedNotSkipped(t *testing.T) {
	// Point Home at the real install tree: a test binary lives in a temp
	// directory, so the executable-anchored default finds no modules there.
	t.Setenv("FACET_STUDIO_HOME", realHome(t))

	_, err := seedConsumingCapability("xibodev.facet")

	if err != nil && strings.Contains(err.Error(), "not installed") {
		t.Skip("xibodev.facet is not installed here; nothing to assert about")
	}
	if err == nil {
		// A capability was found. That is legitimate ONLY if it really accepts
		// a seed -- silently returning some other capability is exactly the
		// bug this guards, and a t.Skip here would let that pass.
		found, ferr := seedConsumingCapability("xibodev.facet")
		if ferr != nil {
			t.Fatalf("inconsistent lookup: %v", ferr)
		}
		if !mentionsSeed(t, "xibodev.facet", found) {
			t.Fatalf("resolved %q as the seed consumer, but it neither names a"+
				" seed nor accepts seed_path -- step 3 would invoke the wrong"+
				" capability and report success", found)
		}
		return
	}

	msg := err.Error()
	for _, want := range []string{
		"no capability that accepts a seed", // what is missing
		"creative.tools.run",                // what it DOES declare, so a reader can check
		"producer side",                     // whose half is met
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, msg)
		}
	}
}

// realHome locates the repo's .local beside this source file, so the test does
// not depend on the working directory it happens to run in.
func realHome(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("cannot locate the source tree")
	}
	// cmd/facet-studio/handoff_test.go -> repo root -> .local
	return filepath.Join(filepath.Dir(thisFile), "..", "..", ".local")
}

// A module that is not installed must be distinguishable from one that is
// installed but lacks the capability -- they need different actions.
func TestAnAbsentModuleIsADifferentErrorFromAMissingCapability(t *testing.T) {
	_, err := seedConsumingCapability("no.such.module")
	if err == nil {
		t.Fatal("a module that is not installed resolved a capability")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("an absent module was not reported as absent:\n%v", err)
	}
	if strings.Contains(err.Error(), "no capability that accepts a seed") {
		t.Fatalf("an absent module was reported as a missing capability:\n%v", err)
	}
}

// mentionsSeed reports whether a capability genuinely accepts a seed, by the
// SAME evidence the resolver uses: a "seed" property in its request schema,
// looked up through the descriptor's RequestSchemas map.
//
// It used to check for an id containing "seed" or a "seed_path" field, which
// were guesses about a naming convention rather than the declared contract --
// the same guesses that made the resolver miss a consumer that had published
// its half.
func mentionsSeed(t *testing.T, moduleID, capabilityID string) bool {
	t.Helper()
	runners, err := discover()
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	for _, r := range runners {
		d, _, describeErr := r.Describe(context.Background())
		if describeErr != nil || d.Module != moduleID {
			continue
		}
		for _, c := range d.Capabilities {
			if c.ID == capabilityID {
				return declaresSeedField(d.RequestSchemas[c.RequestSchema])
			}
		}
	}
	return false
}
