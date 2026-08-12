// Command thebutton serves the game and its embedded frontend from a single
// binary. Everything interesting lives in internal/: config resolves settings,
// game holds the rules, store owns SQLite, server is the HTTP layer.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/hwhang0917/the-button/internal/config"
	"github.com/hwhang0917/the-button/internal/events"
	"github.com/hwhang0917/the-button/internal/server"
	"github.com/hwhang0917/the-button/internal/store"
)

// The embed pattern is why main stays at the repo root: //go:embed cannot
// reach above its own directory, so a cmd/ layout would break the single binary.
//
//go:embed all:web/dist
var distFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.DevMode {
		log.Println("DEV_MODE: every roll succeeds — do not run in production")
	}

	st, err := store.Open(cfg.DBPath, cfg.Rules.RetiredShieldRefund)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	srv, err := server.New(cfg, st, events.New(cfg.EventsDir))
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("the button listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, srv.Handler(dist)))
}
