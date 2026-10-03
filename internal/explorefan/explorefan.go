// Package explorefan provides pure functions to validate exploration specifications
// and construct deterministic packet documents for parallel explorer lenses and synthesis.
package explorefan

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Lens identifies an exploration perspective.
type Lens string

const (
	LensStructural Lens = "structural"
	LensTextual    Lens = "textual"
	LensHistorical Lens = "historical"
)

// Lenses defines the ordered exploration lenses run in parallel.
var Lenses = []Lens{LensStructural, LensTextual, LensHistorical}

// DefaultExecutor is the default executor runtime for exploration lanes.
const DefaultExecutor = "agy"

// DefaultModel is the default LLM model for exploration lanes.
const DefaultModel = "gemini-3.8-flash-medium"

var prefixPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// Spec configures an exploration fan-out run.
type Spec struct {
	Prefix    string
	Objective string
	Scope     []string
	Executor  string
	Model     string
}

// WithDefaults returns a copy of Spec with default executor and model applied if empty.
func (s Spec) WithDefaults() Spec {
	if s.Executor == "" {
		s.Executor = DefaultExecutor
	}
	if s.Model == "" {
		s.Model = DefaultModel
	}
	return s
}

// Validate checks that s meets all exploration spec invariants.
func Validate(s Spec) error {
	if !prefixPattern.MatchString(s.Prefix) {
		return errors.New("explorefan: prefix must match ^[a-z0-9][a-z0-9-]{0,39}$")
	}

	trimmedObj := strings.TrimSpace(s.Objective)
	if trimmedObj == "" {
		return errors.New("explorefan: objective cannot be empty")
	}
	if len(s.Objective) > 2000 {
		return errors.New("explorefan: objective exceeds 2000 bytes")
	}
	if !utf8.ValidString(s.Objective) {
		return errors.New("explorefan: objective must be valid UTF-8")
	}
	if strings.ContainsRune(s.Objective, 0) {
		return errors.New("explorefan: objective cannot contain NUL byte")
	}

	if len(s.Scope) > 20 {
		return errors.New("explorefan: scope entries cannot exceed 20")
	}
	for i, entry := range s.Scope {
		if entry == "" {
			return fmt.Errorf("explorefan: scope entry [%d] cannot be empty", i)
		}
		if len(entry) > 200 {
			return fmt.Errorf("explorefan: scope entry [%d] exceeds 200 bytes", i)
		}
		if strings.HasPrefix(entry, "/") {
			return fmt.Errorf("explorefan: scope entry [%d] cannot have leading slash", i)
		}
		if strings.ContainsRune(entry, 0) {
			return fmt.Errorf("explorefan: scope entry [%d] cannot contain NUL byte", i)
		}
		segments := strings.Split(entry, "/")
		for _, seg := range segments {
			if seg == ".." {
				return fmt.Errorf("explorefan: scope entry [%d] cannot contain '..' segment", i)
			}
		}
	}

	return nil
}

