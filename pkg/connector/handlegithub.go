package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"
)

type ghUser struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type ghRepo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type ghDiscussion struct {
	NodeID    string `json:"node_id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	User      ghUser `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ghComment struct {
	NodeID    string `json:"node_id"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	User      ghUser `json:"user"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ParentID  *int64 `json:"parent_id,omitempty"`
	Parent    *struct {
		NodeID string `json:"node_id"`
	} `json:"parent,omitempty"`
}

func parseGitHubTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now()
	}
	return t
}

func userID(u ghUser) networkid.UserID {
	if u.NodeID != "" {
		return networkid.UserID(u.NodeID)
	}
	return networkid.UserID(u.Login)
}

func (gc *GHDConnector) queueForRepoLogins(ctx context.Context, repoID int64, build func(loginID networkid.UserLoginID) bridgev2.RemoteEvent) {
	logins, err := gc.getUserLoginsForRepo(ctx, repoID)
	if err != nil {
		gc.br.Log.Error().Err(err).Int64("repo_id", repoID).Msg("Failed to look up installations for repo")
		return
	}
	for _, loginID := range logins {
		ul := gc.br.GetCachedUserLoginByID(networkid.UserLoginID(loginID))
		if ul == nil {
			continue
		}
		evt := build(networkid.UserLoginID(loginID))
		gc.br.QueueRemoteEvent(ul, evt)
	}
}

func (gc *GHDConnector) handleDiscussionCreated(ctx context.Context, disc ghDiscussion, repo ghRepo, loginID networkid.UserLoginID) {
	portalKey := networkid.PortalKey{ID: networkid.PortalID(disc.NodeID), Receiver: loginID}
	ul := gc.br.GetCachedUserLoginByID(loginID)
	if ul == nil {
		return
	}
	client, _ := ul.Client.(*GHDClient)
	if client != nil {
		client.markPortalActive(portalKey.ID)
	}
	gc.br.QueueRemoteEvent(ul, &simplevent.ChatResync{
		EventMeta: simplevent.EventMeta{
			Type:         bridgev2.RemoteEventChatResync,
			PortalKey:    portalKey,
			CreatePortal: true,
			Timestamp:    parseGitHubTime(disc.CreatedAt),
		},
		GetChatInfoFunc: func(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
			if portal.Metadata == nil {
				portal.Metadata = &PortalMetadata{}
			}
			meta := portal.Metadata.(*PortalMetadata)
			*meta = PortalMetadata{
				Kind:          PortalKindDiscussion,
				RepoOwner:     repo.Owner.Login,
				RepoName:      repo.Name,
				RepoID:        repo.ID,
				DiscussionNum: disc.Number,
				URL:           disc.HTMLURL,
				RepositoryURL: repo.HTMLURL,
			}
			topic := fmt.Sprintf("%s#%d — %s", repo.FullName, disc.Number, disc.HTMLURL)
			return &bridgev2.ChatInfo{
				Name:  ptr.Ptr(disc.Title),
				Topic: ptr.Ptr(topic),
				Members: &bridgev2.ChatMemberList{
					Members: []bridgev2.ChatMember{
						{EventSender: bridgev2.EventSender{Sender: userID(disc.User)}, Membership: event.MembershipJoin},
					},
				},
			}, nil
		},
	})
	// Bridge the discussion OP as the first message.
	gc.br.QueueRemoteEvent(ul, gc.buildDiscussionMessage(portalKey, disc))
}

func (gc *GHDConnector) buildDiscussionMessage(portalKey networkid.PortalKey, disc ghDiscussion) *simplevent.Message[ghDiscussion] {
	return &simplevent.Message[ghDiscussion]{
		EventMeta: simplevent.EventMeta{
			Type:         bridgev2.RemoteEventMessage,
			PortalKey:    portalKey,
			CreatePortal: true,
			Sender:       bridgev2.EventSender{Sender: userID(disc.User)},
			Timestamp:    parseGitHubTime(disc.CreatedAt),
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Str("discussion_id", disc.NodeID)
			},
		},
		ID:   networkid.MessageID(disc.NodeID),
		Data: disc,
		ConvertMessageFunc: func(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data ghDiscussion) (*bridgev2.ConvertedMessage, error) {
			html := gfmToMatrixHTML(data.Body)
			return &bridgev2.ConvertedMessage{
				Parts: []*bridgev2.ConvertedMessagePart{{
					Type: event.EventMessage,
					Content: &event.MessageEventContent{
						MsgType:       event.MsgText,
						Body:          data.Body,
						Format:        event.FormatHTML,
						FormattedBody: html,
					},
					DBMetadata: &MessageMetadata{IsDiscussionOP: true},
				}},
			}, nil
		},
	}
}

