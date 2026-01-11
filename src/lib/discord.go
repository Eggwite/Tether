package lib

import (
	"strings"
	"tether/src/utils"
)

// Discord sends Spotify album art as "spotify:abc123hash" which must be
// transformed to "https://i.scdn.co/image/abc123hash" for display.
// Non-Spotify assets are returned unchanged.
func FormatSpotifyAlbumArt(assetID string) string {
	if after, ok := strings.CutPrefix(assetID, "spotify:"); ok {
		return "https://i.scdn.co/image/" + after
	}
	return assetID
}

// EnrichAvatarDecorationData adds a "link" field to avatar_decoration_data.
// Takes the raw avatar decoration map (from Discord's user object) and constructs
// the full CDN URL using the "asset" field.
// Returns the enriched map, or nil if input is nil or missing the asset field.
// URL format: https://cdn.discordapp.com/avatar-decoration-presets/{asset}.png?size=240&passthrough=true
func EnrichAvatarDecorationData(raw any) any {
	if raw == nil {
		return nil
	}

	m := utils.MarshalToMap(raw)
	if m == nil {
		return raw
	}

	asset := utils.GetString(m["asset"])
	if asset == "" {
		return raw
	}

	m["avatar_decoration_url"] = "https://cdn.discordapp.com/avatar-decoration-presets/" + asset + ".png?size=240&passthrough=true"
	return m
}

// EnrichEmojiData adds a "link" field to emoji objects in activities.
// Custom emojis have an ID and may be animated. The CDN URL uses .gif for animated
// emojis and .png for static ones.
// Returns the enriched map, or the original value if no ID is present (Unicode emoji).
// URL format: https://cdn.discordapp.com/emojis/{id}.{ext}?size=32
func EnrichEmojiData(raw any) any {
	if raw == nil {
		return nil
	}

	m := utils.MarshalToMap(raw)
	if m == nil {
		return raw
	}

	// Unicode emojis don't have an ID, only custom emojis do
	emojiID := utils.GetString(m["id"])
	if emojiID == "" {
		return raw // <-- Returns original for Unicode emojis without ID
	}

	// Determine extension based on animated flag
	ext := "png"
	if animated, ok := m["animated"].(bool); ok && animated {
		ext = "gif"
	}

	m["emoji_url"] = "https://cdn.discordapp.com/emojis/" + emojiID + "." + ext + "?size=32"
	return m
}
