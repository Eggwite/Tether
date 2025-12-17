package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tether/src/api"
	"tether/src/store"

	"github.com/go-chi/chi/v5"
)

func TestSnapshotHandler(t *testing.T) {
	st := store.NewPresenceStore()
	st.SetPresence("1447110828783566973", store.PresenceData{DiscordStatus: "online", DiscordUser: store.DiscordUser{ID: "1447110828783566973"}})
	handler := api.SnapshotHandler{Store: st}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/1447110828783566973", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "1447110828783566973")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Accept either the old envelope {success,data} or the new direct presence object.
	var root any
	if err := json.NewDecoder(rec.Body).Decode(&root); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	var dataMap map[string]any
	if m, ok := root.(map[string]any); ok {
		if s, exists := m["success"]; exists {
			if sb, ok := s.(bool); ok && !sb {
				t.Fatalf("unexpected non-success payload: %+v", m)
			}
			if d, ok := m["data"].(map[string]any); ok {
				dataMap = d
			}
		} else {
			dataMap = m
		}
	}
	if dataMap == nil {
		t.Fatalf("unexpected payload: %+v", root)
	}
	// Map into PresenceData to assert fields
	var pres store.PresenceData
	b, _ := json.Marshal(dataMap)
	if err := json.Unmarshal(b, &pres); err != nil {
		t.Fatalf("unmarshal presence failed: %v", err)
	}
	if pres.DiscordStatus != "online" {
		t.Fatalf("unexpected payload: %+v", pres)
	}
}

func TestSnapshotHandlerNotFound(t *testing.T) {
	st := store.NewPresenceStore()
	handler := api.SnapshotHandler{Store: st}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/missing", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
