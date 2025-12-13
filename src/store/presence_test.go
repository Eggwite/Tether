package store

import (
	"testing"
)

type panickyReplicator struct{}

func (p panickyReplicator) Publish(evt PresenceEvent) error {
	panic("replicator panic")
}

func TestBroadcastWithPanickingReplicatorDoesNotCrash(t *testing.T) {
	st := NewPresenceStore()
	// Add a replicator that panics. broadcast should not cause the test to panic.
	st.AddReplicator(panickyReplicator{})

	// This should not panic even though replicator.Publish panics internally.
	st.SetPresence("user123", PresenceData{DiscordStatus: "online", DiscordUser: DiscordUser{ID: "user123"}})
}