// LensPacket generates a complete markdown packet file for a single exploration lens.
func LensPacket(s Spec, l Lens) (id string, markdown []byte) {
	s = s.WithDefaults()
	id = fmt.Sprintf("%s-%s", s.Prefix, string(l))

	var b bytes.Buffer
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", id))
	b.WriteString(fmt.Sprintf("executor: %s\n", s.Executor))
	// Not "explore": routed_by values that equal an SDD phase name make admission derive sdd-* skills.
	b.WriteString("routed_by: explorer-fanout\n")
	b.WriteString(fmt.Sprintf("model: %s\n", s.Model))
	b.WriteString("read_only: true\n")
	b.WriteString("lane_role: lens\n")
	b.WriteString("route: fanout\n")
	b.WriteString(fmt.Sprintf("route_evidence: explorer fan-out lens %s\n", string(l)))
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# Packet %s\n\n", id))
	b.WriteString("## Goal\n\n")
	b.WriteString(fmt.Sprintf("Perform read-only exploration using the %s lens.\n\n", string(l)))

	b.WriteString("### Objective\n\n")
	writeFenced(&b, s.Objective)
	b.WriteString("\n")

	if len(s.Scope) > 0 {
		b.WriteString("### Scope\n\n")
		for _, sc := range s.Scope {
			b.WriteString(fmt.Sprintf("- %s\n", sc))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Lens Method\n\n")
	switch l {
	case LensStructural:
		b.WriteString("Query CodeGraph first before broad filesystem exploration using the `codegraph_explore` MCP tool\n")
		b.WriteString("or read-only upstream CLI commands (`codegraph status`, `codegraph query`, `codegraph explore`,\n")
		b.WriteString("`codegraph node`, `codegraph files`, `codegraph callers`, `codegraph callees`, `codegraph impact`,\n")
		b.WriteString("`codegraph affected`). Trace symbol definitions, type hierarchies, callers, callees, dependency\n")
		b.WriteString("chains, and blast radius of potential modifications. Ground every finding with exact `path:line`\n")
		b.WriteString("pointers and explicit symbol signatures.\n\n")
	case LensTextual:
		b.WriteString("High-speed textual search across the workspace using `rg`, `fd`, and `bat` (never use `cat`, `grep`,\n")
		b.WriteString("`find`, or `ls`). Search exact keywords, error strings, configuration keys, log statements,\n")
		b.WriteString("type usages, and pattern occurrences across the workspace. Report literal matches, match\n")
		b.WriteString("distributions across packages, and verbatim code snippets with exact `path:line` references.\n")
		b.WriteString("Include negative search results (patterns confirmed absent) when relevant.\n\n")
	case LensHistorical:
		b.WriteString("Git archaeology across repository history using `git log -S <symbol>`, `git log -G <regex>`,\n")
		b.WriteString("`git blame -L <start>,<end> <path>`, and `git log --stat` over the target area. Trace commit history,\n")
		b.WriteString("commit messages, bug fixes, refactor rationales, and past architectural shifts. Report relevant\n")
		b.WriteString("commit hashes, commit summaries, author rationale, past regression contexts, and timeline evolution.\n")
		b.WriteString("Ground every historical claim in specific commits or blame ranges.\n\n")
	}

	b.WriteString("### Rules\n\n")
	b.WriteString("- Strictly read-only: never edit files, create new files, or run mutating commands.\n")
	b.WriteString("- Bounded evidence: return at most about 2,000 tokens of exact `path:line` evidence.\n")
	b.WriteString("- Fact vs. assumption discipline: separate verified facts from assumptions.\n")
	b.WriteString("- List open questions and technical risks for the orchestrator.\n")
	b.WriteString("- The `summary` field of `.lucind/result.json` must contain the complete evidence report.\n\n")

	b.WriteString("## Done criteria\n\n")
	b.WriteString("- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**\n")
	b.WriteString("- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).**\n")
	b.WriteString("- [ ] **Exploration completed and evidence report written into envelope summary.**\n\n")

	b.WriteString("## Hard stops\n\n")
	b.WriteString("- Any mutating command or edit is required.\n")
	b.WriteString("- Any credential value would need to be chosen, generated, or written.\n")
	b.WriteString("- Satisfying one instruction in this packet would require violating another.\n\n")

	b.WriteString("## Return\n\n")
	b.WriteString("Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.\n\n")
	b.WriteString("The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.\n\n")
	b.WriteString("Omit the `commit` field (or leave it empty) per read-only envelope convention. Do not commit.\n")

	return id, b.Bytes()
}

// LensOutput captures the result of one exploration lens lane.
type LensOutput struct {
	Lens    Lens
	OK      bool
	Summary string
	Failure string
}

// SynthesisPacket generates a complete markdown packet file for the synthesis lane.
func SynthesisPacket(s Spec, outputs []LensOutput) (id string, markdown []byte) {
	s = s.WithDefaults()
	id = fmt.Sprintf("%s-synthesis", s.Prefix)

	var b bytes.Buffer
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("id: %s\n", id))
	b.WriteString(fmt.Sprintf("executor: %s\n", s.Executor))
	b.WriteString("routed_by: explorer-fanout\n")
	b.WriteString(fmt.Sprintf("model: %s\n", s.Model))
	b.WriteString("read_only: true\n")
	b.WriteString("lane_role: synthesis\n")
	b.WriteString("route: fanout\n")
	b.WriteString("route_evidence: explorer fan-out synthesis\n")
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# Packet %s\n\n", id))
	b.WriteString("## Goal\n\n")
	b.WriteString("Synthesize the findings from the structural, textual, and historical exploration lenses into a single cohesive handoff.\n\n")

	b.WriteString("### Objective\n\n")
	writeFenced(&b, s.Objective)
	b.WriteString("\n")

	if len(s.Scope) > 0 {
		b.WriteString("### Scope\n\n")
		for _, sc := range s.Scope {
			b.WriteString(fmt.Sprintf("- %s\n", sc))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Lens Outputs\n\n")
	for _, out := range outputs {
		if out.OK {
			b.WriteString(fmt.Sprintf("## Lens: %s\n\n", string(out.Lens)))
			trunc := truncateTail(out.Summary, 6000)
			writeFenced(&b, trunc)
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("## Lens: %s (failed)\n\n", string(out.Lens)))
			reason := out.Failure
			if strings.TrimSpace(reason) == "" {
				reason = "lens execution failed with no diagnosis"
			}
			writeFenced(&b, reason)
			b.WriteString("\n")
		}
	}

	b.WriteString("## Synthesis Rules\n\n")
	b.WriteString("- Merge the lens findings into ONE handoff of at most about 2,000 tokens.\n")
	b.WriteString("- Keep contradictions and conflicting signals between lenses clearly visible; do not paper over differences.\n")
	b.WriteString("- Add no claim that is not substantiated in a lens output.\n")
	b.WriteString("- Cite exact `path:line` references for code facts.\n")
	b.WriteString("- End with open questions and unresolved technical risks for the orchestrator.\n")
	b.WriteString("- Write the synthesized handoff into the `summary` field of `.lucind/result.json`.\n\n")

	b.WriteString("## Done criteria\n\n")
	b.WriteString("- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.**\n")
	b.WriteString("- [ ] **The worktree carries no unique commits and no working-tree changes relative to the lane's birth point (`git status --porcelain` empty AND `HEAD` equals `git merge-base HEAD <primary HEAD>`).**\n")
	b.WriteString("- [ ] **Synthesis handoff completed and written into envelope summary.**\n\n")

	b.WriteString("## Hard stops\n\n")
	b.WriteString("- Any mutating command or edit is required.\n")
	b.WriteString("- Any credential value would need to be chosen, generated, or written.\n")
	b.WriteString("- Satisfying one instruction in this packet would require violating another.\n\n")

	b.WriteString("## Return\n\n")
	b.WriteString("Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the dispatching binary reads. Printed output alone will be read as a lane that produced nothing.\n\n")
	b.WriteString("The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well the work went.\n\n")
	b.WriteString("Omit the `commit` field (or leave it empty) per read-only envelope convention. Do not commit.\n")

	return id, b.Bytes()
}

func writeFenced(b *bytes.Buffer, content string) {
	f := fenceFor(content)
	b.WriteString(f)
	b.WriteString("\n")
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(f)
	b.WriteString("\n")
}

func fenceFor(s string) string {
	maxRun := 0
	curRun := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			curRun++
			if curRun > maxRun {
				maxRun = curRun
			}
		} else {
			curRun = 0
		}
	}
	count := 3
	if maxRun >= count {
		count = maxRun + 1
	}
	return strings.Repeat("`", count)
}

func truncateTail(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	start := len(s) - maxBytes
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return "[truncated]\n" + s[start:]
}
