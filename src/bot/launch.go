package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"tether/src/lib"
	"tether/src/middleware"
	"tether/src/store"
	"tether/src/utils"
	wsmetrics "tether/src/websocket"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var rawLogCount int32
var latencySamples utils.LatencyRing

const rawLogLimit int32 = 3

// Launch connects to Discord when a token is provided; otherwise it no-ops.
// It wires PRESENCE_UPDATE and GUILD_MEMBER handlers to keep cached presence
// and identity in sync, including guild-scoped fields like primary_guild.
//
//	st := store.NewPresenceStore()
//	sess, _ := bot.Launch(os.Getenv("DISCORD_TOKEN"), st)
//
// WebSocket server can subscribe to st.Subscribe() to broadcast updates.
func Launch(token string, st *store.PresenceStore) (*discordgo.Session, error) {
	if token == "" {
		utils.Log.Warn("discord bot disabled: DISCORD_TOKEN not set")
		return nil, nil
	}
	startTime := time.Now()

	guildID := os.Getenv("GUILD_ID")
	adminIDs := parseAdminIDs(os.Getenv("ADMIN_USER_IDS"))

	sess, err := discordgo.New("Bot " + token)
	if err != nil {
		utils.Log.WithError(err).Error("failed to create discord session")
		return nil, err
	}

	sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildPresences | discordgo.IntentsGuildMembers
	// Request presence updates for all members when possible (requires privileged intents and a guild ID).
	sess.State.TrackPresences = true

	sess.AddHandler(func(s *discordgo.Session, ev *discordgo.Event) {
		if ev == nil {
			return
		}
		switch ev.Type {
		case "PRESENCE_UPDATE":
			logGatewayEvent("PRESENCE_UPDATE", ev.RawData)
			handleRawPresence(st, ev.RawData)
		case "GUILD_MEMBER_ADD", "GUILD_MEMBER_UPDATE":
			logGatewayEvent(ev.Type, ev.RawData)
			lib.MergeRawUser(st, ev.RawData)
		case "GUILD_MEMBER_REMOVE":
			logGatewayEvent(ev.Type, ev.RawData)
			handleRawMemberRemove(st, ev.RawData)
		case "GUILD_MEMBERS_CHUNK":
			logGatewayEvent(ev.Type, ev.RawData)
			lib.MergeChunkRawMembers(st, ev.RawData)
			lib.UpsertChunkPresences(st, ev.RawData)
		}
	})

	sess.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		utils.Log.WithFields(logrus.Fields{
			"bot":    r.User.Username,
			"guilds": len(r.Guilds),
		}).Info("bot ready")
		if guildID != "" {
			if err := s.RequestGuildMembers(guildID, "", 0, "", true); err != nil {
				utils.Log.WithError(err).WithField("guild_id", guildID).Error("guild member request failed")
			} else {
				utils.Log.WithField("guild_id", guildID).Info("requested guild members")
			}
		}
		if err := registerCommands(s, guildID); err != nil {
			utils.Log.WithError(err).Warn("failed to register commands")
		}
		updateBotStatus(s, st)
		recordLatencySample(s)
	})

	sess.AddHandler(handleInteractions(st, adminIDs, startTime))

	if err := sess.Open(); err != nil {
		utils.Log.WithError(err).Error("failed to open discord session")
		return nil, err
	}

	utils.Log.Info("discord bot connected")
	stopLoop := startStatusAndLatencyLoop(sess, st)
	sess.AddHandlerOnce(func(*discordgo.Session, *discordgo.Disconnect) {
		if stopLoop != nil {
			stopLoop()
		}
	})
	return sess, nil
}

// handleRawPresence builds a fresh presence snapshot from the raw Gateway payload and stores it.
func handleRawPresence(st *store.PresenceStore, raw json.RawMessage) {
	payload, ok := utils.UnmarshalToMap(raw)
	if !ok {
		utils.Log.Warn("handleRawPresence: failed to unmarshal payload")
		return
	}

	userMap, memberMap := utils.ExtractRawIdentityFromPayload(payload)
	presence, userID, ok := lib.BuildPresenceFromRaw(payload, userMap, memberMap)
	if !ok {
		if userID != "" {
			st.RemovePresence(userID)
			utils.Log.WithField("user_id", userID).Info("removed presence (offline or invalid)")
		}
		return
	}

	if prev, exists := st.GetPresence(userID); exists {
		presence.DiscordUser = lib.MergeDiscordUser(prev.DiscordUser, presence.DiscordUser)
	}

	st.SetPresence(userID, presence)
}

func handleRawMemberRemove(st *store.PresenceStore, raw json.RawMessage) {
	payload, ok := utils.UnmarshalToMap(raw)
	if !ok {
		return
	}
	userID := utils.ExtractUserID(payload)
	if userID == "" {
		return
	}
	st.RemovePresence(userID)
	utils.Log.WithField("user_id", userID).Info("removed presence from member remove")
}

