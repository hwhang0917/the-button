package main

import (
	"crypto/rand"
	"math/big"
)

// chanceTable[n] is the % chance of going from n to n+1 stars.
var chanceTable = [16]int{100, 90, 81, 72, 63, 55, 48, 41, 35, 29, 24, 19, 15, 11, 8, 5}

const maxStars = 15

// maxStarsFor is the star cap at a prestige level: +5 per prestige, clamped
// at 30 to match the prestige skin ladder and keep the star row renderable.
func maxStarsFor(prestige int) int {
	return maxStars + 5*min(prestige, 3)
}

// overflowCoinPer prices stars rolled past the cap. Kept below the 15💰/star
// marginal sell value so deep risk stays a gamble, not income.
const overflowCoinPer = 5

// Risk levels 0 (safe) through maxRisk: odds ÷(level+1), and the star payout
// scales with the odds taken (see gainFor).
const maxRisk = 3

// Skill shop: streaks sell for triangle-number coins; skills persist on the
// player row. Charm adds to the effective chance AFTER the risk division and
// that same chance feeds gainFor, so charm trades payout for survival instead
// of being strictly better.
const (
	charmBonusPct = 2 // success % per charm level
	charmCap      = 5
	headstartCap  = 3
)

// retiredShieldRefund is what the removed 🛡️ protection scroll used to cost;
// the store bootstrap pays it back per unspent charge exactly once.
const retiredShieldRefund = 25

var (
	charmPrices     = [charmCap]int{10, 30, 90, 270, 810}
	headstartPrices = [headstartCap]int{20, 100, 400}
)

// Prestige: a maxed streak converts to coins and promotes the player's star
// tier along the card-rarity ladder (0=common, 1=rare, 2=holo, 3+=prismatic).
// Levels are unbounded — prestige 4 is prismatic-2, 5 is prismatic-3, and so
// on — and rank orders prestige, then stars. Coins are shop currency only.
var prestigeRewards = [3]int{300, 450, 600}

// prestigeRewardFor is the payout for prestiging from the given level; every
// prismatic lap pays the top reward.
func prestigeRewardFor(prestige int) int {
	return prestigeRewards[min(prestige, len(prestigeRewards)-1)]
}

// Lottery: a 15-coin scratch ticket. The outcome is rolled server-side at
// purchase; the client-side scratching is theater over a printed ticket.
// Exponential ladder (×5 per tier) with a life-changing 1-in-1000 jackpot;
// EV ≈ 12.2 (81% payback), wins ~1 in 3.3 tickets — still a coin sink.
const lotteryPrice = 15

var lotteryTable = []struct{ prize, permille int }{
	{2000, 1},  // 1등 0.1%
	{400, 8},   // 2등 0.8%
	{80, 40},   // 3등 4%
	{15, 250},  // 4등 25% (money back)
}

// Card pack: the only source of cards. Tier and rarity are rolled per card,
// higher tiers and rarer finishes less likely.
const packPrice = 30

// packBonusPct are the independent odds of a 2nd and 3rd card on top of the
// guaranteed one, so a pack averages 1.65 cards for its price.
var packBonusPct = [2]int{45, 20}

// Quota refill: buy back this hour's spent clicks. 10 clicks yield ~14 coins
// on average, so 60 is a deeply negative-EV convenience — fun, not income.
const refillPrice = 60

var packTierTable = []struct {
	tier     string
	permille int
}{
	{"unrank", 320}, {"bronze", 260}, {"silver", 180},
	{"gold", 120}, {"platinum", 80}, {"diamond", 40},
}

var packRarityTable = []struct {
	rarity   string
	permille int
}{
	{"common", 600}, {"rare", 250}, {"holo", 120}, {"prismatic", 30},
}

// rollPack draws the pack's card: tier and rarity rolled independently.
func rollPack() (string, string) {
	tier := packTierTable[0].tier
	if n, err := rand.Int(rand.Reader, big.NewInt(1000)); err == nil {
		roll := int(n.Int64())
		for _, e := range packTierTable {
			if roll < e.permille {
				tier = e.tier
				break
			}
			roll -= e.permille
		}
	}
	rarity := packRarityTable[0].rarity
	if n, err := rand.Int(rand.Reader, big.NewInt(1000)); err == nil {
		roll := int(n.Int64())
		for _, e := range packRarityTable {
			if roll < e.permille {
				rarity = e.rarity
				break
			}
			roll -= e.permille
		}
	}
	return tier, rarity
}

