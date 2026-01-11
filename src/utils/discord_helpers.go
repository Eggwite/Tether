package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatSpotifyAlbumArt turns spotify:<hash> into the full CDN image URL.
func FormatSpotifyAlbumArt(assetID string) string {
	if after, ok := strings.CutPrefix(assetID, "spotify:"); ok {
		return "https://i.scdn.co/image/" + after
	}
	return assetID
}

// EnrichAvatarDecorationData adds avatar_decoration_url when asset is present.
func EnrichAvatarDecorationData(raw any) any {
	if raw == nil {
		return nil
	}
	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}
	asset := GetString(m["asset"])
	if asset == "" {
		return raw
	}
	m["avatar_decoration_url"] = "https://cdn.discordapp.com/avatar-decoration-presets/" + asset + ".png?size=240&passthrough=true"
	return m
}

// EnrichEmojiData adds emoji_url for custom emojis; Unicode emoji is returned unchanged.
func EnrichEmojiData(raw any) any {
	if raw == nil {
		return nil
	}
	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}
	emojiID := GetString(m["id"])
	if emojiID == "" {
		return raw
	}
	ext := "png"
	if animated, ok := m["animated"].(bool); ok && animated {
		ext = "gif"
	}
	m["emoji_url"] = "https://cdn.discordapp.com/emojis/" + emojiID + "." + ext + "?size=32"
	return m
}

// EnrichPrimaryGuildData adds badge_url when identity_guild_id and badge are present.
func EnrichPrimaryGuildData(raw any) any {
	if raw == nil {
		return nil
	}
	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}
	identityGuildID := GetString(m["identity_guild_id"])
	badge := GetString(m["badge"])
	if identityGuildID == "" || badge == "" {
		return raw
	}
	m["badge_url"] = "https://cdn.discordapp.com/clan-badges/" + identityGuildID + "/" + badge + ".png?size=32"
	return m
}

// EnrichActivityAssets adds *_image_url fields for activity assets.
func EnrichActivityAssets(raw any) any {
	if raw == nil {
		return nil
	}
	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}
	appID := GetString(m["application_id"])
	assetsVal, ok := m["assets"].(map[string]any)
	if !ok {
		return raw
	}
	assets := MarshalToMap(assetsVal)
	if assets == nil {
		return raw
	}

	buildURL := func(asset string) string {
		if asset == "" {
			return ""
		}
		if strings.HasPrefix(asset, "mp:external/") {
			return "https://media.discordapp.net/" + strings.TrimPrefix(asset, "mp:")
		}
		if appID == "" {
			return ""
		}
		return "https://cdn.discordapp.com/app-assets/" + appID + "/" + asset + ".webp"
	}

	if li := GetString(assets["large_image"]); li != "" {
		if url := buildURL(li); url != "" {
			assets["large_image_url"] = url
		}
	}
	if si := GetString(assets["small_image"]); si != "" {
		if url := buildURL(si); url != "" {
			assets["small_image_url"] = url
		}
	}

	m["assets"] = assets
	return m
}

// BuildAvatarURL builds Discord CDN avatar URLs for custom or default avatars.
func BuildAvatarURL(userID, avatar, discriminator string) string {
	if avatar != "" {
		ext := "webp"
		size := 256
		if strings.HasPrefix(avatar, "a_") {
			ext = "gif"
			size = 64
		}
		return "https://cdn.discordapp.com/avatars/" + userID + "/" + avatar + "." + ext + "?size=" + fmt.Sprintf("%d", size)
	}
	var index int
	if discriminator == "0" || discriminator == "" {
		if userID != "" {
			id := GetInt64(userID)
			index = int((id >> 22) % 6)
		}
	} else {
		index = int(GetInt64(discriminator) % 5)
	}
	return "https://cdn.discordapp.com/embed/avatars/" + fmt.Sprintf("%d", index) + ".png?size=128"
}

// ClientStatusActive reports if a given platform key has a non-empty status.
func ClientStatusActive(status interface{}, platform string) bool {
	if status == nil {
		return false
	}
	m := MarshalToMap(status)
	if m == nil {
		return false
	}
	val := GetString(m[strings.ToLower(platform)])
	return val != ""
}

// ExtractUserID pulls user.id from a payload map.
func ExtractUserID(payload map[string]any) string {
	userVal, ok := payload["user"].(map[string]any)
	if !ok {
		return ""
	}
	return GetString(userVal["id"])
}

// ExtractRawActivities returns the raw activities slice from a payload.
func ExtractRawActivities(payload map[string]any) []any {
	rawActivities, ok := payload["activities"].([]any)
	if !ok {
		return nil
	}
	return rawActivities
}

// ExtractTimestamps reads timestamps.start/end from an activity map.
func ExtractTimestamps(raw map[string]any) (start, end int64) {
	if raw == nil {
		return 0, 0
	}
	val := GetNested(raw, "timestamps")
	obj, ok := val.(map[string]any)
	if !ok {
		return 0, 0
	}
	return GetInt64(obj["start"]), GetInt64(obj["end"])
}

// IsSpotifyActivity detects Spotify activities via type or name.
func IsSpotifyActivity(act map[string]any) bool {
	actType := GetInt64(act["type"])
	actName := GetString(act["name"])
	return actType == 2 || actName == "Spotify"
}

// GetSpotifyTrackID returns sync_id or track_id from an activity.
func GetSpotifyTrackID(act map[string]any) string {
	syncID := GetString(act["sync_id"])
	if syncID == "" {
		syncID = GetString(act["track_id"])
	}
	return syncID
}

// GetSpotifyPartyID extracts party.id from an activity.
func GetSpotifyPartyID(act map[string]any) string {
	party, ok := act["party"].(map[string]any)
	if !ok {
		return ""
	}
	return GetString(party["id"])
}

// LooksLikeMemberPayload heuristically detects member-like payloads.
func LooksLikeMemberPayload(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	keys := []string{"roles", "joined_at", "nick", "communication_disabled_until"}
	for _, k := range keys {
		if _, ok := payload[k]; ok {
			return true
		}
	}
	return false
}

// ExtractRawIdentity parses user/member objects from raw JSON.
func ExtractRawIdentity(raw json.RawMessage) (user, member map[string]any) {
	payload, ok := UnmarshalToMap(raw)
	if !ok {
		return nil, nil
	}
	return ExtractRawIdentityFromPayload(payload)
}

// ExtractRawIdentityFromPayload returns user/member from an already-parsed payload.
func ExtractRawIdentityFromPayload(payload map[string]any) (user, member map[string]any) {
	if payload == nil {
		return nil, nil
	}
	userVal, _ := payload["user"].(map[string]any)
	var memberMap map[string]any
	if m, ok := payload["member"].(map[string]any); ok {
		memberMap = m
	} else if LooksLikeMemberPayload(payload) {
		memberMap = payload
	}
	return userVal, memberMap
}
