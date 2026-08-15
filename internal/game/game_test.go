package game

import "testing"

// alwaysLose is the mirror of AlwaysWin: every percentage roll fails. Together
// they replace the old "loop 200 times and hope a 2% roll lands" patterns with
// exact assertions.
type alwaysLose struct{ CryptoRNG }

func (alwaysLose) Pct(int) bool { return false }

// seqRNG plays a scripted sequence of roll outcomes, then loses forever. Used
// to drive the reroll cards down an exact path.
type seqRNG struct {
	outcomes []bool
	i        int
}

func (s *seqRNG) Pct(int) bool {
	if s.i >= len(s.outcomes) {
		return false
	}
	v := s.outcomes[s.i]
	s.i++
	return v
}

func (s *seqRNG) Intn(n int) int { return CryptoRNG{}.Intn(n) }

// withRNG is the usual fixture: the shipped rules with chance under test control.
func withRNG(rng RNG) Rules {
	r := Default()
	r.RNG = rng
	return r
}

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("the shipped rules must pass validation: %v", err)
	}
}

func TestChanceTableMonotonic(t *testing.T) {
	table := Default().ChanceTable
	for i := 1; i < len(table); i++ {
		if table[i] >= table[i-1] {
			t.Errorf("chance_table[%d]=%d is not below chance_table[%d]=%d", i, table[i], i-1, table[i-1])
		}
	}
	if table[0] != 100 {
		t.Errorf("first click must be guaranteed, got %d", table[0])
	}
}

func TestResolveBounds(t *testing.T) {
	r := Default()
	for _, cap := range []int{r.MaxStars, 30} {
		for stars := range cap {
			for risk := 0; risk <= r.MaxRisk; risk++ {
				res := r.Resolve(Click{Stars: stars, Risk: risk, Cap: cap})
				if res.Success {
					if res.Stars <= stars || res.Stars > cap {
						t.Fatalf("cap=%d stars=%d risk=%d: success moved to %d", cap, stars, risk, res.Stars)
					}
					// stars always step one; risk pays its reward in coins
					if want := min(stars+1, cap); res.Stars != want {
						t.Fatalf("cap=%d stars=%d risk=%d: gained to %d, want %d", cap, stars, risk, res.Stars, want)
					}
					// no bonus at ★0: nothing is staked at the reset floor
					want := 0
					if stars > 0 {
						want = r.RiskCoinsFor(r.baseChance(stars), risk, 0)
					}
					if res.Jackpot != want {
						t.Fatalf("cap=%d stars=%d risk=%d: jackpot %d, want %d", cap, stars, risk, res.Jackpot, want)
					}
				} else if res.Stars != 0 {
					t.Fatalf("cap=%d stars=%d risk=%d: fail must reset to 0, got %d", cap, stars, risk, res.Stars)
				}
			}
		}
	}
}

func TestMaxStarsFor(t *testing.T) {
	r := Default()
	// the skin cap bounds the star cap: laps past it are constant-cost
	for prestige, want := range map[int]int{0: 15, 1: 20, 2: 25, 3: 30, 9: 30} {
		if got := r.MaxStarsFor(prestige); got != want {
			t.Errorf("MaxStarsFor(%d) = %d, want %d", prestige, got, want)
		}
	}
	// past the table the odds keep tightening — one point per 5-star lap,
	// floored at 1% so every star below the cap stays winnable
	for stars, want := range map[int]int{15: 5, 19: 5, 20: 4, 25: 3, 30: 2, 35: 1, 60: 1} {
		if got := r.ChanceFor(stars, 0, 100); got != want {
			t.Errorf("ChanceFor(%d, 0, 100) = %d, want %d", stars, got, want)
		}
	}
	// the risk division clamps to 1% too, never 0 below the cap
	if got := r.ChanceFor(40, 3, 100); got != 1 {
		t.Errorf("ChanceFor(40, 3, 100) = %d, want 1", got)
	}
	if got := r.ChanceFor(30, 0, 30); got != 0 {
		t.Errorf("chance at the cap must be 0, got %d", got)
	}
}

