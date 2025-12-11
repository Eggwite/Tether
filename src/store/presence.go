package store

import "sync"

// Timestamps mirrors Lanyard's timestamp shape.
type Timestamps struct {
	Start     int64 `json:"start,omitempty"`
	End       int64 `json:"end,omitempty"`
	CreatedAt int64 `json:"created_at,omitempty"`
	ChangedAt int64 `json:"changed_at,omitempty"`
}

// Activity is a transparent activity payload. We keep it as an untyped map so
// new Discord fields flow through untouched.
type Activity map[string]any

// Spotify mirrors the Lanyard spotify payload when listening_to_spotify is true.
type Spotify struct {
	TrackID    string     `json:"track_id,omitempty"`
	Timestamps Timestamps `json:"timestamps,omitempty"`
	Song       string     `json:"song,omitempty"`
	Artist     string     `json:"artist,omitempty"`
	AlbumArt   string     `json:"album_art_url,omitempty"`
	Album      string     `json:"album,omitempty"`
}

// DiscordUser contains the minimal public Discord user fields Lanyard relays.
type DiscordUser struct {
	ID                   string `json:"id,omitempty"`
	Username             string `json:"username,omitempty"`
	GlobalName           string `json:"global_name,omitempty"`
	DisplayName          string `json:"display_name,omitempty"`
	Avatar               string `json:"avatar,omitempty"`
	AvatarURL            string `json:"avatar_url,omitempty"`
	Discriminator        string `json:"discriminator,omitempty"`
	AvatarDecorationData any    `json:"avatar_decoration_data"`
	PrimaryGuild         any    `json:"primary_guild"`
	Collectibles         any    `json:"collectibles"`
	DisplayNameStyles    any    `json:"display_name_styles"`
	Bot                  bool   `json:"bot"`
	PublicFlags          int    `json:"public_flags"`
}

// PresenceData is the top-level payload compatible with Lanyard's REST/WS shape.
type PresenceData struct {
	ActiveOnDiscordMobile   bool              `json:"active_on_discord_mobile"`
	ActiveOnDiscordDesktop  bool              `json:"active_on_discord_desktop"`
	ActiveOnDiscordWeb      bool              `json:"active_on_discord_web"`
	ActiveOnDiscordEmbedded bool              `json:"active_on_discord_embedded"`
	ListeningToSpotify      bool              `json:"listening_to_spotify"`
	KV                      map[string]string `json:"kv,omitempty"`
	Spotify                 *Spotify          `json:"spotify"`
	DiscordUser             DiscordUser       `json:"discord_user"`
	DiscordStatus           string            `json:"discord_status"`
	Activities              []Activity        `json:"activities"`
	SuggestedUserIfExists   *string           `json:"suggested_user_if_exists,omitempty"`
}

// PrettyPresence binds a user ID to their current Lanyard-compatible snapshot.
// This mirrors what Lanyard caches in ETS so we can store or fan out updates in
// a single, typed value without changing the external JSON hierarchy.
type PrettyPresence struct {
	UserID   string       `json:"user_id"`
	Presence PresenceData `json:"data"`
}

// PublicFields is the public envelope shape used by REST and WebSocket replies
// (success flag plus optional data or error), matching Lanyard's public API.
type PublicFields struct {
	Success bool          `json:"success"`
	Data    *PresenceData `json:"data,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// PresenceEvent represents a store mutation.
type PresenceEvent struct {
	UserID   string
	Presence PresenceData
	Removed  bool
}

// Replicator can optionally fan out presence mutations (e.g., Redis pub/sub)
// to keep multiple nodes in sync. It is best-effort and non-blocking.
type Replicator interface {
	Publish(evt PresenceEvent) error
}

// Example Redis wiring (pseudo-code):
//   type RedisReplicator struct { client *redis.Client }
//   func (r RedisReplicator) Publish(evt PresenceEvent) error { return r.client.Publish(ctx, "presence", evt).Err() }
//   store.AddReplicator(RedisReplicator{client})

// PresenceStore keeps the latest presence snapshot in-memory (RWMutex-backed
// map, akin to Lanyard's ETS) and fans out events to subscribers and optional
// cross-node replicators. All public methods are concurrency-safe.
type PresenceStore struct {
	mu            sync.RWMutex
	data          map[string]PresenceData
	watchers      map[int]chan PresenceEvent
	nextWatcherID int
	replicators   []Replicator
}

func NewPresenceStore() *PresenceStore {
	return &PresenceStore{
		data:     make(map[string]PresenceData),
		watchers: make(map[int]chan PresenceEvent),
	}
}

func (s *PresenceStore) Subscribe() (int, <-chan PresenceEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextWatcherID
	s.nextWatcherID++
	ch := make(chan PresenceEvent, 16)
	s.watchers[id] = ch

	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if c, ok := s.watchers[id]; ok {
			delete(s.watchers, id)
			close(c)
		}
	}
	return id, ch, cancel
}

// AddReplicator registers a best-effort publisher (e.g., Redis) for multi-node
// fanout. Calls are made asynchronously during broadcast to avoid blocking the
// in-memory hot path.
func (s *PresenceStore) AddReplicator(r Replicator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replicators = append(s.replicators, r)
}

func (s *PresenceStore) GetPresence(userID string) (PresenceData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.data[userID]
	return p, ok
}

func (s *PresenceStore) GetAllPresences() map[string]PresenceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := make(map[string]PresenceData, len(s.data))
	for k, v := range s.data {
		snapshot[k] = v
	}
	return snapshot
}

// Count returns the number of tracked presences.
func (s *PresenceStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *PresenceStore) SetPresence(userID string, presence PresenceData) {
	s.mu.Lock()
	s.data[userID] = presence
	s.mu.Unlock()
	s.broadcast(PresenceEvent{UserID: userID, Presence: presence})
}

// SetPresenceQuiet updates presence without broadcasting (for staged updates).
func (s *PresenceStore) SetPresenceQuiet(userID string, presence PresenceData) {
	s.mu.Lock()
	s.data[userID] = presence
	s.mu.Unlock()
}

// UpdatePresenceQuiet applies mutation without broadcasting.
func (s *PresenceStore) UpdatePresenceQuiet(userID string, update func(PresenceData) PresenceData) {
	if update == nil {
		return
	}
	s.mu.Lock()
	current, ok := s.data[userID]
	if !ok {
		current = PresenceData{DiscordStatus: "offline"}
	}
	updated := update(current)
	s.data[userID] = updated
	s.mu.Unlock()
}

func (s *PresenceStore) RemovePresence(userID string) {
	s.mu.Lock()
	delete(s.data, userID)
	s.mu.Unlock()
	s.broadcast(PresenceEvent{UserID: userID, Removed: true})
}

func (s *PresenceStore) BroadcastPresence(userID string) {
	s.mu.RLock()
	data, exists := s.data[userID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	s.broadcast(PresenceEvent{UserID: userID, Presence: data})
}

// PrettySnapshot returns the combined user ID + presence shape Lanyard exposes.
func (s *PresenceStore) PrettySnapshot(userID string) (PrettyPresence, bool) {
	p, ok := s.GetPresence(userID)
	if !ok {
		return PrettyPresence{}, false
	}
	return PrettyPresence{UserID: userID, Presence: p}, true
}

func (s *PresenceStore) broadcast(evt PresenceEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.watchers {
		select {
		case ch <- evt:
		default:
			// Drop when a watcher is slow to keep the store non-blocking.
		}
	}
	for _, r := range s.replicators {
		replicator := r
		go func() { _ = replicator.Publish(evt) }()
	}
}
