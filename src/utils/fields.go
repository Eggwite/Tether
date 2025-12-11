package utils

import "fmt"

// MergeStringField updates target with source if source is non-empty.
// Pattern: new value wins, but empty string doesn't overwrite existing value.
// Prevents: Losing username when processing an event that omits it.
func MergeStringField(target, source string) string {
	if source != "" {
		return source
	}
	return target
}

// MergeIntField updates target with source if source is non-zero.
// Used for fields like PublicFlags where 0 might indicate "not provided"
// rather than "flags are actually zero".
// If source is non-zero, it replaces target. Zero sources are ignored.
func MergeIntField(target, source int) int {
	if source != 0 {
		return source
	}
	return target
}

// MergeAnyField updates target with source if source is non-nil.
// Used for Discord's new identity fields (primary_guild, collectibles, etc.)
// which are interface{} types containing nested structures.
// These fields are often nil in PRESENCE_UPDATE but populated in GUILD_MEMBER_UPDATE.
// This ensures we keep the richer data when available.
func MergeAnyField(target, source any) any {
	if source != nil {
		return source
	}
	return target
}

// ExtractStringField safely extracts a string from a map.
// Uses fmt.Sprintf(%v) to handle cases where Discord sends:
// - Actual strings: "123456"
// - Numbers as strings: user IDs like 672569780716175370
// - Other types that need string conversion
// Returns empty string if key doesn't exist.
func ExtractStringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// ExtractIntField safely extracts an int from a map.
// Handles JSON unmarshal quirk: numbers become float64 when decoding to interface{}.
// Discord sends integers for: public_flags, type fields, counts, etc.
// After json.Unmarshal into map[string]any, these are float64 and need conversion.
func ExtractIntField(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		}
	}
	return 0
}

// ExtractBoolField safely extracts a bool from a map.
// Discord uses booleans for: bot flag, mute/deaf status, etc.
// Returns false if key doesn't exist or value isn't boolean.
// Note: false default is appropriate since missing boolean flags typically mean "false".
func ExtractBoolField(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
