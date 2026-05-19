package connector

import (
	"context"
	"fmt"
	"net/http"
	"time"

	githubv4 "github.com/shurcooL/githubv4"
)

type graphQLClient struct {
	client *githubv4.Client
}

func newGraphQLClient(accessToken string) *graphQLClient {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	src := oauthStaticToken{accessToken: accessToken}
	return &graphQLClient{
		client: githubv4.NewClient(src.Client(httpClient)),
	}
}

type oauthStaticToken struct {
	accessToken string
}

func (t oauthStaticToken) Client(c *http.Client) *http.Client {
	if c == nil {
		c = http.DefaultClient
	}
	return &http.Client{
		Transport: &oauthTransport{
			base:  c.Transport,
			token: t.accessToken,
		},
		Timeout: c.Timeout,
	}
}

type oauthTransport struct {
	base  http.RoundTripper
	token string
}

func (t *oauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t.base.RoundTrip(req)
}

type viewerInfo struct {
	NodeID     string
	Login      string
	DatabaseID int64
	AvatarURL  string
	Name       string
}

func (g *graphQLClient) getViewer(ctx context.Context) (*viewerInfo, error) {
	var query struct {
		Viewer struct {
			ID         githubv4.ID
			Login      githubv4.String
			DatabaseID githubv4.Int
			AvatarURL  githubv4.URI
			Name       githubv4.String
		}
	}
	if err := g.client.Query(ctx, &query, nil); err != nil {
		return nil, err
	}
	name := string(query.Viewer.Name)
	if name == "" {
		name = string(query.Viewer.Login)
	}
	return &viewerInfo{
		NodeID:     fmt.Sprint(query.Viewer.ID),
		Login:      string(query.Viewer.Login),
		DatabaseID: int64(query.Viewer.DatabaseID),
		AvatarURL:  query.Viewer.AvatarURL.String(),
		Name:       name,
	}, nil
}

type discussionInfo struct {
	ID        string
	Number    int
	Title     string
	Body      string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    userInfo
	Repository struct {
		Owner struct {
			Login string
		}
		Name       string
		DatabaseID int64
		URL        string
	}
}

type userInfo struct {
	ID        string
	Login     string
	AvatarURL string
	Name      string
}

type commentInfo struct {
	ID        string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    userInfo
	ReplyTo   *struct {
		ID string
	}
}

func (g *graphQLClient) getDiscussion(ctx context.Context, discussionID githubv4.ID) (*discussionInfo, error) {
	var query struct {
		Node struct {
			Discussion struct {
				ID        githubv4.ID
				Number    int
				Title     githubv4.String
				Body      githubv4.String
				URL       githubv4.URI
				CreatedAt githubv4.DateTime
				UpdatedAt githubv4.DateTime
				Author    struct {
					ID        githubv4.ID
					Login     githubv4.String
					AvatarURL githubv4.URI
					Name      githubv4.String
				}
				Repository struct {
					Owner struct {
						Login githubv4.String
					}
					Name       githubv4.String
					DatabaseID githubv4.Int
					URL        githubv4.URI
				}
			} `graphql:"... on Discussion"`
		} `graphql:"node(id: $id)"`
	}
	vars := map[string]any{"id": discussionID}
	if err := g.client.Query(ctx, &query, vars); err != nil {
		return nil, err
	}
	raw := query.Node.Discussion
	if fmt.Sprint(raw.ID) == "" {
		return nil, fmt.Errorf("node %s is not a discussion", discussionID)
	}
	name := string(raw.Author.Name)
	if name == "" {
		name = string(raw.Author.Login)
	}
	return &discussionInfo{
		ID:        fmt.Sprint(raw.ID),
		Number:    raw.Number,
		Title:     string(raw.Title),
		Body:      string(raw.Body),
		URL:       raw.URL.String(),
		CreatedAt: raw.CreatedAt.Time,
		UpdatedAt: raw.UpdatedAt.Time,
		Author: userInfo{
			ID:        fmt.Sprint(raw.Author.ID),
			Login:     string(raw.Author.Login),
			AvatarURL: raw.Author.AvatarURL.String(),
			Name:      name,
		},
		Repository: struct {
			Owner struct {
				Login string
			}
			Name       string
			DatabaseID int64
			URL        string
		}{
			Owner:      struct{ Login string }{Login: string(raw.Repository.Owner.Login)},
			Name:       string(raw.Repository.Name),
			DatabaseID: int64(raw.Repository.DatabaseID),
			URL:        raw.Repository.URL.String(),
		},
	}, nil
}

func (g *graphQLClient) addDiscussionComment(ctx context.Context, discussionID githubv4.ID, body string, replyToID *githubv4.ID) (githubv4.ID, error) {
	var mutation struct {
		AddDiscussionComment struct {
			Comment struct {
				ID githubv4.ID
			}
		} `graphql:"addDiscussionComment(input: $input)"`
	}
	input := githubv4.AddDiscussionCommentInput{
		DiscussionID: discussionID,
		Body:         githubv4.String(body),
	}
	if replyToID != nil {
		input.ReplyToID = replyToID
	}
	if err := g.client.Mutate(ctx, &mutation, input, nil); err != nil {
		return "", err
	}
	return mutation.AddDiscussionComment.Comment.ID, nil
}

