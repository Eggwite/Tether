package concurrency

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestGoSafeRecovers ensures that a panic inside GoSafe does not crash the
// process and that subsequent goroutines can still run.
func TestGoSafeRecovers(t *testing.T) {
	var got int32

	// Trigger a panic inside GoSafe.
	GoSafe(func() {
		panic("test-panic")
	})

	// Schedule another goroutine via GoSafe that sets a flag when run.
	GoSafe(func() {
		atomic.StoreInt32(&got, 1)
	})

	// Wait up to 1s for the follow-up goroutine to run.
	start := time.Now()
	for time.Since(start) < 1*time.Second {
		if atomic.LoadInt32(&got) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&got) == 0 {
		t.Fatalf("expected follow-up goroutine to run after recovered panic")
	}
}
