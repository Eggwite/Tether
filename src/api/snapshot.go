package api

import (
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
		utils.WriteJSON(w, http.StatusBadRequest, utils.Response{Success: false, Error: "user id missing"})
		return
	}
	presence, ok := h.Store.GetPresence(userID)
	if !ok {
		utils.WriteJSON(w, http.StatusNotFound, utils.UserNotFound())
		return
	}
	utils.WriteJSON(w, http.StatusOK, utils.Success(presence))
}

// HealthHandler is a simple readiness probe.
type HealthHandler struct{}

func (HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, utils.Response{
		Success: true,
		Data:    map[string]string{"status": "ok"},
	})
}