func TestRiskCoinsPayTheOddsBack(t *testing.T) {
	// odds paid back as coins at the star→coin rate (OverflowCoinPer = 5)
	r := Default()
	cases := []struct{ base, risk, charm, want int }{
		{100, 0, 0, 0}, // safe mode pays no coin bonus
		{5, 0, 0, 0},
		{100, 1, 0, 10},  // eff 50% → 2 × ω
		{99, 2, 0, 15},   // eff 33% → 3 × ω
		{100, 3, 0, 20},  // eff 25% → 4 × ω
		{81, 3, 0, 25},   // the early-game gamble: 81% ÷4 → 20% pays 25
		{20, 1, 0, 50},   // eff 10% → 10 × ω
		{8, 3, 0, 250},   // eff 2% → 50 × ω
		{0, 3, 0, 0},     // a dead chance still cannot divide by zero
		// the roll clamps both to 1%, but the payout keeps the odds apart
		{3, 2, 0, 500},
		{3, 3, 0, 665},
		// charm raises the effective odds, trading payout for survival
		{5, 1, 6, 60}, // eff 2.5+6 = 8.5% → round(100/8.5) = 12 × ω
	}
	for _, c := range cases {
		if got := r.RiskCoinsFor(c.base, c.risk, c.charm); got != c.want {
			t.Errorf("RiskCoinsFor(%d, %d, %d) = %d, want %d", c.base, c.risk, c.charm, got, c.want)
		}
	}
}

func TestFirstClickAlwaysSucceeds(t *testing.T) {
	// ★0 is a 100% roll, so even an RNG that refuses everything must succeed
	r := withRNG(alwaysLose{})
	if res := r.Resolve(Click{Cap: r.MaxStars}); !res.Success || res.Stars != 1 {
		t.Fatalf("100%% click failed: %+v", res)
	}
}

func TestTierBoundaries(t *testing.T) {
	r := Default()
	want := map[int]string{0: "unrank", 1: "bronze", 3: "bronze", 4: "silver",
		7: "gold", 10: "platinum", 13: "diamond", 15: "diamond"}
	for stars, tier := range want {
		if got := r.TierFor(stars, r.MaxStars); got != tier {
			t.Errorf("TierFor(%d, base) = %q, want %q", stars, got, tier)
		}
	}
	// a prestige-doubled cap stretches every threshold in ratio
	scaled := map[int]string{0: "unrank", 1: "unrank", 2: "bronze", 7: "bronze",
		8: "silver", 14: "gold", 20: "platinum", 25: "platinum", 26: "diamond", 30: "diamond"}
	for stars, tier := range scaled {
		if got := r.TierFor(stars, 30); got != tier {
			t.Errorf("TierFor(%d, 30) = %q, want %q", stars, got, tier)
		}
	}
}

func TestTierRankLadder(t *testing.T) {
	r := Default()
	for i, tier := range r.Tiers {
		if got := r.TierRank(tier.Name); got != i {
			t.Errorf("TierRank(%q) = %d, want %d", tier.Name, got, i)
		}
	}
}

func TestNextTierMin(t *testing.T) {
	r := Default()
	for stars, want := range map[int]int{0: 1, 1: 4, 5: 7, 9: 10, 12: 13, 13: 0, 20: 0} {
		if got := r.NextTierMin(stars, r.MaxStars); got != want {
			t.Errorf("NextTierMin(%d, base) = %d, want %d", stars, got, want)
		}
	}
	// thresholds scale with the cap: at 30, silver starts at 8, diamond at 26
	for stars, want := range map[int]int{1: 2, 2: 8, 20: 26, 26: 0} {
		if got := r.NextTierMin(stars, 30); got != want {
			t.Errorf("NextTierMin(%d, 30) = %d, want %d", stars, got, want)
		}
	}
}

func TestStreakValue(t *testing.T) {
	cases := []struct{ stars, floor, want int }{
		{0, 0, 0},
		{5, 0, 15},
		{5, 3, 9}, // only the stars above the floor pay
		{3, 5, 0}, // below the floor there is nothing to sell
		{15, 0, 120},
	}
	for _, c := range cases {
		if got := StreakValue(c.stars, c.floor); got != c.want {
			t.Errorf("StreakValue(%d, %d) = %d, want %d", c.stars, c.floor, got, c.want)
		}
	}
}

