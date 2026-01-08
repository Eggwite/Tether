package api

import (
	"tether/src/store"
	"tether/src/utils"
)

// PublicPresenceFromStore converts internal store.PresenceData into the
// public-facing nested JSON object that groups clients and exposes
// `status` instead of `discord_status`.
func PublicPresenceFromStore(p store.PresenceData) map[string]any {
	// Delegate to utils to keep response shaping consistent across packages.
	return utils.PublicPresenceFromStore(p)
}
