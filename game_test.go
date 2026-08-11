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
			res := resolveClick(stars, risk)
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
		if res := resolveClick(0, 0); !res.Success || res.Stars != 1 {
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
		if _, err := s.consumeQuota("hash", limit); err != nil {
			t.Fatalf("click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.consumeQuota("hash", limit); err != errQuotaExceeded {
		t.Fatalf("expected quota exceeded, got %v", err)
	}
	if _, err := s.consumeQuota("otherhash", limit); err != nil {
		t.Fatalf("other IP should have its own quota: %v", err)
	}
	if err := s.grantQuota("hash", 2); err != nil {
		t.Fatalf("grant failed: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := s.consumeQuota("hash", limit); err != nil {
			t.Fatalf("granted click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.consumeQuota("hash", limit); err != errQuotaExceeded {
		t.Fatalf("expected quota exceeded after spending grant, got %v", err)
	}
}

func TestTierRankLadder(t *testing.T) {
	for want, name := range []string{"unrank", "bronze", "silver", "gold", "platinum", "diamond"} {
		if got := tierRank(name); got != want {
			t.Errorf("tierRank(%q) = %d, want %d", name, got, want)
		}
	}
}
