package lib

import (
	"encoding/json"

	"tether/src/logging"
	"tether/src/store"
	"tether/src/utils"

	"github.com/sirupsen/logrus"
)

// MergeDiscordUser merges identity fields (canonical implementation).
func MergeDiscordUser(base store.DiscordUser, incoming store.DiscordUser) store.DiscordUser {
	logging.Log.WithFields(logrus.Fields{
		"base_id":     base.ID,
		"incoming_id": incoming.ID,
	}).Debug("Merging Discord user data")

	base.ID = utils.MergeStringField(base.ID, incoming.ID)
	base.Username = utils.MergeStringField(base.Username, incoming.Username)
	base.GlobalName = utils.MergeStringField(base.GlobalName, incoming.GlobalName)
	base.Avatar = utils.MergeStringField(base.Avatar, incoming.Avatar)
	base.AvatarDecorationData = utils.MergeAnyField(base.AvatarDecorationData, incoming.AvatarDecorationData)
	base.PrimaryGuild = utils.MergeAnyField(base.PrimaryGuild, incoming.PrimaryGuild)
	base.Collectibles = utils.MergeAnyField(base.Collectibles, incoming.Collectibles)
	base.DisplayNameStyles = utils.MergeAnyField(base.DisplayNameStyles, incoming.DisplayNameStyles)
	// public_flags should only be overwritten when explicitly present in the payload.
	if incoming.PublicFlagsPresent {
		base.PublicFlagsRaw = incoming.PublicFlagsRaw
		base.PublicFlagsPresent = true
	}
	base.PublicFlags = utils.PublicFlagsToNames(base.PublicFlagsRaw)

	// Regenerate avatar URL after merging avatar fields
	base.AvatarURL = BuildAvatarURL(base.ID, base.Avatar, "")

	return base
}

// MergeRawUser overlays identity fields onto a tracked user's presence entry.
// Called on GUILD_MEMBER_ADD/UPDATE when Discord sends updated user data
// without an accompanying presence event.
func MergeRawUser(st *store.PresenceStore, raw json.RawMessage) {
	userMap, memberMap := utils.ExtractRawIdentity(raw)
	if userMap == nil {
		logging.Log.Warn("MergeRawUser: failed to extract user identity from raw JSON")
		return
	}
	userID := utils.ExtractStringField(userMap, "id")
	if userID == "" {
		logging.Log.Warn("MergeRawUser: user map missing required 'id' field")
		return
	}

	incomingUser := discordUserFromRaw(userMap, memberMap)
	st.UpdatePresenceQuiet(userID, func(prev store.PresenceData) store.PresenceData {
		prev.DiscordUser = MergeDiscordUser(prev.DiscordUser, incomingUser)
		return prev
	})

	logging.Log.WithFields(logrus.Fields{
		"user_id":  userID,
		"username": incomingUser.Username,
	}).Info("user identity updated from member event")
}

// discordUserFromRaw builds DiscordUser from raw JSON maps
func discordUserFromRaw(user map[string]any, member map[string]any) store.DiscordUser {
	userID := utils.ExtractStringField(user, "id")
	logging.Log.WithField("user_id", userID).Debug("Building Discord user from raw JSON")

	userData := store.DiscordUser{
		ID:                   userID,
		Username:             utils.GetString(user["username"]),
		GlobalName:           utils.GetString(user["global_name"]),
		Avatar:               utils.GetString(user["avatar"]),
		PublicFlagsRaw:       utils.ExtractIntField(user, "public_flags"),
		AvatarDecorationData: EnrichAvatarDecorationData(user["avatar_decoration_data"]),
		PrimaryGuild:         EnrichPrimaryGuildData(user["primary_guild"]),
		Collectibles:         user["collectibles"],
		DisplayNameStyles:    user["display_name_styles"],
	}
	_, userData.PublicFlagsPresent = user["public_flags"]
	userData.PublicFlags = utils.PublicFlagsToNames(userData.PublicFlagsRaw)

	// Member-level overrides
	if member != nil {
		logging.Log.WithField("user_id", userID).Debug("Applying member-level overrides")
		// Check for member-level avatar override
		if memberAvatar := utils.GetString(member["avatar"]); memberAvatar != "" {
			userData.Avatar = memberAvatar
		}
		userData.AvatarDecorationData = utils.MergeAnyField(userData.AvatarDecorationData, EnrichAvatarDecorationData(member["avatar_decoration_data"]))
		userData.PrimaryGuild = utils.MergeAnyField(userData.PrimaryGuild, EnrichPrimaryGuildData(member["primary_guild"]))
		userData.Collectibles = utils.MergeAnyField(userData.Collectibles, member["collectibles"])
		userData.DisplayNameStyles = utils.MergeAnyField(userData.DisplayNameStyles, member["display_name_styles"])
	}

	// Generate avatar URL after all overrides are applied
	userData.AvatarURL = BuildAvatarURL(userData.ID, userData.Avatar, "")

	return userData
}