// rollPackCards draws a pack's contents: one guaranteed card plus a bonus roll
// per packBonusPct entry.
func rollPackCards() []cardDrop {
	tier, rarity := rollPack()
	drawn := []cardDrop{{Tier: tier, Rarity: rarity}}
	for _, pct := range packBonusPct {
		if !rollPct(pct) {
			continue
		}
		tier, rarity := rollPack()
		drawn = append(drawn, cardDrop{Tier: tier, Rarity: rarity})
	}
	return drawn
}

// rollLottery returns the prize for one ticket, 0 for 꽝.
func rollLottery() int {
	n, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return 0 // broken entropy source: house wins
	}
	roll := int(n.Int64())
	for _, e := range lotteryTable {
		if roll < e.permille {
			return e.prize
		}
		roll -= e.permille
	}
	return 0
}

// cardEffect is one card's talisman payload. A card is armed from the
// collection and fires on the very next click whatever the outcome — there is
// no tier gate, so the decision is purely *when* to spend it. The zero value
// is inert, which is why each entry below names only the fields it uses.
type cardEffect struct {
	Chance    int  // + roll chance (the roll only, never the payout)
	Guarantee bool // the roll always wins; the payout drops to the safe-mode rate
	Bonus     int  // + flat stars on success
	Mult      int  // × star gain (0 means 1)
	MaxRisk   bool // settle the payout as if risk == maxRisk
	TierJump  bool // success lands at least on the next tier's first star
	BestJump  bool // success lands at least on the personal best
	Keep      bool // fail keeps every star
	Half      bool // fail keeps ceil(stars/2)
	Rerolls   int  // extra rolls granted on a fail
	Refund    bool // the click doesn't consume quota
	CoinWin   int  // 💰 per star held after a success
	CoinLoss  int  // 💰 per star lost to a fail
	Card      bool // a free pack card on success
}

func (e cardEffect) armed() bool { return e != cardEffect{} }

// cardEffects is the whole card design: all 24 (tier, rarity) pairs, keyed
// "tier/rarity". Rarity is the power band — the six tier variants inside a
// band are comparable in strength and differ in flavour, so "any prismatic" is
// the jackpot and *which* prismatic is the collection chase.
//
// Guarantee must never pair with Mult (TestCardEffectTable enforces it): a
// guaranteed win settles at the safe-mode rate, because settling it at
// gainFor(payChance, maxRisk) would hand out round(100/2)=50 stars plus the
// overflow jackpot on every ★14 risk-3 click, farmable forever.
var cardEffects = map[string]cardEffect{
	// 커먼 — small, always useful
	"unrank/common":   {Chance: 5},    // 🐣 병아리 부적
	"bronze/common":   {Refund: true}, // 🥉 동전 한 닢
	"silver/common":   {Bonus: 1},     // 🥈 은빛 덤
	"gold/common":     {CoinWin: 2},   // 🥇 황금손
	"platinum/common": {Half: true},   // 💍 완충 반지
	"diamond/common":  {CoinLoss: 3},  // 💎 보험금

	// 레어 — meaningful
	"unrank/rare":   {Chance: 10},               // 🌱 네잎클로버
	"bronze/rare":   {Keep: true, Refund: true}, // 🛡️ 되감기
	"silver/rare":   {Rerolls: 1},               // 🗡️ 한 번 더
	"gold/rare":     {Chance: 5, Bonus: 1},      // 👑 왕관
	"platinum/rare": {Bonus: 2},                 // 🔱 삼지창
	"diamond/rare":  {Card: true},               // 🦄 유니콘

	// 홀로 — strong
	"unrank/holo":   {Chance: 20},     // 🍀 여신의 미소
	"bronze/holo":   {Keep: true},     // 🏺 불사조 항아리
	"silver/holo":   {TierJump: true}, // 🌙 달빛 사다리
	"gold/holo":     {Mult: 2},        // 🏆 곱배기
	"platinum/holo": {Rerolls: 2},     // 🔮 예언구
	"diamond/holo":  {MaxRisk: true},  // 🐉 용의 심장

	// 프리즘 — the jokers
	"unrank/prismatic":   {Guarantee: true},                       // 🌈 무지개
	"bronze/prismatic":   {Mult: 3},                               // 🔥 폭주
	"silver/prismatic":   {Guarantee: true, Refund: true},         // ❄️ 절대영도
	"gold/prismatic":     {Guarantee: true, Bonus: 2},             // ⚡ 벼락
	"platinum/prismatic": {Guarantee: true, BestJump: true},       // 🌊 해일
	"diamond/prismatic":  {Guarantee: true, Bonus: 4, Card: true}, // 🌌 특이점
}