func logGatewayEvent(eventType string, raw json.RawMessage) {
	fields := logrus.Fields{"event": eventType}
	if payload, ok := utils.UnmarshalToMap(raw); ok {
		if uid := utils.ExtractUserID(payload); uid != "" {
			fields["user_id"] = uid
		}
		if acts, ok := payload["activities"].([]any); ok {
			fields["activities"] = len(acts)
		}
		if members, ok := payload["members"].([]any); ok {
			fields["members"] = len(members)
		}
	}

	if atomic.AddInt32(&rawLogCount, 1) <= rawLogLimit {
		fields["payload"] = string(raw)
	}

	utils.Log.WithFields(fields).Info("gateway event received")
}

func updateBotStatus(s *discordgo.Session, st *store.PresenceStore) {
	if s == nil || st == nil {
		return
	}
	count := st.Count()
	status := fmt.Sprintf("Not stalking %d members", count)
	_ = s.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: "online",
		Activities: []*discordgo.Activity{{
			Name: status,
			Type: discordgo.ActivityTypeWatching,
		}},
	})
}

func parseAdminIDs(env string) map[string]struct{} {
	admins := make(map[string]struct{})
	for _, id := range strings.Split(env, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			admins[id] = struct{}{}
		}
	}
	return admins
}

func registerCommands(s *discordgo.Session, guildID string) error {
	if s == nil || s.State == nil || s.State.User == nil {
		return fmt.Errorf("session not ready")
	}
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "status",
			Description: "Show bot status",
		},
		{
			Name:        "lag",
			Description: "Show gateway latency",
		},
	}
	// If guildID is set, register as guild commands for instant availability
	if guildID != "" {
		if _, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, commands); err != nil {
			return err
		}
		return nil
	}
	_, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, "", commands)
	return err
}

func handleInteractions(st *store.PresenceStore, admins map[string]struct{}, start time.Time) func(*discordgo.Session, *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, ic *discordgo.InteractionCreate) {
		if ic == nil || ic.Type != discordgo.InteractionApplicationCommand {
			return
		}
		data := ic.ApplicationCommandData()

		userID := ""
		if ic.Member != nil && ic.Member.User != nil {
			userID = ic.Member.User.ID
		} else if ic.User != nil {
			userID = ic.User.ID
		}
		if _, ok := admins[userID]; !ok {
			_ = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: "Unauthorized",
				},
			})
			return
		}

		switch data.Name {
		case "status":
			uptime := time.Since(start).Truncate(time.Second)
			count := 0
			if st != nil {
				count = st.Count()
			}
			rt := snapshotRuntime()
			content := fmt.Sprintf(
				"Uptime: %s\nTracked members: %d\nGoroutines: %d\nHeap: %.2f MB\nSys: %.2f MB",
				uptime, count, rt.goroutines, rt.heapMB, rt.sysMB,
			)
			_ = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags:   discordgo.MessageFlagsEphemeral,
					Content: content,
				},
			})
		case "lag":
			lat := s.HeartbeatLatency().Round(time.Millisecond)
			p99 := latencySamples.P99().Round(time.Millisecond)
			apiP99 := middleware.APIP99().Round(time.Millisecond)
			wsP99 := wsmetrics.MessageP99().Round(time.Millisecond)
			latMs := lat.Milliseconds()
			p99Ms := p99.Milliseconds()
			apiMs := apiP99.Milliseconds()
			wsMs := wsP99.Milliseconds()
			_ = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Content: fmt.Sprintf(
						"Gateway latency: %d ms (p99 last 100: %d ms)\nHTTP p99 (last 100): %d ms\nWS send p99 (last 100): %d ms",
						latMs, p99Ms, apiMs, wsMs,
					),
				},
			})
		}
	}
}

type runtimeSnap struct {
	goroutines int
	heapMB     float64
	sysMB      float64
}

func snapshotRuntime() runtimeSnap {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return runtimeSnap{
		goroutines: runtime.NumGoroutine(),
		heapMB:     float64(m.HeapAlloc) / (1024 * 1024),
		sysMB:      float64(m.Sys) / (1024 * 1024),
	}
}

func startStatusAndLatencyLoop(s *discordgo.Session, st *store.PresenceStore) func() {
	if s == nil {
		return nil
	}
	ticker := time.NewTicker(30 * time.Second)
	stop := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateBotStatus(s, st)
				recordLatencySample(s)
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

func recordLatencySample(s *discordgo.Session) {
	if s == nil {
		return
	}
	lat := s.HeartbeatLatency()
	if lat <= 0 {
		return
	}
	latencySamples.Record(lat)
}
