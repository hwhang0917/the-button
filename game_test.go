package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChanceTableMonotonic(t *testing.T) {
	for i := 1; i < len(chanceTable); i++ {
		if chanceTable[i] >= chanceTable[i-1] {
			t.Errorf("chanceTable[%d]=%d not below chanceTable[%d]=%d", i, chanceTable[i], i-1, chanceTable[i-1])
		}
	}
	if chanceTable[0] != 100 {
		t.Errorf("first click must be guaranteed, got %d", chanceTable[0])
	}
}

func TestResolveClickBounds(t *testing.T) {
	for _, cap := range []int{maxStars, 30} {
		for stars := 0; stars < cap; stars++ {
			for risk := 0; risk <= maxRisk; risk++ {
				res := resolveClick(stars, risk, skills{}, cap, 0)
				if res.Success {
					if res.Stars <= stars || res.Stars > cap {
						t.Errorf("cap=%d stars=%d risk=%d: success moved to %d", cap, stars, risk, res.Stars)
					}
					gain := gainFor(chanceFor(stars, risk, cap), risk)
					if want := min(stars+gain, cap); res.Stars != want {
						t.Errorf("cap=%d stars=%d risk=%d: gained to %d, want %d", cap, stars, risk, res.Stars, want)
					}
					if want := overflowCoinPer * max(0, stars+gain-cap); res.Jackpot != want {
						t.Errorf("cap=%d stars=%d risk=%d: jackpot %d, want %d", cap, stars, risk, res.Jackpot, want)
					}
				} else if res.Stars != 0 {
					t.Errorf("cap=%d stars=%d risk=%d: fail must reset to 0, got %d", cap, stars, risk, res.Stars)
				}
			}
		}
	}
}

func TestMaxStarsFor(t *testing.T) {
	for prestige, want := range map[int]int{0: 15, 1: 20, 2: 25, 3: 30, 9: 30} {
		if got := maxStarsFor(prestige); got != want {
			t.Errorf("maxStarsFor(%d) = %d, want %d", prestige, got, want)
		}
	}
	// veteran zone rolls at the table's 5%% floor, never 0, until the cap
	for stars := 15; stars < 30; stars++ {
		if got := chanceFor(stars, 0, 30); got != 5 {
			t.Errorf("chanceFor(%d, 0, 30) = %d, want 5", stars, got)
		}
	}
	if got := chanceFor(30, 0, 30); got != 0 {
		t.Errorf("chance at the cap must be 0, got %d", got)
	}
}

func TestGainForPaysTheOddsBack(t *testing.T) {
	cases := []struct{ chance, risk, want int }{
		{100, 0, 1}, // safe mode is always one star
		{5, 0, 1},
		{50, 1, 2},
		{33, 2, 3},
		{25, 3, 4},
		{10, 1, 10},
		{1, 3, 100},
	}
	for _, c := range cases {
		if got := gainFor(c.chance, c.risk); got != c.want {
			t.Errorf("gainFor(%d, %d) = %d, want %d", c.chance, c.risk, got, c.want)
		}
	}
}

func TestFirstClickAlwaysSucceeds(t *testing.T) {
	for i := 0; i < 50; i++ {
		if res := resolveClick(0, 0, skills{}, maxStars, 0); !res.Success || res.Stars != 1 {
			t.Fatalf("100%% click failed: %+v", res)
		}
	}
}

func TestTierBoundaries(t *testing.T) {
	want := map[int]string{0: "unrank", 1: "bronze", 3: "bronze", 4: "silver",
		7: "gold", 10: "platinum", 13: "diamond", 15: "diamond"}
	for stars, tier := range want {
		if got := tierFor(stars); got != tier {
			t.Errorf("tierFor(%d) = %q, want %q", stars, got, tier)
		}
	}
}

func TestCacheHeaders(t *testing.T) {
	h := cacheHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cases := map[string]string{
		"/assets/index-abc123.js": cacheForever,
		"/star.png":               cacheDaily,
		"/click.wav":              cacheDaily,
		"/":                       cacheNever,
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if got := rec.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control = %q, want %q", path, got, want)
		}
	}
}