func effectFor(tier, rarity string) cardEffect { return cardEffects[tier+"/"+rarity] }

func validTier(name string) bool {
	for _, t := range tiers {
		if t.Name == name {
			return true
		}
	}
	return false
}

func validRarity(name string) bool {
	for _, r := range rarities {
		if r == name {
			return true
		}
	}
	return false
}

// nextRarity is the fusion target one step up the ladder; prismatic is final.
func nextRarity(r string) (string, bool) {
	for i, name := range rarities {
		if name == r && i+1 < len(rarities) {
			return rarities[i+1], true
		}
	}
	return "", false
}

// defusion breaks one card into cards of the rarity below — deliberately
// lossy: fusing costs 3, defusing returns only 2.
const defuseYield = 2

func prevRarity(r string) (string, bool) {
	for i, name := range rarities {
		if name == r && i > 0 {
			return rarities[i-1], true
		}
	}
	return "", false
}

type skills struct {
	Charm     int
	Headstart int
	Card      cardEffect // the armed card, inert when nothing is armed
}

// effChanceFor is the roll chance after risk division and charm bonus.
func effChanceFor(stars, risk, charm, cap int) int {
	if stars >= cap {
		return 0
	}
	return min(100, chanceFor(stars, risk, cap)+charmBonusPct*charm)
}

func tri(n int) int { return n * (n + 1) / 2 }

// streakValue is the coin payout for selling a streak down to the head-start
// floor. Only stars above the floor pay — otherwise sell-land-sell at the
// floor would print free coins.
func streakValue(stars, floor int) int {
	return tri(stars) - tri(min(floor, stars))
}

// priceFor returns the next purchase price of a skill at the given level, or
// ok=false when the skill is unknown or capped.
func priceFor(skill string, level int) (int, bool) {
	switch skill {
	case "charm":
		if level >= charmCap {
			return 0, false
		}
		return charmPrices[level], true
	case "headstart":
		if level >= headstartCap {
			return 0, false
		}
		return headstartPrices[level], true
	}
	return 0, false
}

var tiers = []struct {
	Name     string
	MinStars int
}{
	{"unrank", 0},
	{"bronze", 1},
	{"silver", 4},
	{"gold", 7},
	{"platinum", 10},
	{"diamond", 13},
}

// tierRank is a tier's position on the ladder: 0 for unrank up to 5 for
// diamond. Doubles as the bonus-click reward for reaching it.
func tierRank(name string) int {
	for i, t := range tiers {
		if t.Name == name {
			return i
		}
	}
	return 0
}

func tierFor(stars int) string {
	name := tiers[0].Name
	for _, t := range tiers {
		if stars >= t.MinStars {
			name = t.Name
		}
	}
	return name
}

// nextTierMin is the first star count of the tier above the one holding stars,
// or 0 at the top of the ladder — the 🌙 달빛 사다리 card jumps to it.
func nextTierMin(stars int) int {
	for _, t := range tiers {
		if t.MinStars > stars {
			return t.MinStars
		}
	}
	return 0
}

// chanceFor returns the success % for the next click at the given star count.
// Stars past the table (prestige-raised caps) roll at the 5% floor.
func chanceFor(stars, risk, cap int) int {
	if stars >= cap {
		return 0
	}
	return chanceTable[min(stars, len(chanceTable)-1)] / (risk + 1)
}

