package connector

// PortalKind identifies what type of GitHub entity a portal represents.
type PortalKind string

const (
	PortalKindDiscussion   PortalKind = "discussion"
	PortalKindIssue        PortalKind = "issue"
	PortalKindPullRequest  PortalKind = "pull_request"
)

// PortalMetadata is stored in the portal metadata JSON column.
type PortalMetadata struct {
	Kind           PortalKind `json:"kind"`
	RepoOwner      string     `json:"repo_owner"`
	RepoName       string     `json:"repo_name"`
	RepoID         int64      `json:"repo_id"`
	DiscussionNum  int        `json:"discussion_number,omitempty"`
	URL            string     `json:"url"`
	RepositoryURL  string     `json:"repository_url"`
}

// MessageMetadata stores GitHub-specific fields for bridged messages.
type MessageMetadata struct {
	ParentCommentID string `json:"parent_comment_id,omitempty"`
	IsDiscussionOP  bool   `json:"is_discussion_op,omitempty"`
}

// UserLoginMetadata stores OAuth tokens and GitHub identity for a logged-in user.
type UserLoginMetadata struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	AccessExpiresAt  int64  `json:"access_expires_at"`  // Unix seconds
	RefreshExpiresAt int64  `json:"refresh_expires_at"` // Unix seconds
	Login            string `json:"login"`
	DatabaseID       int64  `json:"database_id"`
	NodeID           string `json:"node_id"`
}
