package dispatch

// Process exit codes used by dispatch workflows.
const (
	// ExitDone indicates normal successful completion.
	ExitDone = 0
	// ExitError indicates an unexpected runtime or configuration error.
	ExitError = 1
	// ExitFailed indicates the lane execution failed.
	ExitFailed = 3
	// ExitTimeout indicates the lane execution timed out.
	ExitTimeout = 4
	// ExitAutoSkillsUnavailable indicates automatic skill selection was unavailable.
	ExitAutoSkillsUnavailable = 5
)
