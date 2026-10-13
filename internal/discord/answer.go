package discord

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"docket/internal/support"
)

// discordLimit is Discord's maximum message length; chunks stay a bit below it.
const discordLimit = 1900

// start answers in the background, or says why it can't right now. Messages
// are never queued, so channel traffic cannot pile up agent runs: at most one
// answer per thread and maxConcurrent per project run at a time.
func (b *Bot) start(threadID, session, author, question string) {
	b.mu.Lock()
	if b.busy[threadID] {
		b.mu.Unlock()
		b.reply(threadID, nil, "⏳ I'm still working on the previous message in this thread — send this again once I've answered.")
		return
	}
	select {
	case b.sem <- struct{}{}:
	default:
		b.mu.Unlock()
		b.reply(threadID, nil, "⏳ I'm busy answering other questions right now — please try again in a few minutes.")
		return
	}
	b.busy[threadID] = true
	b.mu.Unlock()
	go func() {
		defer func() {
			<-b.sem
			b.mu.Lock()
			delete(b.busy, threadID)
			b.mu.Unlock()
		}()
		b.answer(threadID, session, author, question)
	}()
}

// answer runs Backend Support for one message and posts the reply in the thread.
func (b *Bot) answer(threadID, session, author, question string) {
	if session == "" { // a follow-up may arrive right after the first answer saved its session
		session, _, _ = b.db.SupportThread(threadID)
	}
	stop := b.typing(threadID)
	start := time.Now()
	a, err := support.Ask(b.ctx, b.paths, b.db, support.Request{Session: session, Author: author, Question: question})
	stop()
	if b.ctx.Err() != nil {
		return // disconnected (token/channel changed or daemon stopping)
	}
	if err != nil {
		b.db.Log("support", "discord: answering %s failed: %v", author, err)
		b.reply(threadID, nil, "⚠️ Sorry, I couldn't answer that one (details are in the docket activity log). Please try again.")
		return
	}
	if err := b.db.SaveSupportThread(threadID, a.Session, author); err != nil {
		b.db.Log("support", "discord: save session failed: %v", err)
	}
	for _, chunk := range chunks(a.Text, discordLimit) {
		b.reply(threadID, nil, chunk)
	}
	b.db.Log("support", "answered %s on Discord in %s", author, time.Since(start).Round(time.Second))
}

// typing shows "Backend Support is typing…" until stop is called.
func (b *Bot) typing(channelID string) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(8 * time.Second) // the indicator lasts ~10s per call
		defer t.Stop()
		for {
			_ = b.s.ChannelTyping(channelID)
			select {
			case <-done:
				return
			case <-t.C:
			}
		}
	}()
	return func() { close(done) }
}

// reply sends text without pinging anyone, whatever the agent wrote.
func (b *Bot) reply(channelID string, ref *discordgo.MessageReference, text string) {
	_, err := b.s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: text, Reference: ref, AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	if err != nil {
		b.db.Log("support", "discord: send failed: %v", err)
	}
}

// chunks splits text into messages of at most n characters, preferring line
// breaks and keeping ``` code fences balanced across chunks.
func chunks(text string, n int) []string {
	var out []string
	var cur strings.Builder
	inFence := false
	flush := func() {
		s := cur.String()
		if inFence {
			s += "\n```"
		}
		if strings.Trim(s, "` \n") != "" { // skip chunks that would be empty fences
			out = append(out, s)
		}
		cur.Reset()
		if inFence {
			cur.WriteString("```\n")
		}
	}
	for _, line := range strings.Split(text, "\n") {
		for len([]rune(line)) > n-10 { // a single huge line: hard split
			r := []rune(line)
			if cur.Len() > 0 {
				flush()
			}
			cur.WriteString(string(r[:n-10]))
			flush()
			line = string(r[n-10:])
		}
		if cur.Len()+len(line)+1 > n-8 {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n")
		}
		cur.WriteString(line)
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
	}
	inFence = false
	flush()
	return out
}
