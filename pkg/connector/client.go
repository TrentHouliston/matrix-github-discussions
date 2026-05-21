package connector

import (
	"context"
	"fmt"
	"sync"
	"time"

	githubv4 "github.com/shurcooL/githubv4"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/status"
	"maunium.net/go/mautrix/event"
)

type GHDClient struct {
	UserLogin *bridgev2.UserLogin
	connector *GHDConnector
	graphql   *graphQLClient
	loggedIn  bool

	pollMu      sync.Mutex
	activeUntil map[networkid.PortalID]time.Time
}

var (
	_ bridgev2.NetworkAPI                  = (*GHDClient)(nil)
	_ bridgev2.EditHandlingNetworkAPI      = (*GHDClient)(nil)
	_ bridgev2.ReactionHandlingNetworkAPI  = (*GHDClient)(nil)
	_ bridgev2.RedactionHandlingNetworkAPI = (*GHDClient)(nil)
)

func (c *GHDClient) Connect(ctx context.Context) {
	if err := c.ensureValidToken(ctx); err != nil {
		c.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "github-token-invalid",
			Message:    err.Error(),
		})
		return
	}
	if _, err := c.graphql.getViewer(ctx); err != nil {
		c.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "github-api-error",
			Message:    "Failed to verify GitHub credentials",
			Info:       map[string]any{"go_error": err.Error()},
		})
		return
	}
	c.loggedIn = true
	go c.runReactionPollLoop(ctx)
}

func (c *GHDClient) Disconnect() {}

func (c *GHDClient) IsLoggedIn() bool {
	return c.loggedIn
}

func (c *GHDClient) LogoutRemote(ctx context.Context) {}

func (c *GHDClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	return &event.RoomFeatures{
		MaxTextLength:    65536,
		Thread:           event.CapLevelFullySupported,
		Reply:            event.CapLevelFullySupported,
		Edit:             event.CapLevelFullySupported,
		Delete:           event.CapLevelFullySupported,
		Reaction:         event.CapLevelPartialSupport,
		AllowedReactions: AllowedMatrixReactions,
	}
}

func (c *GHDClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	meta := c.UserLogin.Metadata.(*UserLoginMetadata)
	return networkid.UserID(meta.NodeID) == userID
}

func (c *GHDClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}
	disc, err := c.graphql.getDiscussion(ctx, githubv4.ID(portal.ID))
	if err != nil {
		return nil, err
	}
	topic := fmt.Sprintf("%s/%s#%d — %s", disc.Repository.Owner.Login, disc.Repository.Name, disc.Number, disc.URL)
	members := []bridgev2.ChatMember{
		{
			EventSender: bridgev2.EventSender{
				IsFromMe: true,
				Sender:   networkid.UserID(c.UserLogin.Metadata.(*UserLoginMetadata).NodeID),
			},
			Membership: event.MembershipJoin,
			PowerLevel: ptr.Ptr(int(50)),
		},
		{
			EventSender: bridgev2.EventSender{Sender: networkid.UserID(disc.Author.ID)},
			Membership:  event.MembershipJoin,
			PowerLevel:  ptr.Ptr(50),
		},
	}
	return &bridgev2.ChatInfo{
		Name:  ptr.Ptr(disc.Title),
		Topic: ptr.Ptr(topic),
		Members: &bridgev2.ChatMemberList{
			IsFull:  false,
			Members: members,
		},
	}, nil
}

func (c *GHDClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	name := ghost.Name
	if name == "" {
		name = string(ghost.ID)
	}
	return &bridgev2.UserInfo{
		Name:        ptr.Ptr(name),
		Identifiers: []string{fmt.Sprintf("https://github.com/users/%s", ghost.ID)},
	}, nil
}

func (c *GHDClient) matrixBody(msg *bridgev2.MatrixMessage) string {
	body := msg.Content.Body
	if msg.Content.FormattedBody != "" {
		md := matrixHTMLToGFM(msg.Content.FormattedBody)
		if md != "" {
			return md
		}
	}
	return body
}

