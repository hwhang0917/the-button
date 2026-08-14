package game

import "fmt"

// Skill is a purchasable ladder. The cap is len(Prices), so a ladder and its
// cap can never disagree.
type Skill struct {
	BonusPct int   `yaml:"bonus_pct" json:"bonusPct"`
	Prices   []int `yaml:"prices"    json:"prices"`
}

func (s Skill) Cap() int { return len(s.Prices) }

// PriceAt is the cost of the next level, or ok=false once capped.
func (s Skill) PriceAt(level int) (int, bool) {
	if level < 0 || level >= len(s.Prices) {
		return 0, false
	}
	return s.Prices[level], true
}

// LotteryPrize is one rung of the scratch-ticket ladder. Permille odds that do
// not add up to 1000 leave the remainder as 꽝.
type LotteryPrize struct {
	Prize    int `yaml:"prize"    json:"prize"`
	Permille int `yaml:"permille" json:"permille"`
}

type Lottery struct {
	Price  int            `yaml:"price"  json:"price"`
	Prizes []LotteryPrize `yaml:"prizes" json:"prizes"`
}

// Weight is one entry of a permille draw table.
type Weight struct {
	Name     string `json:"name"`
	Permille int    `json:"permille"`
}

// Pack is the card pack: always one card, plus an independent roll per
// BonusPct entry, with tier and rarity drawn from separate tables.
type Pack struct {
	Price    int      `json:"price"`
	BonusPct []int    `json:"bonusPct"`
	Tiers    []Weight `json:"tiers"`
	Rarities []Weight `json:"rarities"`
}

// Rules is the complete game definition. Everything here is data so config.yml
// can retune the whole economy without a recompile; the algorithms that consume
// it live in resolve.go.
type Rules struct {
	// Odds
	ChanceTable []int `json:"chanceTable"` // [n] is the % chance of going from n to n+1 stars
	MaxRisk     int   `json:"maxRisk"`     // risk levels 0 (safe) through MaxRisk
	MaxStars    int   `json:"maxStars"`    // the base cap, before prestige raises it

	// Prestige: a maxed streak converts to coins and promotes the player's star
	// tier along the rarity ladder. Levels are unbounded — past the skin cap
	// every lap pays the top reward and reuses the last skin.
	PrestigeStarBonus int   `json:"prestigeStarBonus"`
	PrestigeSkinCap   int   `json:"prestigeSkinCap"`
	PrestigeRewards   []int `json:"prestigeRewards"`

	// OverflowCoinPer prices stars rolled past the cap. Kept below the marginal
	// sell value of a star so deep risk stays a gamble, not income.
	OverflowCoinPer int `json:"overflowCoinPer"`

	Tiers    []Tier                `json:"tiers"`
	Rarities []string              `json:"rarities"`
	Cards    map[string]CardEffect `json:"cards"`

	// Economy. Charm adds to the effective chance AFTER the risk division and
	// that same chance feeds GainFor, so charm trades payout for survival
	// instead of being strictly better.
	Charm       Skill   `json:"charm"`
	Headstart   Skill   `json:"headstart"`
	Stamina     Skill   `json:"stamina"`
	Lottery     Lottery `json:"lottery"`
	Pack        Pack    `json:"pack"`
	RefillPrice int     `json:"refillPrice"`

	// Fusion is deliberately lossy: FuseCost in, one out; defusing one returns
	// only DefuseYield.
	FuseCost    int `json:"fuseCost"`
	DefuseYield int `json:"defuseYield"`

	// CardSell[i] is the coin payout for selling one card of Rarities[i].
	CardSell []int `json:"cardSell"`

	// RetiredShieldRefund is what the removed 🛡️ protection scroll used to
	// cost; the store bootstrap pays it back per unspent charge exactly once.
	RetiredShieldRefund int `json:"-"`

	Quota int `json:"-"` // clicks per player per hour

	// RNG is swapped for AlwaysWin under DEV_MODE and scripted in tests.
	RNG RNG `json:"-"`
}

