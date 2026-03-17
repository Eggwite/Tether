package lib

import (
	"tether/src/store"
	"tether/src/utils"
)

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
		partyID := utils.GetSpotifyPartyID(act)

		// Extract timestamps (these change continuously during playback)
		start, end := utils.ExtractTimestamps(act)

		albumArt := ""
		album := ""
		if assets, ok := act["assets"].(map[string]any); ok {
			if img := utils.GetString(assets["large_image"]); img != "" {
				albumArt = FormatSpotifyAlbumArt(img)
			}
			album = utils.GetString(assets["large_text"])
		}

		song := utils.GetString(act["details"])
		artist := utils.GetString(act["state"])

		if prev.Spotify == nil {
			prev.Spotify = &store.Spotify{}
		}

		// helper converters
		strPtr := func(s string) *string {
			if s == "" {
				return nil
			}
			return &s
		}

		// Update all fields to reflect current playback state
		prev.Spotify.TrackID = strPtr(trackID)
		prev.Spotify.Timestamps = &store.Timestamps{Start: start, End: end}
		if partyID != "" {
			prev.Spotify.PartyID = strPtr(partyID)
		} else {
			prev.Spotify.PartyID = nil
		}
		if song != "" {
			prev.Spotify.Song = strPtr(song)
		} else {
			prev.Spotify.Song = nil
		}
		if artist != "" {
			prev.Spotify.Artist = strPtr(artist)
		} else {
			prev.Spotify.Artist = nil
		}
		if albumArt != "" {
			prev.Spotify.AlbumArt = strPtr(albumArt)
		} else {
			prev.Spotify.AlbumArt = nil
		}
		if album != "" {
			prev.Spotify.Album = strPtr(album)
		} else {
			prev.Spotify.Album = nil
		}

		return prev
	}

	return prev
}
