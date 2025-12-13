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

	var resp struct {
		Success bool               `json:"success"`
		Data    store.PresenceData `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !resp.Success || resp.Data.DiscordStatus != "online" {
		t.Fatalf("unexpected payload: %+v", resp)
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

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
