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
	ErrInvalidReadOnly = errors.New("packet: frontmatter read_only must be a boolean (true or false)")
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
	// condition would be implicit routing, which the skill forbids.
	RoutedBy string
	// Model selects which model the executor dispatches with. It is
	// optional: an absent model key leaves this at its zero value. Each
	// executor owns its own default (see executor.Executor.DefaultModel),
	// applied by internal/run when this field is empty — not filled in
	// here, so Parse keeps reflecting frontmatter literally.
	Model string
	// ReadOnly marks this packet as exploration or read-only: it must
	// produce no commits and leave a clean worktree. When absent, it
	// defaults to false (write packet).
	ReadOnly bool
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
		case "read_only":
			switch strings.TrimSpace(value) {
			case "true":
				p.ReadOnly = true
			case "false":
				p.ReadOnly = false
			default:
				return Packet{}, ErrInvalidReadOnly
			}
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
