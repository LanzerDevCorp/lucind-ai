package conflicttriage

import "context"

// TriageInvoker executes a triage prompt in a worktree. Tests stub this so
// CI never places a live LLM call. Production wiring is intentionally unset.
type TriageInvoker func(ctx context.Context, worktreePath, prompt string) (output string, err error)
