package main

import "testing"

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
