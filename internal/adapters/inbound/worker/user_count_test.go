package worker_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pphaiaiai/backend-challenge/internal/adapters/inbound/worker"
)

const tick = 5 * time.Millisecond

type fakeCounter struct {
	calls atomic.Int32
	count func(ctx context.Context) (int64, error)
}

func (f *fakeCounter) Count(ctx context.Context) (int64, error) {
	f.calls.Add(1)
	return f.count(ctx)
}

// syncBuffer is a bytes.Buffer safe for concurrent use by the logger and the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// start runs the reporter in a goroutine and returns a function that stops it
// and waits for Run to return.
func start(t *testing.T, counter worker.UserCounter, logs *syncBuffer, interval time.Duration) (stop func()) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(logs, nil))
	reporter := worker.NewUserCountReporter(counter, interval, logger)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		reporter.Run(ctx)
		close(done)
	}()

	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Run did not return after the context was cancelled")
		}
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 1s")
		}
		time.Sleep(tick)
	}
}

func TestUserCountReporter_LogsCountOnEveryTick(t *testing.T) {
	counter := &fakeCounter{count: func(context.Context) (int64, error) { return 42, nil }}
	logs := &syncBuffer{}

	stop := start(t, counter, logs, tick)
	waitFor(t, func() bool { return counter.calls.Load() >= 3 })
	stop()

	if got := strings.Count(logs.String(), `msg="user count" total=42`); got < 3 {
		t.Errorf("logged the count %d times, want at least 3; logs:\n%s", got, logs)
	}
}

func TestUserCountReporter_LogsErrors(t *testing.T) {
	counter := &fakeCounter{count: func(context.Context) (int64, error) { return 0, errors.New("db down") }}
	logs := &syncBuffer{}

	stop := start(t, counter, logs, tick)
	waitFor(t, func() bool { return strings.Contains(logs.String(), "db down") })
	stop()

	if !strings.Contains(logs.String(), `msg="count users failed"`) {
		t.Errorf("error not logged; logs:\n%s", logs)
	}
}

func TestUserCountReporter_ShutdownDuringCountIsNotAnError(t *testing.T) {
	counter := &fakeCounter{count: func(ctx context.Context) (int64, error) {
		<-ctx.Done() // block until shutdown
		return 0, ctx.Err()
	}}
	logs := &syncBuffer{}

	// A long interval, so the per-tick timeout cannot expire before we cancel.
	stop := start(t, counter, logs, 200*time.Millisecond)
	waitFor(t, func() bool { return counter.calls.Load() >= 1 })
	stop()

	if strings.Contains(logs.String(), "count users failed") {
		t.Errorf("cancelled count was logged as an error; logs:\n%s", logs)
	}
}
