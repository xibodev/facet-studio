package main

import (
	"encoding/json"
	"testing"

	"github.com/xibodev/facet-studio/pkg/modproto"
)

// The bug this exists for: Capability.RequestSchema is an ID into the
// descriptor's RequestSchemas map, and the protocol has always said so. The
// resolver read the ID AS the schema and tried to parse
// "creative.tools.run.request/v1" as JSON. It failed silently, the resolver
// found nothing, and the host told a module author that the consumer had "not
// published the other half" of the handoff. It had.
//
// An accurate-sounding refusal pointed at the wrong party.
func TestASchemaBodyDeclaringSeedIsRecognised(t *testing.T) {
	body := json.RawMessage(`{
		"type": "object",
		"properties": {
			"tool":  {"type": "string"},
			"input": {"type": "object"},
			"seed":  {"type": "object",
			          "required": ["schema", "path", "digest"],
			          "properties": {"schema": {"type": "string"},
			                         "path":   {"type": "string"},
			                         "digest": {"type": "string"}}}
		}
	}`)

	if !declaresSeedField(body) {
		t.Fatal("a schema declaring a top-level seed object was not recognised")
	}
}

// A schema ID is not a schema. Passing one must not accidentally match.
func TestASchemaIDIsNotASchema(t *testing.T) {
	if declaresSeedField(json.RawMessage("creative.tools.run.request/v1")) {
		t.Fatal("a schema ID was treated as a schema declaring a seed")
	}
}

// The word "seed" in prose must not count. Substring-matching would invoke a
// capability that merely mentions a random seed, which is worse than finding
// nothing: the handoff would report success having done the wrong thing.
func TestSeedInProseDoesNotCount(t *testing.T) {
	body := json.RawMessage(`{
		"type": "object",
		"properties": {
			"noise": {"type": "integer", "description": "the random seed to use"}
		}
	}`)

	if declaresSeedField(body) {
		t.Fatal("the word seed in a description was read as a declared field")
	}
}

// A schema the host cannot parse declares nothing. The host acts on what a
// module states, never on what it might have meant.
func TestAnUnparseableSchemaDeclaresNothing(t *testing.T) {
	for _, s := range []string{"", "{not json", "null", "[]"} {
		if declaresSeedField(json.RawMessage(s)) {
			t.Errorf("unparseable schema %q was read as declaring a seed", s)
		}
	}
}

// A schema with no seed is not a seed consumer.
func TestASchemaWithoutSeedIsNotAConsumer(t *testing.T) {
	body := json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"}}}`)

	if declaresSeedField(body) {
		t.Fatal("a schema with no seed property was read as a seed consumer")
	}
}

// The lookup itself, which is where the bug actually was: the resolver must
// dereference Capability.RequestSchema through the descriptor's RequestSchemas
// map. Testing declaresSeedField alone could not catch reading the ID as a
// schema, because that function never sees the map.
func TestTheResolverDereferencesTheSchemaID(t *testing.T) {
	d := &modproto.Descriptor{
		Module: "test.consumer",
		Capabilities: []modproto.Capability{
			{ID: "thing.status", RequestSchema: "thing.status.request/v1"},
			{ID: "thing.run", RequestSchema: "thing.run.request/v1"},
		},
		RequestSchemas: map[string]json.RawMessage{
			"thing.status.request/v1": json.RawMessage(
				`{"type":"object","properties":{"job_id":{"type":"string"}}}`),
			"thing.run.request/v1": json.RawMessage(
				`{"type":"object","properties":{"seed":{"type":"object"}}}`),
		},
	}

	got := seedCapabilityIn(d)

	if got != "thing.run" {
		t.Fatalf("resolved %q, want thing.run -- the schema ID must be looked"+
			" up in RequestSchemas, not parsed as a schema", got)
	}
}

// When several capabilities accept a seed, prefer the one that DOES the work.
// Taking the first match resolved an estimate capability, which reads the seed
// and prices a job rather than producing anything -- so the handoff would have
// proven the plumbing while producing no artifact.
func TestTheWorkingCapabilityIsPreferredOverAnEstimate(t *testing.T) {
	seed := json.RawMessage(`{"type":"object","properties":{"seed":{"type":"object"}}}`)
	d := &modproto.Descriptor{
		Module: "test.consumer",
		Capabilities: []modproto.Capability{
			{ID: "creative.tools.estimate", RequestSchema: "e/v1"},
			{ID: "creative.tools.run", RequestSchema: "r/v1"},
			{ID: "creative.output.review", RequestSchema: "v/v1"},
		},
		RequestSchemas: map[string]json.RawMessage{"e/v1": seed, "r/v1": seed, "v/v1": seed},
	}

	if got := seedCapabilityIn(d); got != "creative.tools.run" {
		t.Fatalf("resolved %q, want creative.tools.run", got)
	}
}

// No capability accepting a seed resolves to nothing, so the caller reports the
// consumer has not published its half rather than invoking something arbitrary.
func TestNoSeedCapabilityResolvesToNothing(t *testing.T) {
	d := &modproto.Descriptor{
		Module:       "test.consumer",
		Capabilities: []modproto.Capability{{ID: "a.b", RequestSchema: "s/v1"}},
		RequestSchemas: map[string]json.RawMessage{
			"s/v1": json.RawMessage(`{"type":"object","properties":{"x":{"type":"string"}}}`)},
	}

	if got := seedCapabilityIn(d); got != "" {
		t.Fatalf("resolved %q from a descriptor with no seed capability", got)
	}
}
