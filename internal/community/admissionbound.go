package community

import (
	"fmt"
	"strings"
)

// Who this deployment may answer, rendered from the mounted access policy so it
// cannot drift from the gate. See docs/sirens-echo-access.md.

// AdmissionBound states the admitted surface in one line. Counts and shape
// only: an id is an identifier and the guard refuses a reply carrying one.
func AdmissionBound(policy *AccessPolicy) string {
	if policy == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if guilds := guildBound(policy.Guilds); guilds != "" {
		parts = append(parts, guilds)
	}
	parts = append(parts, directMessageBound(policy.DirectMessages))
	if agents := len(policy.Agents.Allow); agents > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d agent account(s) are admitted alongside members", agents))
	}
	return "You answer where this deployment admits you, and nowhere else. " +
		strings.Join(parts, ". ") + ". A message outside that is refused before you see it."
}

// guildBound counts rooms rather than naming them, and says whether the member
// set inside them is open or listed.
func guildBound(guilds []GuildAccess) string {
	if len(guilds) == 0 {
		return "No guild is admitted"
	}
	channels, open, listed := 0, 0, 0
	for _, guild := range guilds {
		if guild.Channels.All {
			channels = -1
		} else if channels >= 0 {
			channels += len(guild.Channels.IDs)
		}
		if guild.Users.All {
			open++
			continue
		}
		listed++
	}
	where := fmt.Sprintf("%d channel(s)", channels)
	if channels < 0 {
		where = "every channel"
	}
	switch {
	case listed == 0:
		return fmt.Sprintf("Any member may address you, in %s across %d guild(s)",
			where, len(guilds))
	case open == 0:
		return fmt.Sprintf("Only listed members may address you, in %s across %d guild(s)",
			where, len(guilds))
	}
	return fmt.Sprintf(
		"In %s across %d guild(s), %d admit any member and %d admit a listed set",
		where, len(guilds), open, listed)
}

// directMessageBound reports the posture without the accounts, since a count of
// one and the id of that one are very different things to put in a prompt.
func directMessageBound(direct DirectMessageAccess) string {
	switch len(direct.Allow) {
	case 0:
		return "Direct messages are refused"
	case 1:
		return "Direct messages are limited to one account"
	}
	return fmt.Sprintf("Direct messages are limited to %d accounts", len(direct.Allow))
}