// gainFor is the stars won on a successful click: safe mode always steps one
// star; risk mode pays the odds back — round(100/chance) — so the longer the
// shot, the bigger the payout, and the expected gain per click stays flat.
func gainFor(chance, risk int) int {
	if risk <= 0 || chance <= 0 {
		return 1
	}
	return max(1, (100+chance/2)/chance)
}

// devMode forces every percentage roll to succeed (clicks at any risk level,
// card drops). Set from the DEV_MODE env var at startup; never for production.
var devMode bool

// rollPct returns true with pct% probability, using crypto/rand so results
// cannot be predicted or replayed by clients.
func rollPct(pct int) bool {
	if devMode {
		return true
	}
	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		// crypto/rand failing means the OS entropy source is broken; nothing
		// sensible to do but treat the roll as a fail.
		return false
	}
	return int(n.Int64()) < pct
}

var rarities = []string{"common", "rare", "holo", "prismatic"}

type cardDrop struct {
	Tier   string `json:"tier"`
	Rarity string `json:"rarity"`
}

type clickResult struct {
	Success      bool      `json:"success"`
	Stars        int       `json:"stars"`
	Gained       int       `json:"gained"`
	Tier         string    `json:"tier"`
	TierUp       bool      `json:"tierUp"`
	Win          bool      `json:"win"`
	Card         *cardDrop `json:"card"`
	TalismanUsed bool      `json:"talismanUsed"`
	Refund       bool      `json:"refund"`  // the click is handed back to the quota
	Jackpot      int       `json:"jackpot"` // coins from overflow and card effects
}

// resolveClick runs one enchant attempt: roll, then apply the gain or the
// reset. The armed card (sk.Card) is the only fail protection in the game and
// burns on this click whatever the outcome. Without a keeping card the reset
// lands at the head-start floor — but never above where the streak was, so
// failing below the floor is never profitable.
func resolveClick(stars, risk int, sk skills, cap, best int) clickResult {
	prevTier := tierFor(stars)
	e := sk.Card
	// charm feeds both roll and payout (higher chance, lower reward), but a
	// card's chance bonus boosts ONLY the roll — the consumed card is its
	// price, so it must not shrink the risk-mode star reward
	payRisk := risk
	if e.MaxRisk {
		payRisk = maxRisk
	}
	payChance := effChanceFor(stars, payRisk, sk.Charm, cap)
	chance := min(100, effChanceFor(stars, risk, sk.Charm, cap)+e.Chance)

	success := e.Guarantee || rollPct(chance)
	for r := 0; !success && r < e.Rerolls; r++ {
		success = rollPct(chance)
	}

	if !success {
		newStars := min(sk.Headstart, stars)
		switch {
		case e.Keep:
			newStars = stars
		case e.Half:
			// never worse than the head-start floor would have been
			newStars = max((stars+1)/2, newStars)
		}
		return clickResult{
			Stars:        newStars,
			Tier:         tierFor(newStars),
			TalismanUsed: e.armed(),
			Refund:       e.Refund,
			Jackpot:      e.CoinLoss * (stars - newStars),
		}
	}

	// a guaranteed win settles at the safe-mode rate — see cardEffects
	gain := 1
	if !e.Guarantee {
		gain = gainFor(payChance, payRisk)
	}
	gain *= max(1, e.Mult)
	gain += e.Bonus
	newStars := min(stars+gain, cap)
	// jumps are floors, so they can lift a streak but never cut one short
	if e.TierJump {
		newStars = max(newStars, min(nextTierMin(stars), cap))
	}
	if e.BestJump {
		newStars = max(newStars, min(best, cap))
	}
	var card *cardDrop
	if e.Card {
		tier, rarity := rollPack()
		card = &cardDrop{Tier: tier, Rarity: rarity}
	}
	return clickResult{
		Success:      true,
		Stars:        newStars,
		Gained:       newStars - stars,
		Tier:         tierFor(newStars),
		TierUp:       tierFor(newStars) != prevTier,
		Win:          newStars >= cap,
		Card:         card,
		TalismanUsed: e.armed(),
		Refund:       e.Refund,
		Jackpot:      overflowCoinPer*max(0, stars+gain-cap) + e.CoinWin*newStars,
	}
}
