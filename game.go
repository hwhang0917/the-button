package main

import (
	"crypto/rand"
	"math/big"
)

// chanceTable[n] is the % chance of going from n to n+1 stars.
var chanceTable = [16]int{100, 90, 81, 72, 63, 55, 48, 41, 35, 29, 24, 19, 15, 11, 8, 5}

const maxStars = 15

// Risk levels 0 (safe) through maxRisk: odds ÷(level+1), card-drop odds
// ×(level+1), and the star payout scales with the odds taken (see gainFor).
const maxRisk = 3

const baseCardDropPct = 5

// Skill shop: streaks sell for triangle-number coins; skills persist on the
// player row. Charm adds to the effective chance AFTER the risk division and
// that same chance feeds gainFor, so charm trades payout for survival instead
// of being strictly better.
const (
	charmBonusPct = 2 // success % per charm level
	charmCap      = 5
	headstartCap  = 3
	shieldPrice   = 25
)

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
// EV ≈ 9.2 (61% payback) — a fun coin sink, never an income source.
const lotteryPrice = 15

var lotteryTable = []struct{ prize, permille int }{
	{500, 5},   // 1등 0.5%
	{100, 20},  // 2등 2%
	{30, 80},   // 3등 8%
	{15, 150},  // 4등 15% (money back)
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

type skills struct {
	Charm     int
	Headstart int
	Shield    bool
}

// effChanceFor is the roll chance after risk division and charm bonus.
func effChanceFor(stars, risk, charm int) int {
	if stars >= maxStars {
		return 0
	}
	return min(100, chanceFor(stars, risk)+charmBonusPct*charm)
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
	case "shield":
		return shieldPrice, true // uncapped consumable
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

// chanceFor returns the success % for the next click at the given star count.
func chanceFor(stars, risk int) int {
	if stars >= maxStars {
		return 0
	}
	return chanceTable[stars] / (risk + 1)
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

// rollPct returns true with pct% probability, using crypto/rand so results
// cannot be predicted or replayed by clients.
func rollPct(pct int) bool {
	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		// crypto/rand failing means the OS entropy source is broken; nothing
		// sensible to do but treat the roll as a fail.
		return false
	}
	return int(n.Int64()) < pct
}

var rarities = []string{"common", "rare", "holo", "prismatic"}

// rarityWeights shifts toward rarer finishes the deeper the streak.
func rarityWeights(stars int) [4]int {
	common := 60 - 3*stars
	if common < 10 {
		common = 10
	}
	return [4]int{common, 25 + stars, 12 + stars, 3 + stars}
}

type cardDrop struct {
	Tier   string `json:"tier"`
	Rarity string `json:"rarity"`
}

// rollCard rolls the post-success card drop; nil means no drop.
func rollCard(stars, risk int) *cardDrop {
	if !rollPct(baseCardDropPct * (risk + 1)) {
		return nil
	}
	w := rarityWeights(stars)
	total := 0
	for _, v := range w {
		total += v
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return nil
	}
	pick := int(n.Int64())
	for i, v := range w {
		if pick < v {
			return &cardDrop{Tier: tierFor(stars), Rarity: rarities[i]}
		}
		pick -= v
	}
	return nil // unreachable
}

type clickResult struct {
	Success    bool      `json:"success"`
	Stars      int       `json:"stars"`
	Gained     int       `json:"gained"`
	Tier       string    `json:"tier"`
	TierUp     bool      `json:"tierUp"`
	Win        bool      `json:"win"`
	Card       *cardDrop `json:"card"`
	ShieldUsed bool      `json:"shieldUsed"`
}

// resolveClick runs one enchant attempt: roll, apply gain or reset, then roll
// the card drop on success. A shield keeps the stars on fail; otherwise the
// reset lands at the head-start floor — but never above where the streak was,
// so failing below the floor is never profitable.
func resolveClick(stars, risk int, sk skills) clickResult {
	prevTier := tierFor(stars)
	chance := effChanceFor(stars, risk, sk.Charm)
	if !rollPct(chance) {
		if sk.Shield {
			return clickResult{Stars: stars, Tier: prevTier, ShieldUsed: true}
		}
		floor := min(sk.Headstart, stars)
		return clickResult{Stars: floor, Tier: tierFor(floor)}
	}
	newStars := min(stars+gainFor(chance, risk), maxStars)
	return clickResult{
		Success: true,
		Stars:   newStars,
		Gained:  newStars - stars,
		Tier:    tierFor(newStars),
		TierUp:  tierFor(newStars) != prevTier,
		Win:     newStars == maxStars,
		Card:    rollCard(newStars, risk),
	}
}
