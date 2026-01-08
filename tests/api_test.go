package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tether/src/api"
	"tether/src/store"

	"github.com/go-chi/chi/v5"
)

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
