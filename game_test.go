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
	for stars := 0; stars < maxStars; stars++ {
		for risk := 0; risk <= maxRisk; risk++ {
			res := resolveClick(stars, risk, skills{})
			if res.Success {
				if res.Stars <= stars || res.Stars > maxStars {
					t.Errorf("stars=%d risk=%d: success moved to %d", stars, risk, res.Stars)
				}
				if want := min(stars+gainFor(chanceFor(stars, risk), risk), maxStars); res.Stars != want {
					t.Errorf("stars=%d risk=%d: gained to %d, want %d", stars, risk, res.Stars, want)
				}
			} else if res.Stars != 0 {
				t.Errorf("stars=%d risk=%d: fail must reset to 0, got %d", stars, risk, res.Stars)
			}
		}
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
		if res := resolveClick(0, 0, skills{}); !res.Success || res.Stars != 1 {
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

func TestResolveClickShield(t *testing.T) {
	fails := 0
	for i := 0; i < 200; i++ {
		res := resolveClick(14, 0, skills{Shield: true}) // 8% base: fails dominate
		if res.Success {
			if res.ShieldUsed {
				t.Fatal("success must not consume a shield")
			}
			continue
		}
		fails++
		if !res.ShieldUsed || res.Stars != 14 {
			t.Fatalf("shielded fail should keep stars: %+v", res)
		}
	}
	if fails == 0 {
		t.Fatal("no fails observed in 200 rolls at 8%")
	}
}

func TestHeadstartFloor(t *testing.T) {
	sawFail := false
	for i := 0; i < 100; i++ {
		// risk 3 at ★14 → 2% odds: fails are near-certain
		if res := resolveClick(14, maxRisk, skills{Headstart: 3}); !res.Success {
			sawFail = true
			if res.Stars != 3 {
				t.Fatalf("fail should land at the floor, got %d", res.Stars)
			}
		}
		// below the floor a fail must never gain stars
		if res := resolveClick(1, maxRisk, skills{Headstart: 3}); !res.Success && res.Stars > 1 {
			t.Fatalf("fail below floor gained stars: %d", res.Stars)
		}
	}
	if !sawFail {
		t.Fatal("no fails observed at 2% odds")
	}
}

func TestEffChance(t *testing.T) {
	if got := effChanceFor(0, 0, charmCap); got != 100 {
		t.Errorf("charm must cap at 100, got %d", got)
	}
	base, charmed := effChanceFor(9, 1, 0), effChanceFor(9, 1, charmCap)
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

	if ok, _ := s.consumeShield("a"); ok {
		t.Fatal("consuming with 0 charges must fail")
	}
	if ok, _ := s.buySkill("a", "shield_charges", 25, 0); !ok {
		t.Fatal("shield buy failed")
	}
	if ok, _ := s.consumeShield("a"); !ok {
		t.Fatal("consume with a charge failed")
	}

	if err := s.savePlayerStars("a", 5); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.sellStreak("a", 15, 0, 5); !ok {
		t.Fatal("sell failed")
	}
	if ok, _ := s.sellStreak("a", 15, 0, 5); ok {
		t.Fatal("stale stars pin must reject the sell")
	}
	p, _ = s.getOrCreatePlayer("a")
	if p.Stars != 0 || p.Coins != 80 { // 90 - 25 shield + 15 sale
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
	if ok, _ := s.buyPack("a", packPrice, "gold", "rare"); ok {
		t.Fatal("broke pack purchase must fail")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 200 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.buyPack("a", packPrice, "gold", "rare"); !ok {
		t.Fatal("funded pack purchase failed")
	}
	p, _ := s.getOrCreatePlayer("a")
	if p.Coins != 200-packPrice {
		t.Fatalf("coins = %d after pack", p.Coins)
	}
	cards, _ := s.getCards("a")
	if len(cards) != 1 || cards[0].Count != 1 {
		t.Fatalf("pack card missing: %v", cards)
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
	if err := s.savePlayerStars("a", maxStars); err != nil {
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
		if err := s.savePlayerStars("a", maxStars); err != nil {
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

func TestResolveClickTalisman(t *testing.T) {
	// chance talisman burns on the click regardless of outcome, boosts only the
	// roll, and must NOT shrink the risk-mode payout
	base := gainFor(chanceFor(5, 1), 1)
	for i := 0; i < 100; i++ {
		res := resolveClick(5, 1, skills{TalBonus: talRarePct})
		if !res.TalismanUsed {
			t.Fatal("chance talisman must burn on any outcome")
		}
		if res.Success && res.Gained != base {
			t.Fatalf("talisman must not change the payout: gained %d, want %d", res.Gained, base)
		}
	}
	// holo talisman saves before the purchased shield
	sawFail := false
	for i := 0; i < 200; i++ {
		res := resolveClick(14, maxRisk, skills{TalShield: true, Shield: true})
		if !res.Success {
			sawFail = true
			if !res.TalismanUsed || res.ShieldUsed || res.Stars != 14 {
				t.Fatalf("talisman must save before shield: %+v", res)
			}
		}
	}
	if !sawFail {
		t.Fatal("no fails at 2% odds")
	}
	// prismatic doubles the gain, clamped at maxStars
	for i := 0; i < 200; i++ {
		res := resolveClick(0, 0, skills{TalDouble: true})
		if !res.Success {
			t.Fatal("100% click failed")
		}
		if res.Stars != 2 || !res.TalismanUsed {
			t.Fatalf("double talisman: %+v", res)
		}
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
