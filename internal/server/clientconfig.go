package server

import (
	"net/http"

	"github.com/hwhang0917/the-button/internal/game"
)

// clientConfig is the tunable surface the frontend needs in order to render
// prices, odds and card effects. It exists so those numbers live in exactly one
// place: before this endpoint the client hand-mirrored them, and any config.yml
// edit would have left the UI quoting stale values.
//
// Names and artwork stay client-side — see the note in config.yml.example.
type clientConfig struct {
	game.Rules // its json tags hide Quota, RNG and the shield refund
	Nickname   struct {
		MinLen int `json:"minLen"`
		MaxLen int `json:"maxLen"`
	} `json:"nickname"`
}

func (s *Server) publicConfig() clientConfig {
	var c clientConfig
	c.Rules = s.cfg.Rules
	c.Nickname.MinLen = s.cfg.NicknameMin
	c.Nickname.MaxLen = s.cfg.NicknameMax
	return c
}

// handleConfig serves the pre-marshalled snapshot. It cannot change without a
// restart, but it is deliberately not cached for long: a redeploy that retunes
// the economy must reach clients on their next load.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheNever)
	w.Write(s.clientConfig)
}
