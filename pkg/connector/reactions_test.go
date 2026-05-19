package connector

import (
	"testing"

	githubv4 "github.com/shurcooL/githubv4"
)

func TestReactionEmojiMapping(t *testing.T) {
	content, err := matrixEmojiToGitHubReaction("👍")
	if err != nil {
		t.Fatal(err)
	}
	if content != githubv4.ReactionContentThumbsUp {
		t.Fatalf("expected THUMBS_UP, got %s", content)
	}
	emoji, ok := githubReactionToMatrixEmoji(githubv4.ReactionContentHeart)
	if !ok || emoji != "❤️" {
		t.Fatalf("expected heart emoji, got %q ok=%v", emoji, ok)
	}
	if _, err := matrixEmojiToGitHubReaction("🦄"); err == nil {
		t.Fatal("expected error for unsupported emoji")
	}
}
