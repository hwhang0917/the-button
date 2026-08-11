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
	minNicknameLen  = 2
	maxNicknameLen  = 16
	linkTTL         = 10 * time.Minute
	linkCodeLen     = 8
	// failed claim attempts tolerated per minute before the endpoint locks
	claimFailLimit = 20
)

// no I/O/0/1 lookalikes; exactly 32 chars so a byte &31 picks without modulo bias
const linkAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Hangul syllables, English letters, digits, and underscore; mirrored in NicknameModal.vue.
var nicknameRe = regexp.MustCompile(fmt.Sprintf(`^[A-Za-z0-9_가-힣]{%d,%d}$`, minNicknameLen, maxNicknameLen))

type config struct {
	Port      string
	DBPath    string
	EventsDir string // anonymous NDJSON gameplay events; empty = telemetry off
	Quota     int    // clicks per player per hour
}

func loadConfig() config {
	cfg := config{
		Port:      envOr("PORT", "8080"),
		DBPath:    envOr("DB_PATH", "./thebutton.db"),
		EventsDir: os.Getenv("EVENTS_DIR"),
		Quota:     5,
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
	cfg    config
	store  *store
	events *eventLogger

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
	srv := &server{cfg: cfg, store: st, events: newEventLogger(cfg.EventsDir), linkCodes: map[string]linkCode{}}

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
	mux.HandleFunc("POST /api/prestige", srv.handlePrestige)
	mux.HandleFunc("POST /api/lottery", srv.handleLottery)
	mux.HandleFunc("POST /api/talisman", srv.handleTalisman)
	mux.HandleFunc("POST /api/fuse", srv.handleFuse)
	mux.HandleFunc("POST /api/buy", srv.handleBuy)
	mux.HandleFunc("POST /api/link/new", srv.handleLinkNew)
	mux.HandleFunc("POST /api/link/claim", srv.handleLinkClaim)
	mux.HandleFunc("GET /api/leaderboard", srv.handleLeaderboard)
	mux.HandleFunc("GET /api/cards", srv.handleCards)
	mux.Handle("/", cacheHeaders(http.FileServerFS(dist)))

	log.Printf("the button listening on :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

const (
	cacheForever = "public, max-age=31536000, immutable" // vite-fingerprinted bundles
	cacheDaily   = "public, max-age=86400"               // media that only changes with a release
	cacheNever   = "no-cache"                            // HTML shell must pick up new bundle names
)

// cacheHeaders makes browsers cache the embedded static assets; without it the
// embed.FS has no modtimes, so nothing was cacheable and every visit re-downloaded.
func cacheHeaders(next http.Handler) http.Handler {
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
	Prestige       int    `json:"prestige"`
	TalismanTier   string `json:"talismanTier"`
	TalismanRarity string `json:"talismanRarity"`
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
		Prestige:       p.Prestige,
		TalismanTier:   p.TalismanTier,
		TalismanRarity: p.TalismanRarity,
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
		s.events.log("quota_empty", pid(p.Token), nil)
		writeError(w, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	sk := skills{
		Charm:     p.CharmLevel,
		Headstart: p.HeadstartLevel,
		Shield:    p.ShieldCharges > 0,
	}
	// the armed talisman only acts while the streak is inside its tier
	if p.TalismanTier != "" && p.TalismanTier == tierFor(p.Stars) {
		switch p.TalismanRarity {
		case "common":
			sk.TalBonus = talCommonPct
		case "rare":
			sk.TalBonus = talRarePct
		case "holo":
			sk.TalShield = true
		case "prismatic":
			sk.TalDouble = true
		}
	}
	res := resolveClick(p.Stars, body.Risk, sk)
	if res.TalismanUsed {
		if err := s.store.clearTalisman(p.Token); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
		s.events.log("talisman_proc", pid(p.Token), map[string]any{"tier": p.TalismanTier, "rarity": p.TalismanRarity})
		p.TalismanTier, p.TalismanRarity = "", ""
	}
	s.events.log("roll", pid(p.Token), map[string]any{
		"stars_before": p.Stars,
		"stars_after":  res.Stars,
		"risk":         body.Risk,
		"chance":       effChanceFor(p.Stars, body.Risk, p.CharmLevel),
		"success":      res.Success,
		"shield_used":  res.ShieldUsed,
		"tier_up":      res.TierUp,
		"win":          res.Win,
		"card":         res.Card != nil,
		"charm":        p.CharmLevel,
		"headstart":    p.HeadstartLevel,
		"prestige":     p.Prestige,
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
		Chance         int    `json:"chance"`
		QuotaLeft      int    `json:"quotaLeft"`
		BonusClicks    int    `json:"bonusClicks"`
		ShieldCharges  int    `json:"shieldCharges"`
		TalismanTier   string `json:"talismanTier"`
		TalismanRarity string `json:"talismanRarity"`
	}{res, chanceFor(res.Stars, 0), quotaLeft + bonus, bonus, shieldCharges, p.TalismanTier, p.TalismanRarity})
}

// handleSell converts the whole streak to coins and drops stars to the
// head-start floor. Consumes no quota; also the replay path after a win.
func (s *server) handleSell(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	if p.Stars >= maxStars {
		// a maxed streak must go through prestige, not the plain sell
		writeError(w, http.StatusConflict, "prestige_instead")
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
	s.events.log("sell", pid(p.Token), map[string]any{"stars": p.Stars, "gain": gain})
	writeJSON(w, http.StatusOK, map[string]any{
		"coins":  p.Coins + gain,
		"gained": gain,
		"stars":  floor,
		"tier":   tierFor(floor),
		"chance": chanceFor(floor, 0),
	})
}

// handlePrestige converts a maxed streak into a big point payout and a star-tier
// promotion (common → rare → holo → prismatic; repeats at the cap still pay).
func (s *server) handlePrestige(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	if p.Stars < maxStars {
		writeError(w, http.StatusConflict, "not_won")
		return
	}
	reward := prestigeRewardFor(p.Prestige)
	floor := min(p.HeadstartLevel, maxStars)
	done, err := s.store.prestigeStreak(p.Token, reward, floor, p.Stars)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !done {
		writeError(w, http.StatusConflict, "retry")
		return
	}
	s.events.log("prestige", pid(p.Token), map[string]any{"level": p.Prestige + 1, "reward": reward})
	writeJSON(w, http.StatusOK, map[string]any{
		"coins":    p.Coins + reward,
		"gained":   reward,
		"prestige": p.Prestige + 1,
		"stars":    floor,
		"tier":     tierFor(floor),
		"chance":   chanceFor(floor, 0),
	})
}

// handleLottery sells one scratch ticket: roll first, settle atomically, and
// let the client scratch the pre-decided result off at its leisure.
func (s *server) handleLottery(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	prize := rollLottery()
	bought, err := s.store.playLottery(p.Token, lotteryPrice, prize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !bought {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	s.events.log("lottery", pid(p.Token), map[string]any{"prize": prize})
	writeJSON(w, http.StatusOK, map[string]int{
		"prize": prize,
		"coins": p.Coins - lotteryPrice + prize,
	})
}

// handleTalisman consumes one copy of a card and arms it as the single
// talisman slot; the effect fires later, on a click inside the card's tier.
func (s *server) handleTalisman(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	var body struct {
		Tier   string `json:"tier"`
		Rarity string `json:"rarity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json")
		return
	}
	if !validTier(body.Tier) || !validRarity(body.Rarity) {
		writeError(w, http.StatusBadRequest, "bad_card")
		return
	}
	armed, err := s.store.armTalisman(p.Token, body.Tier, body.Rarity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !armed {
		writeError(w, http.StatusConflict, "cannot_arm")
		return
	}
	s.events.log("talisman_arm", pid(p.Token), map[string]any{"tier": body.Tier, "rarity": body.Rarity})
	writeJSON(w, http.StatusOK, map[string]string{"talismanTier": body.Tier, "talismanRarity": body.Rarity})
}

// handleFuse burns 3 copies of a card into 1 of the next rarity, same tier.
func (s *server) handleFuse(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	var body struct {
		Tier   string `json:"tier"`
		Rarity string `json:"rarity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_json")
		return
	}
	next, ok2 := nextRarity(body.Rarity)
	if !ok2 || !validTier(body.Tier) {
		writeError(w, http.StatusBadRequest, "cannot_fuse")
		return
	}
	fused, err := s.store.fuseCards(p.Token, body.Tier, body.Rarity, next)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !fused {
		writeError(w, http.StatusConflict, "cannot_fuse")
		return
	}
	s.events.log("fuse", pid(p.Token), map[string]any{"tier": body.Tier, "from": body.Rarity, "to": next})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
	s.events.log("buy", pid(p.Token), map[string]any{"skill": body.Skill, "price": price, "level": *cur})
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
