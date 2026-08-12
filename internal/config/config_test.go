package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwhang0917/the-button/internal/game"
)

// writeConfig points CONFIG_PATH at a temp file holding the given YAML.
func writeConfig(t *testing.T, yaml string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
}

func TestLoadDefaultsWithNoFile(t *testing.T) {
	// an absent config.yml is normal operation, not an error
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "absent.yml"))
	if _, err := Load(); err == nil {
		t.Fatal("an explicitly pointed-at missing file should fail loudly")
	}

	t.Setenv("CONFIG_PATH", "")
	dir := t.TempDir()
	t.Chdir(dir) // no config.yml here
	cfg, err := Load()
	if err != nil {
		t.Fatalf("defaults must load with no file: %v", err)
	}
	want := game.Default()
	if cfg.Rules.Quota != want.Quota || cfg.Rules.Pack.Price != want.Pack.Price {
		t.Errorf("quota=%d packPrice=%d, want %d/%d",
			cfg.Rules.Quota, cfg.Rules.Pack.Price, want.Quota, want.Pack.Price)
	}
	if len(cfg.Rules.Cards) != len(want.Cards) {
		t.Errorf("cards = %d, want %d", len(cfg.Rules.Cards), len(want.Cards))
	}
	if cfg.Port != "8080" || cfg.NicknameMax != 16 {
		t.Errorf("server defaults not applied: %+v", cfg)
	}
}

