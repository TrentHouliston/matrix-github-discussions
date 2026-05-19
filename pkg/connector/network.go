package connector

// RemoteEntityKind classifies remote portal/message targets for future Issue/PR support.
type RemoteEntityKind string

const (
	RemoteEntityDiscussion RemoteEntityKind = "discussion"
	RemoteEntityIssue      RemoteEntityKind = "issue"
	RemoteEntityPullRequest RemoteEntityKind = "pull_request"
)

// RemoteEntity is a minimal abstraction for webhook routing extensions.
type RemoteEntity struct {
	Kind      RemoteEntityKind
	NodeID    string
	RepoID    int64
	RepoOwner string
	RepoName  string
}