func TestHeadstartFloor(t *testing.T) {
	r := withRNG(alwaysLose{})
	res := r.Resolve(Click{Stars: 14, Risk: r.MaxRisk, Cap: r.MaxStars, Headstart: 3})
	if res.Success || res.Stars != 3 {
		t.Fatalf("a fail should land on the floor: %+v", res)
	}
	// below the floor a fail must never gain stars
	res = r.Resolve(Click{Stars: 1, Risk: r.MaxRisk, Cap: r.MaxStars, Headstart: 3})
	if res.Stars > 1 {
		t.Fatalf("fail below the floor gained stars: %d", res.Stars)
	}
}

func TestEffChance(t *testing.T) {
	r := Default()
	if got := r.EffChanceFor(0, 0, r.Charm.Cap(), r.MaxStars); got != 100 {
		t.Errorf("charm must not push past 100, got %d", got)
	}
	// charm adds after the risk division
	if got := r.EffChanceFor(4, 1, 2, r.MaxStars); got != 63/2+4 {
		t.Errorf("EffChanceFor(4, 1, 2) = %d, want %d", got, 63/2+4)
	}
	if got := r.EffChanceFor(r.MaxStars, 0, 0, r.MaxStars); got != 0 {
		t.Errorf("a maxed streak has no chance left, got %d", got)
	}
}

func TestCardSell(t *testing.T) {
	r := Default()
	if v, ok := r.SellValueFor("holo"); !ok || v != 18 {
		t.Fatalf("holo sell = %d %v, want 18", v, ok)
	}
	if _, ok := r.SellValueFor("mythic"); ok {
		t.Fatal("an unknown rarity must not price")
	}
	r.CardSell = []int{1, 2}
	if err := r.Validate(); err == nil {
		t.Fatal("a card_sell shorter than the rarity ladder must fail validation")
	}
}

func TestPriceLadders(t *testing.T) {
	r := Default()
	for _, s := range []Skill{r.Charm, r.Headstart, r.Stamina, r.Magnet, r.Golden} {
		for i := 1; i < len(s.Prices); i++ {
			if s.Prices[i] <= s.Prices[i-1] {
				t.Errorf("prices must ascend: %v", s.Prices)
			}
		}
		if _, ok := s.PriceAt(s.Cap()); ok {
			t.Error("a capped skill must not price another level")
		}
		if _, ok := s.PriceAt(0); !ok {
			t.Error("level 0 must be purchasable")
		}
	}
	if _, ok := r.PriceFor("nonesuch", 0); ok {
		t.Error("an unknown skill must not price")
	}
}

// TestSkillProcs pins the magnet/golden success riders: the procs fire on
// their scripted rolls, pay a card / newStars coins, and at level 0 consume no
// RNG at all — the guarantee that lets pre-skill scripted tests keep passing.
func TestSkillProcs(t *testing.T) {
	click := Click{Stars: 3, Cap: 15, Magnet: 3, Golden: 3}

	seq := &seqRNG{outcomes: []bool{true, true, true}} // click, magnet, golden
	res := withRNG(seq).Resolve(click)
	if !res.Success || res.Card == nil || res.Jackpot != res.Stars {
		t.Errorf("both procs should fire: card=%v jackpot=%d (want %d)", res.Card, res.Jackpot, res.Stars)
	}

	seq = &seqRNG{outcomes: []bool{true}} // procs bought but both rolls miss
	res = withRNG(seq).Resolve(click)
	if res.Card != nil || res.Jackpot != 0 {
		t.Errorf("missed procs must pay nothing: card=%v jackpot=%d", res.Card, res.Jackpot)
	}

	seq = &seqRNG{outcomes: []bool{true, true, true}}
	res = withRNG(seq).Resolve(Click{Stars: 3, Cap: 15}) // skills at level 0
	if res.Card != nil || res.Jackpot != 0 {
		t.Errorf("level 0 must not proc: card=%v jackpot=%d", res.Card, res.Jackpot)
	}
	if seq.i != 1 {
		t.Errorf("level 0 consumed %d rolls, want 1 — extra rolls would shift every scripted sequence", seq.i)
	}
}

func TestPrestigeReward(t *testing.T) {
	r := Default()
	// past the ladder every prismatic lap pays the top reward
	for prestige, want := range map[int]int{0: 300, 1: 450, 2: 600, 3: 600, 12: 600} {
		if got := r.PrestigeRewardFor(prestige); got != want {
			t.Errorf("PrestigeRewardFor(%d) = %d, want %d", prestige, got, want)
		}
	}
}

