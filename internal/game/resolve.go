package game

// ChanceFor is the success % for the next click at the given star count. Stars
// past the table (prestige-raised caps) roll at the table's last entry.
func (r Rules) ChanceFor(stars, risk, cap int) int {
	if stars >= cap {
		return 0
	}
	return r.ChanceTable[min(stars, len(r.ChanceTable)-1)] / (risk + 1)
}

// EffChanceFor is the roll chance after risk division and the charm bonus.
func (r Rules) EffChanceFor(stars, risk, charm, cap int) int {
	if stars >= cap {
		return 0
	}
	return min(100, r.ChanceFor(stars, risk, cap)+r.Charm.BonusPct*charm)
}

// roll settles a percentage against the RNG. The certainties are decided here
// rather than delegated: a 100% roll must always win and a 0% roll must always
// lose whatever the RNG does, which is what makes "the first click always
// succeeds" a property of the rules instead of an accident of crypto/rand.
func (r Rules) roll(pct int) bool {
	switch {
	case pct >= 100:
		return true
	case pct <= 0:
		return false
	}
	return r.RNG.Pct(pct)
}

// GainFor is the stars won on a successful click: safe mode always steps one
// star; risk mode pays the odds back — round(100/chance) — so the longer the
// shot, the bigger the payout, and the expected gain per click stays flat.
func GainFor(chance, risk int) int {
	if risk <= 0 || chance <= 0 {
		return 1
	}
	return max(1, (100+chance/2)/chance)
}

func tri(n int) int { return n * (n + 1) / 2 }

// StreakValue is the coin payout for selling a streak down to the head-start
// floor. Only stars above the floor pay — otherwise sell-land-sell at the floor
// would print free coins.
func StreakValue(stars, floor int) int {
	return tri(stars) - tri(min(floor, stars))
}

// draw walks a permille table, returning the first entry's name. An empty table
// yields "" — Validate rules that out at boot.
func (r Rules) draw(table []Weight) string {
	if len(table) == 0 {
		return ""
	}
	roll := r.RNG.Intn(1000)
	for _, e := range table {
		if roll < e.Permille {
			return e.Name
		}
		roll -= e.Permille
	}
	return table[0].Name
}

// RollPack draws one card: tier and rarity rolled independently.
func (r Rules) RollPack() Card {
	return Card{Tier: r.draw(r.Pack.Tiers), Rarity: r.draw(r.Pack.Rarities)}
}

// RollPackCards draws a pack's contents: one guaranteed card plus an
// independent roll per Pack.BonusPct entry.
func (r Rules) RollPackCards() []Card {
	drawn := []Card{r.RollPack()}
	for _, pct := range r.Pack.BonusPct {
		if r.RNG.Pct(pct) {
			drawn = append(drawn, r.RollPack())
		}
	}
	return drawn
}

// RollLottery returns the prize for one ticket, 0 for 꽝.
func (r Rules) RollLottery() int {
	roll := r.RNG.Intn(1000)
	for _, p := range r.Lottery.Prizes {
		if roll < p.Permille {
			return p.Prize
		}
		roll -= p.Permille
	}
	return 0
}

// Click is one enchant attempt: the player's position plus whatever they have
// brought to bear on it.
type Click struct {
	Stars     int
	Risk      int
	Cap       int // the star cap at this prestige level (Rules.MaxStarsFor)
	Best      int // personal best, for the 🌊 해일 card
	Charm     int
	Headstart int
	Magnet    int // card-drop proc level
	Golden    int // coin-win proc level
	Card      CardEffect // the armed card, inert when nothing is armed
}

// Result is the outcome of one click. The JSON shape is the click API's
// response body.
type Result struct {
	Success      bool   `json:"success"`
	Stars        int    `json:"stars"`
	Gained       int    `json:"gained"`
	Tier         string `json:"tier"`
	TierUp       bool   `json:"tierUp"`
	Win          bool   `json:"win"`
	Card         *Card  `json:"card"`
	TalismanUsed bool   `json:"talismanUsed"`
	Saved        bool   `json:"saved"`   // a keep card held the streak on this fail
	Refund       bool   `json:"refund"`  // the click is handed back to the quota
	Jackpot      int    `json:"jackpot"` // coins from overflow and card effects
}

// Resolve runs one enchant attempt: roll, then apply the gain or the reset. The
// armed card is the only fail protection in the game and burns on this click
// whatever the outcome. Without a keeping card the reset lands at the head-start
// floor — but never above where the streak was, so failing below the floor is
// never profitable.
func (r Rules) Resolve(c Click) Result {
	prevTier := r.TierFor(c.Stars)
	e := c.Card
	// charm feeds both roll and payout (higher chance, lower reward), but a
	// card's chance bonus boosts ONLY the roll — the consumed card is its
	// price, so it must not shrink the risk-mode star reward
	payRisk := c.Risk
	if e.MaxRisk {
		payRisk = r.MaxRisk
	}
	payChance := r.EffChanceFor(c.Stars, payRisk, c.Charm, c.Cap)
	chance := min(100, r.EffChanceFor(c.Stars, c.Risk, c.Charm, c.Cap)+e.Chance)

	success := e.Guarantee || r.roll(chance)
	for i := 0; !success && i < e.Rerolls; i++ {
		success = r.roll(chance)
	}

	if !success {
		newStars := min(c.Headstart, c.Stars)
		switch {
		case e.Keep:
			newStars = c.Stars
		case e.Half:
			// never worse than the head-start floor would have been
			newStars = max((c.Stars+1)/2, newStars)
		}
		return Result{
			Stars:        newStars,
			Tier:         r.TierFor(newStars),
			TalismanUsed: e.Armed(),
			// only the keep effect earns the shield message — a chance-only card
			// burning while the streak sits at the head-start floor must not,
			// even though the stars happen to survive either way
			Saved:        e.Keep && c.Stars > 0,
			Refund:       e.Refund,
			Jackpot:      e.CoinLoss * (c.Stars - newStars),
		}
	}

	// a guaranteed win settles at the safe-mode rate — see DefaultCards
	gain := 1
	if !e.Guarantee {
		gain = GainFor(payChance, payRisk)
	}
	gain *= max(1, e.Mult)
	gain += e.Bonus
	newStars := min(c.Stars+gain, c.Cap)
	// jumps are floors, so they can lift a streak but never cut one short
	if e.TierJump {
		newStars = max(newStars, min(r.NextTierMin(c.Stars), c.Cap))
	}
	if e.BestJump {
		newStars = max(newStars, min(c.Best, c.Cap))
	}
	// skill procs roll only at level > 0 so a skill-less click consumes the
	// same RNG sequence as before these skills existed
	var card *Card
	if e.Card || (c.Magnet > 0 && r.roll(r.Magnet.BonusPct*c.Magnet)) {
		drawn := r.RollPack()
		card = &drawn
	}
	golden := 0
	if c.Golden > 0 && r.roll(r.Golden.BonusPct*c.Golden) {
		golden = newStars // star-scaled: big wins only deep in a streak
	}
	return Result{
		Success:      true,
		Stars:        newStars,
		Gained:       newStars - c.Stars,
		Tier:         r.TierFor(newStars),
		TierUp:       r.TierFor(newStars) != prevTier,
		Win:          newStars >= c.Cap,
		Card:         card,
		TalismanUsed: e.Armed(),
		Refund:       e.Refund,
		Jackpot:      r.OverflowCoinPer*max(0, c.Stars+gain-c.Cap) + e.CoinWin*newStars + golden,
	}
}
