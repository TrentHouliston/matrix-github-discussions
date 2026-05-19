package connector

import (
	"context"
	"fmt"
	"time"

	githubv4 "github.com/shurcooL/githubv4"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
)

func (c *GHDClient) runReactionPollLoop(ctx context.Context) {
	interval := time.Duration(c.connector.Config.ReactionPollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollActivePortalReactions(ctx)
		}
	}
}

func (c *GHDClient) pollActivePortalReactions(ctx context.Context) {
	if err := c.ensureValidToken(ctx); err != nil {
		return
	}
	c.pollMu.Lock()
	var active []networkid.PortalID
	now := time.Now()
	for id, until := range c.activeUntil {
		if now.Before(until) {
			active = append(active, id)
		}
	}
	c.pollMu.Unlock()

	for _, portalID := range active {
		portal, err := c.UserLogin.Bridge.GetPortalByKey(ctx, networkid.PortalKey{
			ID:       portalID,
			Receiver: c.UserLogin.ID,
		})
		if err != nil || portal == nil || portal.MXID == "" {
			continue
		}
		c.pollPortalReactions(ctx, portal)
	}
}

func (c *GHDClient) pollPortalReactions(ctx context.Context, portal *bridgev2.Portal) {
	comments, err := c.graphql.getDiscussionComments(ctx, githubv4.ID(portal.ID), 50)
	if err != nil {
		c.UserLogin.Log.Debug().Err(err).Str("portal_id", string(portal.ID)).Msg("Failed to fetch comments for reaction poll")
		return
	}
	// Include discussion OP node for reactions on the discussion itself.
	targets := []githubv4.ID{githubv4.ID(portal.ID)}
	for _, comment := range comments {
		targets = append(targets, comment.ID)
	}
	for _, subjectID := range targets {
		groups, err := c.graphql.getCommentReactions(ctx, subjectID)
		if err != nil {
			continue
		}
		syncData := buildReactionSyncData(groups)
		if len(syncData.Users) == 0 {
			continue
		}
		c.UserLogin.Bridge.QueueRemoteEvent(c.UserLogin, &simplevent.ReactionSync{
			EventMeta: simplevent.EventMeta{
				Type:      bridgev2.RemoteEventReactionSync,
				PortalKey: portal.PortalKey,
				Timestamp: time.Now(),
			},
			TargetMessage: networkid.MessageID(fmt.Sprint(subjectID)),
			Reactions:     syncData,
		})
	}
}

func buildReactionSyncData(groups []reactionGroupInfo) *bridgev2.ReactionSyncData {
	data := &bridgev2.ReactionSyncData{
		Users:       make(map[networkid.UserID]*bridgev2.ReactionSyncUser),
		HasAllUsers: true,
	}
	for _, group := range groups {
		emoji, ok := githubReactionToMatrixEmoji(group.Content)
		if !ok {
			continue
		}
		emojiID := networkid.EmojiID(emoji)
		for _, user := range group.Users {
			uid := networkid.UserID(user.ID)
			if data.Users[uid] == nil {
				data.Users[uid] = &bridgev2.ReactionSyncUser{HasAllReactions: true}
			}
			data.Users[uid].Reactions = append(data.Users[uid].Reactions, &bridgev2.BackfillReaction{
				Sender:  bridgev2.EventSender{Sender: uid},
				Emoji:   emoji,
				EmojiID: emojiID,
			})
		}
	}
	return data
}