func TestLotteryTable(t *testing.T) {
	r := Default()
	total := 0
	for i, p := range r.Lottery.Prizes {
		total += p.Permille
		if i > 0 && p.Prize >= r.Lottery.Prizes[i-1].Prize {
			t.Errorf("prizes must descend: %v", r.Lottery.Prizes)
		}
	}
	if total >= 1000 {
		t.Fatalf("win chances must leave room for 꽝, got %d‰", total)
	}
	valid := map[int]bool{0: true}
	for _, p := range r.Lottery.Prizes {
		valid[p.Prize] = true
	}
	for range 2000 {
		if prize := r.RollLottery(); !valid[prize] {
			t.Fatalf("lottery paid an off-table prize: %d", prize)
		}
	}
}

func TestPackTables(t *testing.T) {
	r := Default()
	diamonds, prismatics := 0, 0
	const runs = 10000
	for range runs {
		c := r.RollPack()
		if !r.ValidTier(c.Tier) || !r.ValidRarity(c.Rarity) {
			t.Fatalf("rolled an invalid card %s/%s", c.Tier, c.Rarity)
		}
		if c.Tier == "diamond" {
			diamonds++
		}
		if c.Rarity == "prismatic" {
			prismatics++
		}
	}
	// 4% / 3% expected; generous non-flaky bounds
	if diamonds > 1000 || prismatics > 800 {
		t.Errorf("high-end draws suspiciously common: diamond %d prismatic %d", diamonds, prismatics)
	}
}

func TestRollPackCards(t *testing.T) {
	r := Default()
	total := 0
	const runs = 10000
	for range runs {
		drawn := r.RollPackCards()
		if len(drawn) < 1 || len(drawn) > 1+len(r.Pack.BonusPct) {
			t.Fatalf("pack drew %d cards", len(drawn))
		}
		total += len(drawn)
	}
	// 1 + 0.45 + 0.20 = 1.65 expected; wide bounds keep this non-flaky
	if mean := float64(total) / runs; mean < 1.55 || mean > 1.75 {
		t.Errorf("pack averages %.2f cards, want ~1.65", mean)
	}
}

func TestRarityLadder(t *testing.T) {
	r := Default()
	for _, c := range []struct{ from, want string }{
		{"common", "rare"}, {"rare", "holo"}, {"holo", "prismatic"},
	} {
		if got, ok := r.NextRarity(c.from); !ok || got != c.want {
			t.Errorf("NextRarity(%q) = %q, %v", c.from, got, ok)
		}
		if got, ok := r.PrevRarity(c.want); !ok || got != c.from {
			t.Errorf("PrevRarity(%q) = %q, %v", c.want, got, ok)
		}
	}
	if _, ok := r.NextRarity("prismatic"); ok {
		t.Error("the top rarity must not fuse further")
	}
	if _, ok := r.PrevRarity("common"); ok {
		t.Error("the bottom rarity must not defuse further")
	}
}

// TestCardEffectTable guards the design invariants of the card set rather than
// any single card's numbers. Validate enforces the same rules at boot, so this
// also covers a hand-edited config.yml.
func TestCardEffectTable(t *testing.T) {
	r := Default()
	for _, tier := range r.Tiers {
		for _, rarity := range r.Rarities {
			key := Card{Tier: tier.Name, Rarity: rarity}.Key()
			e, ok := r.Cards[key]
			if !ok {
				t.Errorf("no effect defined for %s", key)
				continue
			}
			if !e.Armed() {
				t.Errorf("%s is inert — every card must do something", key)
			}
			if e.Guarantee && e.Mult != 0 {
				t.Errorf("%s pairs Guarantee with Mult", key)
			}
			if e.Chance > 20 {
				t.Errorf("%s chance bonus %d exceeds the +20 ceiling", key, e.Chance)
			}
		}
	}
	if len(r.Cards) != len(r.Tiers)*len(r.Rarities) {
		t.Fatalf("cards has %d entries, want %d", len(r.Cards), len(r.Tiers)*len(r.Rarities))
	}
	if r.EffectFor("", "").Armed() {
		t.Fatal("an empty talisman slot must resolve to no effect")
	}
}

