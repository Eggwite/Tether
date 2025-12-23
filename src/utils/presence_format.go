package utils

import "tether/src/store"

// PublicPresenceFromStore converts internal store.PresenceData into a
// public-facing nested JSON object that groups clients and replaces
// the `discord_status` field with `status`.
func PublicPresenceFromStore(p store.PresenceData) map[string]any {
	out := make(map[string]any)

	// status replaces discord_status
	out["status"] = p.DiscordStatus

	// clients grouping
	clients := map[string]any{
		"active":  p.ActiveClients,
		"primary": p.PrimaryActiveClient,
	}
	out["clients"] = clients

	// activities and spotify pass-through
	out["activities"] = p.Activities
	out["spotify"] = p.Spotify

	// preserve discord_user under the same key for now
	out["discord_user"] = p.DiscordUser

	// keep listening flag and suggested user if present
	out["listening_to_spotify"] = p.ListeningToSpotify

	return out
}
