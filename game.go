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
	Success bool      `json:"success"`
	Stars   int       `json:"stars"`
	Gained  int       `json:"gained"`
	Tier    string    `json:"tier"`
	TierUp  bool      `json:"tierUp"`
	Win     bool      `json:"win"`
	Card    *cardDrop `json:"card"`
}

// resolveClick runs one enchant attempt: roll, apply gain or full reset,
// then roll the card drop on success.
func resolveClick(stars, risk int) clickResult {
	prevTier := tierFor(stars)
	chance := chanceFor(stars, risk)
	if !rollPct(chance) {
		return clickResult{Stars: 0, Tier: tierFor(0)}
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
