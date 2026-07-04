// Package recovery provides utilities for safe goroutine execution with
// panic recovery, structured logging, and Sentry exception capture.
package recovery

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	pkgSentry "NetyAdmin/internal/pkg/sentry"
)

// GoSafe launches a goroutine that runs fn with panic recovery.
//
// On panic, the recovered value is logged via slog.Error (with the goroutine
// name, panic value, and full stack trace) and captured as an exception in
// Sentry. The panic is NOT re-propagated — recovery is the whole point of
// this helper; re-panicking would defeat the purpose of using GoSafe over a
// raw `go func() { ... }()`.
//
// name is a short, stable label identifying the goroutine's purpose (e.g.
// "pubsub:dispatch", "logbus:loop", "task:worker", "open_platform:record").
// It is included as the slog message so panics can be attributed to their
// origin in log aggregators and Sentry.
//
// Use GoSafe for ALL async goroutines that may run user-provided callbacks
// or interact with external systems (DB / Redis / RPC). Without recovery,
// a single panic can silently kill a goroutine and leave the system in an
// inconsistent state (e.g. a PubSub dispatch loop that never runs again,
// or a LogBus flush loop that stops draining its buffer).
//
// Note: GoSafe does NOT manage sync.WaitGroup — callers must do wg.Add
// before calling GoSafe and put `defer wg.Done()` inside fn (defers fire
// during panic propagation, before GoSafe's recover swallows it).
func GoSafe(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error(name,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				pkgSentry.CaptureException(fmt.Errorf("%v", r))
			}
		}()
		fn()
	}()
}
