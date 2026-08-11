package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed all:web/dist
var distFS embed.FS

const (
	sessionCookie   = "bt_token"
	sessionMaxAge   = 365 * 24 * 60 * 60
	leaderboardSize = 20
	minNicknameLen  = 3
	maxNicknameLen  = 16
	linkTTL         = 10 * time.Minute
	linkCodeLen     = 8
	// failed claim attempts tolerated per minute before the endpoint locks
	claimFailLimit = 20
)

// no I/O/0/1 lookalikes; exactly 32 chars so a byte &31 picks without modulo bias
const linkAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// English letters, digits, and underscore only; mirrored in NicknameModal.vue.
var nicknameRe = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_]{%d,%d}$`, minNicknameLen, maxNicknameLen))

type config struct {
	Port   string
	DBPath string
	Quota  int // clicks per player per hour
}

func loadConfig() config {
	cfg := config{
		Port:   envOr("PORT", "8080"),
		DBPath: envOr("DB_PATH", "./thebutton.db"),
		Quota:  5,
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

type linkCode struct {
	token   string
	expires time.Time
}

type server struct {
	cfg   config
	store *store

	// ponytail: in-memory link codes — lost on restart, single process only
	mu          sync.Mutex
	linkCodes   map[string]linkCode
	claimFails  int
	claimWindow time.Time
}

func main() {
	cfg := loadConfig()
	st, err := openStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	srv := &server{cfg: cfg, store: st, linkCodes: map[string]linkCode{}}

	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", srv.handleState)
	mux.HandleFunc("POST /api/click", srv.handleClick)
	mux.HandleFunc("POST /api/nickname", srv.handleNickname)
	mux.HandleFunc("DELETE /api/player", srv.handleDeletePlayer)
	mux.HandleFunc("POST /api/sell", srv.handleSell)
	mux.HandleFunc("POST /api/buy", srv.handleBuy)
	mux.HandleFunc("POST /api/link/new", srv.handleLinkNew)
	mux.HandleFunc("POST /api/link/claim", srv.handleLinkClaim)
	mux.HandleFunc("GET /api/leaderboard", srv.handleLeaderboard)
	mux.HandleFunc("GET /api/cards", srv.handleCards)
	mux.Handle("/", http.FileServerFS(dist))

	log.Printf("the button listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
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
	Stars          int    `json:"stars"`
	BestStars      int    `json:"bestStars"`
	Tier           string `json:"tier"`
	Chance         int    `json:"chance"`
	QuotaLeft      int    `json:"quotaLeft"`
	Quota          int    `json:"quota"`
	Nickname       string `json:"nickname"`
	Win            bool   `json:"win"`
	Coins          int    `json:"coins"`
	ShieldCharges  int    `json:"shieldCharges"`
	CharmLevel     int    `json:"charmLevel"`
	HeadstartLevel int    `json:"headstartLevel"`
}

func (s *server) stateFor(p *player, quotaLeft int) stateResponse {
	return stateResponse{
		Stars:          p.Stars,
		BestStars:      p.BestStars,
		Tier:           tierFor(p.Stars),
		Chance:         chanceFor(p.Stars, 0),
		QuotaLeft:      quotaLeft,
		Quota:          s.cfg.Quota,
		Nickname:       p.Nickname,
		Win:            p.Stars >= maxStars,
		Coins:          p.Coins,
		ShieldCharges:  p.ShieldCharges,
		CharmLevel:     p.CharmLevel,
		HeadstartLevel: p.HeadstartLevel,
	}
}

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	used, err := s.store.quotaUsed(p.Token)
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
	quotaLeft, err := s.store.consumeQuota(p.Token, s.cfg.Quota)
	if errors.Is(err, errQuotaExceeded) {
		writeError(w, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	res := resolveClick(p.Stars, body.Risk, skills{
		Charm:     p.CharmLevel,
		Headstart: p.HeadstartLevel,
		Shield:    p.ShieldCharges > 0,
	})
	shieldCharges := p.ShieldCharges
	if res.ShieldUsed {
		burned, err := s.store.consumeShield(p.Token)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
		if burned {
			shieldCharges--
		} else {
			// a racing click burned the last charge first: real reset
			res.ShieldUsed = false
			res.Stars = min(p.HeadstartLevel, p.Stars)
			res.Tier = tierFor(res.Stars)
		}
	}
	// first time above the lifetime-best tier: refund clicks equal to the new
	// tier's rank (gating on best stops farming the free bronze click)
	bonus := 0
	if res.TierUp && tierRank(res.Tier) > tierRank(tierFor(p.BestStars)) {
		bonus = tierRank(res.Tier)
		if err := s.store.grantQuota(p.Token, bonus); err != nil {
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
		Chance        int `json:"chance"`
		QuotaLeft     int `json:"quotaLeft"`
		BonusClicks   int `json:"bonusClicks"`
		ShieldCharges int `json:"shieldCharges"`
	}{res, chanceFor(res.Stars, 0), quotaLeft + bonus, bonus, shieldCharges})
}

// handleSell converts the whole streak to coins and drops stars to the
// head-start floor. Consumes no quota; also the replay path after a win.
func (s *server) handleSell(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	gain := streakValue(p.Stars, p.HeadstartLevel)
	if gain <= 0 {
		writeError(w, http.StatusConflict, "nothing_to_sell")
		return
	}
	floor := min(p.HeadstartLevel, p.Stars)
	sold, err := s.store.sellStreak(p.Token, gain, floor, p.Stars)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !sold {
		writeError(w, http.StatusConflict, "retry")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"coins":  p.Coins + gain,
		"gained": gain,
		"stars":  floor,
		"tier":   tierFor(floor),
		"chance": chanceFor(floor, 0),
	})
}

func (s *server) handleBuy(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	var body struct {
		Skill string `json:"skill"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json")
		return
	}
	// whitelist: skill name -> column + the current value used as level and pin
	var col string
	var cur *int
	switch body.Skill {
	case "shield":
		col, cur = "shield_charges", &p.ShieldCharges
	case "charm":
		col, cur = "charm_level", &p.CharmLevel
	case "headstart":
		col, cur = "headstart_level", &p.HeadstartLevel
	default:
		writeError(w, http.StatusBadRequest, "bad_skill")
		return
	}
	price, ok2 := priceFor(body.Skill, *cur)
	if !ok2 {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	bought, err := s.store.buySkill(p.Token, col, price, *cur)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !bought {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	*cur++
	writeJSON(w, http.StatusOK, map[string]int{
		"coins":          p.Coins - price,
		"shieldCharges":  p.ShieldCharges,
		"charmLevel":     p.CharmLevel,
		"headstartLevel": p.HeadstartLevel,
	})
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
	if err := s.store.setNickname(p.Token, name); errors.Is(err, errNicknameTaken) {
		writeError(w, http.StatusConflict, "name_taken")
		return
	} else if err != nil {
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
	setTokenCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// handleLinkNew mints a one-time code another device can claim to log into
// this account. One live code per player; expired entries are swept here.
func (s *server) handleLinkNew(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	buf := make([]byte, linkCodeLen)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "rand")
		return
	}
	code := make([]byte, linkCodeLen)
	for i, b := range buf {
		code[i] = linkAlphabet[b&31]
	}
	now := time.Now()
	s.mu.Lock()
	for c, lc := range s.linkCodes {
		if now.After(lc.expires) || lc.token == p.Token {
			delete(s.linkCodes, c)
		}
	}
	s.linkCodes[string(code)] = linkCode{token: p.Token, expires: now.Add(linkTTL)}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"code": string(code)})
}

// handleLinkClaim swaps this device's session cookie for the account behind a
// valid code. The code is consumed on success.
func (s *server) handleLinkClaim(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(body.Code))

	s.mu.Lock()
	now := time.Now()
	if now.Sub(s.claimWindow) > time.Minute {
		s.claimWindow, s.claimFails = now, 0
	}
	if s.claimFails >= claimFailLimit {
		s.mu.Unlock()
		writeError(w, http.StatusTooManyRequests, "slow_down")
		return
	}
	lc, found := s.linkCodes[code]
	if !found || now.After(lc.expires) {
		// ponytail: global fail counter; per-IP buckets if lockouts ever matter
		s.claimFails++
		s.mu.Unlock()
		writeError(w, http.StatusNotFound, "bad_code")
		return
	}
	delete(s.linkCodes, code)
	s.mu.Unlock()

	setTokenCookie(w, lc.token, sessionMaxAge)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
