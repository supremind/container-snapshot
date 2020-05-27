package constants

// special worker exit codes for corresponding worker errors, used to update snapshot conditions
const (
	ExitCodeInvalidImage int32 = 100 + iota
	ExitCodeDockerCommit
	ExitCodeDockerPush
)
