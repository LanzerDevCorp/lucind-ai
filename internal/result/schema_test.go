package result_test

import (
	"encoding/json"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/result"
)

func TestSchemaJSONParsesAsJSON(t *testing.T) {
	data := result.SchemaJSON()

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("SchemaJSON() bytes do not parse as JSON: %v", err)
	}
}

func TestSchemaJSONReturnsDefensiveCopy(t *testing.T) {
	first := result.SchemaJSON()
	if len(first) == 0 {
		t.Fatal("SchemaJSON() returned empty bytes")
	}

	// Mutate the slice the caller got back.
	original := first[0]
	first[0] = original + 1

	second := result.SchemaJSON()
	if second[0] != original {
		t.Errorf("mutating a previously returned slice affected a later call: second[0] = %v, want %v", second[0], original)
	}
}
