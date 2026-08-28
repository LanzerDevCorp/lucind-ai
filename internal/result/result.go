// Package result reads and validates the envelope a dispatched agent leaves
// at ".lucind/result.json" in its worktree once it finishes. The envelope
// is the only signal the binary trusts about what happened in that
// worktree, so it is validated against the authoritative JSON Schema
// (embedded from result.schema.json) before anything downstream reads a
// single field from it. A schema-invalid envelope is never silently
// accepted with zero values: that would let a violated hard stop slip
// through as if nothing had fired.
package result

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

// ErrSchemaInvalid is returned by Read when the envelope's JSON does not
// satisfy result.schema.json — for example a missing required field, an
// unknown top-level property, or a status outside the enum. The wrapped
// error carries the validator's detail.
var ErrSchemaInvalid = errors.New("result: envelope violates the schema")

// HardStop is one packet-declared hard stop and whether it fired. The
// schema requires one entry per hard stop the packet listed, whether or
// not it fired — this is the field that has twice caught a violated hard
// stop that green done-criteria alone would have hidden.
type HardStop struct {
	HardStop string `json:"hard_stop"`
	Fired    bool   `json:"fired"`
	Note     string `json:"note,omitempty"`
}

// FileChange is one canonical path change inside the
// worktree, relative to the worktree root. For a path outside the
// worktree, see ExternalChange — that boundary is why the two types are
// kept separate rather than one field with an ambiguous path.
type FileChange struct {
	Change     string `json:"change"`
	SourcePath string `json:"source_path,omitempty"`
	Path       string `json:"path"`
	Why        string `json:"why,omitempty"`
}

// ExternalChange is one path created, modified, or deleted outside the
// worktree — a config file, a dotfile, machine-level setup. Path is
// absolute or "~"-prefixed, never worktree-relative, so it can never be
// confused with a FileChange.Path.
//
// Work inside the worktree lives on a branch: it can be diffed, merged, or
// thrown away, and Git is the undo. Work outside the worktree has none of
// that — Git never saw it, so there is no diff to review and no branch to
// discard. That is why Why and Revert are both required here even though
// Why is optional on FileChange: there is no diff to infer intent from,
// and no version control to fall back on for reversal. Revert must name
// the backup path or the exact command that restores the previous state —
// without it, an external change is a fact discovered later, not
// something that was audited.
type ExternalChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
	Why    string `json:"why"`
	Revert string `json:"revert"`
}

// DoneCriterion is one done-criterion from the packet, with evidence that
// it was met.
type DoneCriterion struct {
	Criterion string `json:"criterion"`
	Met       bool   `json:"met"`
	Evidence  string `json:"evidence,omitempty"`
}

// Question is one question that blocks the packet, required when Status is
// "blocked".
type Question struct {
	Question       string   `json:"question"`
	WhyBlocking    string   `json:"why_blocking"`
	Options        []string `json:"options,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}

// Deviation is one departure from the packet's stated approach, required
// when Status is "deviated".
type Deviation struct {
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Reason     string `json:"reason"`
	Reversible bool   `json:"reversible,omitempty"`
}

// Finding is something discovered that the packet did not ask about but
// that changes other work.
type Finding struct {
	Finding  string `json:"finding"`
	Evidence string `json:"evidence"`
	Affects  string `json:"affects,omitempty"`
}

// Envelope mirrors result.schema.json.
type Envelope struct {
	PacketID        string           `json:"packet_id"`
	Status          string           `json:"status"`
	Summary         string           `json:"summary"`
	HardStops       []HardStop       `json:"hard_stops"`
	FilesChanged    []FileChange     `json:"files_changed,omitempty"`
	ExternalChanges []ExternalChange `json:"external_changes,omitempty"`
	DoneCriteria    []DoneCriterion  `json:"done_criteria,omitempty"`
	Commit          string           `json:"commit,omitempty"`
	Questions       []Question       `json:"questions,omitempty"`
	Deviations      []Deviation      `json:"deviations,omitempty"`
	Findings        []Finding        `json:"findings,omitempty"`
	SessionID       string           `json:"session_id,omitempty"`
}

// LaneStatus maps the envelope's status field to the project's lane
// vocabulary. The schema's status enum (done, blocked, deviated, failed)
// maps 1:1 onto the four terminal lane.Status values; Read having already
// validated the envelope against the schema is what makes this mapping
// total in practice.
func (e Envelope) LaneStatus() lane.Status {
	switch e.Status {
	case "done":
		return lane.Done
	case "blocked":
		return lane.Blocked
	case "deviated":
		return lane.Deviated
	case "failed":
		return lane.Failed
	default:
		return ""
	}
}

// Read reads the result envelope at path in fsys, validates it against the
// embedded schema, and unmarshals it into an Envelope. fsys is an fs.FS
// rather than a bare filesystem path so that this package is testable with
// fstest.MapFS; real filesystem access stays the caller's concern.
func Read(fsys fs.FS, path string) (Envelope, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return Envelope{}, fmt.Errorf("result: reading %s: %w", path, err)
	}

	var instance any
	if err := json.Unmarshal(data, &instance); err != nil {
		return Envelope{}, fmt.Errorf("result: parsing %s: %w", path, err)
	}

	if err := compiledSchema.Validate(instance); err != nil {
		return Envelope{}, fmt.Errorf("%w: %s", ErrSchemaInvalid, err)
	}

	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("result: parsing %s: %w", path, err)
	}

	return e, nil
}
