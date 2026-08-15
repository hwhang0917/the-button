package store

import (
	"testing"

	"github.com/hwhang0917/the-button/internal/game"
)

// the store takes its economy numbers as arguments now, so the tests name the
// ones they exercise instead of reaching for package constants
const (
	shieldRefund = 25
	packPrice    = 30
	lotteryPrice = 15
	refillPrice  = 60
	fuseCost     = 3
	defuseYield  = 2
	maxStars     = 15
)

func TestQuota(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	const limit = 3
	for i := range limit {
		if _, err := s.ConsumeQuota("tok", limit); err != nil {
			t.Fatalf("click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.ConsumeQuota("tok", limit); err != ErrQuotaExceeded {
		t.Fatalf("expected quota exceeded, got %v", err)
	}
	if _, err := s.ConsumeQuota("othertok", limit); err != nil {
		t.Fatalf("other player should have their own quota: %v", err)
	}
	if err := s.GrantQuota("tok", 2); err != nil {
		t.Fatalf("grant failed: %v", err)
	}
	for i := range 2 {
		if _, err := s.ConsumeQuota("tok", limit); err != nil {
			t.Fatalf("granted click %d rejected: %v", i+1, err)
		}
	}
	if _, err := s.ConsumeQuota("tok", limit); err != ErrQuotaExceeded {
		t.Fatalf("expected quota exceeded after spending grant, got %v", err)
	}
}

func TestSkillStore(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}

	if ok, _ := s.BuySkill("a", "charm_level", 10, 0); ok {
		t.Fatal("buy with 0 coins must fail")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 100 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.BuySkill("a", "charm_level", 10, 0); !ok {
		t.Fatal("funded buy failed")
	}
	if ok, _ := s.BuySkill("a", "charm_level", 10, 0); ok {
		t.Fatal("stale level pin must reject the buy")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Coins != 90 || p.CharmLevel != 1 {
		t.Fatalf("coins=%d charm=%d after buy", p.Coins, p.CharmLevel)
	}

	if err := s.SavePlayerStars("a", 5, 0); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.SellStreak("a", 15, 0, 5); !ok {
		t.Fatal("sell failed")
	}
	if ok, _ := s.SellStreak("a", 15, 0, 5); ok {
		t.Fatal("stale stars pin must reject the sell")
	}
	p, _ = s.GetOrCreatePlayer("a")
	if p.Stars != 0 || p.Coins != 105 { // 100 - 10 charm + 15 sale
		t.Fatalf("stars=%d coins=%d after sell", p.Stars, p.Coins)
	}
}

func TestPlayLottery(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.PlayLottery("a", lotteryPrice, 500); ok {
		t.Fatal("broke player must not buy a ticket")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 20 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.PlayLottery("a", lotteryPrice, 30); !ok {
		t.Fatal("funded ticket rejected")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Coins != 20-lotteryPrice+30 {
		t.Fatalf("coins = %d after win", p.Coins)
	}
}

func TestBuyPackAndRefill(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	drawn := []game.Card{{Tier: "gold", Rarity: "rare"}, {Tier: "unrank", Rarity: "common"}}
	if ok, _ := s.BuyPack("a", packPrice, drawn); ok {
		t.Fatal("broke pack purchase must fail")
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 300 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.BuyPack("a", packPrice, drawn); !ok {
		t.Fatal("funded pack purchase failed")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Coins != 300-packPrice {
		t.Fatalf("coins = %d after pack", p.Coins)
	}
	cards, _ := s.GetCards("a")
	if len(cards) != len(drawn) {
		t.Fatalf("every drawn card must land: %v", cards)
	}
	for _, c := range cards {
		if c.Count != 1 {
			t.Fatalf("card %s/%s count = %d", c.Tier, c.Rarity, c.Count)
		}
	}

	// refill: rejected with no spent clicks, works after spending, capped at
	// the daily limit, and the tally resets when the day rolls over
	const day = "2026-08-11"
	if ok, _ := s.RefillQuota("a", refillPrice, day, 1); ok {
		t.Fatal("refill with nothing spent must fail")
	}
	if _, err := s.ConsumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.RefillQuota("a", refillPrice, day, 1); !ok {
		t.Fatal("refill failed")
	}
	if used, _ := s.QuotaUsed("a"); used != 0 {
		t.Fatalf("quota not reset: used %d", used)
	}
	p, _ = s.GetOrCreatePlayer("a")
	if p.Coins != 300-packPrice-refillPrice {
		t.Fatalf("coins = %d after refill", p.Coins)
	}
	if p.RefillDay != day || p.RefillCount != 1 {
		t.Fatalf("refill tally = %s/%d", p.RefillDay, p.RefillCount)
	}
	if _, err := s.ConsumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.RefillQuota("a", refillPrice, day, 1); ok {
		t.Fatal("second refill past the limit must fail")
	}
	if ok, _ := s.RefillQuota("a", refillPrice, day, 2); !ok {
		t.Fatal("second refill within a higher limit should work")
	}
	if _, err := s.ConsumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.RefillQuota("a", refillPrice, "2026-08-12", 1); !ok {
		t.Fatal("refill on the next day should work")
	}
}

func TestPrestigeStore(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePlayerStars("a", maxStars, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConsumeQuota("a", 10); err != nil {
		t.Fatal(err)
	}

	if ok, _ := s.PrestigeStreak("a", 300, 0, maxStars); !ok {
		t.Fatal("prestige at max stars failed")
	}
	// the promotion refills the hour's clicks
	if used, _ := s.QuotaUsed("a"); used != 0 {
		t.Fatalf("quota not refilled by prestige: used %d", used)
	}
	if ok, _ := s.PrestigeStreak("a", 300, 0, maxStars); ok {
		t.Fatal("stale stars pin must reject a repeat prestige")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Prestige != 1 || p.Coins != 300 || p.Stars != 0 {
		t.Fatalf("after prestige: %+v", p)
	}

	// prestige is unbounded: prismatic laps keep counting past the skin cap
	for i := range 4 {
		if err := s.SavePlayerStars("a", maxStars, 0); err != nil {
			t.Fatal(err)
		}
		if ok, _ := s.PrestigeStreak("a", game.Default().PrestigeRewardFor(p.Prestige), 0, maxStars); !ok {
			t.Fatalf("prestige round %d failed", i)
		}
		p, _ = s.GetOrCreatePlayer("a")
	}
	if p.Prestige != 5 {
		t.Fatalf("prestige must keep counting, got %d", p.Prestige)
	}
}

func TestCancelAndDefuse(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.CancelTalisman("a"); ok {
		t.Fatal("cancel with nothing armed must fail")
	}
	if err := s.AddCard("a", "gold", "rare"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.ArmTalisman("a", "gold", "rare"); !ok {
		t.Fatal("arm failed")
	}
	if ok, _ := s.CancelTalisman("a"); !ok {
		t.Fatal("cancel failed")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.TalismanTier != "" {
		t.Fatal("slot must be cleared after cancel")
	}
	cards, _ := s.GetCards("a")
	if len(cards) != 1 || cards[0].Count != 1 {
		t.Fatalf("card must be refunded: %v", cards)
	}

	// defuse: 1 rare -> 2 commons; rejects at 0
	if ok, _ := s.DefuseCard("a", "gold", "rare", "common", defuseYield); !ok {
		t.Fatal("defuse failed")
	}
	byKey := map[string]int{}
	cards, _ = s.GetCards("a")
	for _, c := range cards {
		byKey[c.Rarity] = c.Count
	}
	if byKey["rare"] != 0 || byKey["common"] != defuseYield {
		t.Fatalf("defuse counts wrong: %v", byKey)
	}
	if ok, _ := s.DefuseCard("a", "gold", "rare", "common", defuseYield); ok {
		t.Fatal("defuse without a copy must fail")
	}
}

func TestShieldRefund(t *testing.T) {
	path := t.TempDir() + "/test.db"
	s, err := Open(path, shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE players SET coins = 5, shield_charges = 2 WHERE token = 'a'`); err != nil {
		t.Fatal(err)
	}
	s.db.Close()

	s, err = Open(path, shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Coins != 5+2*shieldRefund {
		t.Fatalf("coins = %d after the refund", p.Coins)
	}
	s.db.Close()

	// second boot must be a no-op, not a second payout
	s, err = Open(path, shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	p, _ = s.GetOrCreatePlayer("a")
	if p.Coins != 5+2*shieldRefund {
		t.Fatalf("refund paid twice: coins = %d", p.Coins)
	}
}

func TestTalismanStore(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.ArmTalisman("a", "bronze", "common"); ok {
		t.Fatal("arming without a card must fail")
	}
	for range 4 {
		if err := s.AddCard("a", "bronze", "common"); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := s.ArmTalisman("a", "bronze", "common"); !ok {
		t.Fatal("arm failed with cards in hand")
	}
	if ok, _ := s.ArmTalisman("a", "bronze", "common"); ok {
		t.Fatal("second arm must be rejected while the slot is busy")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.TalismanTier != "bronze" || p.TalismanRarity != "common" {
		t.Fatalf("talisman not armed: %+v", p)
	}
	if err := s.ClearTalisman("a"); err != nil {
		t.Fatal(err)
	}

	// fusion: 3 commons -> 1 rare; rejects when short
	if ok, _ := s.FuseCards("a", "bronze", "common", "rare", fuseCost); !ok {
		t.Fatal("fuse with 3 copies failed")
	}
	cards, _ := s.GetCards("a")
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
	if ok, _ := s.FuseCards("a", "bronze", "common", "rare", fuseCost); ok {
		t.Fatal("fuse without 3 copies must fail")
	}
}

func TestSellCard(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreatePlayer("a"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.SellCard("a", "bronze", "common", 2); ok {
		t.Fatal("selling a card never owned must fail")
	}
	if err := s.AddCard("a", "bronze", "common"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.SellCard("a", "bronze", "common", 2); !ok {
		t.Fatal("sell with a copy in hand failed")
	}
	p, _ := s.GetOrCreatePlayer("a")
	if p.Coins != 2 {
		t.Fatalf("coins = %d, want 2", p.Coins)
	}
	// the count-0 row survives as the discovered marker
	cards, _ := s.GetCards("a")
	if len(cards) != 1 || cards[0].Count != 0 {
		t.Fatalf("discovered marker lost: %+v", cards)
	}
	// selling the last copy again must fail and pay nothing
	if ok, _ := s.SellCard("a", "bronze", "common", 2); ok {
		t.Fatal("selling at count 0 must fail")
	}
	if p, _ = s.GetOrCreatePlayer("a"); p.Coins != 2 {
		t.Fatalf("failed sell changed coins: %d", p.Coins)
	}
}

func TestNicknameUnique(t *testing.T) {
	s, err := Open(t.TempDir()+"/test.db", shieldRefund)
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range []string{"a", "b"} {
		if _, err := s.GetOrCreatePlayer(tok); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetNickname("a", "Hero"); err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if err := s.SetNickname("b", "hero"); err != ErrNicknameTaken {
		t.Fatalf("case-insensitive dupe should be rejected, got %v", err)
	}
	if err := s.SetNickname("a", "hero"); err != nil {
		t.Fatalf("renaming to own name should pass: %v", err)
	}
}