// Default is the shipped game — the single source of truth for defaults, which
// is why config.yml.example is documentation rather than a second definition
// that could drift from it.
func Default() Rules {
	return Rules{
		ChanceTable:       []int{100, 90, 81, 72, 63, 55, 48, 41, 35, 29, 24, 19, 15, 11, 8, 5},
		MaxRisk:           3,
		MaxStars:          15,
		PrestigeStarBonus: 5,
		PrestigeSkinCap:   3,
		PrestigeRewards:   []int{300, 450, 600},
		OverflowCoinPer:   5,
		Tiers:             DefaultTiers(),
		Rarities:          DefaultRarities(),
		Cards:             DefaultCards(),
		Charm:             Skill{BonusPct: 2, Prices: []int{10, 30, 90, 270, 810}},
		Headstart:         Skill{Prices: []int{20, 100, 400}},
		// compounding +25%/level: at the default quota of 10 a maxed player
		// reaches 31 clicks an hour, about the session length the hourly bucket
		// is actually fun at. Priced steeply because it compounds income.
		Stamina: Skill{BonusPct: 25, Prices: []int{20, 60, 180, 540, 1620}},
		// Exponential ladder (×5 per rung) with a 1-in-1000 jackpot; EV ≈ 12.2
		// (81% payback), wins ~1 in 3.3 tickets — still a coin sink.
		Lottery: Lottery{Price: 15, Prizes: []LotteryPrize{
			{2000, 1}, // 1등 0.1%
			{400, 8},  // 2등 0.8%
			{80, 40},  // 3등 4%
			{15, 250}, // 4등 25% (money back)
		}},
		Pack: Pack{
			Price:    30,
			BonusPct: []int{45, 20}, // averages 1.65 cards per pack
			Tiers: []Weight{
				{"unrank", 320}, {"bronze", 260}, {"silver", 180},
				{"gold", 120}, {"platinum", 80}, {"diamond", 40},
			},
			Rarities: []Weight{
				{"common", 600}, {"rare", 250}, {"holo", 120}, {"prismatic", 30},
			},
		},
		// 10 clicks yield ~14 coins on average, so 60 is a deeply negative-EV
		// convenience — fun, not income.
		RefillPrice:         60,
		FuseCost:            3,
		DefuseYield:         2,
		// ×3 per step matches FuseCost, so selling is fusion-neutral (3 commons
		// sell for exactly one rare) and defuse-then-sell stays lossy. Keep the
		// weighted pack EV (~6.5/card at default weights) well under the pack
		// price per card (~18.2), or buy-pack-then-sell becomes a coin printer.
		CardSell: []int{2, 6, 18, 54},
		RetiredShieldRefund: 25,
		Quota:               10,
		RNG:                 CryptoRNG{},
	}
}

// MaxStarsFor is the star cap at a prestige level. The skin cap bounds it too,
// so the star row stays renderable however far prestige runs.
func (r Rules) MaxStarsFor(prestige int) int {
	return r.MaxStars + r.PrestigeStarBonus*min(prestige, r.PrestigeSkinCap)
}

// QuotaFor is the hourly click allowance at a stamina level. Each level
// compounds the configured base rather than adding a fixed number, because the
// base is a config knob: a flat "+5 clicks" would be a rounding error at
// quota 120 and would double the game at quota 3. Compounding is applied step
// by step, and every level is worth at least one click, so a small base still
// gets a real upgrade instead of rounding away to nothing.
func (r Rules) QuotaFor(level int) int {
	q := r.Quota
	for range min(max(level, 0), r.Stamina.Cap()) {
		q = max(q+1, (q*(100+r.Stamina.BonusPct)+50)/100)
	}
	return q
}

// PrestigeRewardFor is the payout for prestiging from the given level; every
// lap past the ladder pays the top reward.
func (r Rules) PrestigeRewardFor(prestige int) int {
	return r.PrestigeRewards[min(prestige, len(r.PrestigeRewards)-1)]
}