func (c *GHDClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (*bridgev2.MatrixMessageResponse, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}
	body := c.matrixBody(msg)
	var replyToID *githubv4.ID
	if msg.ThreadRoot != nil {
		id := githubv4.ID(msg.ThreadRoot.ID)
		replyToID = &id
	}
	commentID, err := c.graphql.addDiscussionComment(ctx, githubv4.ID(msg.Portal.ID), body, replyToID)
	if err != nil {
		return nil, err
	}
	meta := c.UserLogin.Metadata.(*UserLoginMetadata)
	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID:       networkid.MessageID(fmt.Sprint(commentID)),
			SenderID: networkid.UserID(meta.NodeID),
		},
	}, nil
}

func (c *GHDClient) HandleMatrixEdit(ctx context.Context, msg *bridgev2.MatrixEdit) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}
	body := msg.Content.Body
	if msg.Content.FormattedBody != "" {
		if md := matrixHTMLToGFM(msg.Content.FormattedBody); md != "" {
			body = md
		}
	}
	return c.graphql.updateDiscussionComment(ctx, githubv4.ID(msg.EditTarget.ID), body)
}

func (c *GHDClient) HandleMatrixMessageRemove(ctx context.Context, msg *bridgev2.MatrixMessageRemove) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}
	return c.graphql.deleteDiscussionComment(ctx, githubv4.ID(msg.TargetMessage.ID))
}

func (c *GHDClient) PreHandleMatrixReaction(ctx context.Context, msg *bridgev2.MatrixReaction) (bridgev2.MatrixReactionPreResponse, error) {
	meta := c.UserLogin.Metadata.(*UserLoginMetadata)
	emoji := msg.Content.RelatesTo.Key
	if _, err := matrixEmojiToGitHubReaction(emoji); err != nil {
		return bridgev2.MatrixReactionPreResponse{}, err
	}
	return bridgev2.MatrixReactionPreResponse{
		SenderID: networkid.UserID(meta.NodeID),
		EmojiID:  networkid.EmojiID(emoji),
		Emoji:    emoji,
	}, nil
}

func (c *GHDClient) HandleMatrixReaction(ctx context.Context, msg *bridgev2.MatrixReaction) (*database.Reaction, error) {
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}
	content, err := matrixEmojiToGitHubReaction(msg.Content.RelatesTo.Key)
	if err != nil {
		return nil, err
	}
	subjectID := githubv4.ID(msg.TargetMessage.ID)
	if err := c.graphql.addReaction(ctx, subjectID, content); err != nil {
		return nil, err
	}
	return &database.Reaction{}, nil
}

func (c *GHDClient) HandleMatrixReactionRemove(ctx context.Context, msg *bridgev2.MatrixReactionRemove) error {
	if err := c.ensureValidToken(ctx); err != nil {
		return err
	}
	content, err := matrixEmojiToGitHubReaction(msg.TargetReaction.Emoji)
	if err != nil {
		return err
	}
	return c.graphql.removeReaction(ctx, githubv4.ID(msg.TargetReaction.MessageID), content)
}

func (c *GHDClient) markPortalActive(portalID networkid.PortalID) {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	if c.activeUntil == nil {
		c.activeUntil = make(map[networkid.PortalID]time.Time)
	}
	windows := c.connector.Config.ReactionPollActiveWindows
	if windows <= 0 {
		windows = 5
	}
	interval := time.Duration(c.connector.Config.ReactionPollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	c.activeUntil[portalID] = time.Now().Add(interval * time.Duration(windows))
}

func (c *GHDClient) isPortalActive(portalID networkid.PortalID) bool {
	c.pollMu.Lock()
	defer c.pollMu.Unlock()
	until, ok := c.activeUntil[portalID]
	return ok && time.Now().Before(until)
}
