package api

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

	// activities pass-through, but strip the raw Spotify activity to avoid
	// duplicating data already present in the top-level `spotify` object.
	filtered := make([]store.Activity, 0, len(p.Activities))
	for _, a := range p.Activities {
		if isSpotifyActivity(map[string]any(a)) {
			continue
		}
		filtered = append(filtered, a)
	}
	out["activities"] = filtered
	out["spotify"] = p.Spotify

	// preserve discord_user under the same key
	out["discord_user"] = p.DiscordUser

	return out
}

// isSpotifyActivity checks for Spotify activities using type or name.
func isSpotifyActivity(act map[string]any) bool {
	actType, _ := act["type"].(float64)
	actName, _ := act["name"].(string)
	return int(actType) == 2 || actName == "Spotify"
}
