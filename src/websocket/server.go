package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"tether/src/concurrency"
	"tether/src/logging"
	"tether/src/store"
	"tether/src/utils"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var sendLatency utils.LatencyRing

const (
	opEvent      = 0
	opHello      = 1
	opInitialize = 2
	opHeartbeat  = 3

	heartbeatJitter    = time.Second // tolerance window
	maxHeartbeatMisses = 3           // after 3 missed beats, drop

	heartbeatIntervalMs = 30000
	heartbeatTimeoutMs  = heartbeatIntervalMs * 2
)

type wsMessage struct {
	Op  int    `json:"op"`
	Seq int64  `json:"seq,omitempty"`
	T   string `json:"t,omitempty"`
	D   any    `json:"d,omitempty"`
}

type helloPayload struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type initPayload struct {
	SubscribeToIDs []string `json:"subscribe_to_ids"`
	SubscribeToID  string   `json:"subscribe_to_id"`
}

type presenceEnvelope struct {
	UserID  string              `json:"user_id"`
	Data    *store.PresenceData `json:"data,omitempty"`
	Removed bool                `json:"removed,omitempty"`
}

type connState struct {
	subs          map[string]struct{}
	lastHeartbeat time.Time
	misses        int
	mu            sync.Mutex // protects lastHeartbeat and misses
	writeMu       sync.Mutex // serializes writes to the websocket.Conn
}

// Server manages WebSocket subscriptions keyed by user ID. Clients should
// subscribe to users that share a guild with the bot (PRESENCE + MEMBERS
// intents enabled) so guild-scoped identity fields like primary_guild are
// available when the gateway includes them.
type Server struct {
	store    *store.PresenceStore
	upgrader websocket.Upgrader
	stateMu  sync.Mutex
	state    map[*websocket.Conn]*connState
	seq      int64
	cancel   func()
}

// MessageP99 returns the p99 of recent websocket send latencies.
func MessageP99() time.Duration {
	return sendLatency.P99()
}

func NewServer(store *store.PresenceStore) *Server {
	ws := &Server{
		store: store,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		state: make(map[*websocket.Conn]*connState),
	}
	_, events, cancel := store.Subscribe()
	ws.cancel = cancel
	concurrency.GoSafe(func() {
		for evt := range events {
			ws.broadcast(evt)
		}
	})
	return ws
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	compression := r.URL.Query().Get("compression") == "zlib_json"
	upgrader := s.upgrader
	upgrader.EnableCompression = compression

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logging.Log.WithError(err).Warn("ws upgrade failed")
		return
	}
	// Cap inbound frame size to bound decompression/processing work.
	conn.SetReadLimit(1 << 20) // 1 MiB
	if compression {
		conn.EnableWriteCompression(true)
	}
	s.registerConn(conn)
	s.sendHello(conn)
	go s.watchHeartbeats(conn)
	s.handleConn(conn)
}

func (s *Server) registerConn(conn *websocket.Conn) {
	s.stateMu.Lock()
	s.state[conn] = &connState{subs: make(map[string]struct{}), lastHeartbeat: time.Now()}
	s.stateMu.Unlock()
}

func (s *Server) sendHello(conn *websocket.Conn) {
	hello := wsMessage{Op: opHello, D: helloPayload{HeartbeatInterval: heartbeatIntervalMs}}
	_ = s.writeJSON(conn, hello)
}

func (s *Server) handleConn(conn *websocket.Conn) {
	defer s.cleanupConn(conn)
	for {
		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Op {
		case opInitialize:
			s.handleInit(conn, msg.D)
		case opHeartbeat:
			s.touchHeartbeat(conn)
			_ = s.writeJSON(conn, wsMessage{Op: opHeartbeat})
		default:
			s.closeWithCode(conn, 4004, "unknown_opcode")
			return
		}
	}
}

func (s *Server) handleInit(conn *websocket.Conn, raw any) {
	if raw == nil {
		s.closeWithCode(conn, 4005, "requires_data_object")
		return
	}
	if _, ok := raw.(map[string]any); !ok {
		s.closeWithCode(conn, 4005, "requires_data_object")
		return
	}

	payload := s.decodeInitPayload(raw)
	s.stateMu.Lock()
	state, ok := s.state[conn]
	if !ok {
		s.stateMu.Unlock()
		return
	}
	state.subs = make(map[string]struct{})
	if payload.SubscribeToID != "" {
		state.subs[payload.SubscribeToID] = struct{}{}
	}
	for _, id := range payload.SubscribeToIDs {
		if id != "" {
			state.subs[id] = struct{}{}
		}
	}
	if len(state.subs) == 0 {
		s.stateMu.Unlock()
		s.closeWithCode(conn, 4006, "invalid_payload")
		return
	}
	s.stateMu.Unlock()
	for userID := range state.subs {
		if presence, ok := s.store.GetPresence(userID); ok {
			s.sendEvent(conn, "INIT_STATE", presenceEnvelope{UserID: userID, Data: &presence})
		}
	}
}

