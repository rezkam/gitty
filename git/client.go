package git

// Client exposes the git operations required by the CLI workflow.
type Client interface {
	IsRepository(path string) bool
	StagedDiff(path string) (string, error)
	CreateCommit(message string) error
}

// CommandClient executes git operations via command line invocations.
type CommandClient struct{}

// NewCommandClient constructs a Client backed by the local git CLI.
func NewCommandClient() Client {
	return &CommandClient{}
}

// IsRepository reports whether the provided path resides within a git worktree.
func (CommandClient) IsRepository(path string) bool {
	return IsGitDirectory(path)
}

// StagedDiff returns the diff of staged changes in the supplied path.
func (CommandClient) StagedDiff(path string) (string, error) {
	return GetStagedDiff(path)
}

// CreateCommit creates a commit with the provided message.
func (CommandClient) CreateCommit(message string) error {
	return CreateCommitMessage(message)
}
