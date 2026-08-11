package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEventLoggerWritesNDJSON(t *testing.T) {
	dir := t.TempDir()
	l := newEventLogger(dir)
	if l == nil {
		t.Fatal("logger should be enabled with a dir")
	}
	l.log("roll", pid("tok"), map[string]any{"stars_before": 3, "success": true})
	l.log("sell", pid("tok"), map[string]any{"gain": 6})

	path := filepath.Join(dir, time.Now().Format("2006-01-02")+".ndjson")
	var lines []map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		lines = lines[:0]
		if f, err := os.Open(path); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				var e map[string]any
				if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
					t.Fatalf("invalid NDJSON line %q: %v", sc.Text(), err)
				}
				lines = append(lines, e)
			}
			f.Close()
		}
		if len(lines) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 events, got %d", len(lines))
	}
	e := lines[0]
	if e["type"] != "roll" || e["pid"] != pid("tok") || e["stars_before"] != float64(3) {
		t.Fatalf("merged event wrong: %v", e)
	}
	if _, err := time.Parse(time.RFC3339, e["ts"].(string)); err != nil {
		t.Fatalf("bad ts: %v", err)
	}
}

func TestEventLoggerDisabledAndNilSafe(t *testing.T) {
	if l := newEventLogger(""); l != nil {
		t.Fatal("empty dir must disable telemetry")
	}
	var l *eventLogger
	l.log("roll", "x", nil) // must not panic
}

func TestEventLoggerDropsWhenFull(t *testing.T) {
	// no writer goroutine draining: fill the channel manually and make sure
	// log() never blocks
	l := &eventLogger{ch: make(chan map[string]any, 1)}
	done := make(chan struct{})
	go func() {
		l.log("a", "p", nil)
		l.log("b", "p", nil) // would block without the drop path
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("log() blocked on a full queue")
	}
}

func TestPidStableAndAnonymous(t *testing.T) {
	first, second := pid("tok"), pid("tok")
	if first != second {
		t.Fatal("pid must be stable")
	}
	if pid("tok") == pid("other") {
		t.Fatal("pids must differ per token")
	}
	if len(pid("tok")) != 12 || pid("tok") == "tok" {
		t.Fatalf("pid shape wrong: %q", pid("tok"))
	}
}
