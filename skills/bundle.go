// Package skills embeds the only Claude Code skills spawned agents may use.
// Layout: <group>/<skill>/SKILL.md where group is "common" or a role name;
// manifest.yaml maps each role to its allowed skill names.
package skills

import "embed"

//go:embed manifest.yaml common techlead developer tester advisor support
var FS embed.FS
