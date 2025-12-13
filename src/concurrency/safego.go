package concurrency

import (
	"fmt"
	"runtime/debug"

	"tether/src/logging"
)

// GoSafe runs fn in a new goroutine and recovers from panics, logging the
// panic and stack via the project's `Log`. Panics are logged; process
// lifecycle (restarts) should be handled by the runtime/container.
func GoSafe(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logging.Log.WithFields(map[string]any{
					"panic": r,
				}).Error("recovered panic in background goroutine: " + fmt.Sprintf("%v", r) + "\n" + stack)
			}
		}()
		fn()
	}()
}
