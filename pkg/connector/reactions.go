package connector

import (
	"fmt"
	"strings"

	githubv4 "github.com/shurcooL/githubv4"
)

// matrixEmojiToGitHub maps Matrix reaction keys to GitHub ReactionContent values.
var matrixEmojiToGitHub = map[string]githubv4.ReactionContent{
	"👍":  githubv4.ReactionContentThumbsUp,
	"👎":  githubv4.ReactionContentThumbsDown,
	"😄":  githubv4.ReactionContentLaugh,
	"😕":  githubv4.ReactionContentConfused,
	"❤️": githubv4.ReactionContentHeart,
	"❤":  githubv4.ReactionContentHeart,
	"🎉":  githubv4.ReactionContentHooray,
	"🚀":  githubv4.ReactionContentRocket,
	"👀":  githubv4.ReactionContentEyes,
}

var githubToMatrixEmoji = map[githubv4.ReactionContent]string{
	githubv4.ReactionContentThumbsUp:   "👍",
	githubv4.ReactionContentThumbsDown: "👎",
	githubv4.ReactionContentLaugh:      "😄",
	githubv4.ReactionContentConfused:   "😕",
	githubv4.ReactionContentHeart:      "❤️",
	githubv4.ReactionContentHooray:     "🎉",
	githubv4.ReactionContentRocket:     "🚀",
	githubv4.ReactionContentEyes:       "👀",
}

var AllowedMatrixReactions = []string{"👍", "👎", "😄", "😕", "❤️", "🎉", "🚀", "👀"}

func matrixEmojiToGitHubReaction(emoji string) (githubv4.ReactionContent, error) {
	if content, ok := matrixEmojiToGitHub[emoji]; ok {
		return content, nil
	}
	return "", fmt.Errorf("unsupported reaction emoji %q", emoji)
}

func githubReactionToMatrixEmoji(content githubv4.ReactionContent) (string, bool) {
	emoji, ok := githubToMatrixEmoji[content]
	return emoji, ok
}

func githubReactionEmojiID(content githubv4.ReactionContent) string {
	return strings.ToLower(string(content))
}
