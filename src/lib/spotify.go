package lib

import (
	"tether/src/store"
	"tether/src/utils"

	"github.com/bwmarrin/discordgo"
)

// buildSpotify creates a Spotify object from Discord activity data
func buildSpotify(act *discordgo.Activity, raw map[string]any) *store.Spotify {
	start, end := utils.ExtractTimestamps(raw)

	trackID := utils.GetSpotifyTrackID(raw)

	albumArt := ""
	if assets, ok := raw["assets"].(map[string]any); ok {
		if img := utils.GetString(assets["large_image"]); img != "" {
			albumArt = utils.FormatSpotifyAlbumArt(img)
		}
	}

	album := ""
	if assets, ok := raw["assets"].(map[string]any); ok {
		album = utils.GetString(assets["large_text"])
	}

	return &store.Spotify{
		TrackID:    trackID,
		Timestamps: store.Timestamps{Start: start, End: end},
		Song:       utils.FirstNonEmpty(act.Details, utils.GetString(raw["details"])),
		Artist:     utils.FirstNonEmpty(act.State, utils.GetString(raw["state"])),
		AlbumArt:   albumArt,
		Album:      album,
	}
}

// patchSpotifyFromRaw extracts Spotify track ID and timestamps from raw activities.
// This updates the Spotify object even if a track_id already exists, because the
// track_id changes when the song changes. Also updates timestamps which change
// continuously during playback.
func patchSpotifyFromRaw(prev store.PresenceData, rawActivities []any) store.PresenceData {
	// Scan raw activities for Spotify data
	for _, item := range rawActivities {
		act, ok := item.(map[string]any)
		if !ok || !utils.IsSpotifyActivity(act) {
			continue
		}

		// Extract track_id (always update if present, even if we had one before)
		trackID := utils.GetSpotifyTrackID(act)
		if trackID == "" {
			continue
		}

		// Extract timestamps (these change continuously during playback)
		start, end := utils.ExtractTimestamps(act)

		albumArt := ""
		album := ""
		if assets, ok := act["assets"].(map[string]any); ok {
			if img := utils.GetString(assets["large_image"]); img != "" {
				albumArt = utils.FormatSpotifyAlbumArt(img)
			}
			album = utils.GetString(assets["large_text"])
		}

		song := utils.GetString(act["details"])
		artist := utils.GetString(act["state"])

		if prev.Spotify == nil {
			prev.Spotify = &store.Spotify{}
		}

		// Update all fields to reflect current playback state
		prev.Spotify.TrackID = trackID
		prev.Spotify.Timestamps = store.Timestamps{Start: start, End: end}
		if song != "" {
			prev.Spotify.Song = song
		}
		if artist != "" {
			prev.Spotify.Artist = artist
		}
		if albumArt != "" {
			prev.Spotify.AlbumArt = albumArt
		}
		if album != "" {
			prev.Spotify.Album = album
		}

		// Also update sync_id in Activities list (thanks discordgo >:[)
		for i, a := range prev.Activities {
			if utils.IsSpotifyActivity(a) {
				prev.Activities[i]["sync_id"] = trackID
				// prev.Activities[i]["track_id"] = trackID --- to maintain parity with Lanyard (because of course)
			}
		}
		return prev
	}

	return prev
}
