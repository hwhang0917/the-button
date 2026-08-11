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
// Exponential ladder (×5 per tier) with a life-changing 1-in-1000 jackpot;
// EV ≈ 12.2 (81% payback), wins ~1 in 3.3 tickets — still a coin sink.
const lotteryPrice = 15

var lotteryTable = []struct{ prize, permille int }{
	{2000, 1},  // 1등 0.1%
	{400, 8},   // 2등 0.8%
	{80, 40},   // 3등 4%
	{15, 250},  // 4등 25% (money back)
}

// Card pack: one random card from any tier, higher tiers rarer. A pity path
// for talisman supply and collection completion; always yields a card.
const packPrice = 30

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

// Talisman one-shot effects by card rarity (armed via the collection, fires
// only while the streak is inside the card's tier).
const (
	talCommonPct = 5  // +chance on the next in-tier click
	talRarePct   = 10 // +chance on the next in-tier click
)

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
	Shield    bool
	// active talisman effects, pre-resolved by the handler for the current tier
	TalBonus  int  // +chance from a common/rare talisman
	TalShield bool // holo: keep stars on fail
	TalDouble bool // prismatic: double stars on success
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
	Card         *cardDrop `json:"card"`
	ShieldUsed   bool      `json:"shieldUsed"`
	TalismanUsed bool      `json:"talismanUsed"`
}

// resolveClick runs one enchant attempt: roll, apply gain or reset, then roll
// the card drop on success. A shield keeps the stars on fail; otherwise the
// reset lands at the head-start floor — but never above where the streak was,
// so failing below the floor is never profitable.
func resolveClick(stars, risk int, sk skills) clickResult {
	prevTier := tierFor(stars)
	// charm feeds both roll and payout (higher chance, lower reward), but the
	// talisman bonus boosts ONLY the roll — the consumed card is its price,
	// so it must not shrink the risk-mode star reward
	payChance := effChanceFor(stars, risk, sk.Charm)
	chance := payChance
	if payChance > 0 && sk.TalBonus > 0 {
		chance = min(100, payChance+sk.TalBonus)
	}
	// a chance talisman burns on the click no matter the outcome
	talUsed := sk.TalBonus > 0
	if !rollPct(chance) {
		if sk.TalShield {
			// the scoped talisman saves before a purchased scroll would
			return clickResult{Stars: stars, Tier: prevTier, TalismanUsed: true}
		}
		if sk.Shield {
			return clickResult{Stars: stars, Tier: prevTier, ShieldUsed: true, TalismanUsed: talUsed}
		}
		floor := min(sk.Headstart, stars)
		return clickResult{Stars: floor, Tier: tierFor(floor), TalismanUsed: talUsed}
	}
	gain := gainFor(payChance, risk)
	if sk.TalDouble {
		gain *= 2
		talUsed = true
	}
	newStars := min(stars+gain, maxStars)
	return clickResult{
		Success:      true,
		Stars:        newStars,
		Gained:       newStars - stars,
		Tier:         tierFor(newStars),
		TierUp:       tierFor(newStars) != prevTier,
		Win:          newStars == maxStars,
		Card:         rollCard(newStars, risk),
		TalismanUsed: talUsed,
	}
}
