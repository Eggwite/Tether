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
		utils.WriteJSON(w, http.StatusBadRequest, utils.ErrorResponse(
			"INVALID_USER_ID",
			"The provided user ID is invalid",
			http.StatusBadRequest,
			false,
			nil,
		))
		return
	}

	// Validate that userID consists only of digits (Discord snowflake IDs).
	for _, ch := range userID {
		if ch < '0' || ch > '9' {
			utils.WriteJSON(w, http.StatusBadRequest, utils.ErrorResponse(
				"INVALID_USER_ID",
				"The provided user ID is invalid",
				http.StatusBadRequest,
				false,
				nil,
			))
			return
		}
	}

	presence, ok := h.Store.GetPresence(userID)
	if !ok {
		utils.WriteJSON(w, http.StatusNotFound, utils.ErrorResponse(
			"USER_NOT_FOUND",
			"User is not being monitored by Tether",
			http.StatusNotFound,
			false,
			nil,
		))
		return
	}

	public := utils.PublicPresenceFromStore(presence)
	utils.WriteJSON(w, http.StatusOK, utils.SuccessResponse(public))
}

// HealthHandler is a simple readiness probe.
type HealthHandler struct{}

func (HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// MissingUserHandler handles requests to /v1/users or /v1/users/ (no user ID provided).
type MissingUserHandler struct{}

func (MissingUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusBadRequest, utils.ErrorResponse(
		"INVALID_USER_ID",
		"The provided user ID is invalid",
		http.StatusBadRequest,
		false,
		nil,
	))
}
