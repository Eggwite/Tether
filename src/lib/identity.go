package lib

import (
	"encoding/json"

	"tether/src/logging"
	"tether/src/store"
	"tether/src/utils"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// buildDiscordUser constructs a DiscordUser from discordgo.User
func buildDiscordUser(u *discordgo.User) store.DiscordUser {
	if u == nil {
		logging.Log.Warn("buildDiscordUser called with nil user")
		return store.DiscordUser{}
	}

	logging.Log.WithFields(logrus.Fields{
		"user_id":  u.ID,
		"username": u.Username,
	}).Debug("Building Discord user from discordgo.User")

	du := store.DiscordUser{
		ID:            u.ID,
		Username:      u.Username,
		Discriminator: u.Discriminator,
		Avatar:        u.Avatar,
		Bot:           u.Bot,
		PublicFlags:   int(u.PublicFlags),
		GlobalName:    u.GlobalName,
	}

	// Generate avatar URL
	du.AvatarURL = utils.BuildAvatarURL(du.ID, du.Avatar, du.Discriminator)

	if du.DisplayName == "" {
		du.DisplayName = utils.FirstNonEmpty(du.GlobalName, du.Username)
		logging.Log.WithFields(logrus.Fields{
			"user_id":      du.ID,
			"display_name": du.DisplayName,
		}).Debug("Set display name from global name or username")
	}

	return du
}

// MergeDiscordUser merges identity fields (canonical implementation).
func MergeDiscordUser(base store.DiscordUser, incoming store.DiscordUser) store.DiscordUser {
	logging.Log.WithFields(logrus.Fields{
		"base_id":     base.ID,
		"incoming_id": incoming.ID,
	}).Debug("Merging Discord user data")

	base.ID = utils.MergeStringField(base.ID, incoming.ID)
	base.Username = utils.MergeStringField(base.Username, incoming.Username)
	base.GlobalName = utils.MergeStringField(base.GlobalName, incoming.GlobalName)
	base.DisplayName = utils.MergeStringField(base.DisplayName, incoming.DisplayName)
	base.Avatar = utils.MergeStringField(base.Avatar, incoming.Avatar)
	base.Discriminator = utils.MergeStringField(base.Discriminator, incoming.Discriminator)
	base.AvatarDecorationData = utils.MergeAnyField(base.AvatarDecorationData, incoming.AvatarDecorationData)
	base.PrimaryGuild = utils.MergeAnyField(base.PrimaryGuild, incoming.PrimaryGuild)
	base.Collectibles = utils.MergeAnyField(base.Collectibles, incoming.Collectibles)
	base.DisplayNameStyles = utils.MergeAnyField(base.DisplayNameStyles, incoming.DisplayNameStyles)

	base.Bot = incoming.Bot || base.Bot
	base.PublicFlags = utils.MergeIntField(base.PublicFlags, incoming.PublicFlags)

	if base.DisplayName == "" {
		base.DisplayName = utils.FirstNonEmpty(base.GlobalName, base.Username)
	}

	// Regenerate avatar URL after merging avatar/discriminator fields
	base.AvatarURL = utils.BuildAvatarURL(base.ID, base.Avatar, base.Discriminator)

	return base
}

// MergeRawUser extracts user/member data from raw JSON
func MergeRawUser(st *store.PresenceStore, raw json.RawMessage) {
	logging.Log.Debug("Processing raw user data")
	userMap, memberMap := utils.ExtractRawIdentity(raw)
	if userMap == nil {
		logging.Log.Warn("Failed to extract user identity from raw JSON")
		return
	}
	mergeRawUserFromMaps(st, userMap, memberMap)
}

func mergeRawUserFromMaps(st *store.PresenceStore, userMap, memberMap map[string]any) {
	if userMap == nil {
		logging.Log.Debug("mergeRawUserFromMaps called with nil userMap")
		return
	}
	userID := utils.ExtractStringField(userMap, "id")
	if userID == "" {
		logging.Log.Warn("User map missing required 'id' field")
		return
	}

	logging.Log.WithField("user_id", userID).Debug("Merging raw user data")

	du := discordUserFromRaw(userMap, memberMap)
	st.UpdatePresenceQuiet(userID, func(prev store.PresenceData) store.PresenceData {
		prev.DiscordUser = MergeDiscordUser(prev.DiscordUser, du)
		return prev
	})

	logging.Log.WithFields(logrus.Fields{
		"user_id":      userID,
		"display_name": du.DisplayName,
	}).Info("Raw user data merged successfully")
}

// MergeChunkRawMembers processes GUILD_MEMBERS_CHUNK member list
func MergeChunkRawMembers(st *store.PresenceStore, raw json.RawMessage) {
	logging.Log.Debug("Processing GUILD_MEMBERS_CHUNK")

	payload, ok := utils.UnmarshalToMap(raw)
	if !ok {
		logging.Log.Error("Failed to unmarshal GUILD_MEMBERS_CHUNK payload")
		return
	}

	membersVal, ok := payload["members"].([]any)
	if !ok {
		logging.Log.Warn("GUILD_MEMBERS_CHUNK missing 'members' array")
		return
	}

	logging.Log.WithField("member_count", len(membersVal)).Info("Processing guild members chunk")

	processedCount := 0
	for _, entry := range membersVal {
		memberMap, ok := entry.(map[string]any)
		if !ok {
			logging.Log.Debug("Skipping invalid member entry (not a map)")
			continue
		}
		userMap, _ := memberMap["user"].(map[string]any)
		if userMap == nil {
			logging.Log.Debug("Skipping member entry with nil user")
			continue
		}
		mergeRawUserFromMaps(st, userMap, memberMap)
		processedCount++
	}

	logging.Log.WithFields(logrus.Fields{
		"total_members":     len(membersVal),
		"processed_members": processedCount,
	}).Info("Guild members chunk processed")
}

// discordUserFromRaw builds DiscordUser from raw JSON maps
func discordUserFromRaw(user map[string]any, member map[string]any) store.DiscordUser {
	userID := utils.ExtractStringField(user, "id")
	logging.Log.WithField("user_id", userID).Debug("Building Discord user from raw JSON")

	du := store.DiscordUser{
		ID:                   userID,
		Username:             utils.GetString(user["username"]),
		GlobalName:           utils.GetString(user["global_name"]),
		DisplayName:          utils.GetString(user["display_name"]),
		Avatar:               utils.GetString(user["avatar"]),
		Discriminator:        utils.GetString(user["discriminator"]),
		Bot:                  utils.ExtractBoolField(user, "bot"),
		PublicFlags:          utils.ExtractIntField(user, "public_flags"),
		AvatarDecorationData: utils.EnrichAvatarDecorationData(user["avatar_decoration_data"]),
		PrimaryGuild:         utils.EnrichPrimaryGuildData(user["primary_guild"]),
		Collectibles:         user["collectibles"],
		DisplayNameStyles:    user["display_name_styles"],
	}

	// Member-level overrides
	if member != nil {
		logging.Log.WithField("user_id", userID).Debug("Applying member-level overrides")
		if v := utils.GetString(member["display_name"]); v != "" {
			du.DisplayName = v
		}
		// Check for member-level avatar override
		if memberAvatar := utils.GetString(member["avatar"]); memberAvatar != "" {
			du.Avatar = memberAvatar
		}
		du.AvatarDecorationData = utils.MergeAnyField(du.AvatarDecorationData, utils.EnrichAvatarDecorationData(member["avatar_decoration_data"]))
		du.PrimaryGuild = utils.MergeAnyField(du.PrimaryGuild, utils.EnrichPrimaryGuildData(member["primary_guild"]))
		du.Collectibles = utils.MergeAnyField(du.Collectibles, member["collectibles"])
		du.DisplayNameStyles = utils.MergeAnyField(du.DisplayNameStyles, member["display_name_styles"])
	}

	// Generate avatar URL after all overrides are applied
	du.AvatarURL = utils.BuildAvatarURL(du.ID, du.Avatar, du.Discriminator)

	return du
}

// BuildDiscordUserFromRaw exposes raw identity parsing for callers that need to stage updates.
func BuildDiscordUserFromRaw(user map[string]any, member map[string]any) store.DiscordUser {
	return discordUserFromRaw(user, member)
}