// A partial file must override only its own keys — this is the property that
// lets someone tweak one price without restating the whole game.
func TestPartialFileKeepsOtherDefaults(t *testing.T) {
	writeConfig(t, `
economy:
  pack:
    price: 45
game:
  quota: 40
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := game.Default()
	if cfg.Rules.Pack.Price != 45 {
		t.Errorf("pack price = %d, want 45", cfg.Rules.Pack.Price)
	}
	if cfg.Rules.Quota != 40 {
		t.Errorf("quota = %d, want 40", cfg.Rules.Quota)
	}
	if cfg.Rules.Lottery.Price != want.Lottery.Price {
		t.Errorf("untouched lottery price changed to %d", cfg.Rules.Lottery.Price)
	}
	if len(cfg.Rules.Cards) != len(want.Cards) {
		t.Errorf("untouched cards changed to %d entries", len(cfg.Rules.Cards))
	}
	// the pack draw tables must survive as ordered slices, not map order
	if len(cfg.Rules.Pack.Tiers) != len(want.Pack.Tiers) {
		t.Fatalf("pack tier table = %v", cfg.Rules.Pack.Tiers)
	}
	for i, w := range cfg.Rules.Pack.Tiers {
		if w != want.Pack.Tiers[i] {
			t.Errorf("pack tier[%d] = %+v, want %+v", i, w, want.Pack.Tiers[i])
		}
	}
}

func TestEnvBeatsFile(t *testing.T) {
	writeConfig(t, "game:\n  quota: 40\n")
	t.Setenv("QUOTA", "7")
	t.Setenv("PORT", "9999")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Rules.Quota != 7 {
		t.Errorf("QUOTA env should win: got %d", cfg.Rules.Quota)
	}
	if cfg.Port != "9999" {
		t.Errorf("PORT env should win: got %s", cfg.Port)
	}
}

func TestDevModeFromEnv(t *testing.T) {
	writeConfig(t, "game:\n  quota: 10\n")
	t.Setenv("DEV_MODE", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DevMode {
		t.Fatal("DEV_MODE env must switch dev mode on")
	}
	if _, ok := cfg.Rules.RNG.(game.AlwaysWin); !ok {
		t.Fatalf("dev mode must swap in AlwaysWin, got %T", cfg.Rules.RNG)
	}
}

// Cards merge per key, and each card entry replaces the whole effect. So
// retuning one card is a two-line config, but you always state that card
// completely — a half-specified card cannot inherit stray fields from the
// default it replaces.
func TestCardsMergePerKeyAndReplacePerCard(t *testing.T) {
	writeConfig(t, `
cards:
  unrank/common: {chance: 40}
  bronze/rare: {keep: true}
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := game.Default()
	if len(cfg.Rules.Cards) != len(want.Cards) {
		t.Fatalf("cards = %d entries, want all %d to survive", len(cfg.Rules.Cards), len(want.Cards))
	}
	if got := cfg.Rules.Cards["unrank/common"]; got != (game.CardEffect{Chance: 40}) {
		t.Errorf("overridden card = %+v, want only chance 40", got)
	}
	// 되감기 defaults to keep+refund; naming only keep must drop the refund
	if got := cfg.Rules.Cards["bronze/rare"]; got != (game.CardEffect{Keep: true}) {
		t.Errorf("a card entry must replace wholesale, got %+v", got)
	}
	if got := cfg.Rules.Cards["gold/holo"]; got != want.Cards["gold/holo"] {
		t.Errorf("untouched card changed to %+v", got)
	}
}

func TestInvalidConfigsFailLoudly(t *testing.T) {
	cases := map[string]struct{ yaml, wantSubstr string }{
		"guarantee with mult": {
			"cards:\n  unrank/common: {guarantee: true, mult: 3}\n", "cards",
		},
		"pack table off by one": {
			"economy:\n  pack:\n    tier_permille: {unrank: 319, bronze: 260, silver: 180, gold: 120, platinum: 80, diamond: 40}\n",
			"1000‰",
		},
		"unknown pack tier": {
			"economy:\n  pack:\n    tier_permille: {mythic: 1000}\n", "unknown tier",
		},
		"non-ascending tiers": {
			"game:\n  tiers:\n    - {name: unrank, min_stars: 0}\n    - {name: bronze, min_stars: 0}\n", "ascend",
		},
		"tiers not starting at zero": {
			"game:\n  tiers:\n    - {name: unrank, min_stars: 3}\n", "min_stars must be 0",
		},
		"quota below one": {"game:\n  quota: 0\n", "quota"},
		"chance table not starting at 100": {
			"game:\n  chance_table: [90, 80]\n", "chance_table[0]",
		},
		"lottery certain to pay": {
			"economy:\n  lottery:\n    prizes:\n      - {prize: 10, permille: 1000}\n", "꽝",
		},
		"defuse not lossy": {
			"economy:\n  fuse_cost: 3\n  defuse_yield: 3\n", "lossy",
		},
		"nickname range inverted": {
			"server:\n  nickname: {min_len: 9, max_len: 2}\n", "nickname",
		},
		"malformed yaml": {"game:\n  quota: [1, 2\n", "yaml"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			writeConfig(t, c.yaml)
			_, err := Load()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.wantSubstr) {
				t.Errorf("error %q should mention %q", err, c.wantSubstr)
			}
		})
	}
}

// The example file is documentation, so it must stay loadable and agree with
// the built-in defaults it claims to document.
func TestExampleConfigMatchesDefaults(t *testing.T) {
	path, err := filepath.Abs("../../config.yml.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no example file: %v", err)
	}
	t.Setenv("CONFIG_PATH", path)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("config.yml.example must load: %v", err)
	}
	want := game.Default()
	got := cfg.Rules
	if got.Quota != want.Quota || got.MaxStars != want.MaxStars || got.MaxRisk != want.MaxRisk {
		t.Errorf("example drifted from defaults: quota %d/%d maxStars %d/%d maxRisk %d/%d",
			got.Quota, want.Quota, got.MaxStars, want.MaxStars, got.MaxRisk, want.MaxRisk)
	}
	if got.Pack.Price != want.Pack.Price || got.Lottery.Price != want.Lottery.Price || got.RefillPrice != want.RefillPrice {
		t.Errorf("example prices drifted from defaults")
	}
	for key, wantEffect := range want.Cards {
		if got.Cards[key] != wantEffect {
			t.Errorf("example cards.%s = %+v, want %+v", key, got.Cards[key], wantEffect)
		}
	}
}
