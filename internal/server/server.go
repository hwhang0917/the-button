// Package server is the HTTP layer: routing, middleware, and the handlers that
// turn requests into store writes and game.Rules calls.
package server

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hwhang0917/the-button/internal/config"
	"github.com/hwhang0917/the-button/internal/events"
	"github.com/hwhang0917/the-button/internal/store"
)

const (
	sessionCookie = "bt_token"
	sessionMaxAge = 365 * 24 * 60 * 60
)

// no I/O/0/1 lookalikes; exactly 32 chars so a byte &31 picks without modulo bias
const linkAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type linkCode struct {
	token   string
	expires time.Time
}

type Server struct {
	cfg    config.Config
	store  *store.Store
	events *events.Logger

	// Hangul syllables, English letters, digits and underscore, sized from
	// config; the client rebuilds the same pattern from /api/config.
	nicknameRe *regexp.Regexp

	// the published tunables never change without a restart, so they are
	// marshalled once at boot and served as a fixed byte slice
	clientConfig []byte

	// ponytail: in-memory link codes — lost on restart, single process only
	mu          sync.Mutex
	linkCodes   map[string]linkCode
	claimFails  int
	claimWindow time.Time
}

func New(cfg config.Config, st *store.Store, log *events.Logger) (*Server, error) {
	s := &Server{
		cfg:        cfg,
		store:      st,
		events:     log,
		nicknameRe: regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_가-힣]{%d,%d}$`, cfg.NicknameMin, cfg.NicknameMax)),
		linkCodes:  map[string]linkCode{},
	}
	body, err := json.Marshal(s.publicConfig())
	if err != nil {
		return nil, fmt.Errorf("marshal client config: %w", err)
	}
	s.clientConfig = body
	return s, nil
}

// Handler wires the API onto the embedded frontend.
func (s *Server) Handler(dist fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("POST /api/click", s.handleClick)
	mux.HandleFunc("POST /api/nickname", s.handleNickname)
	mux.HandleFunc("DELETE /api/player", s.handleDeletePlayer)
	mux.HandleFunc("POST /api/sell", s.handleSell)
	mux.HandleFunc("POST /api/prestige", s.handlePrestige)
	mux.HandleFunc("POST /api/lottery", s.handleLottery)
	mux.HandleFunc("POST /api/pack", s.handlePack)
	mux.HandleFunc("POST /api/refill", s.handleRefill)
	mux.HandleFunc("POST /api/talisman", s.handleTalisman)
	mux.HandleFunc("POST /api/talisman/cancel", s.handleTalismanCancel)
	mux.HandleFunc("POST /api/fuse", s.handleFuse)
	mux.HandleFunc("POST /api/defuse", s.handleDefuse)
	mux.HandleFunc("POST /api/card/sell", s.handleSellCard)
	mux.HandleFunc("POST /api/buy", s.handleBuy)
	mux.HandleFunc("POST /api/link/new", s.handleLinkNew)
	mux.HandleFunc("POST /api/link/claim", s.handleLinkClaim)
	mux.HandleFunc("GET /api/leaderboard", s.handleLeaderboard)
	mux.HandleFunc("GET /api/cards", s.handleCards)
	if s.cfg.ShowDocs {
		mux.HandleFunc("GET /api/docs", handleDocs)
		mux.HandleFunc("GET /api/docs/openapi.yml", handleDocsSpec)
	}
	mux.Handle("/", CacheHeaders(GzipText(fileServerWith404(dist))))
	return mux
}

// fileServerWith404 serves dist/404.html for paths missing from the build, so
// junk URLs get a styled page instead of the stdlib's plain-text 404.
func fileServerWith404(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err != nil {
			body, err := fs.ReadFile(dist, "404.html")
			// unknown /api/ paths fall through to this catch-all too, and an
			// API client should get a plain 404, not a styled HTML page
			if err != nil || strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			w.Write(body)
			return
		}
		files.ServeHTTP(w, r)
	})
}

const (
	cacheForever = "public, max-age=31536000, immutable" // vite-fingerprinted bundles
	cacheDaily   = "public, max-age=86400"               // media that only changes with a release
	cacheNever   = "no-cache"                            // HTML shell must pick up new bundle names
)

// CacheHeaders makes browsers cache the embedded static assets; without it the
// embed.FS has no modtimes, so nothing was cacheable and every visit
// re-downloaded.
func CacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			w.Header().Set("Cache-Control", cacheForever)
		case strings.ContainsRune(r.URL.Path[1:], '.'):
			w.Header().Set("Cache-Control", cacheDaily)
		default:
			w.Header().Set("Cache-Control", cacheNever)
		}
		next.ServeHTTP(w, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w gzipWriter) WriteHeader(code int) {
	// ServeContent sets the uncompressed length; ours differs, so drop it
	w.Header().Del("Content-Length")
	// the file server's error path strips Content-Encoding (fs.go serveError,
	// Go 1.23+), but the body still runs through gz — re-assert it or 404s
	// render as raw gzip bytes
	w.Header().Set("Content-Encoding", "gzip")
	w.ResponseWriter.WriteHeader(code)
}

func (w gzipWriter) Write(b []byte) (int, error) { return w.gz.Write(b) }

// GzipText compresses the text assets in flight — the JS bundle is the last big
// transfer after the media went lossy. Media types are already compressed.
func GzipText(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ext := path.Ext(r.URL.Path)
		textAsset := ext == ".js" || ext == ".css" || ext == ".html" || ext == ".svg" ||
			ext == ".json" || !strings.ContainsRune(r.URL.Path[1:], '.')
		if !textAsset || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(gzipWriter{w, gz}, r)
	})
}

func setTokenCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// sessionToken returns the player token from the cookie, minting one if absent.
func sessionToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie(sessionCookie); err == nil && len(c.Value) == 64 {
		return c.Value, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	setTokenCookie(w, token, sessionMaxAge)
	return token, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) player(w http.ResponseWriter, r *http.Request) (*store.Player, bool) {
	token, err := sessionToken(w, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session")
		return nil, false
	}
	p, err := s.store.GetOrCreatePlayer(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return nil, false
	}
	return p, true
}

// namedPlayer is player() plus the signup gate: gameplay mutations are
// rejected until the player picks a nickname (or links a device), so the
// client's welcome screen cannot be bypassed by calling the API directly.
func (s *Server) namedPlayer(w http.ResponseWriter, r *http.Request) (*store.Player, bool) {
	p, ok := s.player(w, r)
	if !ok {
		return nil, false
	}
	if p.Nickname == "" {
		writeError(w, http.StatusForbidden, "nickname_required")
		return nil, false
	}
	return p, true
}
