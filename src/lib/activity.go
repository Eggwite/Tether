package lib

import (
	"encoding/json"

	"tether/src/store"
	"tether/src/utils"
)

// patchActivitiesFromRaw updates the Activities list with raw JSON data
func patchActivitiesFromRaw(prev store.PresenceData, rawActivities []any) store.PresenceData {
	if len(rawActivities) == 0 {
		return prev
	}

	acts := make([]store.Activity, 0, len(rawActivities))
	for _, rawItem := range rawActivities {
		if m, ok := rawItem.(map[string]any); ok {
			// Enrich emoji with CDN link if present
			if emoji, exists := m["emoji"]; exists {
				m["emoji"] = utils.EnrichEmojiData(emoji)
			}
			// Enrich activity asset URLs if present
			if enriched := utils.EnrichActivityAssets(m); enriched != nil {
				if em, ok := enriched.(map[string]any); ok {
					m = em
				}
			}
			acts = append(acts, store.Activity(m))
		}
	}

	if len(acts) > 0 {
		prev.Activities = acts
	}

	return prev
}

// UpsertChunkPresences replaces presence snapshots from a GUILD_MEMBERS_CHUNK raw payload.
// It builds presences directly from raw maps to retain all fields and avoids discordgo structs.
func UpsertChunkPresences(st *store.PresenceStore, raw json.RawMessage) {
	payload, ok := utils.UnmarshalToMap(raw)
	if !ok {
		return
	}

	memberLookup := buildMemberLookup(payload)
	rawPresences, ok := payload["presences"].([]any)
	if !ok {
		return
	}

	for _, item := range rawPresences {
		pres, ok := item.(map[string]any)
		if !ok {
			continue
		}

		member := memberLookup[utils.ExtractUserID(pres)]
		userMap := pres["user"].(map[string]any)
		presence, userID, ok := BuildPresenceFromRaw(pres, userMap, member)
		if !ok {
			if userID != "" {
				st.RemovePresence(userID)
			}
			continue
		}

		st.SetPresenceQuiet(userID, presence)
		st.BroadcastPresence(userID)
	}
}

func buildMemberLookup(payload map[string]any) map[string]map[string]any {
	members, ok := payload["members"].([]any)
	if !ok {
		return nil
	}

	lookup := make(map[string]map[string]any, len(members))
	for _, entry := range members {
		memberMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		userMap, _ := memberMap["user"].(map[string]any)
		if id := utils.ExtractStringField(userMap, "id"); id != "" {
			lookup[id] = memberMap
		}
	}

	return lookup
}