func TestResolveEffects(t *testing.T) {
	base := Default()
	win := withRNG(AlwaysWin{})
	lose := withRNG(alwaysLose{})

	// Chance boosts only the roll — never the risk-mode coin payout
	want := base.RiskCoinsFor(base.baseChance(5), 1, 0)
	res := win.Resolve(Click{Stars: 5, Risk: 1, Cap: win.MaxStars, Card: base.Cards["unrank/common"]})
	if !res.TalismanUsed {
		t.Fatal("an armed card must burn on any outcome")
	}
	if res.Gained != 1 || res.Jackpot != want {
		t.Fatalf("a chance card must not change the payout: gained %d jackpot %d, want 1/%d", res.Gained, res.Jackpot, want)
	}
	if res = lose.Resolve(Click{Stars: 5, Risk: 1, Cap: lose.MaxStars, Card: base.Cards["unrank/common"]}); !res.TalismanUsed {
		t.Fatal("an armed card must burn on a fail too")
	}

	// Guarantee settles at the safe-mode rate even at max risk. This is the
	// exploit regression: paying the risk coin bonus on a click nobody had to
	// win would print ~50 coins per guarantee card.
	res = lose.Resolve(Click{Stars: 14, Risk: lose.MaxRisk, Cap: lose.MaxStars, Card: base.Cards["unrank/prismatic"]})
	if !res.Success || res.Gained != 1 || res.Jackpot != 0 {
		t.Fatalf("a guaranteed win must gain exactly 1 with no jackpot: %+v", res)
	}

	// 기적 line: guarantee + a bonus that climbs with the tier
	for key, want := range map[string]int{
		"bronze/prismatic": 2, "silver/prismatic": 3, "gold/prismatic": 4, "diamond/prismatic": 5,
	} {
		res := lose.Resolve(Click{Risk: lose.MaxRisk, Cap: lose.MaxStars, Card: base.Cards[key]})
		if !res.Success || res.Gained != want {
			t.Fatalf("%s gained %d, want %d", key, res.Gained, want)
		}
	}
	if res = lose.Resolve(Click{Cap: lose.MaxStars, Card: base.Cards["diamond/prismatic"]}); res.Card == nil {
		t.Fatal("은하 기적 must drop a card on success")
	}

	// Keep holds every star on a fail — and only keep earns the shield message
	if res = lose.Resolve(Click{Stars: 14, Cap: lose.MaxStars, Card: base.Cards["silver/holo"]}); res.Success || res.Stars != 14 || !res.Saved {
		t.Fatalf("별빛 수호 must keep the streak and report the save: %+v", res)
	}
	// A chance-only card failing at the head-start floor leaves the stars
	// untouched anyway; that must not read as a shield save
	if res = lose.Resolve(Click{Stars: 3, Headstart: 3, Cap: lose.MaxStars, Card: base.Cards["unrank/common"]}); res.Stars != 3 || res.Saved {
		t.Fatalf("a chance card at the floor must not claim a save: %+v", res)
	}
	// Half rounds up, and never lands below the head-start floor
	if res = lose.Resolve(Click{Stars: 7, Cap: lose.MaxStars, Card: base.Cards["unrank/holo"]}); res.Stars != 4 {
		t.Fatalf("별먼지 수호 on ★7 should land on ★4, got %d", res.Stars)
	}
	if res = lose.Resolve(Click{Stars: 4, Cap: lose.MaxStars, Headstart: 3, Card: base.Cards["unrank/holo"]}); res.Stars != 3 {
		t.Fatalf("별먼지 수호 must not land below the floor, got %d", res.Stars)
	}
	// CoinLoss pays per star surrendered: half of ★10 keeps 5, pays 2×5
	if res = lose.Resolve(Click{Stars: 10, Cap: lose.MaxStars, Card: base.Cards["bronze/holo"]}); res.Stars != 5 || res.Jackpot != 2*5 {
		t.Fatalf("별조각 수호 should pay 2 per lost star: %+v", res)
	}
	// CoinWin pays per star held after the win (bonus stars included)
	if res = win.Resolve(Click{Cap: win.MaxStars, Card: base.Cards["diamond/rare"]}); res.Gained != 1+3 || res.Jackpot != 3*4 {
		t.Fatalf("은하 결실 should pay 3 per star held: %+v", res)
	}

	// Rerolls: 은하 수호 gets two extra rolls, so a win on the third lands and
	// a fourth-roll win is already too late — and keep still covers a full miss
	r := withRNG(&seqRNG{outcomes: []bool{false, false, true}})
	if res = r.Resolve(Click{Stars: 14, Risk: r.MaxRisk, Cap: r.MaxStars, Card: base.Cards["diamond/holo"]}); !res.Success {
		t.Fatal("은하 수호 should convert a fail on its second reroll")
	}
	r = withRNG(&seqRNG{outcomes: []bool{false, false, false, true}})
	if res = r.Resolve(Click{Stars: 14, Risk: r.MaxRisk, Cap: r.MaxStars, Card: base.Cards["diamond/holo"]}); res.Success || res.Stars != 14 {
		t.Fatal("은하 수호 must stop after two rerolls and keep the streak")
	}

	// Refund is reported so the handler can hand the click back
	if res = win.Resolve(Click{Cap: win.MaxStars, Card: base.Cards["gold/holo"]}); !res.Refund {
		t.Fatalf("달빛 수호 must refund the click: %+v", res)
	}

	// Retired-from-defaults mechanics stay supported for config overrides
	if res = win.Resolve(Click{Cap: win.MaxStars, Card: CardEffect{Mult: 2}}); res.Gained != 2 {
		t.Fatalf("×2 mult: %+v", res)
	}
	if res = win.Resolve(Click{Stars: 1, Cap: win.MaxStars, Card: CardEffect{TierJump: true}}); res.Stars != 4 {
		t.Fatalf("tier jump from ★1 should reach silver at ★4, got %d", res.Stars)
	}
	if res = lose.Resolve(Click{Cap: lose.MaxStars, Best: 9, Card: CardEffect{Guarantee: true, BestJump: true}}); res.Stars != 9 {
		t.Fatalf("best jump should restore the personal best, got %d", res.Stars)
	}
	if res = lose.Resolve(Click{Stars: 6, Cap: lose.MaxStars, Best: 2, Card: CardEffect{Guarantee: true, BestJump: true}}); res.Stars != 7 {
		t.Fatalf("best jump must never cut a streak short, got %d", res.Stars)
	}
	// MaxRisk keeps the safe click's odds but settles at the max-risk coin
	// payout — and obeys the stake rule: the guaranteed ★0 click pays nothing
	if res = lose.Resolve(Click{Cap: lose.MaxStars, Card: CardEffect{MaxRisk: true}}); !res.Success || res.Gained != 1 || res.Jackpot != 0 {
		t.Fatalf("★0 max-risk click must succeed with no bonus (nothing staked): %+v", res)
	}
	want = base.RiskCoinsFor(base.baseChance(3), base.MaxRisk, 0)
	if res = win.Resolve(Click{Stars: 3, Cap: win.MaxStars, Card: CardEffect{MaxRisk: true}}); res.Jackpot != want {
		t.Fatalf("max-risk card should settle at the max-risk coin payout %d: %+v", want, res)
	}
}

