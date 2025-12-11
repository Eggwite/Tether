package lib

import (
	"strings"

	"tether/src/store"
	"tether/src/utils"
)

// BuildPresenceFromRaw constructs a PresenceData snapshot directly from a raw Gateway payload.
// It avoids discordgo structs so fields that discordgo omits (sync_id, etc.) remain intact.
// Returns presence, userID, ok (false when user is missing or the status is offline).
func BuildPresenceFromRaw(payload map[string]any, user map[string]any, member map[string]any) (store.PresenceData, string, bool) {
	userID := utils.ExtractUserID(payload)
	if userID == "" && user != nil {
		userID = utils.ExtractStringField(user, "id")
	}

	status := strings.ToLower(utils.GetString(payload["status"]))
	if status == "" {
		status = "offline"
	}

	if userID == "" || status == "offline" {
		return store.PresenceData{}, userID, false
	}

	rawActivities := utils.ExtractRawActivities(payload)
	clientStatus := payload["client_status"]

	presence := store.PresenceData{
		ActiveOnDiscordDesktop:  utils.ClientStatusActive(clientStatus, "desktop"),
		ActiveOnDiscordMobile:   utils.ClientStatusActive(clientStatus, "mobile"),
		ActiveOnDiscordWeb:      utils.ClientStatusActive(clientStatus, "web"),
		ActiveOnDiscordEmbedded: utils.ClientStatusActive(clientStatus, "embedded"),
		DiscordStatus:           status,
	}

	presence = patchActivitiesFromRaw(presence, rawActivities)
	presence = patchSpotifyFromRaw(presence, rawActivities)
	presence.ListeningToSpotify = presence.Spotify != nil || hasSpotifyActivity(rawActivities)

	user = pickUserMap(user, member)
	if user == nil || member == nil {
		u, m := utils.ExtractRawIdentityFromPayload(payload)
		if user == nil {
			user = u
		}
		if member == nil {
			member = m
		}
		user = pickUserMap(user, member)
	}

	presence.DiscordUser = BuildDiscordUserFromRaw(user, member)

	return presence, userID, true
}

// hasSpotifyActivity checks whether any activity is Spotify so we can mark
// listening_to_spotify even if the Spotify object was not built.
func hasSpotifyActivity(rawActivities []any) bool {
	for _, item := range rawActivities {
		if act, ok := item.(map[string]any); ok && utils.IsSpotifyActivity(act) {
			return true
		}
	}
	return false
}

// pickUserMap chooses the richest available user map, preferring member.user
// when presence.user only contains an ID.
func pickUserMap(user map[string]any, member map[string]any) map[string]any {
	if hasIdentityFields(user) {
		return user
	}
	if member != nil {
		if mUser, ok := member["user"].(map[string]any); ok && hasIdentityFields(mUser) {
			return mUser
		}
	}
	if user != nil {
		return user
	}
	if member != nil {
		if mUser, ok := member["user"].(map[string]any); ok {
			return mUser
		}
	}
	return nil
}

// hasIdentityFields reports whether the provided user map contains meaningful
// identity data beyond an ID.
func hasIdentityFields(user map[string]any) bool {
	if user == nil {
		return false
	}
	if utils.GetString(user["username"]) != "" || utils.GetString(user["avatar"]) != "" || utils.GetString(user["global_name"]) != "" || utils.GetString(user["display_name"]) != "" {
		return true
	}
	if utils.GetString(user["discriminator"]) != "" {
		return true
	}
	if utils.ExtractIntField(user, "public_flags") != 0 {
		return true
	}
	if user["avatar_decoration_data"] != nil || user["primary_guild"] != nil || user["collectibles"] != nil || user["display_name_styles"] != nil {
		return true
	}
	return false
}
