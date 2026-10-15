package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"docket/internal/config"
	"docket/internal/daemon"
	"docket/internal/devdb"
)

const discordUsage = `usage: docket discord set-token [<bot-token>] | clear-token (global; no argument = read from stdin)
       docket discord channel <channel-id> | channel off   (this project)
       docket discord db [<postgres://…|mongodb://…/db>] | db off   (this project; no URL = read from stdin)
       docket discord status`

// discordCmd configures Discord Connect: Backend Support answers @mentions
// of the bot in the project's channel (read-only agent, one thread per question).
func discordCmd(a args, sub string) error {
	switch sub {
	case "set-token":
		tok := ""
		if len(a.pos) >= 2 && a.pos[1] != "-" {
			tok = a.pos[1]
		} else { // read from stdin so the token stays out of shell history and `ps`
			fmt.Fprint(os.Stderr, "Discord bot token: ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			tok = line
		}
		tok = strings.TrimSpace(tok)
		if strings.Count(tok, ".") != 2 {
			return fmt.Errorf("that does not look like a Discord bot token (Developer Portal → your app → Bot → Reset Token)\n%s", discordUsage)
		}
		return config.SetDiscordToken(tok)
	case "clear-token":
		return config.SetDiscordToken("")
	case "channel":
		if len(a.pos) < 2 {
			return fmt.Errorf("%s", discordUsage)
		}
		db, p, err := openDB(a)
		if err != nil {
			return err
		}
		defer db.Close()
		proj, err := db.Project()
		if err != nil {
			return err
		}
		ch := strings.TrimSpace(a.pos[1])
		if ch == "off" {
			ch = ""
		} else if strings.Trim(ch, "0123456789") != "" {
			return fmt.Errorf("channel id must be numeric (Discord: Settings → Advanced → Developer Mode, then right-click the channel → Copy Channel ID)")
		}
		proj.Settings.DiscordChannel = ch
		if err := db.SaveSettings(proj.Settings); err != nil {
			return err
		}
		kick(p) // the daemon (re)connects on its next pass
		return nil
	case "db":
		return discordDB(a)
	case "status":
		cfg, _ := config.Load()
		fmt.Printf("bot token: %s\n", map[bool]string{true: "set", false: "not set"}[cfg.DiscordToken != ""])
		db, p, err := openDB(a)
		if err != nil {
			return nil // outside a project: global status only
		}
		defer db.Close()
		proj, err := db.Project()
		if err != nil {
			return err
		}
		dbInfo := "not set"
		if proj.Settings.SupportDB != "" {
			dbInfo = devdb.Redact(proj.Settings.SupportDB)
		}
		fmt.Printf("channel:   %s\ndev db:    %s\ndaemon:    %s\n", orNone(proj.Settings.DiscordChannel), dbInfo,
			map[bool]string{true: "running (see `docket` Dashboard activity for connection state)", false: "stopped"}[daemon.Alive(p)])
		return nil
	}
	return fmt.Errorf("%s", discordUsage)
}

// discordDB sets the dev database Backend Support may query (read-only, via
// the support-db-mcp server). The URL is checked by connecting before saving.
func discordDB(a args) error {
	raw := ""
	if len(a.pos) >= 2 && a.pos[1] != "-" {
		raw = a.pos[1]
	} else { // stdin keeps the password out of shell history and `ps`
		fmt.Fprint(os.Stderr, "Dev database URL (postgres://… or mongodb://…/db): ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		raw = line
	}
	raw = strings.TrimSpace(raw)
	db, _, err := openDB(a)
	if err != nil {
		return err
	}
	defer db.Close()
	proj, err := db.Project()
	if err != nil {
		return err
	}
	if raw == "off" {
		proj.Settings.SupportDB = ""
		return db.SaveSettings(proj.Settings)
	}
	if _, err := devdb.Kind(raw); err != nil {
		return fmt.Errorf("%v\n%s", err, discordUsage)
	}
	if err := devdb.Ping(raw); err != nil {
		return fmt.Errorf("cannot connect, not saved: %v", err)
	}
	proj.Settings.SupportDB = raw
	if err := db.SaveSettings(proj.Settings); err != nil {
		return err
	}
	fmt.Printf("Backend Support can now query %s (read-only). Prefer a read-only DB user.\n", devdb.Redact(raw))
	return nil
}

// supportDBMCP serves the project's dev database to Backend Support over
// stdio MCP. It reads the URL from project settings so the agent never sees it.
func supportDBMCP(a args) error {
	db, _, err := openDB(a)
	if err != nil {
		return err
	}
	proj, err := db.Project()
	db.Close()
	if err != nil {
		return err
	}
	if proj.Settings.SupportDB == "" {
		return fmt.Errorf("no dev database set (docket discord db <url>)")
	}
	srv, err := devdb.NewServer(proj.Settings.SupportDB)
	if err != nil {
		return err
	}
	return srv.Serve(context.Background(), os.Stdin, os.Stdout)
}

func orNone(s string) string {
	if s == "" {
		return "not set"
	}
	return s
}
