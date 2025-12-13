package utils

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes the payload as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

// Below are utility functions for handling JSON data

// GetString safely extracts a string from any value.
// Required because JSON unmarshal into interface{} preserves concrete string types,
// but we need safe extraction that won't panic on type mismatches.
func GetString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetInt64 converts common JSON-decoded numeric types to int64.
// Handles float64 (default for numbers in interface{}), int types, and json.Number (UseNumber).
// Useful for possible Discord payloads where timestamps/IDs appear as numbers in map[string]any.
func GetInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return 0
}

// GetNested retrieves a value from nested maps using a path of keys.
// Discord payloads often have annoyingly nested structures like:
//
//	{"activities": [{"assets": {"large_image": "spotify:abc123"}}]}
//
// This function safely traverses these paths without panicking on missing keys.
// Returns nil if any key in the path doesn't exist.
func GetNested(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = obj[k]
		if !ok {
			return nil
		}
	}
	return cur
}

// FirstNonEmpty returns the first non-empty string from the list.
// Discord's Gateway can omit fields or send them in different events.
// For example: details might come from discordgo struct OR raw JSON fallback.
// This enables graceful fallback chains: FirstNonEmpty(structField, rawField, "default")
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// UnmarshalToMap unmarshals JSON into a map.
// Converts json.RawMessage (raw Gateway event payload) into map[string]any for processing.
// Returns (map, true) on success, (nil, false) on failure.
// Used when we need to access fields that discordgo doesn't parse (collectibles, etc.)
func UnmarshalToMap(raw json.RawMessage) (map[string]any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	return m, true
}

// MarshalToMap turns any value into map[string]any via JSON marshal/unmarshal.
// Useful for treating discordgo structs as maps while honoring json tags.
func MarshalToMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	// Fast path: avoid marshal/unmarshal when already a map
	if m, ok := v.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}
