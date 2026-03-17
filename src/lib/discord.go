package lib

import (
	"tether/src/utils"
)

// Discord sends Spotify album art as "spotify:abc123hash" which must be
// transformed to "https://i.scdn.co/image/abc123hash" for display.
// Non-Spotify assets are returned unchanged.
func FormatSpotifyAlbumArt(assetID string) string {
	return utils.FormatSpotifyAlbumArt(assetID)
}

// EnrichAvatarDecorationData adds a "link" field to avatar_decoration_data.
// Takes the raw avatar decoration map (from Discord's user object) and constructs
// the full CDN URL using the "asset" field.
// Returns the enriched map, or nil if input is nil or missing the asset field.
// URL format: https://cdn.discordapp.com/avatar-decoration-presets/{asset}.png?size=240&passthrough=true
func EnrichAvatarDecorationData(raw any) any {
	return utils.EnrichAvatarDecorationData(raw)
}

// EnrichEmojiData adds a "link" field to emoji objects in activities.
// Custom emojis have an ID and may be animated. The CDN URL uses .gif for animated
// emojis and .png for static ones.
// Returns the enriched map, or the original value if no ID is present (Unicode emoji).
// URL format: https://cdn.discordapp.com/emojis/{id}.{ext}?size=32
func EnrichEmojiData(raw any) any {
	return utils.EnrichEmojiData(raw)
}

// EnrichPrimaryGuildData keeps discord domain helpers discoverable inside lib.
func EnrichPrimaryGuildData(raw any) any {
	return utils.EnrichPrimaryGuildData(raw)
}

// BuildAvatarURL keeps discord domain helpers discoverable inside lib.
func BuildAvatarURL(userID, avatar, discriminator string) string {
	return utils.BuildAvatarURL(userID, avatar, discriminator)
}
