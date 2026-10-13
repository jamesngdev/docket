package daemon

import (
	"time"

	"docket/internal/config"
	"docket/internal/discord"
)

// discordRetry spaces reconnect attempts after a failed connect.
const discordRetry = 2 * time.Minute

// discordState is the daemon's Discord connection (Backend Support).
type discordState struct {
	bot     *discord.Bot
	token   string // token the bot is connected with
	lastTry time.Time
}

// syncDiscord connects, reconnects or disconnects the bot so it matches the
// global token and the project's channel. Called every tick, so `docket discord`
// and `:discord` changes apply within seconds without restarting the daemon.
func (d *Daemon) syncDiscord() {
	cfg, _ := config.Load()
	proj, err := d.project()
	want := err == nil && cfg.DiscordToken != "" && proj.Settings.DiscordChannel != ""
	ds := &d.discord
	if ds.bot != nil && (!want || ds.token != cfg.DiscordToken) {
		ds.bot.Close()
		ds.bot, ds.token = nil, ""
		d.db.Log("support", "discord disconnected")
	}
	if !want || ds.bot != nil {
		return
	}
	if cfg.DiscordToken == ds.token && time.Since(ds.lastTry) < discordRetry {
		return // same token failed recently
	}
	ds.lastTry, ds.token = time.Now(), cfg.DiscordToken
	bot, err := discord.Start(cfg.DiscordToken, d.paths, d.db)
	if err != nil {
		d.db.Log("support", "discord connect failed (retry in %s): %v", discordRetry, err)
		return
	}
	ds.bot = bot
	d.db.Log("support", "discord connected as %s, answering @mentions in channel %s", bot.User, proj.Settings.DiscordChannel)
}

// closeDiscord disconnects on daemon shutdown.
func (d *Daemon) closeDiscord() {
	if d.discord.bot != nil {
		d.discord.bot.Close()
	}
}
