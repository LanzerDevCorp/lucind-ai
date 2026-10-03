// Package packet parses a dispatch packet: a Markdown document whose
// frontmatter carries the fields the binary needs to route and run a lane,
// and whose body is the prompt handed to the executor verbatim.
package packet

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillset"
)

// delimiter opens and closes the frontmatter block.
const delimiter = "---"

// Parse returns one of these when a document cannot be dispatched as
// written. A packet missing any of them is not a packet with defaults; it
// is an instruction the binary would have to invent, which is exactly what
// dispatch must never do.
var (
	ErrNoFrontmatter                     = errors.New("packet: document has no closed --- frontmatter block")
	ErrMissingID                         = errors.New("packet: frontmatter is missing a non-empty id")
	ErrMissingExecutor                   = errors.New("packet: frontmatter is missing a non-empty executor")
	ErrMissingRoutedBy                   = errors.New("packet: frontmatter is missing a non-empty routed_by")
	ErrEmptyBody                         = errors.New("packet: body is empty, there is no prompt to dispatch")
	ErrInvalidReadOnly                   = errors.New("packet: frontmatter read_only must be a boolean (true or false)")
	ErrInvalidLegacyMain                 = errors.New("packet: frontmatter legacy_main must be a boolean (true or false)")
	ErrInvalidAllowedPaths               = errors.New("packet: frontmatter allowed_paths must be a JSON array of strings")
	ErrInvalidReadOnlyPaths              = errors.New("packet: frontmatter read_only_paths must be a JSON array of strings")
	ErrInvalidLaneRole                   = errors.New("packet: frontmatter lane_role is invalid")
	ErrInvalidSDDPhase                   = errors.New("packet: frontmatter sdd_phase is invalid")
	ErrInvalidAdhocSkills                = errors.New("packet: frontmatter adhoc_skills must be a JSON array of strings")
	ErrInvalidRoute                      = errors.New("packet: frontmatter route must be inline, worker, or fanout")
	ErrInvalidNamedSkillsOnly            = errors.New("packet: frontmatter named_skills_only must be a boolean (true or false)")
	ErrInvalidVerification               = errors.New("packet: frontmatter verification must be a JSON array of strings")
	ErrInvalidKnownEnvironmentalFailures = errors.New("packet: frontmatter known_environmental_failures must be a JSON array of strings")
	ErrInvalidCommitMessage              = errors.New("packet: frontmatter commit_message is invalid")
	ErrCommitMessageNeedsVerification    = errors.New("packet: frontmatter commit_message requires non-empty verification")
	ErrInvalidMaxIterations              = errors.New("packet: frontmatter max_iterations must be an integer between 1 and 4")
	ErrInvalidEscalation                 = errors.New("packet: frontmatter escalation is invalid")
	ErrLoopNeedsVerification             = errors.New("packet: loop and escalation require non-empty verification")
)

var commitMessageRegex = regexp.MustCompile(`^(feat|fix|docs|refactor|test|chore|perf|build|ci|style|revert)(\([a-z0-9._/-]+\))?!?: \S.*$`)

// ValidateCommitMessage validates that msg matches Conventional Commit header rules,
// is at most 100 characters, contains no newlines, and contains no attribution trailers.
func ValidateCommitMessage(msg string) error {
	if strings.ContainsAny(msg, "\r\n") {
		return ErrInvalidCommitMessage
	}
	if len(msg) > 100 {
		return ErrInvalidCommitMessage
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "co-authored-by") || strings.Contains(lower, "generated with") {
		return ErrInvalidCommitMessage
	}
	if !commitMessageRegex.MatchString(msg) {
		return ErrInvalidCommitMessage
	}
	return nil
}

// Authoring is immutable typed input retained for candidate evidence. It is
// nil for manually authored packets, which remain on the legacy evidence path.
type Authoring struct {
	ContractVersion string
	Digest          string
	ContractJSON    []byte
	BindingJSON     []byte
}

