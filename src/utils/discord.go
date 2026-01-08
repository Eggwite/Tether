package utils

import (
	"encoding/json"
	"fmt"
	"strings"
)

// File Overview:
// 1. DISCORDGO FIELD OMISSIONS: The discordgo library doesn't include all fields
//    that Discord's Gateway sends, especially:
//    - sync_id/track_id for Spotify activities (critical for tracking songs)
//    - session_id in activities (for identifying specific activity sessions)
//    - New identity fields: primary_guild, collectibles, avatar_decoration_data
//
// 2. GATEWAY EVENT INCONSISTENCIES: Discord sends different field subsets in different events:
//    - PRESENCE_UPDATE: Has activities but may omit some user identity fields
//    - GUILD_MEMBER_UPDATE: Has full user identity but no activities
//    - GUILD_MEMBERS_CHUNK: Batches both presences and members with varying fields
//
// 3. SPOTIFY TRACKING: Spotify integration requires sync_id (track ID) which discordgo
//    doesn't expose through its Activity struct.

// Discord sends Spotify album art as "spotify:abc123hash" which must be
// transformed to "https://i.scdn.co/image/abc123hash" for display (Lanyard also does this.)
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

// EnrichEmojiData adds a "link" field to emoji objects in activities.
// Custom emojis have an ID and may be animated. The CDN URL uses .gif for animated
// emojis and .png for static ones.
// Returns the enriched map, or the original value if no ID is present (Unicode emoji).
// URL format: https://cdn.discordapp.com/emojis/{id}.{ext}?size=32
func EnrichEmojiData(raw any) any {
	if raw == nil {
		return nil
	}

	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}

	// Unicode emojis don't have an ID, only custom emojis do
	emojiID := GetString(m["id"])
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

// EnrichPrimaryGuildData adds a "badge_url" field to primary_guild objects.
// The primary_guild contains clan/server identity data. When both identity_guild_id
// and badge fields are present, constructs the CDN URL for the clan badge.
// Returns the enriched map, or the original value if required fields are missing.
// URL format: https://cdn.discordapp.com/clan-badges/{identity_guild_id}/{badge}.png?size=32
func EnrichPrimaryGuildData(raw any) any {
	if raw == nil {
		return nil
	}

	m := MarshalToMap(raw)
	if m == nil {
		return raw
	}

	// Both identity_guild_id and badge are required for the badge URL
	identityGuildID := GetString(m["identity_guild_id"])
	badge := GetString(m["badge"])
	if identityGuildID == "" || badge == "" {
		return raw
	}

	m["badge_url"] = "https://cdn.discordapp.com/clan-badges/" + identityGuildID + "/" + badge + ".png?size=32"
	return m
}

// EnrichActivityAssets adds *_url fields for activity assets (large/small images).
// Handles Discord CDN app-assets and media proxy external URLs.
// large_image_url/small_image_url will be added when resolvable.
// - If asset starts with "mp:external/", uses https://media.discordapp.net/{asset without "mp:" prefix}
// - Otherwise uses https://cdn.discordapp.com/app-assets/{application_id}/{asset}.webp
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

// BuildAvatarURL generates the Discord CDN URL for a user's avatar.
// Handles both custom avatars and default avatars based on discriminator.
// For custom avatars: animated (a_ prefix) get .gif, static get .webp
// For default avatars: uses discriminator or user ID to determine which default
// Returns the avatar URL with appropriate size parameter.
func BuildAvatarURL(userID, avatar, discriminator string) string {
	if avatar != "" {
		// User has a custom avatar
		var ext string
		var size int
		if strings.HasPrefix(avatar, "a_") {
			// Animated avatar
			ext = "gif"
			size = 64
		} else {
			// Static avatar
			ext = "webp"
			size = 256
		}
		return "https://cdn.discordapp.com/avatars/" + userID + "/" + avatar + "." + ext + "?size=" + fmt.Sprintf("%d", size)
	}

	// User has a default avatar
	// Calculate index based on discriminator format
	var index int
	if discriminator == "0" || discriminator == "" {
		// New username system (post-discriminator): use user ID shifted right 22 bits, mod 6
		if userID != "" {
			// Parse user ID as int64 and apply bitshift
			id := GetInt64(userID)
			index = int((id >> 22) % 6)
		}
	} else {
		// Old discriminator system: use discriminator mod 5
		index = int(GetInt64(discriminator) % 5)
	}
	return "https://cdn.discordapp.com/embed/avatars/" + fmt.Sprintf("%d", index) + ".png?size=128"
}

// Returns true if the platform key exists and has a non-empty status value.
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

