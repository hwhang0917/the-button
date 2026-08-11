package main

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

//go:embed all:web/dist
var distFS embed.FS

const (
	sessionCookie   = "bt_token"
	leaderboardSize = 20
	minNicknameLen  = 3
	maxNicknameLen  = 16
)

// English letters, digits, and underscore only; mirrored in NicknameModal.vue.
var nicknameRe = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_]{%d,%d}$`, minNicknameLen, maxNicknameLen))

type config struct {
	Port   string
	DBPath string
	IPSalt string
	Quota  int // clicks per IP per hour
}

func loadConfig() config {
	cfg := config{
		Port:   envOr("PORT", "8080"),
		DBPath: envOr("DB_PATH", "./thebutton.db"),
		IPSalt: os.Getenv("IP_SALT"),
		Quota:  5,
	}
	if cfg.IPSalt == "" {
		log.Fatal("IP_SALT is required (used to hash client IPs for the hourly quota)")
	}
	if q := os.Getenv("QUOTA"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 1 {
			log.Fatalf("invalid QUOTA %q", q)
		}
		cfg.Quota = n
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type server struct {
	cfg   config
	store *store
}

func main() {
	cfg := loadConfig()
	st, err := openStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	srv := &server{cfg: cfg, store: st}

	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", srv.handleState)
	mux.HandleFunc("POST /api/click", srv.handleClick)
	mux.HandleFunc("POST /api/nickname", srv.handleNickname)
	mux.HandleFunc("DELETE /api/player", srv.handleDeletePlayer)
	mux.HandleFunc("GET /api/leaderboard", srv.handleLeaderboard)
	mux.HandleFunc("GET /api/cards", srv.handleCards)
	mux.Handle("/", http.FileServerFS(dist))

	log.Printf("the button listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
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
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return token, nil
}

// ipHash hashes the client IP with the server salt; the raw IP is never stored.
func (s *server) ipHash(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	} else {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	sum := sha256.Sum256([]byte(s.cfg.IPSalt + ip))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *server) player(w http.ResponseWriter, r *http.Request) (*player, bool) {
	token, err := sessionToken(w, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session")
		return nil, false
	}
	p, err := s.store.getOrCreatePlayer(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return nil, false
	}
	return p, true
}

type stateResponse struct {
	Stars      int    `json:"stars"`
	BestStars  int    `json:"bestStars"`
	Tier       string `json:"tier"`
	Chance     int    `json:"chance"`
	QuotaLeft  int    `json:"quotaLeft"`
	Quota      int    `json:"quota"`
	Nickname   string `json:"nickname"`
	Win        bool   `json:"win"`
}

func (s *server) stateFor(p *player, quotaLeft int) stateResponse {
	return stateResponse{
		Stars:      p.Stars,
		BestStars:  p.BestStars,
		Tier:       tierFor(p.Stars),
		Chance:     chanceFor(p.Stars, 0),
		QuotaLeft:  quotaLeft,
		Quota:      s.cfg.Quota,
		Nickname:   p.Nickname,
		Win:        p.Stars >= maxStars,
	}
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	used, err := s.store.quotaUsed(s.ipHash(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, s.stateFor(p, max(0, s.cfg.Quota-used)))
}

func (s *server) handleClick(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	if p.Stars >= maxStars {
		writeError(w, http.StatusConflict, "already_won")
		return
	}
	var body struct {
		Risk int `json:"risk"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&body) // empty body = normal click
	}
	body.Risk = min(max(body.Risk, 0), maxRisk)
	quotaLeft, err := s.store.consumeQuota(s.ipHash(r), s.cfg.Quota)
	if errors.Is(err, errQuotaExceeded) {
		writeError(w, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	res := resolveClick(p.Stars, body.Risk)
	// first time above the lifetime-best tier: refund clicks equal to the new
	// tier's rank (gating on best stops farming the free bronze click)
	bonus := 0
	if res.TierUp && tierRank(res.Tier) > tierRank(tierFor(p.BestStars)) {
		bonus = tierRank(res.Tier)
		if err := s.store.grantQuota(s.ipHash(r), bonus); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
	}
	if err := s.store.savePlayerStars(p.Token, res.Stars); err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if res.Card != nil {
		if err := s.store.addCard(p.Token, res.Card.Tier, res.Card.Rarity); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
	}
	writeJSON(w, http.StatusOK, struct {
		clickResult
		Chance      int `json:"chance"`
		QuotaLeft   int `json:"quotaLeft"`
		BonusClicks int `json:"bonusClicks"`
	}{res, chanceFor(res.Stars, 0), quotaLeft + bonus, bonus})
}

func (s *server) handleNickname(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json")
		return
	}
	name := strings.TrimSpace(body.Name)
	if !nicknameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_nickname")
		return
	}
	if err := s.store.setNickname(p.Token, name); err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nickname": name})
}

func (s *server) handleDeletePlayer(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.store.deletePlayer(c.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.leaderboard(leaderboardSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *server) handleCards(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	cards, err := s.store.getCards(p.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, cards)
}
