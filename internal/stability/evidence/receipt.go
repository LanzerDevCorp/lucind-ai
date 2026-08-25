package evidence

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// TrialRecord holds execution results and diagnostic evidence for one Stability Trial.
type TrialRecord struct {
	TrialNumber int                `json:"trial_number"`
	Verdict     string             `json:"verdict"`
	Diagnostics DiagnosticEvidence `json:"diagnostics,omitempty"`
}

// Receipt is an immutable, content-addressed stability certification record.
type Receipt struct {
	ReceiptID      string            `json:"receipt_id"`
	CandidateSHA   string            `json:"candidate_sha"`
	BuildVersion   string            `json:"build_version"`
	Verdict        string            `json:"verdict"`
	CreatedAt      string            `json:"created_at"`
	BaselineCheck  string            `json:"baseline_check,omitempty"`
	FixtureDigests map[string]string `json:"fixture_digests,omitempty"`
	Trials         []TrialRecord     `json:"trials,omitempty"`
}

// CanonicalJSON serializes the Receipt into canonical JSON according to RFC 8785 (JCS).
func (r Receipt) CanonicalJSON() ([]byte, error) {
	return MarshalCanonical(r)
}

// WriteReceipt serializes the receipt canonically and writes it to the destination path.
func WriteReceipt(path string, r Receipt) error {
	data, err := r.CanonicalJSON()
	if err != nil {
		return fmt.Errorf("stability/evidence: canonicalize receipt: %w", err)
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("stability/evidence: create receipt parent directory: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("stability/evidence: write receipt file: %w", err)
	}

	return nil
}

// MarshalCanonical serializes any Go data structure into RFC 8785 (JCS) canonical JSON.
func MarshalCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonicalValue(&buf, reflect.ValueOf(v)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonicalValue(buf *bytes.Buffer, val reflect.Value) error {
	if !val.IsValid() {
		buf.WriteString("null")
		return nil
	}

	switch val.Kind() {
	case reflect.Interface, reflect.Pointer:
		if val.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return writeCanonicalValue(buf, val.Elem())

	case reflect.Bool:
		if val.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(val.Int(), 10))
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		buf.WriteString(strconv.FormatUint(val.Uint(), 10))
		return nil

	case reflect.Float32:
		f := val.Float()
		buf.WriteString(formatCanonicalFloat(f, 32))
		return nil

	case reflect.Float64:
		f := val.Float()
		buf.WriteString(formatCanonicalFloat(f, 64))
		return nil

	case reflect.String:
		writeCanonicalString(buf, val.String())
		return nil

	case reflect.Slice, reflect.Array:
		if val.Kind() == reflect.Slice && val.IsNil() {
			buf.WriteString("null")
			return nil
		}
		buf.WriteByte('[')
		n := val.Len()
		for i := 0; i < n; i++ {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalValue(buf, val.Index(i)); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil

	case reflect.Map:
		if val.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return writeCanonicalMap(buf, val)

	case reflect.Struct:
		return writeCanonicalStruct(buf, val)

	default:
		return fmt.Errorf("stability/evidence: unsupported type for canonical JSON: %s", val.Type())
	}
}

func writeCanonicalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
}

func formatCanonicalFloat(f float64, bitSize int) string {
	// Format float according to ECMAScript / RFC 8785 specifications
	s := strconv.FormatFloat(f, 'f', -1, bitSize)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
		return s
	}
	// Truncate unnecessary trailing zeros after decimal point
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func compareUTF16(a, b string) int {
	u1 := utf16.Encode([]rune(a))
	u2 := utf16.Encode([]rune(b))
	minLen := len(u1)
	if len(u2) < minLen {
		minLen = len(u2)
	}
	for i := 0; i < minLen; i++ {
		if u1[i] < u2[i] {
			return -1
		} else if u1[i] > u2[i] {
			return 1
		}
	}
	if len(u1) < len(u2) {
		return -1
	} else if len(u1) > len(u2) {
		return 1
	}
	return 0
}

type memberEntry struct {
	key   string
	value reflect.Value
}

func writeCanonicalMap(buf *bytes.Buffer, val reflect.Value) error {
	keys := val.MapKeys()
	entries := make([]memberEntry, 0, len(keys))

	for _, k := range keys {
		keyStr := fmt.Sprintf("%v", k.Interface())
		entries = append(entries, memberEntry{
			key:   keyStr,
			value: val.MapIndex(k),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return compareUTF16(entries[i].key, entries[j].key) < 0
	})

	buf.WriteByte('{')
	for i, entry := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		writeCanonicalString(buf, entry.key)
		buf.WriteByte(':')
		if err := writeCanonicalValue(buf, entry.value); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func writeCanonicalStruct(buf *bytes.Buffer, val reflect.Value) error {
	t := val.Type()
	entries := make([]memberEntry, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" { // unexported field
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}

		fieldName := field.Name
		omitempty := false
		if tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, p := range parts[1:] {
				if p == "omitempty" {
					omitempty = true
				}
			}
		}

		fieldVal := val.Field(i)
		if omitempty && fieldVal.IsZero() {
			continue
		}

		entries = append(entries, memberEntry{
			key:   fieldName,
			value: fieldVal,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return compareUTF16(entries[i].key, entries[j].key) < 0
	})

	buf.WriteByte('{')
	for i, entry := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		writeCanonicalString(buf, entry.key)
		buf.WriteByte(':')
		if err := writeCanonicalValue(buf, entry.value); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}