// PriceFor is the next purchase price of a skill, or ok=false when the skill is
// unknown or capped.
func (r Rules) PriceFor(skill string, level int) (int, bool) {
	switch skill {
	case "charm":
		return r.Charm.PriceAt(level)
	case "headstart":
		return r.Headstart.PriceAt(level)
	case "stamina":
		return r.Stamina.PriceAt(level)
	}
	return 0, false
}

// Validate rejects a config that would produce a broken or exploitable game.
// It runs at boot so a hand-edited config.yml fails loudly instead of serving
// nonsense — these checks used to live only in the Go tests.
func (r Rules) Validate() error {
	if len(r.ChanceTable) == 0 {
		return fmt.Errorf("game.chance_table must not be empty")
	}
	if r.ChanceTable[0] != 100 {
		return fmt.Errorf("game.chance_table[0] must be 100 so the first click always succeeds, got %d", r.ChanceTable[0])
	}
	for i, c := range r.ChanceTable {
		if c < 0 || c > 100 {
			return fmt.Errorf("game.chance_table[%d] = %d, must be 0-100", i, c)
		}
	}
	if r.MaxRisk < 0 {
		return fmt.Errorf("game.max_risk must not be negative, got %d", r.MaxRisk)
	}
	if r.MaxStars < 1 {
		return fmt.Errorf("game.max_stars must be at least 1, got %d", r.MaxStars)
	}
	if r.Quota < 1 {
		return fmt.Errorf("game.quota must be at least 1, got %d", r.Quota)
	}
	if len(r.PrestigeRewards) == 0 {
		return fmt.Errorf("game.prestige.rewards must not be empty")
	}
	if len(r.Tiers) == 0 {
		return fmt.Errorf("game.tiers must not be empty")
	}
	if r.Tiers[0].MinStars != 0 {
		return fmt.Errorf("game.tiers[0].min_stars must be 0, got %d", r.Tiers[0].MinStars)
	}
	for i := 1; i < len(r.Tiers); i++ {
		if r.Tiers[i].MinStars <= r.Tiers[i-1].MinStars {
			return fmt.Errorf("game.tiers must ascend by min_stars: %q (%d) does not follow %q (%d)",
				r.Tiers[i].Name, r.Tiers[i].MinStars, r.Tiers[i-1].Name, r.Tiers[i-1].MinStars)
		}
	}
	if len(r.Rarities) == 0 {
		return fmt.Errorf("game.rarities must not be empty")
	}
	if err := r.validateCards(); err != nil {
		return err
	}
	if err := validateSkill("economy.charm", r.Charm); err != nil {
		return err
	}
	if err := validateSkill("economy.headstart", r.Headstart); err != nil {
		return err
	}
	if err := validateSkill("economy.stamina", r.Stamina); err != nil {
		return err
	}
	if r.Stamina.BonusPct < 1 {
		return fmt.Errorf("economy.stamina.bonus_pct must be at least 1, got %d", r.Stamina.BonusPct)
	}
	if err := r.validateDraws(); err != nil {
		return err
	}
	for name, price := range map[string]int{
		"economy.lottery.price": r.Lottery.Price,
		"economy.pack.price":    r.Pack.Price,
		"economy.refill_price":  r.RefillPrice,
	} {
		if price < 0 {
			return fmt.Errorf("%s must not be negative, got %d", name, price)
		}
	}
	if r.FuseCost < 2 {
		return fmt.Errorf("economy.fuse_cost must be at least 2, got %d", r.FuseCost)
	}
	if r.DefuseYield < 1 || r.DefuseYield >= r.FuseCost {
		return fmt.Errorf("economy.defuse_yield must be 1..%d so fusing stays lossy, got %d", r.FuseCost-1, r.DefuseYield)
	}
	if len(r.CardSell) != len(r.Rarities) {
		return fmt.Errorf("economy.card_sell has %d prices, want one per rarity (%d)", len(r.CardSell), len(r.Rarities))
	}
	for i, price := range r.CardSell {
		if price < 0 {
			return fmt.Errorf("economy.card_sell[%d] must not be negative, got %d", i, price)
		}
		if i > 0 && price < r.CardSell[i-1] {
			return fmt.Errorf("economy.card_sell must not descend: %d follows %d", price, r.CardSell[i-1])
		}
	}
	return nil
}