func (gc *GHDConnector) handleDiscussionEdited(ctx context.Context, disc ghDiscussion, repo ghRepo) {
	gc.queueForRepoLogins(ctx, repo.ID, func(loginID networkid.UserLoginID) bridgev2.RemoteEvent {
		portalKey := networkid.PortalKey{ID: networkid.PortalID(disc.NodeID), Receiver: loginID}
		return &simplevent.Message[ghDiscussion]{
			EventMeta: simplevent.EventMeta{
				Type:      bridgev2.RemoteEventEdit,
				PortalKey: portalKey,
				Sender:    bridgev2.EventSender{Sender: userID(disc.User)},
				Timestamp: parseGitHubTime(disc.UpdatedAt),
			},
			ID:            networkid.MessageID(disc.NodeID),
			TargetMessage: networkid.MessageID(disc.NodeID),
			Data:          disc,
			ConvertEditFunc: func(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, data ghDiscussion) (*bridgev2.ConvertedEdit, error) {
				return makeTextEdit(existing, data.Body), nil
			},
		}
	})
}

func (gc *GHDConnector) handleDiscussionComment(ctx context.Context, action string, comment ghComment, disc ghDiscussion, repo ghRepo) {
	gc.queueForRepoLogins(ctx, repo.ID, func(loginID networkid.UserLoginID) bridgev2.RemoteEvent {
		portalKey := networkid.PortalKey{ID: networkid.PortalID(disc.NodeID), Receiver: loginID}
		ul := gc.br.GetCachedUserLoginByID(loginID)
		if ul != nil {
			if client, ok := ul.Client.(*GHDClient); ok {
				client.markPortalActive(portalKey.ID)
			}
		}
		switch action {
		case "created":
			msg := &simplevent.Message[commentPayload]{
				EventMeta: simplevent.EventMeta{
					Type:         bridgev2.RemoteEventMessage,
					PortalKey:    portalKey,
					CreatePortal: true,
					Sender:       bridgev2.EventSender{Sender: userID(comment.User)},
					Timestamp:    parseGitHubTime(comment.CreatedAt),
				},
				ID:                 networkid.MessageID(comment.NodeID),
				Data:               commentPayload{Comment: comment, Discussion: disc},
				ConvertMessageFunc: convertCommentMessage,
			}
			return msg
		case "edited":
			return &simplevent.Message[commentPayload]{
				EventMeta: simplevent.EventMeta{
					Type:      bridgev2.RemoteEventEdit,
					PortalKey: portalKey,
					Sender:    bridgev2.EventSender{Sender: userID(comment.User)},
					Timestamp: parseGitHubTime(comment.UpdatedAt),
				},
				ID:              networkid.MessageID(comment.NodeID),
				TargetMessage:   networkid.MessageID(comment.NodeID),
				Data:            commentPayload{Comment: comment, Discussion: disc},
				ConvertEditFunc: convertCommentEdit,
			}
		case "deleted":
			return &simplevent.MessageRemove{
				EventMeta: simplevent.EventMeta{
					Type:      bridgev2.RemoteEventMessageRemove,
					PortalKey: portalKey,
					Sender:    bridgev2.EventSender{Sender: userID(comment.User)},
					Timestamp: time.Now(),
				},
				TargetMessage: networkid.MessageID(comment.NodeID),
			}
		default:
			return nil
		}
	})
}

type commentPayload struct {
	Comment    ghComment
	Discussion ghDiscussion
}

func convertCommentMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data commentPayload) (*bridgev2.ConvertedMessage, error) {
	comment := data.Comment
	html := gfmToMatrixHTML(comment.Body)
	converted := &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType:       event.MsgText,
				Body:          comment.Body,
				Format:        event.FormatHTML,
				FormattedBody: html,
			},
			DBMetadata: &MessageMetadata{},
		}},
	}
	parentID := ""
	if comment.Parent != nil {
		parentID = comment.Parent.NodeID
	}
	if parentID != "" {
		converted.ThreadRoot = ptr.Ptr(networkid.MessageID(parentID))
		converted.Parts[0].DBMetadata = &MessageMetadata{ParentCommentID: parentID}
	}
	return converted, nil
}

func convertCommentEdit(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message, data commentPayload) (*bridgev2.ConvertedEdit, error) {
	return makeTextEdit(existing, data.Comment.Body), nil
}

func (gc *GHDConnector) handleDiscussionStateChange(ctx context.Context, action string, disc ghDiscussion, repo ghRepo) {
	if action != "closed" && action != "locked" {
		return
	}
	gc.queueForRepoLogins(ctx, repo.ID, func(loginID networkid.UserLoginID) bridgev2.RemoteEvent {
		portalKey := networkid.PortalKey{ID: networkid.PortalID(disc.NodeID), Receiver: loginID}
		topic := disc.HTMLURL + " (" + action + ")"
		return &simplevent.ChatInfoChange{
			EventMeta: simplevent.EventMeta{
				Type:      bridgev2.RemoteEventChatInfoChange,
				PortalKey: portalKey,
				Timestamp: time.Now(),
			},
			ChatInfoChange: &bridgev2.ChatInfoChange{
				ChatInfo: &bridgev2.ChatInfo{
					Topic: ptr.Ptr(topic),
				},
			},
		}
	})
}
