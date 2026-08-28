// Package packet parses a dispatch packet: a Markdown document whose
// frontmatter carries the fields the binary needs to route and run a lane,
// and whose body is the prompt handed to the executor verbatim.
package packet

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// delimiter opens and closes the frontmatter block.
const delimiter = "---"

// Parse returns one of these when a document cannot be dispatched as
// written. A packet missing any of them is not a packet with defaults; it
// is an instruction the binary would have to invent, which is exactly what
// dispatch must never do.
var (
	ErrNoFrontmatter        = errors.New("packet: document has no closed --- frontmatter block")
	ErrMissingID            = errors.New("packet: frontmatter is missing a non-empty id")
	ErrMissingExecutor      = errors.New("packet: frontmatter is missing a non-empty executor")
	ErrMissingRoutedBy      = errors.New("packet: frontmatter is missing a non-empty routed_by")
	ErrEmptyBody            = errors.New("packet: body is empty, there is no prompt to dispatch")
	ErrInvalidReadOnly      = errors.New("packet: frontmatter read_only must be a boolean (true or false)")
	ErrInvalidLegacyMain    = errors.New("packet: frontmatter legacy_main must be a boolean (true or false)")
	ErrInvalidAllowedPaths  = errors.New("packet: frontmatter allowed_paths must be a JSON array of strings")
	ErrInvalidReadOnlyPaths = errors.New("packet: frontmatter read_only_paths must be a JSON array of strings")
)

// Authoring is immutable typed input retained for candidate evidence. It is
// nil for manually authored packets, which remain on the legacy evidence path.
type Authoring struct {
	ContractVersion string
	Digest          string
	ContractJSON    []byte
	BindingJSON     []byte
}

// Packet is one unit of delegated work.
type Packet struct {
	// ID names the lane, its branch, and its worktree directory.
	ID string
	// Executor selects the runtime that carries out the work.
	Executor string
	// RoutedBy is the condition that caused this packet to be routed to
	// Executor — never the executor's own name. The executor is the
	// outcome of a routing decision, not its reason: recording it as the
	// condition would be implicit routing, which the skill forbids.
	RoutedBy string
	// Model selects which model the executor dispatches with. It is
	// optional: an absent model key leaves this at its zero value. Each
	// executor owns its own default (see executor.Executor.DefaultModel),
	// applied by internal/run when this field is empty — not filled in
	// here, so Parse keeps reflecting frontmatter literally.
	Model string
	// Agent selects which named opencode agent (passed as --agent) the
	// executor dispatches with. It is optional and only meaningful for the
	// opencode executor -- other executors ignore it. Like Model, an absent
	// agent key leaves this at its zero value; Parse reflects frontmatter
	// literally and injects no default.
	Agent string
	// ReadOnly marks this packet as exploration or read-only: it must
	// produce no commits and leave a clean worktree. When absent, it
	// defaults to false (write packet).
	ReadOnly bool
	// AllowedPaths restricts the repository-relative paths this packet is
	// permitted to touch. When omitted or empty, the packet is undeclared
	// and path checks are skipped.
	AllowedPaths []string
	// ReadOnlyPaths declares executor-visible inputs without granting write
	// authority. AllowedPaths remains the sole write scope.
	ReadOnlyPaths []string
	// Authoring carries a compiled contract when one was admitted in-process.
	Authoring *Authoring
	// Feature identifies the target feature for parent integration.
	Feature string
	// ParentRef is the target parent git reference (e.g. refs/heads/feature/foo).
	ParentRef string
	// BaseSHA is the immutable commit SHA where the feature was branched.
	BaseSHA string
	// ExpectedParentSHA is the expected commit SHA of ParentRef before promotion.
	ExpectedParentSHA string
	// LegacyMain indicates legacy mode dispatch targeting main.
	LegacyMain bool
	// SDDPhase is the optional planning/apply phase declared in frontmatter
	// (sdd_phase). Omitted or empty keys leave this at "".
	SDDPhase string
	// FanoutGroup is the optional fan-out group declared in frontmatter
	// (fanout_group). Omitted or empty keys leave this at "".
	FanoutGroup string
	// Skill is the optional static skill name declared in frontmatter
	// (skill). Omitted or empty keys leave this at "". Parse never invents
	// live Skill telemetry — it only reflects the frontmatter key.
	Skill string
	// Path is the on-disk packet path. Parse does not set it; the CLI
	// assigns it from the --packet flag after a successful Parse.
	Path string
	// Body is the Markdown prompt, passed to the executor unchanged.
	Body string
}

// Parse reads a packet document from r.
func Parse(r io.Reader) (Packet, error) {
	var p Packet

	sc := bufio.NewScanner(r)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != delimiter {
		return Packet{}, ErrNoFrontmatter
	}

	closed := false
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == delimiter {
			closed = true
			break
		}
		key, value, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(key) {
		case "id":
			p.ID = strings.TrimSpace(value)
		case "executor":
			p.Executor = strings.TrimSpace(value)
		case "routed_by":
			p.RoutedBy = strings.TrimSpace(value)
		case "model":
			p.Model = strings.TrimSpace(value)
		case "agent":
			p.Agent = strings.TrimSpace(value)
		case "read_only":
			switch strings.TrimSpace(value) {
			case "true":
				p.ReadOnly = true
			case "false":
				p.ReadOnly = false
			default:
				return Packet{}, ErrInvalidReadOnly
			}
		case "feature":
			p.Feature = strings.TrimSpace(value)
		case "parent_ref":
			p.ParentRef = strings.TrimSpace(value)
		case "base_sha":
			p.BaseSHA = strings.TrimSpace(value)
		case "expected_parent_sha":
			p.ExpectedParentSHA = strings.TrimSpace(value)
		case "legacy_main":
			switch strings.TrimSpace(value) {
			case "true":
				p.LegacyMain = true
			case "false":
				p.LegacyMain = false
			default:
				return Packet{}, ErrInvalidLegacyMain
			}
		case "sdd_phase":
			p.SDDPhase = strings.TrimSpace(value)
		case "fanout_group":
			p.FanoutGroup = strings.TrimSpace(value)
		case "skill":
			p.Skill = strings.TrimSpace(value)
		case "allowed_paths":
			trimmed := strings.TrimSpace(value)
			var paths []string
			if err := json.Unmarshal([]byte(trimmed), &paths); err != nil {
				return Packet{}, ErrInvalidAllowedPaths
			}
			p.AllowedPaths = paths
		case "read_only_paths":
			trimmed := strings.TrimSpace(value)
			var paths []string
			if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal([]byte(trimmed), &paths) != nil {
				return Packet{}, ErrInvalidReadOnlyPaths
			}
			p.ReadOnlyPaths = paths
		}
	}

	if !closed {
		return Packet{}, ErrNoFrontmatter
	}

	var body strings.Builder
	for sc.Scan() {
		body.WriteString(sc.Text())
		body.WriteString("\n")
	}
	if err := sc.Err(); err != nil {
		return Packet{}, err
	}
	p.Body = strings.TrimLeft(body.String(), "\n")

	switch {
	case p.ID == "":
		return Packet{}, ErrMissingID
	case p.Executor == "":
		return Packet{}, ErrMissingExecutor
	case p.RoutedBy == "":
		return Packet{}, ErrMissingRoutedBy
	case strings.TrimSpace(p.Body) == "":
		return Packet{}, ErrEmptyBody
	}

	return p, nil
}
