// Package packet parses a dispatch packet: a Markdown document whose
// frontmatter carries the fields the binary needs to route and run a lane,
// and whose body is the prompt handed to the executor verbatim.
package packet

import (
	"bufio"
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
	ErrNoFrontmatter   = errors.New("packet: document has no closed --- frontmatter block")
	ErrMissingID       = errors.New("packet: frontmatter is missing a non-empty id")
	ErrMissingExecutor = errors.New("packet: frontmatter is missing a non-empty executor")
	ErrMissingRoutedBy = errors.New("packet: frontmatter is missing a non-empty routed_by")
	ErrEmptyBody       = errors.New("packet: body is empty, there is no prompt to dispatch")
)

// Packet is one unit of delegated work.
type Packet struct {
	// ID names the lane, its branch, and its worktree directory.
	ID string
	// Executor selects the runtime that carries out the work.
	Executor string
	// RoutedBy is the condition that caused this packet to be routed to
	// Executor — never the executor's own name. The executor is the
	// outcome of a routing decision, not its reason: recording it as the
	// condition would be implicit routing, which the skill forbids. See
	// plugin/claude-code/skills/lucind-ai/references/state-files.md.
	RoutedBy string
	// Model selects which model the executor dispatches with. It is
	// optional: an absent model key leaves this at its zero value. The
	// project default is filled in by internal/run, not here — see
	// run.DefaultModel for why the default lives there instead.
	Model string
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
