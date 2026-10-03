package run

var (
	Bisect       = bisect
	TryCombine   = tryCombine
	PacketDigest = packetDigest

	// DispatcherCommitForTest exposes the commit step so tests can drive it
	// directly with injected seams.
	DispatcherCommitForTest = dispatcherCommit
)
