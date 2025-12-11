package api

import (
	"encoding/json"
	"net/http"

	"tether/src/store"
	"tether/src/utils"

	"github.com/go-chi/chi/v5"
)

// SnapshotHandler serves GET /v1/users/{id}.
type SnapshotHandler struct {
	Store *store.PresenceStore
}

func (h SnapshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		h.writeJSON(w, http.StatusBadRequest, utils.Response{Success: false, Error: "user id missing"})
		return
	}
	presence, ok := h.Store.GetPresence(userID)
	if !ok {
		h.writeJSON(w, http.StatusNotFound, utils.NotFound())
		return
	}
	h.writeJSON(w, http.StatusOK, utils.Success(presence))
}

func (SnapshotHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) // keep characters like '&' readable
	_ = enc.Encode(payload)
}

// HealthHandler is a simple readiness probe.
type HealthHandler struct{}

func (HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
