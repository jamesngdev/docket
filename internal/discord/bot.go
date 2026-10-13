// Package discord connects a project to its Discord channel: @mentioning the
// bot there opens a thread answered by the read-only Backend Support agent;
// @mentioning it again inside that thread continues the same agent session.
// Messages that do not mention the bot are always ignored.
//
// Every project daemon opens its own gateway connection with the shared bot
// token and only reacts to its own channel and the threads it created.
package discord

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"

	"docket/internal/paths"
	"docket/internal/store"
)

// maxConcurrent caps simultaneous agent answers per project.
const maxConcurrent = 2

// Bot is one live gateway connection serving one project.
type Bot struct {
	s      *discordgo.Session
	paths  paths.Project
	db     *store.DB
	sem    chan struct{}
	ctx    context.Context // cancelled by Close: stops running agents
	cancel context.CancelFunc
	mu     sync.Mutex
	busy   map[string]bool // threads with an answer in progress (one at a time)
	User   string          // bot username, for status messages
}

// Start connects with token and begins handling messages.
func Start(token string, p paths.Project, db *store.DB) (*Bot, error) {
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	// The bot only reads messages that @mention it, and Discord delivers their
	// content without the privileged Message Content intent.
	s.Identify.Intents = discordgo.IntentsGuildMessages
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bot{s: s, paths: p, db: db, sem: make(chan struct{}, maxConcurrent), ctx: ctx, cancel: cancel, busy: map[string]bool{}}
	s.AddHandler(b.onMessage)
	if err := s.Open(); err != nil {
		cancel()
		return nil, fmt.Errorf("discord connect: %w", err)
	}
	if s.State != nil && s.State.User != nil {
		b.User = s.State.User.Username
	}
	return b, nil
}

// Close disconnects and stops answers in progress.
func (b *Bot) Close() {
	b.cancel()
	_ = b.s.Close()
}

func (b *Bot) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || s.State.User == nil || !mentions(m, s.State.User.ID) {
		return // only ever react to an explicit @mention
	}
	me := s.State.User.ID
	author := displayName(m)
	question := cleanMention(m.Content, me)

	// @mention inside a thread we opened: continue that conversation.
	if session, ok, err := b.db.SupportThread(m.ChannelID); err == nil && ok {
		if question != "" {
			b.start(m.ChannelID, session, author, question)
		}
		return
	}

	proj, err := b.db.Project()
	if err != nil || proj.Settings.DiscordChannel == "" || m.ChannelID != proj.Settings.DiscordChannel {
		return
	}
	if question == "" {
		b.reply(m.ChannelID, m.Reference(), "Hi! Ask me anything about this project's docs, questions, features, tickets or code — e.g. `@"+s.State.User.Username+" how does login work?`")
		return
	}
	th, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name: threadName(question, author), AutoArchiveDuration: 1440, Type: discordgo.ChannelTypeGuildPublicThread,
	})
	if err != nil {
		b.reply(m.ChannelID, m.Reference(), "⚠️ I couldn't open a thread (does the bot have *Create Public Threads* permission?): "+err.Error())
		b.db.Log("support", "discord: create thread failed: %v", err)
		return
	}
	if err := b.db.SaveSupportThread(th.ID, "", author); err != nil {
		b.db.Log("support", "discord: save thread failed: %v", err)
	}
	b.start(th.ID, "", author, question)
}

func mentions(m *discordgo.MessageCreate, me string) bool {
	for _, u := range m.Mentions {
		if u.ID == me {
			return true
		}
	}
	return false
}

// cleanMention strips the bot's own mention tokens from a message.
func cleanMention(content, me string) string {
	content = strings.NewReplacer("<@"+me+">", "", "<@!"+me+">", "").Replace(content)
	return strings.TrimSpace(content)
}

func displayName(m *discordgo.MessageCreate) string {
	if m.Member != nil && m.Member.Nick != "" {
		return m.Member.Nick
	}
	if m.Author.GlobalName != "" {
		return m.Author.GlobalName
	}
	return m.Author.Username
}

var spaceRe = regexp.MustCompile(`\s+`)

// threadName is the question's first words (Discord allows 100 characters).
func threadName(question, author string) string {
	name := strings.TrimSpace(spaceRe.ReplaceAllString(question, " "))
	if r := []rune(name); len(r) > 90 {
		name = string(r[:90]) + "…"
	}
	if name == "" {
		name = "Question from " + author
	}
	return name
}
