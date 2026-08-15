package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hwhang0917/the-button/internal/events"
	"github.com/hwhang0917/the-button/internal/game"
	"github.com/hwhang0917/the-button/internal/store"
)

type stateResponse struct {
	Stars          int    `json:"stars"`
	BestStars      int    `json:"bestStars"`
	Tier           string `json:"tier"`
	Chance         int    `json:"chance"`
	MaxStars       int    `json:"maxStars"`
	QuotaLeft      int    `json:"quotaLeft"`
	Quota          int    `json:"quota"`
	Nickname       string `json:"nickname"`
	Win            bool   `json:"win"`
	Coins          int    `json:"coins"`
	CharmLevel     int    `json:"charmLevel"`
	HeadstartLevel int    `json:"headstartLevel"`
	StaminaLevel   int    `json:"staminaLevel"`
	MagnetLevel    int    `json:"magnetLevel"`
	GoldenLevel    int    `json:"goldenLevel"`
	Prestige       int    `json:"prestige"`
	TalismanTier   string `json:"talismanTier"`
	TalismanRarity string `json:"talismanRarity"`
	RefillsLeft    int    `json:"refillsLeft"`
	RefillIn       int    `json:"refillIn"` // seconds until the quota bucket rolls over
	DevMode        bool   `json:"devMode"`
}

func (s *Server) stateFor(p *store.Player, quotaLeft int) stateResponse {
	rules := s.cfg.Rules
	cap := rules.MaxStarsFor(p.Prestige)
	now := time.Now()
	return stateResponse{
		Stars:          p.Stars,
		BestStars:      p.BestStars,
		Tier:           rules.TierFor(p.Stars, cap),
		Chance:         rules.ChanceFor(p.Stars, 0, cap),
		MaxStars:       cap,
		QuotaLeft:      quotaLeft,
		Quota:          rules.QuotaFor(p.StaminaLevel),
		Nickname:       p.Nickname,
		Win:            p.Stars >= cap,
		Coins:          p.Coins,
		CharmLevel:     p.CharmLevel,
		HeadstartLevel: p.HeadstartLevel,
		StaminaLevel:   p.StaminaLevel,
		MagnetLevel:    p.MagnetLevel,
		GoldenLevel:    p.GoldenLevel,
		Prestige:       p.Prestige,
		TalismanTier:   p.TalismanTier,
		TalismanRarity: p.TalismanRarity,
		RefillsLeft:    max(0, rules.RefillsFor(p.Prestige)-refillsUsedToday(p, now)),
		// the quota bucket is keyed by the SERVER's clock hour (store.bucketKey), so
		// the client must count down to this instead of its own top-of-hour guess
		RefillIn:       3600 - now.Minute()*60 - now.Second(),
		DevMode:        s.cfg.DevMode,
	}
}

// refillsUsedToday reads the refill tally, which only counts if it was pinned
// to today — an older refill_day means the allowance has rolled over fresh.
func refillsUsedToday(p *store.Player, now time.Time) int {
	if p.RefillDay == now.Format("2006-01-02") {
		return p.RefillCount
	}
	return 0
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	used, err := s.store.QuotaUsed(p.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, s.stateFor(p, max(0, s.cfg.Rules.QuotaFor(p.StaminaLevel)-used)))
}

