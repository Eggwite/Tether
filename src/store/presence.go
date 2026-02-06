package store

import (
	"sync"
	"tether/src/concurrency"
)

type Timestamps struct {
	Start     int64 `json:"start,omitempty"`
	End       int64 `json:"end,omitempty"`
	CreatedAt int64 `json:"created_at,omitempty"`
	ChangedAt int64 `json:"changed_at,omitempty"`
}

// Activity is a transparent activity payload. We keep it as an untyped map so
// new Discord fields flow through untouched.
type Activity map[string]any

type Spotify struct {
	TrackID    *string     `json:"track_id"`
	PartyID    *string     `json:"party_id"`
	Timestamps *Timestamps `json:"timestamps"`
	Song       *string     `json:"song"`
	Artist     *string     `json:"artist"`
	AlbumArt   *string     `json:"album_art_url"`
	Album      *string     `json:"album"`
}

// PublicClients is the public, stable clients grouping used by REST and WS.
// It matches the documented API schema.
type PublicClients struct {
	Active  []string `json:"active"`
	Primary string   `json:"primary"`
}

// PublicPresence is the public, stable presence snapshot returned by REST and WS.
// It is precomputed at write-time (gateway events) to keep read paths cheap.
type PublicPresence struct {
	Status      string        `json:"status"`
	Activities  []Activity    `json:"activities"`
	Clients     PublicClients `json:"clients"`
	DiscordUser DiscordUser   `json:"discord_user"`
	Spotify     *Spotify      `json:"spotify"`
}

// DiscordUser contains the minimal public Discord user fields Tether relays.
type DiscordUser struct {
	ID                   string   `json:"id,omitempty"`
	Username             string   `json:"username,omitempty"`
	GlobalName           string   `json:"global_name,omitempty"`
	Avatar               string   `json:"avatar,omitempty"`
	AvatarURL            string   `json:"avatar_url,omitempty"`
	AvatarDecorationData any      `json:"avatar_decoration_data"`
	PrimaryGuild         any      `json:"primary_guild"`
	Collectibles         any      `json:"collectibles"`
	DisplayNameStyles    any      `json:"display_name_styles"`
	PublicFlagsRaw       int      `json:"-"`
	PublicFlags          []string `json:"public_flags"`
}

type PresenceData struct {
	// Internal booleans: kept for construction but omitted from public JSON
	ActiveOnDiscordMobile   bool `json:"-"`
	ActiveOnDiscordDesktop  bool `json:"-"`
	ActiveOnDiscordWeb      bool `json:"-"`
	ActiveOnDiscordEmbedded bool `json:"-"`
	// Derived convenience fields summarizing active clients.
	ActiveClients         []string    `json:"active_clients,omitempty"`
	PrimaryActiveClient   string      `json:"primary_active_client,omitempty"`
	Spotify               *Spotify    `json:"spotify"`
	DiscordUser           DiscordUser `json:"discord_user"`
	DiscordStatus         string      `json:"discord_status"`
	Activities            []Activity  `json:"activities"`
	SuggestedUserIfExists *string     `json:"suggested_user_if_exists,omitempty"`
	// Public is the precomputed public-facing snapshot used by REST and WS.
	// It is intentionally omitted from JSON when PresenceData is marshaled.
	Public PublicPresence `json:"-"`
}

func isSpotifyActivity(act map[string]any) bool {
	// Spotify is typically activity type 2, but some payloads rely on name.
	// This mirrors the docs-facing filter behavior.
	if t, ok := act["type"].(float64); ok && int(t) == 2 {
		return true
	}
	if name, ok := act["name"].(string); ok && name == "Spotify" {
		return true
	}
	return false
}

func buildPublicPresence(p PresenceData) PublicPresence {
	active := p.ActiveClients
	if active == nil {
		active = []string{}
	}

	filtered := make([]Activity, 0, len(p.Activities))
	for _, a := range p.Activities {
		if isSpotifyActivity(map[string]any(a)) {
			continue
		}
		filtered = append(filtered, a)
	}

	return PublicPresence{
		Status:      p.DiscordStatus,
		Clients:     PublicClients{Active: active, Primary: p.PrimaryActiveClient},
		Activities:  filtered,
		Spotify:     p.Spotify,
		DiscordUser: p.DiscordUser,
	}
}

func normalizePresence(p PresenceData) PresenceData {
	// Ensure cached public snapshot is always in sync.
	p.Public = buildPublicPresence(p)
	return p
}

type PrettyPresence struct {
	UserID   string       `json:"user_id"`
	Presence PresenceData `json:"data"`
}

// PublicFields is the public envelope shape used by REST and WebSocket replies
type PublicFields struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
	Error   any  `json:"error,omitempty"`
}

// PresenceEvent represents a store mutation.
type PresenceEvent struct {
	UserID   string
	Presence PresenceData
	Removed  bool
}

// Replicator can optionally fan out presence mutations (e.g., via pub/sub)
// to keep multiple nodes in sync. It is best-effort and non-blocking.
type Replicator interface {
	Publish(evt PresenceEvent) error
}

// PresenceStore keeps the latest presence snapshot in-memory (RWMutex-backed
// map) and fans out events to subscribers and optional
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

// AddReplicator registers a best-effort publisher for multi-node
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
	presence = normalizePresence(presence)
	s.mu.Lock()
	s.data[userID] = presence
	s.mu.Unlock()
	s.broadcast(PresenceEvent{UserID: userID, Presence: presence})
}

// SetPresenceQuiet updates presence without broadcasting (for staged updates).
func (s *PresenceStore) SetPresenceQuiet(userID string, presence PresenceData) {
	presence = normalizePresence(presence)
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
	updated = normalizePresence(updated)
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

// PrettySnapshot returns the combined user ID + presence shape Tether exposes.
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
		// Use the project's safe goroutine helper for consistent logging
		concurrency.GoSafe(func() {
			_ = replicator.Publish(evt)
		})
	}
}