// EscalationRung defines an alternate executor and optional model to dispatch
// if verification fails on earlier attempts.
type EscalationRung struct {
	Executor string `json:"executor"`
	Model    string `json:"model,omitempty"`
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
	// LaneRole is the optional lane role declared in frontmatter (lane_role).
	// Closed set: {lens, synthesis, apply, verify, archive, ultrafixer, human}.
	LaneRole string
	// FanoutGroup is the optional fan-out group declared in frontmatter
	// (fanout_group). Omitted or empty keys leave this at "".
	FanoutGroup string
	// Skill is the optional static skill name declared in frontmatter
	// (skill). Omitted or empty keys leave this at "". Parse never invents
	// live Skill telemetry — it only reflects the frontmatter key.
	Skill string
	// AdhocSkills is the optional list of ad-hoc skills declared in frontmatter (adhoc_skills).
	AdhocSkills []string
	// Route is the optional execution routing tier declared in frontmatter (route).
	// Closed set: {"", "inline", "worker", "fanout"}.
	Route string
	// RouteEvidence is the optional free-string explanation for Route (route_evidence).
	RouteEvidence string
	// NamedSkillsOnly is the optional strict boolean flag declaring that only explicitly
	// named skills (stack and ad-hoc) should be loaded, with no lane-role or sdd-* skills.
	NamedSkillsOnly bool
	// Verification is the optional JSON array of exact verification commands to run.
	Verification []string
	// KnownEnvironmentalFailures is the optional JSON array of baseline failure names or commands.
	KnownEnvironmentalFailures []string
	// CommitMessage is the optional Conventional Commit message for dispatcher commit.
	CommitMessage string
	// MaxIterations is the optional maximum write/test/fix iterations on a rung (1..4, absent means 1).
	MaxIterations int
	// Escalation is the optional ordered escalation ladder of up to 3 rungs.
	Escalation []EscalationRung
	// RequiredSkills is the derived list of required skills. Populated by admission
	// or compilation, never parsed directly from frontmatter.
	RequiredSkills []string
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
		case "lane_role":
			p.LaneRole = strings.TrimSpace(value)
		case "sdd_phase":
			p.SDDPhase = strings.TrimSpace(value)
		case "fanout_group":
			p.FanoutGroup = strings.TrimSpace(value)
		case "skill":
			p.Skill = strings.TrimSpace(value)
		case "adhoc_skills":
			trimmed := strings.TrimSpace(value)
			var skills []string
			if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal([]byte(trimmed), &skills) != nil {
				return Packet{}, ErrInvalidAdhocSkills
			}
			p.AdhocSkills = skills
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
		case "route":
			val := strings.TrimSpace(value)
			switch val {
			case "", "inline", "worker", "fanout":
				p.Route = val
			default:
				return Packet{}, ErrInvalidRoute
			}
		case "route_evidence":
			p.RouteEvidence = strings.TrimSpace(value)
		case "named_skills_only":
			switch strings.TrimSpace(value) {
			case "true":
				p.NamedSkillsOnly = true
			case "false":
				p.NamedSkillsOnly = false
			default:
				return Packet{}, ErrInvalidNamedSkillsOnly
			}
		case "verification":
			trimmed := strings.TrimSpace(value)
			var commands []string
			if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal([]byte(trimmed), &commands) != nil {
				return Packet{}, ErrInvalidVerification
			}
			p.Verification = commands
		case "known_environmental_failures":
			trimmed := strings.TrimSpace(value)
			var failures []string
			if len(trimmed) == 0 || trimmed[0] != '[' || json.Unmarshal([]byte(trimmed), &failures) != nil {
				return Packet{}, ErrInvalidKnownEnvironmentalFailures
			}
			p.KnownEnvironmentalFailures = failures
		case "commit_message":
			p.CommitMessage = strings.TrimSpace(value)
		case "max_iterations":
			val := strings.TrimSpace(value)
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 4 {
				return Packet{}, ErrInvalidMaxIterations
			}
			p.MaxIterations = n
		case "escalation":
			trimmed := strings.TrimSpace(value)
			if len(trimmed) == 0 || trimmed[0] != '[' {
				return Packet{}, ErrInvalidEscalation
			}
			dec := json.NewDecoder(strings.NewReader(trimmed))
			dec.DisallowUnknownFields()
			var rungs []EscalationRung
			if err := dec.Decode(&rungs); err != nil {
				return Packet{}, ErrInvalidEscalation
			}
			var extra json.RawMessage
			if err := dec.Decode(&extra); err != io.EOF {
				return Packet{}, ErrInvalidEscalation
			}
			if len(rungs) > 3 {
				return Packet{}, ErrInvalidEscalation
			}
			for _, r := range rungs {
				if strings.TrimSpace(r.Executor) == "" {
					return Packet{}, ErrInvalidEscalation
				}
			}
			p.Escalation = rungs
		}
	}

	if !closed {
		return Packet{}, ErrNoFrontmatter
	}

	if p.CommitMessage != "" {
		if err := ValidateCommitMessage(p.CommitMessage); err != nil {
			return Packet{}, err
		}
		if len(p.Verification) == 0 {
			return Packet{}, ErrCommitMessageNeedsVerification
		}
	}

	if p.MaxIterations > 1 || len(p.Escalation) > 0 {
		if len(p.Verification) == 0 {
			return Packet{}, ErrLoopNeedsVerification
		}
	}

	if p.LaneRole != "" {
		if !skillset.IsValidLaneRole(p.LaneRole) {
			return Packet{}, ErrInvalidLaneRole
		}
		if p.SDDPhase != "" && !skillset.IsValidSDDPhase(p.SDDPhase) {
			return Packet{}, ErrInvalidSDDPhase
		}
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
