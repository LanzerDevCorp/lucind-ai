package result

import (
	_ "embed"
	"encoding/json"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemaJSON is the authoritative result envelope contract. It is embedded
// here — not read from disk at runtime — because the binary, not the
// plugin that dispatches work, is the rightful owner of what a result
// envelope must look like: go:embed also cannot reach outside this
// package's directory, which forces the schema to live where its owner
// lives.
//
//go:embed result.schema.json
var schemaJSON []byte

// schemaResourceURL is an arbitrary, stable identifier used to register and
// then compile the embedded schema. It is never dereferenced over the
// network; jsonschema/v6 only uses it as a resource key.
const schemaResourceURL = "lucind-ai://internal/result/result.schema.json"

// compiledSchema is compiled once at package init from the embedded bytes.
// A schema that fails to compile is a build-time defect, not a runtime one,
// so this panics rather than surfacing a runtime error on first use.
var compiledSchema = mustCompileSchema()

// SchemaJSON returns the raw bytes of the embedded result.schema.json, so a
// caller (for example the executor package, writing it to disk for agy's
// --json-schema flag) can consume the same contract this package validates
// against.
//
// It returns a defensive copy on every call: schemaJSON is a package-level
// slice shared across the whole process, and handing out the live backing
// array would let one caller's mutation corrupt every other caller (and
// this package's own validation) silently.
func SchemaJSON() []byte {
	cp := make([]byte, len(schemaJSON))
	copy(cp, schemaJSON)
	return cp
}

func mustCompileSchema() *jsonschema.Schema {
	var doc any
	if err := json.Unmarshal(schemaJSON, &doc); err != nil {
		panic("result: embedded schema is invalid JSON: " + err.Error())
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaResourceURL, doc); err != nil {
		panic("result: embedded schema failed to register: " + err.Error())
	}

	sch, err := c.Compile(schemaResourceURL)
	if err != nil {
		panic("result: embedded schema failed to compile: " + err.Error())
	}

	return sch
}
