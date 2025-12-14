package utils

import "tether/src/store"

// Success returns the presence object directly for successful responses.
func Success(p store.PresenceData) any {
	return p
}

// UserNotFound returns the error shape {"error": {"code","message"}}.
func UserNotFound() any {
	return map[string]any{
		"error": map[string]any{
			"code":    "user_not_monitored",
			"message": "User is not being monitored by Tether",
		},
	}
}

// PageNotFound returns the error shape for unknown routes.
func PageNotFound() any {
	return map[string]any{
		"error": map[string]any{
			"code":    "page_not_found",
			"message": "Route does not exist",
		},
	}
}