// SellValueFor is the coin payout for selling one card of the given rarity,
// or ok=false for an unknown rarity.
func (r Rules) SellValueFor(rarity string) (int, bool) {
	for i, name := range r.Rarities {
		if name == rarity {
			return r.CardSell[i], true
		}
	}
	return 0, false
}

func (r Rules) validateCards() error {
	want := len(r.Tiers) * len(r.Rarities)
	for _, t := range r.Tiers {
		for _, rarity := range r.Rarities {
			key := Card{Tier: t.Name, Rarity: rarity}.Key()
			e, ok := r.Cards[key]
			if !ok {
				return fmt.Errorf("cards.%s is missing — every tier/rarity pair needs an effect", key)
			}
			if !e.Armed() {
				return fmt.Errorf("cards.%s is inert — every card must do something", key)
			}
			// a guaranteed win settles at the safe-mode rate; letting a
			// multiplier ride on top would mint stars and overflow coins
			if e.Guarantee && e.Mult != 0 {
				return fmt.Errorf("cards.%s sets both guarantee and mult — a guaranteed win must settle at the safe-mode rate", key)
			}
			if e.Chance < 0 || e.Chance > 100 {
				return fmt.Errorf("cards.%s chance = %d, must be 0-100", key, e.Chance)
			}
		}
	}
	if len(r.Cards) != want {
		for key := range r.Cards {
			tier, rarity, _ := splitKey(key)
			if !r.ValidTier(tier) || !r.ValidRarity(rarity) {
				return fmt.Errorf("cards.%s does not match any tier/rarity pair", key)
			}
		}
		return fmt.Errorf("cards has %d entries, want %d (one per tier × rarity)", len(r.Cards), want)
	}
	return nil
}

func validateSkill(name string, s Skill) error {
	if len(s.Prices) == 0 {
		return fmt.Errorf("%s.prices must not be empty", name)
	}
	for i, p := range s.Prices {
		if p < 0 {
			return fmt.Errorf("%s.prices[%d] = %d, must not be negative", name, i, p)
		}
	}
	return nil
}

func (r Rules) validateDraws() error {
	sum := 0
	for _, w := range r.Pack.Tiers {
		if !r.ValidTier(w.Name) {
			return fmt.Errorf("economy.pack.tier_permille has unknown tier %q", w.Name)
		}
		sum += w.Permille
	}
	if sum != 1000 {
		return fmt.Errorf("economy.pack.tier_permille must sum to 1000‰, got %d", sum)
	}
	sum = 0
	for _, w := range r.Pack.Rarities {
		if !r.ValidRarity(w.Name) {
			return fmt.Errorf("economy.pack.rarity_permille has unknown rarity %q", w.Name)
		}
		sum += w.Permille
	}
	if sum != 1000 {
		return fmt.Errorf("economy.pack.rarity_permille must sum to 1000‰, got %d", sum)
	}
	for i, pct := range r.Pack.BonusPct {
		if pct < 0 || pct > 100 {
			return fmt.Errorf("economy.pack.bonus_pct[%d] = %d, must be 0-100", i, pct)
		}
	}
	sum = 0
	for _, p := range r.Lottery.Prizes {
		sum += p.Permille
	}
	if sum >= 1000 {
		return fmt.Errorf("economy.lottery.prizes must leave room for 꽝, got %d‰", sum)
	}
	return nil
}

func splitKey(key string) (tier, rarity string, ok bool) {
	for i := range key {
		if key[i] == '/' {
			return key[:i], key[i+1:], true
		}
	}
	return key, "", false
}
