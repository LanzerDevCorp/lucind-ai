package run

var (
	Bisect       = bisect
	TryCombine   = tryCombine
	PacketDigest = packetDigest

	// DispatcherCommitForTest exposes the commit step so tests can drive it
	// directly with injected seams.
	DispatcherCommitForTest = dispatcherCommit

	// DispatcherVerifyForTest exposes verification so tests can drive it directly.
	DispatcherVerifyForTest = dispatcherVerify

	BuildAttemptPlanForTest = buildAttemptPlan
	FormatFeedbackForTest   = formatFeedback
)
