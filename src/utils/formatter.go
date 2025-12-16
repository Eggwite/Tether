package utils

import "tether/src/store"

// Success returns the presence object directly for successful responses.
func Success(p store.PresenceData) any {
	return p
}

// UserNotFound returns the error shape {"error": {"code","message"}}.
func UserNotFound() any {
	return ErrorResponse(
		"USER_NOT_FOUND",
		"User is not being monitored by Tether",
		404,
		false,
		nil,
	)
}

// PageNotFound returns the error shape for unknown routes.
func PageNotFound() any {
	return ErrorResponse(
		"PAGE_NOT_FOUND",
		"Route does not exist",
		404,
		false,
		nil,
	)
}