func (s *Server) decodeInitPayload(raw any) initPayload {
	var payload initPayload
	data, err := json.Marshal(raw)
	if err != nil {
		return payload
	}
	_ = json.Unmarshal(data, &payload)
	return payload
}

func (s *Server) touchHeartbeat(conn *websocket.Conn) {
	s.stateMu.Lock()
	state, ok := s.state[conn]
	s.stateMu.Unlock()
	if !ok {
		return
	}
	state.mu.Lock()
	state.lastHeartbeat = time.Now()
	state.mu.Unlock()
}

func (s *Server) watchHeartbeats(conn *websocket.Conn) {
	ticker := time.NewTicker(time.Duration(heartbeatIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		s.stateMu.Lock()
		state, ok := s.state[conn]
		s.stateMu.Unlock()
		if !ok {
			return
		}
		// Count missed beats; drop after threshold. Access guarded by state.mu
		state.mu.Lock()
		timeSinceBeat := time.Since(state.lastHeartbeat)
		expected := time.Duration(heartbeatIntervalMs)*time.Millisecond + heartbeatJitter
		if timeSinceBeat > expected {
			state.misses++
		} else {
			state.misses = 0
		}
		misses := state.misses
		state.mu.Unlock()

		if misses >= maxHeartbeatMisses || timeSinceBeat > time.Duration(heartbeatTimeoutMs)*time.Millisecond {
			logging.Log.WithField("conn", conn.RemoteAddr().String()).Warn("ws heartbeat timeout")
			s.cleanupConn(conn)
			return
		}
	}
}

func (s *Server) sendEvent(conn *websocket.Conn, event string, data any) {
	msg := wsMessage{Op: opEvent, Seq: s.nextSeq(), T: event, D: data}
	start := time.Now()
	err := s.writeJSON(conn, msg)
	sendLatency.Record(time.Since(start))
	if err != nil {
		logging.Log.WithError(err).Warn("ws send failed")
		go s.cleanupConn(conn)
	}
}

func (s *Server) writeJSON(conn *websocket.Conn, v any) error {
	s.stateMu.Lock()
	state, ok := s.state[conn]
	s.stateMu.Unlock()
	if !ok {
		return websocket.ErrCloseSent
	}
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	return conn.WriteJSON(v)
}

func (s *Server) writeControl(conn *websocket.Conn, messageType int, data []byte, deadline time.Time) error {
	s.stateMu.Lock()
	state, ok := s.state[conn]
	s.stateMu.Unlock()
	if !ok {
		return websocket.ErrCloseSent
	}
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	return conn.WriteControl(messageType, data, deadline)
}

func (s *Server) broadcast(evt store.PresenceEvent) {
	s.stateMu.Lock()
	targets := make([]*websocket.Conn, 0, len(s.state))
	for conn, state := range s.state {
		if _, ok := state.subs[evt.UserID]; ok {
			targets = append(targets, conn)
		}
	}
	s.stateMu.Unlock()

	if len(targets) == 0 {
		return
	}

	logging.Log.WithFields(logrus.Fields{
		"user_id": evt.UserID,
		"subs":    len(targets),
		"removed": evt.Removed,
	}).Info("gateway event broadcast")

	var payload presenceEnvelope
	if evt.Removed {
		payload = presenceEnvelope{UserID: evt.UserID, Removed: true}
	} else {
		payload = presenceEnvelope{UserID: evt.UserID, Data: &evt.Presence}
	}

	for _, conn := range targets {
		s.sendEvent(conn, "PRESENCE_UPDATE", payload)
	}
}

func (s *Server) cleanupConn(conn *websocket.Conn) {
	s.stateMu.Lock()
	state, ok := s.state[conn]
	delete(s.state, conn)
	s.stateMu.Unlock()
	if ok {
		state.writeMu.Lock()
		_ = conn.Close()
		state.writeMu.Unlock()
	} else {
		_ = conn.Close()
	}
}

func (s *Server) closeWithCode(conn *websocket.Conn, code int, reason string) {
	_ = s.writeControl(conn, websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
	s.cleanupConn(conn)
}

// Close stops store subscription and closes active websocket connections.
func (s *Server) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.stateMu.Lock()
	for conn := range s.state {
		_ = conn.Close()
	}
	s.state = make(map[*websocket.Conn]*connState)
	s.stateMu.Unlock()
}

func (s *Server) nextSeq() int64 {
	return atomic.AddInt64(&s.seq, 1)
}