func (g *graphQLClient) updateDiscussionComment(ctx context.Context, commentID githubv4.ID, body string) error {
	var mutation struct {
		UpdateDiscussionComment struct {
			Comment struct {
				ID githubv4.ID
			}
		} `graphql:"updateDiscussionComment(input: $input)"`
	}
	input := githubv4.UpdateDiscussionCommentInput{
		CommentID: commentID,
		Body:      githubv4.String(body),
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

func (g *graphQLClient) deleteDiscussionComment(ctx context.Context, commentID githubv4.ID) error {
	var mutation struct {
		DeleteDiscussionComment struct {
			ClientMutationID githubv4.String
		} `graphql:"deleteDiscussionComment(input: $input)"`
	}
	input := githubv4.DeleteDiscussionCommentInput{ID: commentID}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

func (g *graphQLClient) addReaction(ctx context.Context, subjectID githubv4.ID, content githubv4.ReactionContent) error {
	var mutation struct {
		AddReaction struct {
			Reaction struct {
				ID githubv4.ID
			}
		} `graphql:"addReaction(input: $input)"`
	}
	input := githubv4.AddReactionInput{
		SubjectID: subjectID,
		Content:   content,
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

func (g *graphQLClient) removeReaction(ctx context.Context, subjectID githubv4.ID, content githubv4.ReactionContent) error {
	var mutation struct {
		RemoveReaction struct {
			Reaction struct {
				ID githubv4.ID
			}
		} `graphql:"removeReaction(input: $input)"`
	}
	input := githubv4.RemoveReactionInput{
		SubjectID: subjectID,
		Content:   content,
	}
	return g.client.Mutate(ctx, &mutation, input, nil)
}

type reactionGroupInfo struct {
	Content           githubv4.ReactionContent
	ViewerHasReacted  bool
	TotalCount        int
	Users             []userInfo
}

func (g *graphQLClient) getCommentReactions(ctx context.Context, commentID githubv4.ID) ([]reactionGroupInfo, error) {
	var query struct {
		Node struct {
			DiscussionComment struct {
				ReactionGroups []struct {
					Content          githubv4.ReactionContent
					ViewerHasReacted bool
					Users            struct {
						TotalCount int
						Nodes      []struct {
							Login     githubv4.String
							ID        githubv4.ID
							AvatarURL githubv4.URI
						}
					}
				}
			} `graphql:"... on DiscussionComment"`
		} `graphql:"node(id: $id)"`
	}
	vars := map[string]any{"id": commentID}
	if err := g.client.Query(ctx, &query, vars); err != nil {
		return nil, err
	}
	var groups []reactionGroupInfo
	for _, rg := range query.Node.DiscussionComment.ReactionGroups {
		gi := reactionGroupInfo{
			Content:          rg.Content,
			ViewerHasReacted: rg.ViewerHasReacted,
			TotalCount:       rg.Users.TotalCount,
		}
		for _, u := range rg.Users.Nodes {
			gi.Users = append(gi.Users, userInfo{
				ID:        fmt.Sprint(u.ID),
				Login:     string(u.Login),
				AvatarURL: u.AvatarURL.String(),
			})
		}
		groups = append(groups, gi)
	}
	return groups, nil
}

func (g *graphQLClient) getDiscussionComments(ctx context.Context, discussionID githubv4.ID, first int) ([]commentInfo, error) {
	var query struct {
		Node struct {
			Discussion struct {
				Comments struct {
					Nodes []struct {
						ID        githubv4.ID
						Body      githubv4.String
						CreatedAt githubv4.DateTime
						UpdatedAt githubv4.DateTime
						Author    struct {
							ID    githubv4.ID
							Login githubv4.String
						}
						ReplyTo *struct {
							ID githubv4.ID
						} `graphql:"replyTo"`
					}
				} `graphql:"comments(first: $first)"`
			} `graphql:"... on Discussion"`
		} `graphql:"node(id: $id)"`
	}
	vars := map[string]any{"id": discussionID, "first": first}
	if err := g.client.Query(ctx, &query, vars); err != nil {
		return nil, err
	}
	var comments []commentInfo
	for _, n := range query.Node.Discussion.Comments.Nodes {
		ci := commentInfo{
			ID:        fmt.Sprint(n.ID),
			Body:      string(n.Body),
			CreatedAt: n.CreatedAt.Time,
			UpdatedAt: n.UpdatedAt.Time,
			Author: userInfo{
				ID:    fmt.Sprint(n.Author.ID),
				Login: string(n.Author.Login),
			},
		}
		if n.ReplyTo != nil {
			ci.ReplyTo = &struct{ ID string }{ID: fmt.Sprint(n.ReplyTo.ID)}
		}
		comments = append(comments, ci)
	}
	return comments, nil
}