func TestQuota(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	const limit = 3
	for i := 0; i < limit; i++ {
		if _, err := s.consumeQuota("tok", limit); err != nil {
			t.Fatalf("click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.consumeQuota("tok", limit); err != errQuotaExceeded {
		t.Fatalf("expected quota exceeded, got %v", err)
	}
	if _, err := s.consumeQuota("othertok", limit); err != nil {
		t.Fatalf("other player should have their own quota: %v", err)
	}
	if err := s.grantQuota("tok", 2); err != nil {
		t.Fatalf("grant failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := s.consumeQuota("tok", limit); err != nil {
			t.Fatalf("granted click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.consumeQuota("tok", limit); err != errQuotaExceeded {
		t.Fatalf("expected quota exceeded after spending grant, got %v", err)
	}
}

func TestStreakValue(t *testing.T) {
	cases := []struct{ stars, floor, want int }{
		{3, 0, 6},
		{15, 0, 120},
		{5, 3, 9},  // only the stars above the floor pay
		{3, 3, 0},  // at the floor: nothing to sell
		{2, 3, 0},  // below the floor must not go negative
		{0, 0, 0},
	}
	for _, c := range cases {
		if got := streakValue(c.stars, c.floor); got != c.want {
			t.Errorf("streakValue(%d, %d) = %d, want %d", c.stars, c.floor, got, c.want)
		}
	}
}

func TestHeadstartFloor(t *testing.T) {
	sawFail := false
	for i := 0; i < 100; i++ {
		// risk 3 at ★14 → 2% odds: fails are near-certain
		if res := resolveClick(14, maxRisk, skills{Headstart: 3}, maxStars, 0); !res.Success {
			sawFail = true
			if res.Stars != 3 {
				t.Fatalf("fail should land at the floor, got %d", res.Stars)
			}
		}
		// below the floor a fail must never gain stars
		if res := resolveClick(1, maxRisk, skills{Headstart: 3}, maxStars, 0); !res.Success && res.Stars > 1 {
			t.Fatalf("fail below floor gained stars: %d", res.Stars)
		}
	}
	if !sawFail {
		t.Fatal("no fails observed at 2% odds")
	}
}

func TestEffChance(t *testing.T) {
	if got := effChanceFor(0, 0, charmCap, maxStars); got != 100 {
		t.Errorf("charm must cap at 100, got %d", got)
	}
	base, charmed := effChanceFor(9, 1, 0, maxStars), effChanceFor(9, 1, charmCap, maxStars)
	if charmed <= base {
		t.Fatalf("charm should raise chance: %d vs %d", charmed, base)
	}
	if gainFor(charmed, 1) >= gainFor(base, 1) {
		t.Errorf("higher chance must pay fewer stars: %d vs %d", gainFor(charmed, 1), gainFor(base, 1))
	}
}

func TestPriceLadders(t *testing.T) {
	for _, ladder := range [][]int{charmPrices[:], headstartPrices[:]} {
		for i := 1; i < len(ladder); i++ {
			if ladder[i] <= ladder[i-1] {
				t.Errorf("ladder not increasing at %d: %v", i, ladder)
			}
		}
	}
	if _, ok := priceFor("charm", charmCap); ok {
		t.Error("capped charm must not be purchasable")
	}
	if _, ok := priceFor("nonsense", 0); ok {
		t.Error("unknown skill must not be purchasable")
	}
}

func TestSkillStore(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}

	if ok, _ := s.buySkill("a", "charm_level", 10, 0); ok {
		t.Fatal("buy with 0 coins must fail")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 100 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.buySkill("a", "charm_level", 10, 0); !ok {
		t.Fatal("funded buy failed")
	}
	if ok, _ := s.buySkill("a", "charm_level", 10, 0); ok {
		t.Fatal("stale level pin must reject the buy")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Coins != 90 || p.CharmLevel != 1 {
		t.Fatalf("coins=%d charm=%d after buy", p.Coins, p.CharmLevel)
	}

	if err := s.savePlayerStars("a", 5, 0); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.sellStreak("a", 15, 0, 5); !ok {
		t.Fatal("sell failed")
	}
	if ok, _ := s.sellStreak("a", 15, 0, 5); ok {
		t.Fatal("stale stars pin must reject the sell")
	}
	p, _ = s.getOrCreatePlayer("a")
	if p.Stars != 0 || p.Coins != 105 { // 100 - 10 charm + 15 sale
		t.Fatalf("stars=%d coins=%d after sell", p.Stars, p.Coins)
	}
}

func TestLotteryTable(t *testing.T) {
	total := 0
	for i, e := range lotteryTable {
		total += e.permille
		if i > 0 && e.prize >= lotteryTable[i-1].prize {
			t.Errorf("prizes must descend: %v", lotteryTable)
		}
	}
	if total >= 1000 {
		t.Fatalf("win chances must leave room for 꽝, got %d‰", total)
	}
	valid := map[int]bool{0: true}
	for _, e := range lotteryTable {
		valid[e.prize] = true
	}
	jackpots := 0
	for i := 0; i < 10000; i++ {
		p := rollLottery()
		if !valid[p] {
			t.Fatalf("rolled a prize not in the table: %d", p)
		}
		if p == lotteryTable[0].prize {
			jackpots++
		}
	}
	if jackpots > 500 { // 0.5% expected; 5% is a generous non-flaky bound
		t.Errorf("jackpot suspiciously common: %d/10000", jackpots)
	}
}

func TestPlayLottery(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.playLottery("a", lotteryPrice, 500); ok {
		t.Fatal("broke player must not buy a ticket")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 20 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.playLottery("a", lotteryPrice, 30); !ok {
		t.Fatal("funded ticket rejected")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Coins != 20-lotteryPrice+30 {
		t.Fatalf("coins = %d after win", p.Coins)
	}
}

func TestPackTables(t *testing.T) {
	tierSum, raritySum := 0, 0
	for _, e := range packTierTable {
		tierSum += e.permille
		if !validTier(e.tier) {
			t.Errorf("unknown tier %q", e.tier)
		}
	}
	for _, e := range packRarityTable {
		raritySum += e.permille
		if !validRarity(e.rarity) {
			t.Errorf("unknown rarity %q", e.rarity)
		}
	}
	if tierSum != 1000 || raritySum != 1000 {
		t.Fatalf("pack tables must sum to 1000‰: tiers %d rarities %d", tierSum, raritySum)
	}
	diamonds, prismatics := 0, 0
	for i := 0; i < 10000; i++ {
		tier, rarity := rollPack()
		if !validTier(tier) || !validRarity(rarity) {
			t.Fatalf("rolled invalid card %s/%s", tier, rarity)
		}
		if tier == "diamond" {
			diamonds++
		}
		if rarity == "prismatic" {
			prismatics++
		}
	}
	if diamonds > 1000 || prismatics > 800 { // 4% / 3% expected; generous non-flaky bounds
		t.Errorf("high-end drops suspiciously common: diamond %d prismatic %d", diamonds, prismatics)
	}
}

func TestBuyPackAndRefill(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	drawn := []cardDrop{{Tier: "gold", Rarity: "rare"}, {Tier: "unrank", Rarity: "common"}}
	if ok, _ := s.buyPack("a", packPrice, drawn); ok {
		t.Fatal("broke pack purchase must fail")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 200 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.buyPack("a", packPrice, drawn); !ok {
		t.Fatal("funded pack purchase failed")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Coins != 200-packPrice {
		t.Fatalf("coins = %d after pack", p.Coins)
	}
	cards, _ := s.getCards("a")
	if len(cards) != len(drawn) {
		t.Fatalf("every drawn card must land: %v", cards)
	}
	for _, c := range cards {
		if c.Count != 1 {
			t.Fatalf("card %s/%s count = %d", c.Tier, c.Rarity, c.Count)
		}
	}

	// refill: rejected with no spent clicks, works after spending, once per day
	const day = "2026-08-11"
	if ok, _ := s.refillQuota("a", refillPrice, day); ok {
		t.Fatal("refill with nothing spent must fail")
	}
	if _, err := s.consumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.refillQuota("a", refillPrice, day); !ok {
		t.Fatal("refill failed")
	}
	if used, _ := s.quotaUsed("a"); used != 0 {
		t.Fatalf("quota not reset: used %d", used)
	}
	p, _ = s.getOrCreatePlayer("a")
	if p.Coins != 200-packPrice-refillPrice {
		t.Fatalf("coins = %d after refill", p.Coins)
	}
	if _, err := s.consumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.refillQuota("a", refillPrice, day); ok {
		t.Fatal("second refill on the same day must fail")
	}
	if ok, _ := s.refillQuota("a", refillPrice, "2026-08-12"); !ok {
		t.Fatal("refill on the next day should work")
	}
}

func TestPrestigeReward(t *testing.T) {
	cases := map[int]int{0: 300, 1: 450, 2: 600, 3: 600} // 3 = repeat at the prismatic cap
	for level, want := range cases {
		if got := prestigeRewardFor(level); got != want {
			t.Errorf("prestigeRewardFor(%d) = %d, want %d", level, got, want)
		}
	}
}

func TestPrestigeStore(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.savePlayerStars("a", maxStars, 0); err != nil {
		t.Fatal(err)
	}

	if ok, _ := s.prestigeStreak("a", 300, 0, maxStars); !ok {
		t.Fatal("prestige at max stars failed")
	}
	if ok, _ := s.prestigeStreak("a", 300, 0, maxStars); ok {
		t.Fatal("stale stars pin must reject a repeat prestige")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Prestige != 1 || p.Coins != 300 || p.Stars != 0 {
		t.Fatalf("after prestige: %+v", p)
	}

	// prestige is unbounded: prismatic laps keep counting past the skin cap
	for i := 0; i < 4; i++ {
		if err := s.savePlayerStars("a", maxStars, 0); err != nil {
			t.Fatal(err)
		}
		if ok, _ := s.prestigeStreak("a", prestigeRewardFor(p.Prestige), 0, maxStars); !ok {
			t.Fatalf("prestige round %d failed", i)
		}
		p, _ = s.getOrCreatePlayer("a")
	}
	if p.Prestige != 5 {
		t.Fatalf("prestige must keep counting, got %d", p.Prestige)
	}
}

func TestCancelAndDefuse(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.cancelTalisman("a"); ok {
		t.Fatal("cancel with nothing armed must fail")
	}
	if err := s.addCard("a", "gold", "rare"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.armTalisman("a", "gold", "rare"); !ok {
		t.Fatal("arm failed")
	}
	if ok, _ := s.cancelTalisman("a"); !ok {
		t.Fatal("cancel failed")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.TalismanTier != "" {
		t.Fatal("slot must be cleared after cancel")
	}
	cards, _ := s.getCards("a")
	if len(cards) != 1 || cards[0].Count != 1 {
		t.Fatalf("card must be refunded: %v", cards)
	}

	// defuse: 1 rare -> 2 commons; rejects at 0
	if ok, _ := s.defuseCard("a", "gold", "rare", "common"); !ok {
		t.Fatal("defuse failed")
	}
	byKey := map[string]int{}
	cards, _ = s.getCards("a")
	for _, c := range cards {
		byKey[c.Rarity] = c.Count
	}
	if byKey["rare"] != 0 || byKey["common"] != defuseYield {
		t.Fatalf("defuse counts wrong: %v", byKey)
	}
	if ok, _ := s.defuseCard("a", "gold", "rare", "common"); ok {
		t.Fatal("defuse without a copy must fail")
	}
}

func TestPrevRarity(t *testing.T) {
	want := map[string]string{"rare": "common", "holo": "rare", "prismatic": "holo"}
	for from, to := range want {
		if got, ok := prevRarity(from); !ok || got != to {
			t.Errorf("prevRarity(%q) = %q,%v", from, got, ok)
		}
	}
	if _, ok := prevRarity("common"); ok {
		t.Error("common must not defuse")
	}
}

func TestNextRarity(t *testing.T) {
	want := map[string]string{"common": "rare", "rare": "holo", "holo": "prismatic"}
	for from, to := range want {
		if got, ok := nextRarity(from); !ok || got != to {
			t.Errorf("nextRarity(%q) = %q,%v", from, got, ok)
		}
	}
	if _, ok := nextRarity("prismatic"); ok {
		t.Error("prismatic must not fuse")
	}
	if _, ok := nextRarity("nonsense"); ok {
		t.Error("unknown rarity must not fuse")
	}
}

// TestCardEffectTable guards the design invariants of the 24-card set rather
// than any single card's numbers.
func TestCardEffectTable(t *testing.T) {
	for _, tier := range tiers {
		for _, rarity := range rarities {
			e, ok := cardEffects[tier.Name+"/"+rarity]
			if !ok {
				t.Errorf("no effect defined for %s/%s", tier.Name, rarity)
				continue
			}
			if !e.armed() {
				t.Errorf("%s/%s is inert — every card must do something", tier.Name, rarity)
			}
			// a guaranteed win settles at the safe-mode rate; letting a
			// multiplier ride on top would print stars and overflow coins
			if e.Guarantee && e.Mult != 0 {
				t.Errorf("%s/%s pairs Guarantee with Mult", tier.Name, rarity)
			}
			if e.Chance > 20 {
				t.Errorf("%s/%s chance bonus %d exceeds the +20 ceiling", tier.Name, rarity, e.Chance)
			}
		}
	}
	if len(cardEffects) != len(tiers)*len(rarities) {
		t.Fatalf("cardEffects has %d entries, want %d", len(cardEffects), len(tiers)*len(rarities))
	}
	// effectFor must stay inert for an empty slot
	if effectFor("", "").armed() {
		t.Fatal("an empty talisman slot must resolve to no effect")
	}
}

func TestResolveClickEffects(t *testing.T) {
	// Chance boosts only the roll — never the risk-mode payout
	base := gainFor(chanceFor(5, 1, maxStars), 1)
	for i := 0; i < 100; i++ {
		res := resolveClick(5, 1, skills{Card: cardEffects["unrank/rare"]}, maxStars, 0)
		if !res.TalismanUsed {
			t.Fatal("an armed card must burn on any outcome")
		}
		if res.Success && res.Gained != base {
			t.Fatalf("chance card must not change the payout: gained %d, want %d", res.Gained, base)
		}
	}
	// Guarantee settles at the safe-mode rate even at max risk: the exploit
	// regression — gainFor(2, 3) would pay 50 stars plus overflow coins
	for i := 0; i < 50; i++ {
		res := resolveClick(14, maxRisk, skills{Card: cardEffects["unrank/prismatic"]}, maxStars, 0)
		if !res.Success || res.Gained != 1 || res.Jackpot != 0 {
			t.Fatalf("guaranteed win must gain exactly 1 with no jackpot: %+v", res)
		}
	}
	// Guarantee + Bonus: ⚡ 벼락 is a flat 3-star step, 🌌 특이점 a 5-star one
	for key, want := range map[string]int{"gold/prismatic": 3, "diamond/prismatic": 5} {
		res := resolveClick(0, maxRisk, skills{Card: cardEffects[key]}, maxStars, 0)
		if !res.Success || res.Gained != want {
			t.Fatalf("%s gained %d, want %d", key, res.Gained, want)
		}
	}
	// 🌌 특이점 also hands over a card
	if res := resolveClick(0, 0, skills{Card: cardEffects["diamond/prismatic"]}, maxStars, 0); res.Card == nil {
		t.Fatal("특이점 must drop a card on success")
	}
	// Mult scales the risk payout
	for i := 0; i < 50; i++ {
		res := resolveClick(0, 0, skills{Card: cardEffects["gold/holo"]}, maxStars, 0)
		if !res.Success || res.Gained != 2 {
			t.Fatalf("×2 card: %+v", res)
		}
	}
	// Keep holds every star on a fail; Half rounds up and CoinLoss pays out
	sawKeep, sawHalf, sawInsured := false, false, false
	for i := 0; i < 300; i++ {
		if res := resolveClick(14, maxRisk, skills{Card: cardEffects["bronze/holo"]}, maxStars, 0); !res.Success {
			sawKeep = true
			if res.Stars != 14 {
				t.Fatalf("불사조 must keep the streak: %+v", res)
			}
		}
		if res := resolveClick(7, maxRisk, skills{Card: cardEffects["platinum/common"]}, maxStars, 0); !res.Success {
			sawHalf = true
			if res.Stars != 4 {
				t.Fatalf("완충 반지 on ★7 should land on ★4, got %d", res.Stars)
			}
		}
		if res := resolveClick(10, maxRisk, skills{Card: cardEffects["diamond/common"]}, maxStars, 0); !res.Success {
			sawInsured = true
			if res.Jackpot != 3*10 {
				t.Fatalf("보험금 should pay 3 per lost star: %+v", res)
			}
		}
	}
	if !sawKeep || !sawHalf || !sawInsured {
		t.Fatal("no fails observed at 2% odds")
	}
	// CoinWin pays per star held after the win
	if res := resolveClick(0, 0, skills{Card: cardEffects["gold/common"]}, maxStars, 0); res.Jackpot != 2 {
		t.Fatalf("황금손 should pay 2 per star held: %+v", res)
	}
	// TierJump and BestJump are floors, never a downgrade
	if res := resolveClick(1, 0, skills{Card: cardEffects["silver/holo"]}, maxStars, 0); res.Stars != 4 {
		t.Fatalf("사다리 from ★1 should reach silver at ★4, got %d", res.Stars)
	}
	if res := resolveClick(0, 0, skills{Card: cardEffects["platinum/prismatic"]}, maxStars, 9); res.Stars != 9 {
		t.Fatalf("해일 should restore the personal best, got %d", res.Stars)
	}
	if res := resolveClick(6, 0, skills{Card: cardEffects["platinum/prismatic"]}, maxStars, 2); res.Stars != 7 {
		t.Fatalf("해일 must never cut a streak short, got %d", res.Stars)
	}
	// Rerolls convert fails that a bare click would have kept
	saved := 0
	for i := 0; i < 400; i++ {
		if res := resolveClick(14, maxRisk, skills{Card: cardEffects["platinum/holo"]}, maxStars, 0); res.Success {
			saved++
		}
	}
	if saved == 0 {
		t.Fatal("2 rerolls at 2% never landed in 400 tries")
	}
	// MaxRisk keeps the safe click's odds but settles at the max-risk payout.
	// ★0 rolls at a guaranteed 100% while paying gainFor(100/4, 3) = 4.
	want := gainFor(chanceFor(0, maxRisk, maxStars), maxRisk)
	res := resolveClick(0, 0, skills{Card: cardEffects["diamond/holo"]}, maxStars, 0)
	if !res.Success || res.Gained != want {
		t.Fatalf("용의 심장 should settle at the max-risk payout %d: %+v", want, res)
	}
	// Refund is reported so the handler can hand the click back
	if res := resolveClick(0, 0, skills{Card: cardEffects["bronze/common"]}, maxStars, 0); !res.Refund {
		t.Fatalf("동전 한 닢 must refund the click: %+v", res)
	}
}

func TestRollPackCards(t *testing.T) {
	total := 0
	const runs = 10000
	for i := 0; i < runs; i++ {
		drawn := rollPackCards()
		if len(drawn) < 1 || len(drawn) > 1+len(packBonusPct) {
			t.Fatalf("pack drew %d cards", len(drawn))
		}
		for _, c := range drawn {
			if !validTier(c.Tier) || !validRarity(c.Rarity) {
				t.Fatalf("pack drew an invalid card %s/%s", c.Tier, c.Rarity)
			}
		}
		total += len(drawn)
	}
	// 1 + 0.45 + 0.20 = 1.65 expected; wide bounds keep this non-flaky
	if mean := float64(total) / runs; mean < 1.55 || mean > 1.75 {
		t.Errorf("pack averages %.2f cards, want ~1.65", mean)
	}
}

// TestShieldRefund covers the one-time migration that retired the 🛡️ scroll.
func TestShieldRefund(t *testing.T) {
	path := t.TempDir() + "/test.db"
	s, err := openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 5, shield_charges = 2 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	s.db.Close()

	s, err = openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Coins != 5+2*retiredShieldRefund {
		t.Fatalf("coins = %d after the refund", p.Coins)
	}
	s.db.Close()

	// second boot must be a no-op, not a second payout
	s, err = openStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = s.getOrCreatePlayer("a")
	if p.Coins != 5+2*retiredShieldRefund {
		t.Fatalf("refund paid twice: coins = %d", p.Coins)
	}
}

func TestTalismanStore(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.getOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.armTalisman("a", "bronze", "common"); ok {
		t.Fatal("arming without a card must fail")
	}
	for i := 0; i < 4; i++ {
		if err := s.addCard("a", "bronze", "common"); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := s.armTalisman("a", "bronze", "common"); !ok {
		t.Fatal("arm failed with cards in hand")
	}
	if ok, _ := s.armTalisman("a", "bronze", "common"); ok {
		t.Fatal("second arm must be rejected while the slot is busy")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.TalismanTier != "bronze" || p.TalismanRarity != "common" {
		t.Fatalf("talisman not armed: %+v", p)
	}
	if err := s.clearTalisman("a"); err != nil {
		t.Fatal(err)
	}

	// fusion: 3 commons -> 1 rare; rejects when short
	if ok, _ := s.fuseCards("a", "bronze", "common", "rare"); !ok {
		t.Fatal("fuse with 3 copies failed")
	}
	cards, _ := s.getCards("a")
	byKey := map[string]int{}
	for _, c := range cards {
		byKey[c.Tier+"/"+c.Rarity] = c.Count
	}
	if byKey["bronze/common"] != 0 || byKey["bronze/rare"] != 1 {
		t.Fatalf("fusion counts wrong: %v", byKey)
	}
	// the count-0 row survives as the discovered marker
	if _, found := byKey["bronze/common"]; !found {
		t.Fatal("count-0 card row must persist")
	}
	if ok, _ := s.fuseCards("a", "bronze", "common", "rare"); ok {
		t.Fatal("fuse without 3 copies must fail")
	}
}

func TestNicknameRule(t *testing.T) {
	ok := []string{"철수", "김밥왕", "Hero_1", "버튼장인_99", "ab"}
	for _, n := range ok {
		if !nicknameRe.MatchString(n) {
			t.Errorf("%q should be allowed", n)
		}
	}
	bad := []string{"a", "한", "hello world", "ㅋㅋㅋ", "가나다라마바사아자차카타파하가나다"}
	for _, n := range bad {
		if nicknameRe.MatchString(n) {
			t.Errorf("%q should be rejected", n)
		}
	}
}

func TestNicknameUnique(t *testing.T) {
	s, err := openStore(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"a", "b"} {
		if _, err := s.getOrCreatePlayer(tok); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.setNickname("a", "Hero"); err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if err := s.setNickname("b", "hero"); err != errNicknameTaken {
		t.Fatalf("case-insensitive dupe should be rejected, got %v", err)
	}
	if err := s.setNickname("a", "hero"); err != nil {
		t.Fatalf("renaming to own name should pass: %v", err)
	}
}

func TestTierRankLadder(t *testing.T) {
	for want, name := range []string{"unrank", "bronze", "silver", "gold", "platinum", "diamond"} {
		if got := tierRank(name); got != want {
			t.Errorf("tierRank(%q) = %d, want %d", name, got, want)
		}
	}
}
