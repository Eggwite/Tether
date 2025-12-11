package tests

import (
	"testing"
	"time"

	"tether/src/store"
)

func TestPresenceStoreSetGet(t *testing.T) {
	st := store.NewPresenceStore()
	p := store.PresenceData{DiscordStatus: "online", DiscordUser: store.DiscordUser{ID: "123"}}
	st.SetPresence("123", p)

	got, ok := st.GetPresence("123")
	if !ok {
		t.Fatalf("expected presence to exist")
	}
	if got.DiscordStatus != "online" {
		t.Fatalf("unexpected status %s", got.DiscordStatus)
	}
}

func TestPresenceStoreBroadcast(t *testing.T) {
	st := store.NewPresenceStore()
	_, ch, cancel := st.Subscribe()
	t.Cleanup(cancel)

	p := store.PresenceData{DiscordStatus: "online", DiscordUser: store.DiscordUser{ID: "abc"}}
	st.SetPresence("abc", p)

	select {
	case evt := <-ch:
		if evt.UserID != "abc" || evt.Removed {
			t.Fatalf("unexpected event %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatalf("no broadcast received")
	}
}
