package discord

import (
	"strings"
	"testing"
)

func TestChunksRespectLimitAndFences(t *testing.T) {
	var b strings.Builder
	b.WriteString("intro\n```go\n")
	for i := 0; i < 300; i++ {
		b.WriteString("fmt.Println(\"line of code number\")\n")
	}
	b.WriteString("```\noutro")
	parts := chunks(b.String(), discordLimit)
	if len(parts) < 2 {
		t.Fatalf("expected several chunks, got %d", len(parts))
	}
	for i, p := range parts {
		if len([]rune(p)) > 2000 {
			t.Errorf("chunk %d has %d chars", i, len([]rune(p)))
		}
		if strings.Count(p, "```")%2 != 0 {
			t.Errorf("chunk %d has unbalanced code fences", i)
		}
	}
	if !strings.HasSuffix(parts[len(parts)-1], "outro") {
		t.Errorf("last chunk lost the tail: %q", parts[len(parts)-1])
	}
	if got := chunks(strings.Repeat("x", 5000), discordLimit); len(got) != 3 {
		t.Errorf("hard split: %d chunks", len(got))
	}
}

func TestMentionAndThreadName(t *testing.T) {
	if got := cleanMention("<@123> how does  login work?", "123"); got != "how does  login work?" {
		t.Errorf("cleanMention = %q", got)
	}
	if got := cleanMention("<@!123>", "123"); got != "" {
		t.Errorf("bare mention = %q", got)
	}
	if got := threadName("how\ndoes   login work?", "an"); got != "how does login work?" {
		t.Errorf("threadName = %q", got)
	}
	if got := threadName("", "an"); got != "Question from an" {
		t.Errorf("empty threadName = %q", got)
	}
	if n := len([]rune(threadName(strings.Repeat("é", 200), "an"))); n > 100 {
		t.Errorf("threadName too long: %d", n)
	}
}
