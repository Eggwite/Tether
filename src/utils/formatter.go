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
// Common cases: user is offline, not in any tracked guilds, or ID is invalid.
func NotFound() Response {
	return Response{Success: false, Error: "user not found"}
}