// ExtractUserID gets the user ID from a raw JSON payload.
// Most Discord Gateway events have a "user" object: {"user": {"id": "123456"}}
// This safely extracts the ID without assuming the payload structure.
// Returns empty string if user or id is missing.
func ExtractUserID(payload map[string]any) string {
	userVal, ok := payload["user"].(map[string]any)
	if !ok {
		return ""
	}
	return GetString(userVal["id"])
}

// ExtractRawActivities gets the activities array from a payload.
// discordgo parses these into Activity structs, but this can drop fields like sync_id.
// We need the raw []any to preserve all fields for later processing.
// Returns nil if activities key doesn't exist or isn't an array.
func ExtractRawActivities(payload map[string]any) []any {
	rawActivities, ok := payload["activities"].([]any)
	if !ok {
		return nil
	}
	return rawActivities
}

// ExtractTimestamps gets timestamps from a raw activity map.
// Activities include timestamps for when they started/end (for Spotify playback).
// Discord sends: {"timestamps": {"start": 1234567890, "end": 1234567999}}
// These are Unix milliseconds as integers, but after JSON unmarshal they become float64.
// Returns (start, end) both as int64, or (0, 0) if timestamps are missing.
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

// IsSpotifyActivity checks if an activity is Spotify-related.
// Spotify activities have type = 2 (ActivityTypeListening) OR name = "Spotify".
// Uses OR logic because Discord may omit the type field in some payloads.
func IsSpotifyActivity(act map[string]any) bool {
	actType := GetInt64(act["type"])
	actName := GetString(act["name"])
	return actType == 2 || actName == "Spotify"
}

// GetSpotifyTrackID extracts the Spotify track ID from a raw activity map.
// DISCORD QUIRK: the gateway commonly provides the track ID as "sync_id".
// Tether treats "sync_id" as an input alias for "track_id" and only exposes
// a single field (`track_id`) in the public Spotify model.
func GetSpotifyTrackID(act map[string]any) string {
	syncID := GetString(act["sync_id"])
	if syncID == "" {
		syncID = GetString(act["track_id"])
	}
	return syncID
}

// GetSpotifyPartyID extracts the party id from a Spotify activity.
// Discord sends it as: {"party": {"id": "spotify:<user_id>"}}
func GetSpotifyPartyID(act map[string]any) string {
	party, ok := act["party"].(map[string]any)
	if !ok {
		return ""
	}
	return GetString(party["id"])
}

// LooksLikeMemberPayload checks if a payload contains member fields.
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

// ExtractRawIdentity parses user and member objects from raw JSON.
// Extracts identity fields that discordgo doesn't parse (primary_guild, collectibles, etc.).
// Returns (user map, member map). Member may be nil if not present in payload.f not present in payload.
func ExtractRawIdentity(raw json.RawMessage) (user, member map[string]any) {
	payload, ok := UnmarshalToMap(raw)
	if !ok {
		return nil, nil
	}

	return ExtractRawIdentityFromPayload(payload)
}

// ExtractRawIdentityFromPayload returns the user/member objects directly from an already-parsed payload.
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

// Define a Presence struct to encapsulate related fields
// This struct mirrors the conceptual grouping of presence data.
type Presence struct {
	Status     string          `json:"status"`
	Clients    PresenceClients `json:"clients"`
	Activities []any           `json:"activities"`
	Spotify    any             `json:"spotify"`
}

type PresenceClients struct {
	Active  []string `json:"active"`
	Primary string   `json:"primary"`
}

// BuildPresence constructs a Presence object from a raw Discord payload.
func BuildPresence(payload map[string]any) *Presence {
	if payload == nil {
		return nil
	}

	// Extract status
	status := GetString(payload["discord_status"])

	// Extract clients
	clients := PresenceClients{
		Active:  ExtractActiveClients(payload["active_clients"]),
		Primary: GetString(payload["primary_active_client"]),
	}

	// Extract activities
	activities := ExtractRawActivities(payload)

	// Extract Spotify data: use the `spotify` field directly and allow it to be nil
	var spotify any
	spotify = payload["spotify"]

	return &Presence{
		Status:     status,
		Clients:    clients,
		Activities: activities,
		Spotify:    spotify,
	}
}

// ExtractActiveClients converts raw active_clients data into a string slice.
func ExtractActiveClients(raw any) []string {
	if raw == nil {
		return nil
	}

	clients, ok := raw.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(clients))
	for _, client := range clients {
		if clientStr := GetString(client); clientStr != "" {
			result = append(result, clientStr)
		}
	}
	return result
}
