package tests

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type UserResponse struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data"`
}

type Response struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data"`
}

func TestCompareLocalAndLanyardAPI(t *testing.T) {

	userID := "672569780716175370"
	localURL := "http://localhost:8080/v1/users/" + userID
	lanyardURL := "https://api.lanyard.rest/v1/users/" + userID

	localResp, err := http.Get(localURL)
	if err != nil {
		t.Fatalf("Failed to hit local API: %v", err)
	}
	defer localResp.Body.Close()
	localBody, _ := io.ReadAll(localResp.Body)

	lanyardResp, err := http.Get(lanyardURL)
	if err != nil {
		t.Fatalf("Failed to hit Lanyard API: %v", err)
	}
	defer lanyardResp.Body.Close()
	lanyardBody, _ := io.ReadAll(lanyardResp.Body)

	var localData UserResponse
	var lanyardData Response

	if err := json.Unmarshal(localBody, &localData); err != nil {
		t.Fatalf("Failed to parse local API response: %v", err)
	}
	if err := json.Unmarshal(lanyardBody, &lanyardData); err != nil {
		t.Fatalf("Failed to parse Lanyard API response: %v", err)
	}

	if !localData.Success || !lanyardData.Success {
		t.Fatalf("One or both APIs did not return success")
	}

	// Compare fields, ignoring rapidly changing ones and unsupported ones
	for k, v := range lanyardData.Data {
		if shouldIgnoreKey(k) {
			continue // skip time-based fields and Lanyard KV (unsupported)
		}
		if localVal, ok := localData.Data[k]; ok {
			if !equalValues(localVal, v) {
				t.Errorf("Field %s mismatch: local=%v lanyard=%v", k, localVal, v)
			}
		} else {
			t.Errorf("Field %s missing in local API", k)
		}
	}
}

func equalValues(a, b any) bool {
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v2 := range x {
			if shouldIgnoreKey(k) {
				continue
			}
			m[k] = normalize(v2)
		}
		return m
	case []any:
		return normalizeSlice(x)
	case float64:
		// Treat whole numbers as ints to avoid float/int string diffs
		if x == math.Trunc(x) {
			return int64(x)
		}
		return x
	default:
		return x
	}
}

// normalizeSlice makes slice comparison order-insensitive by sorting normalized elements.
func normalizeSlice(in []any) []any {
	normalized := make([]normalizedElem, 0, len(in))
	for _, v := range in {
		n := normalize(v)
		key := canonicalString(n)
		normalized = append(normalized, normalizedElem{key: key, val: n})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].key < normalized[j].key
	})
	out := make([]any, len(normalized))
	for i, el := range normalized {
		out[i] = el.val
	}
	return out
}

type normalizedElem struct {
	key string
	val any
}

// canonicalString marshals a value to a string with deterministic ordering for maps.
func canonicalString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

// shouldIgnoreKey removes known time-based or unsupported fields anywhere in the payload.
func shouldIgnoreKey(k string) bool {
	key := strings.ToLower(k)
	switch key {
	case "last_modified", "timestamp", "timestamps", "created_at", "createdat", "kv", "avatar_decoration_url", "emoji_url", "avatar_url", "badge_url", "large_image_url", "small_image_url":
		return true
	case "discord_user":
		return true
	default:
		if strings.HasSuffix(key, "_url") {
			return true
		}
		return false
	}
}
