package utils

import "tether/src/store"

// Response (alias: PublicFields) standardizes REST/WS envelope.
type Response = store.PublicFields

// Success creates a successful response envelope with presence data.
// Used when a user's presence is found and should be returned.
func Success(p store.PresenceData) Response {
	return Response{Success: true, Data: &p}
}

// NotFound creates an error response when a user's presence isn't available.
// Common cases: user is not in any tracked guilds, or ID is invalid.
func UserNotFound() Response {
	return Response{
		Success: false,
		Error: map[string]any{
			"code":    "user_not_monitored",
			"message": "User is not being monitored by Tether",
		},
	}
}

// PageNotFound creates an error response for unknown routes.
// Used when a requested API endpoint does not exist.
func PageNotFound() Response {
	return Response{
		Success: false,
		Error: map[string]any{
			"code":    "page_not_found",
			"message": "Route does not exist",
		},
	}
}
