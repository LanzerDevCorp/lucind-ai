package result_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

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

func TestEnvelopeAndSchemaReflectionPin(t *testing.T) {
	data := result.SchemaJSON()
	var schema struct {
		Properties map[string]struct {
			Type  string `json:"type"`
			Items *struct {
				Type string `json:"type"`
			} `json:"items"`
		} `json:"properties"`
		Required             []string `json:"required"`
		AdditionalProperties bool     `json:"additionalProperties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshaling schema: %v", err)
	}

	if schema.AdditionalProperties {
		t.Error("schema additionalProperties must be false")
	}

	// Check Envelope struct fields via reflection
	envType := reflect.TypeOf(result.Envelope{})
	structFields := make(map[string]reflect.StructField)
	for i := 0; i < envType.NumField(); i++ {
		field := envType.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := tag
		if idx := strings.IndexByte(name, ','); idx != -1 {
			name = name[:idx]
		}
		structFields[name] = field
	}

	// Every struct field with json tag must exist in schema properties
	for jsonName := range structFields {
		if _, ok := schema.Properties[jsonName]; !ok {
			t.Errorf("Envelope struct has field tagged %q which is missing from result.schema.json properties", jsonName)
		}
	}

	// Every schema property must exist in Envelope struct
	for propName := range schema.Properties {
		if _, ok := structFields[propName]; !ok {
			t.Errorf("result.schema.json has property %q which is missing from Envelope struct", propName)
		}
	}

	// Specifically verify SkillsLoaded reflection pin
	field, ok := structFields["skills_loaded"]
	if !ok {
		t.Fatal("Envelope struct is missing SkillsLoaded field with json:\"skills_loaded,omitempty\" tag")
	}
	if field.Type != reflect.TypeOf([]string(nil)) {
		t.Errorf("Envelope.SkillsLoaded type = %v, want []string", field.Type)
	}

	// Specifically verify skills_loaded in schema
	prop, ok := schema.Properties["skills_loaded"]
	if !ok {
		t.Fatal("result.schema.json properties is missing skills_loaded")
	}
	if prop.Type != "array" {
		t.Errorf("schema skills_loaded.type = %q, want %q", prop.Type, "array")
	}
	if prop.Items == nil || prop.Items.Type != "string" {
		t.Errorf("schema skills_loaded.items.type = %v, want string", prop.Items)
	}

	// Verify skills_loaded is optional (not in required)
	for _, req := range schema.Required {
		if req == "skills_loaded" {
			t.Error("skills_loaded must be optional, but was found in schema required list")
		}
	}
}

func TestReadEnvelopeWithSkillsLoaded(t *testing.T) {
	src := `{
		"packet_id": "test-skills",
		"status": "done",
		"summary": "Loaded skills verified.",
		"hard_stops": [],
		"skills_loaded": ["lucind-executor", "sdd-apply"]
	}`
	fsys := fstest.MapFS{
		"result.json": {Data: []byte(src)},
	}

	e, err := result.Read(fsys, "result.json")
	if err != nil {
		t.Fatalf("Read() error = %v, want nil", err)
	}
	if got, want := len(e.SkillsLoaded), 2; got != want {
		t.Fatalf("len(SkillsLoaded) = %d, want %d", got, want)
	}
	if e.SkillsLoaded[0] != "lucind-executor" || e.SkillsLoaded[1] != "sdd-apply" {
		t.Errorf("SkillsLoaded = %v, want [lucind-executor sdd-apply]", e.SkillsLoaded)
	}
}

func TestReadEnvelopeWithInvalidSkillsLoadedFailsValidation(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "skills_loaded as string instead of array",
			json: `{
				"packet_id": "test-skills",
				"status": "done",
				"summary": "Invalid skills loaded.",
				"hard_stops": [],
				"skills_loaded": "lucind-executor"
			}`,
		},
		{
			name: "skills_loaded with non-string items",
			json: `{
				"packet_id": "test-skills",
				"status": "done",
				"summary": "Invalid skills loaded items.",
				"hard_stops": [],
				"skills_loaded": [123, true]
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"result.json": {Data: []byte(tc.json)},
			}
			_, err := result.Read(fsys, "result.json")
			if err == nil {
				t.Fatal("Read() error = nil, want schema validation error")
			}
			if !errors.Is(err, result.ErrSchemaInvalid) {
				t.Errorf("Read() error = %v, want ErrSchemaInvalid", err)
			}
		})
	}
}