func (s *Server) handleClick(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	cap := rules.MaxStarsFor(p.Prestige)
	if p.Stars >= cap {
		writeError(w, http.StatusConflict, "already_won")
		return
	}
	var body struct {
		Risk int `json:"risk"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&body) // empty body = normal click
	}
	body.Risk = min(max(body.Risk, 0), rules.MaxRisk)
	quotaLeft, err := s.store.ConsumeQuota(p.Token, rules.QuotaFor(p.StaminaLevel))
	if errors.Is(err, store.ErrQuotaExceeded) {
		s.events.Log("quota_empty", events.PID(p.Token), nil)
		writeError(w, http.StatusTooManyRequests, "quota_exceeded")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	res := rules.Resolve(game.Click{
		Stars:     p.Stars,
		Risk:      body.Risk,
		Cap:       cap,
		Best:      p.BestStars,
		Charm:     p.CharmLevel,
		Headstart: p.HeadstartLevel,
		Magnet:    p.MagnetLevel,
		Golden:    p.GoldenLevel,
		Card:      rules.EffectFor(p.TalismanTier, p.TalismanRarity), // inert when nothing is armed
	})
	if res.TalismanUsed {
		if err := s.store.ClearTalisman(p.Token); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
		s.events.Log("talisman_proc", events.PID(p.Token), map[string]any{"tier": p.TalismanTier, "rarity": p.TalismanRarity})
		p.TalismanTier, p.TalismanRarity = "", ""
	}
	s.events.Log("roll", events.PID(p.Token), map[string]any{
		"stars_before": p.Stars,
		"stars_after":  res.Stars,
		"risk":         body.Risk,
		"chance":       rules.EffChanceFor(p.Stars, body.Risk, p.CharmLevel, cap),
		"success":      res.Success,
		"tier_up":      res.TierUp,
		"win":          res.Win,
		"jackpot":      res.Jackpot,
		"card":         res.Card != nil,
		"charm":        p.CharmLevel,
		"headstart":    p.HeadstartLevel,
		"prestige":     p.Prestige,
	})
	// first time above the lifetime-best tier: refund clicks equal to the new
	// tier's rank (gating on best stops farming the free bronze click)
	bonus := 0
	if res.TierUp && rules.TierRank(res.Tier) > rules.TierRank(rules.TierFor(p.BestStars, cap)) {
		bonus = rules.TierRank(res.Tier)
	}
	// a refunding card hands this click straight back
	if res.Refund {
		bonus++
	}
	if bonus > 0 {
		if err := s.store.GrantQuota(p.Token, bonus); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
	}
	if err := s.store.SavePlayerStars(p.Token, res.Stars, res.Jackpot); err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if res.Card != nil {
		if err := s.store.AddCard(p.Token, res.Card.Tier, res.Card.Rarity); err != nil {
			writeError(w, http.StatusInternalServerError, "db")
			return
		}
	}
	writeJSON(w, http.StatusOK, struct {
		game.Result
		Chance         int    `json:"chance"`
		QuotaLeft      int    `json:"quotaLeft"`
		BonusClicks    int    `json:"bonusClicks"`
		TalismanTier   string `json:"talismanTier"`
		TalismanRarity string `json:"talismanRarity"`
		Coins          int    `json:"coins"`
	}{res, rules.ChanceFor(res.Stars, 0, cap), quotaLeft + bonus, bonus, p.TalismanTier, p.TalismanRarity, p.Coins + res.Jackpot})
}

// handleSell converts the whole streak to coins and drops stars to the
// head-start floor. Consumes no quota; also the replay path after a win.
func (s *Server) handleSell(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	cap := rules.MaxStarsFor(p.Prestige)
	if p.Stars >= cap {
		// a maxed streak must go through prestige, not the plain sell
		writeError(w, http.StatusConflict, "prestige_instead")
		return
	}
	gain := game.StreakValue(p.Stars, p.HeadstartLevel)
	if gain <= 0 {
		writeError(w, http.StatusConflict, "nothing_to_sell")
		return
	}
	floor := min(p.HeadstartLevel, p.Stars)
	sold, err := s.store.SellStreak(p.Token, gain, floor, p.Stars)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !sold {
		writeError(w, http.StatusConflict, "retry")
		return
	}
	s.events.Log("sell", events.PID(p.Token), map[string]any{"stars": p.Stars, "gain": gain})
	writeJSON(w, http.StatusOK, map[string]any{
		"coins":  p.Coins + gain,
		"gained": gain,
		"stars":  floor,
		"tier":   rules.TierFor(floor, cap),
		"chance": rules.ChanceFor(floor, 0, cap),
	})
}

// handlePrestige converts a maxed streak into a big point payout and a star-tier
// promotion (common → rare → holo → prismatic; repeats at the cap still pay).
func (s *Server) handlePrestige(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	if p.Stars < rules.MaxStarsFor(p.Prestige) {
		writeError(w, http.StatusConflict, "not_won")
		return
	}
	reward := rules.PrestigeRewardFor(p.Prestige)
	floor := min(p.HeadstartLevel, p.Stars)
	done, err := s.store.PrestigeStreak(p.Token, reward, floor, p.Stars)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !done {
		writeError(w, http.StatusConflict, "retry")
		return
	}
	s.events.Log("prestige", events.PID(p.Token), map[string]any{"level": p.Prestige + 1, "reward": reward})
	writeJSON(w, http.StatusOK, map[string]any{
		"coins":    p.Coins + reward,
		"gained":   reward,
		"prestige": p.Prestige + 1,
		"stars":    floor,
		"tier":     rules.TierFor(floor, rules.MaxStarsFor(p.Prestige+1)),
		// the cap the player will roll against after this prestige
		"chance": rules.ChanceFor(floor, 0, rules.MaxStarsFor(p.Prestige+1)),
		// the promotion refilled the hour's clicks
		"quotaLeft": rules.QuotaFor(p.StaminaLevel),
	})
}

// handleLottery sells one scratch ticket: roll first, settle atomically, and
// let the client scratch the pre-decided result off at its leisure.
func (s *Server) handleLottery(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	prize := rules.RollLottery()
	bought, err := s.store.PlayLottery(p.Token, rules.Lottery.Price, prize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !bought {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	s.events.Log("lottery", events.PID(p.Token), map[string]any{"prize": prize})
	writeJSON(w, http.StatusOK, map[string]int{
		"prize": prize,
		"coins": p.Coins - rules.Lottery.Price + prize,
	})
}

// handleTalisman consumes one copy of a card and arms it as the single
// talisman slot; the effect fires on the very next click, whatever tier the
// streak is standing in.
func (s *Server) handleTalisman(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
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
	if !rules.ValidTier(body.Tier) || !rules.ValidRarity(body.Rarity) {
		writeError(w, http.StatusBadRequest, "bad_card")
		return
	}
	armed, err := s.store.ArmTalisman(p.Token, body.Tier, body.Rarity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !armed {
		writeError(w, http.StatusConflict, "cannot_arm")
		return
	}
	s.events.Log("talisman_arm", events.PID(p.Token), map[string]any{"tier": body.Tier, "rarity": body.Rarity})
	writeJSON(w, http.StatusOK, map[string]string{"talismanTier": body.Tier, "talismanRarity": body.Rarity})
}

// handleTalismanCancel disarms the talisman slot and refunds the card copy.
func (s *Server) handleTalismanCancel(w http.ResponseWriter, r *http.Request) {
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	cancelled, err := s.store.CancelTalisman(p.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !cancelled {
		writeError(w, http.StatusConflict, "nothing_armed")
		return
	}
	s.events.Log("talisman_cancel", events.PID(p.Token), map[string]any{"tier": p.TalismanTier, "rarity": p.TalismanRarity})
	writeJSON(w, http.StatusOK, map[string]string{"talismanTier": "", "talismanRarity": ""})
}

// handleDefuse breaks one card into DefuseYield copies of the rarity below —
// lossy on purpose: fusing costs more than defusing returns.
func (s *Server) handleDefuse(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
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
	lower, ok2 := rules.PrevRarity(body.Rarity)
	if !ok2 || !rules.ValidTier(body.Tier) {
		writeError(w, http.StatusBadRequest, "cannot_defuse")
		return
	}
	defused, err := s.store.DefuseCard(p.Token, body.Tier, body.Rarity, lower, rules.DefuseYield)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !defused {
		writeError(w, http.StatusConflict, "cannot_defuse")
		return
	}
	s.events.Log("defuse", events.PID(p.Token), map[string]any{"tier": body.Tier, "from": body.Rarity, "to": lower})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleFuse burns FuseCost copies of a card into 1 of the next rarity, same tier.
func (s *Server) handleFuse(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
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
	next, ok2 := rules.NextRarity(body.Rarity)
	if !ok2 || !rules.ValidTier(body.Tier) {
		writeError(w, http.StatusBadRequest, "cannot_fuse")
		return
	}
	fused, err := s.store.FuseCards(p.Token, body.Tier, body.Rarity, next, rules.FuseCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !fused {
		writeError(w, http.StatusConflict, "cannot_fuse")
		return
	}
	s.events.Log("fuse", events.PID(p.Token), map[string]any{"tier": body.Tier, "from": body.Rarity, "to": next})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSellCard trades one copy of a collected card for its rarity's coin price.
func (s *Server) handleSellCard(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
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
	gain, ok2 := rules.SellValueFor(body.Rarity)
	if !ok2 || !rules.ValidTier(body.Tier) {
		writeError(w, http.StatusBadRequest, "cannot_sell")
		return
	}
	sold, err := s.store.SellCard(p.Token, body.Tier, body.Rarity, gain)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !sold {
		writeError(w, http.StatusConflict, "cannot_sell")
		return
	}
	s.events.Log("card_sell", events.PID(p.Token), map[string]any{"tier": body.Tier, "rarity": body.Rarity, "gain": gain})
	writeJSON(w, http.StatusOK, map[string]int{"coins": p.Coins + gain, "gained": gain})
}

// handlePack sells one card pack: 1-3 cards, any tier, higher tiers rarer.
// Packs are the only source of cards.
func (s *Server) handlePack(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	drawn := rules.RollPackCards()
	bought, err := s.store.BuyPack(p.Token, rules.Pack.Price, drawn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !bought {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	s.events.Log("pack", events.PID(p.Token), map[string]any{"cards": drawn})
	writeJSON(w, http.StatusOK, map[string]any{
		"cards": drawn,
		"coins": p.Coins - rules.Pack.Price,
	})
}

// handleRefill sells back the current hour's spent clicks.
func (s *Server) handleRefill(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	refilled, err := s.store.RefillQuota(p.Token, rules.RefillPrice, time.Now().Format("2006-01-02"), rules.RefillsFor(p.Prestige))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !refilled {
		// broke, already refilled today, or nothing spent this hour
		writeError(w, http.StatusConflict, "cannot_refill")
		return
	}
	s.events.Log("refill", events.PID(p.Token), nil)
	writeJSON(w, http.StatusOK, map[string]int{
		"coins":     p.Coins - rules.RefillPrice,
		"quotaLeft": s.cfg.Rules.QuotaFor(p.StaminaLevel),
	})
}

func (s *Server) handleBuy(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	p, ok := s.namedPlayer(w, r)
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
	case "charm":
		col, cur = "charm_level", &p.CharmLevel
	case "headstart":
		col, cur = "headstart_level", &p.HeadstartLevel
	case "stamina":
		col, cur = "stamina_level", &p.StaminaLevel
	case "magnet":
		col, cur = "magnet_level", &p.MagnetLevel
	case "golden":
		col, cur = "golden_level", &p.GoldenLevel
	default:
		writeError(w, http.StatusBadRequest, "bad_skill")
		return
	}
	price, ok2 := rules.PriceFor(body.Skill, *cur)
	if !ok2 {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	bought, err := s.store.BuySkill(p.Token, col, price, *cur)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	if !bought {
		writeError(w, http.StatusConflict, "cannot_buy")
		return
	}
	*cur++
	s.events.Log("buy", events.PID(p.Token), map[string]any{"skill": body.Skill, "price": price, "level": *cur})
	writeJSON(w, http.StatusOK, map[string]int{
		"coins":          p.Coins - price,
		"charmLevel":     p.CharmLevel,
		"headstartLevel": p.HeadstartLevel,
		"staminaLevel":   p.StaminaLevel,
		"magnetLevel":    p.MagnetLevel,
		"goldenLevel":    p.GoldenLevel,
		"quota":          s.cfg.Rules.QuotaFor(p.StaminaLevel),
	})
}

func (s *Server) handleNickname(w http.ResponseWriter, r *http.Request) {
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
	if !s.nicknameRe.MatchString(name) {
		writeError(w, http.StatusBadRequest, "bad_nickname")
		return
	}
	if err := s.store.SetNickname(p.Token, name); errors.Is(err, store.ErrNicknameTaken) {
		writeError(w, http.StatusConflict, "name_taken")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nickname": name})
}

func (s *Server) handleDeletePlayer(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.store.DeletePlayer(c.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	setTokenCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

// handleLinkNew mints a one-time code another device can claim to log into
// this account. One live code per player; expired entries are swept here.
func (s *Server) handleLinkNew(w http.ResponseWriter, r *http.Request) {
	p, ok := s.namedPlayer(w, r)
	if !ok {
		return
	}
	buf := make([]byte, s.cfg.LinkCodeLen)
	if _, err := rand.Read(buf); err != nil {
		writeError(w, http.StatusInternalServerError, "rand")
		return
	}
	code := make([]byte, s.cfg.LinkCodeLen)
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
	s.linkCodes[string(code)] = linkCode{token: p.Token, expires: now.Add(s.cfg.LinkTTL)}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"code": string(code)})
}

// handleLinkClaim swaps this device's session cookie for the account behind a
// valid code. The code is consumed on success.
func (s *Server) handleLinkClaim(w http.ResponseWriter, r *http.Request) {
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
	if s.claimFails >= s.cfg.ClaimFailLimit {
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

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	rules := s.cfg.Rules
	entries, err := s.store.Leaderboard(s.cfg.LeaderboardSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	for i := range entries {
		// each row's ladder stretches with that player's own prestige cap
		entries[i].Tier = rules.TierFor(entries[i].Stars, rules.MaxStarsFor(entries[i].Prestige))
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleCards(w http.ResponseWriter, r *http.Request) {
	p, ok := s.player(w, r)
	if !ok {
		return
	}
	cards, err := s.store.GetCards(p.Token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db")
		return
	}
	writeJSON(w, http.StatusOK, cards)
}