// TestQuotaForScalesWithBase is the point of the stamina skill: the bonus has to
// stay proportional to whatever quota the server is configured for, because a
// flat "+5 clicks" is a rounding error at 120 and doubles the game at 3.
func TestQuotaForScalesWithBase(t *testing.T) {
	for _, base := range []int{1, 3, 10, 40, 120} {
		r := Default()
		r.Quota = base
		prev := r.QuotaFor(0)
		if prev != base {
			t.Fatalf("base=%d: level 0 must be the plain quota, got %d", base, prev)
		}
		for lvl := 1; lvl <= r.Stamina.Cap(); lvl++ {
			q := r.QuotaFor(lvl)
			if q <= prev {
				t.Errorf("base=%d level=%d: %d does not beat %d — every level must buy something", base, lvl, q, prev)
			}
			prev = q
		}
		// ~1.25^5 = 3.05x, with rounding and the +1 floor pulling small bases up
		if ratio := float64(prev) / float64(base); ratio < 2.9 {
			t.Errorf("base=%d: maxed to %d, only %.2fx — the ladder should roughly triple", base, prev, ratio)
		}
	}
	// levels beyond the ladder must not keep compounding
	r := Default()
	if over, capped := r.QuotaFor(r.Stamina.Cap()+9), r.QuotaFor(r.Stamina.Cap()); over != capped {
		t.Errorf("past the cap quota kept growing: %d vs %d", over, capped)
	}
	if r.QuotaFor(-1) != r.Quota {
		t.Error("a negative level must fall back to the base quota")
	}
}
