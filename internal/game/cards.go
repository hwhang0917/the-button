package game

// Card identifies one collectible. Tier and rarity together are the card's
// identity: with the default ladders that is 6 × 4 = 24 distinct cards.
type Card struct {
	Tier   string `json:"tier"`
	Rarity string `json:"rarity"`
}

// Key is the "tier/rarity" lookup key used by Rules.Cards and config.yml.
func (c Card) Key() string { return c.Tier + "/" + c.Rarity }

// CardEffect is one card's talisman payload. A card is armed from the
// collection and fires on the very next click whatever the outcome — there is
// no tier gate, so the decision is purely *when* to spend it. The zero value is
// inert, which is why each entry in DefaultCards names only the fields it uses.
type CardEffect struct {
	Chance    int  `yaml:"chance"    json:"chance,omitempty"`    // + roll chance (the roll only, never the payout)
	Guarantee bool `yaml:"guarantee" json:"guarantee,omitempty"` // the roll always wins; the payout drops to the safe-mode rate
	Bonus     int  `yaml:"bonus"     json:"bonus,omitempty"`     // + flat stars on success
	Mult      int  `yaml:"mult"      json:"mult,omitempty"`      // × star gain (0 means 1)
	MaxRisk   bool `yaml:"max_risk"  json:"maxRisk,omitempty"`   // settle the payout as if risk == MaxRisk
	TierJump  bool `yaml:"tier_jump" json:"tierJump,omitempty"`  // success lands at least on the next tier's first star
	BestJump  bool `yaml:"best_jump" json:"bestJump,omitempty"`  // success lands at least on the personal best
	Keep      bool `yaml:"keep"      json:"keep,omitempty"`      // fail keeps every star
	Half      bool `yaml:"half"      json:"half,omitempty"`      // fail keeps ceil(stars/2)
	Rerolls   int  `yaml:"rerolls"   json:"rerolls,omitempty"`   // extra rolls granted on a fail
	Refund    bool `yaml:"refund"    json:"refund,omitempty"`    // the click doesn't consume quota
	CoinWin   int  `yaml:"coin_win"  json:"coinWin,omitempty"`   // 💰 per star held after a success
	CoinLoss  int  `yaml:"coin_loss" json:"coinLoss,omitempty"`  // 💰 per star lost to a fail
	Card      bool `yaml:"card"      json:"card,omitempty"`      // a free pack card on success
}

// Armed reports whether anything is actually attached — the zero effect means
// an empty talisman slot.
func (e CardEffect) Armed() bool { return e != CardEffect{} }

// DefaultCards is the shipped card design, keyed "tier/rarity". Rarity is the
// mechanic line — 행운 (roll chance), 결실 (bonus stars + coins), 수호 (fail
// protection), 기적 (guarantee + riders) — and tier is the celestial family
// (별먼지→은하) that strictly increments the line, so power climbs on both
// axes: tier is the small step, rarity the big jump.
//
// Guarantee must never pair with Mult (Rules.Validate rejects it): a guaranteed
// win settles at the safe-mode rate, because settling it at the risk-scaled
// rate would hand out round(100/2)=50 stars plus the overflow jackpot on every
// ★14 risk-3 click, farmable forever. Mult/MaxRisk/TierJump/BestJump no longer
// appear in the defaults but stay supported for config.yml overrides.
func DefaultCards() map[string]CardEffect {
	return map[string]CardEffect{
		// 커먼 — 행운: the roll gets easier, nothing else
		"unrank/common":   {Chance: 2},  // ✨ 별먼지 행운
		"bronze/common":   {Chance: 4},  // 💫 별조각 행운
		"silver/common":   {Chance: 6},  // ⭐ 별빛 행운
		"gold/common":     {Chance: 8},  // 🌙 달빛 행운
		"platinum/common": {Chance: 10}, // ☀️ 태양 행운
		"diamond/common":  {Chance: 12}, // 🌌 은하 행운

		// 레어 — 결실: extra stars, then coins per star held
		"unrank/rare":   {Bonus: 1},             // 🌾 별먼지 결실
		"bronze/rare":   {Bonus: 1, CoinWin: 1}, // 🌰 별조각 결실
		"silver/rare":   {Bonus: 2, CoinWin: 1}, // 🌻 별빛 결실
		"gold/rare":     {Bonus: 2, CoinWin: 2}, // 👑 달빛 결실
		"platinum/rare": {Bonus: 3, CoinWin: 2}, // 🏆 태양 결실
		"diamond/rare":  {Bonus: 3, CoinWin: 3}, // 💎 은하 결실

		// 홀로 — 수호: half saves grow into full keeps with rerolls on top
		"unrank/holo":   {Half: true},               // 🍃 별먼지 수호
		"bronze/holo":   {Half: true, CoinLoss: 2},  // 🛡️ 별조각 수호
		"silver/holo":   {Keep: true},               // 🏰 별빛 수호
		"gold/holo":     {Keep: true, Refund: true}, // 🦉 달빛 수호
		"platinum/holo": {Keep: true, Rerolls: 1},   // 🔥 태양 수호
		"diamond/holo":  {Keep: true, Rerolls: 2},   // 🐉 은하 수호

		// 프리즘 — 기적: a guaranteed win with an ever-larger dowry
		"unrank/prismatic":   {Guarantee: true},                         // 🌠 별먼지 기적
		"bronze/prismatic":   {Guarantee: true, Bonus: 1},               // 🎆 별조각 기적
		"silver/prismatic":   {Guarantee: true, Bonus: 2},               // 🌟 별빛 기적
		"gold/prismatic":     {Guarantee: true, Bonus: 3},               // ⚡ 달빛 기적
		"platinum/prismatic": {Guarantee: true, Bonus: 3, Refund: true}, // 🌈 태양 기적
		"diamond/prismatic":  {Guarantee: true, Bonus: 4, Card: true},   // 🪐 은하 기적
	}
}

// EffectFor is the armed card's payload; an unknown or empty pair is inert.
func (r Rules) EffectFor(tier, rarity string) CardEffect {
	return r.Cards[Card{Tier: tier, Rarity: rarity}.Key()]
}
