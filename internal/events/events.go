package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"maps"
	"os"
	"path/filepath"
	"time"
)

// Logger appends anonymous gameplay events as NDJSON, one file per day,
// for offline analysis with DuckDB (read_json('events/*.ndjson')). Telemetry
// is best-effort by design: a nil logger no-ops and a full queue drops events
// rather than ever stalling gameplay.
const eventQueueSize = 256

type Logger struct {
	ch chan map[string]any
}

// New returns nil (telemetry off) when dir is empty.
func New(dir string) *Logger {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("events: disabled, cannot create %s: %v", dir, err)
		return nil
	}
	l := &Logger{ch: make(chan map[string]any, eventQueueSize)}
	go l.write(dir)
	return l
}

func (l *Logger) write(dir string) {
	var f *os.File
	var day string
	for e := range l.ch {
		d := time.Now().Format("2006-01-02")
		if f == nil || d != day {
			f.Close()
			var err error
			f, err = os.OpenFile(filepath.Join(dir, d+".ndjson"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				log.Printf("events: open: %v", err)
				continue
			}
			day = d
		}
		line, err := json.Marshal(e)
		if err != nil {
			continue
		}
		f.Write(append(line, '\n'))
	}
}

// log queues one event; safe on a nil logger, never blocks.
func (l *Logger) Log(typ, pid string, fields map[string]any) {
	if l == nil {
		return
	}
	e := map[string]any{"ts": time.Now().Format(time.RFC3339), "type": typ, "pid": pid}
	maps.Copy(e, fields)
	select {
	case l.ch <- e:
	default: // queue full: drop telemetry, never stall gameplay
	}
}

// pid is the pseudonymous analytics id: a one-way hash prefix of the session
// token. Links a player's events without storing anything reversible.
func PID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:12]
}
