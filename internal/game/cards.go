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
// power band — the six tier variants inside a band are comparable in strength
// and differ in flavour, so "any prismatic" is the jackpot and *which*
// prismatic is the collection chase.
//
// Guarantee must never pair with Mult (Rules.Validate rejects it): a guaranteed
// win settles at the safe-mode rate, because settling it at the risk-scaled
// rate would hand out round(100/2)=50 stars plus the overflow jackpot on every
// ★14 risk-3 click, farmable forever.
func DefaultCards() map[string]CardEffect {
	return map[string]CardEffect{
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
}

// EffectFor is the armed card's payload; an unknown or empty pair is inert.
func (r Rules) EffectFor(tier, rarity string) CardEffect {
	return r.Cards[Card{Tier: tier, Rarity: rarity}.Key()]
}
