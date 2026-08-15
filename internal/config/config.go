// Package config resolves the server's settings from three layers, lowest
// precedence first: the built-in defaults in game.Default, an optional
// config.yml, and finally environment variables. Env winning last is what keeps
// docker-compose.yml working without mounting a file.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hwhang0917/the-button/internal/game"
)

// DefaultPath is where Load looks when CONFIG_PATH is unset. A missing file is
// normal operation, not an error — it just means "all defaults".
const DefaultPath = "./config.yml"

// Config is everything the process needs to run.
type Config struct {
	Port      string
	DBPath    string
	EventsDir string // anonymous NDJSON gameplay events; empty = telemetry off

	LeaderboardSize int
	NicknameMin     int
	NicknameMax     int
	LinkTTL         time.Duration
	LinkCodeLen     int
	ClaimFailLimit  int

	DevMode  bool
	ShowDocs bool // serve Swagger UI + the OpenAPI spec under /api/docs
	Rules    game.Rules
}

// duration lets config.yml write "10m" instead of a bare number of minutes.
type duration struct{ time.Duration }

func (d *duration) UnmarshalYAML(n *yaml.Node) error {
	parsed, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", n.Value, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalYAML keeps generated files round-trippable: without it the embedded
// time.Duration would serialize as raw nanoseconds, which UnmarshalYAML above
// then refuses to parse.
func (d duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

// The file mirrors config.yml's shape. It is seeded from the defaults before
// unmarshalling, so any key the file omits keeps its default and no field needs
// to be a pointer to tell "absent" from "zero".
type file struct {
	Server  serverFile                 `yaml:"server"`
	Game    gameFile                   `yaml:"game"`
	Economy economyFile                `yaml:"economy"`
	Cards   map[string]game.CardEffect `yaml:"cards"`
}

type serverFile struct {
	Port            string `yaml:"port"`
	DBPath          string `yaml:"db_path"`
	EventsDir       string `yaml:"events_dir"`
	ShowDocs        bool   `yaml:"show_docs"`
	LeaderboardSize int    `yaml:"leaderboard_size"`
	Nickname        struct {
		MinLen int `yaml:"min_len"`
		MaxLen int `yaml:"max_len"`
	} `yaml:"nickname"`
	Link struct {
		TTL            duration `yaml:"ttl"`
		CodeLen        int      `yaml:"code_len"`
		ClaimFailLimit int      `yaml:"claim_fail_limit"`
	} `yaml:"link"`
}

type gameFile struct {
	DevMode         bool  `yaml:"dev_mode"`
	Quota           int   `yaml:"quota"`
	ChanceTable     []int `yaml:"chance_table"`
	MaxRisk         int   `yaml:"max_risk"`
	MaxStars        int   `yaml:"max_stars"`
	OverflowCoinPer int   `yaml:"overflow_coin_per"`
	Prestige        struct {
		StarBonus int   `yaml:"star_bonus"`
		SkinCap   int   `yaml:"skin_cap"`
		Rewards   []int `yaml:"rewards"`
	} `yaml:"prestige"`
	Tiers    []game.Tier `yaml:"tiers"`
	Rarities []string    `yaml:"rarities"`
}

type economyFile struct {
	Charm     game.Skill   `yaml:"charm"`
	Headstart game.Skill   `yaml:"headstart"`
	Stamina   game.Skill   `yaml:"stamina"`
	Magnet    game.Skill   `yaml:"magnet"`
	Golden    game.Skill   `yaml:"golden"`
	Lottery   game.Lottery `yaml:"lottery"`
	Pack      struct {
		Price          int            `yaml:"price"`
		BonusPct       []int          `yaml:"bonus_pct"`
		TierPermille   map[string]int `yaml:"tier_permille"`
		RarityPermille map[string]int `yaml:"rarity_permille"`
	} `yaml:"pack"`
	RefillPrice         int   `yaml:"refill_price"`
	RefillsPerDay       int   `yaml:"refills_per_day"`
	FuseCost            int   `yaml:"fuse_cost"`
	DefuseYield         int   `yaml:"defuse_yield"`
	CardSell            []int `yaml:"card_sell"`
	RetiredShieldRefund int   `yaml:"retired_shield_refund"`
}

// Load resolves defaults → config.yml → env. A missing file is fine; a
// malformed or invalid one is an error, so bad config fails at boot rather than
// halfway through a player's session.
func Load() (Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = DefaultPath
	}

	f := defaultFile()
	switch data, err := os.ReadFile(path); {
	case err == nil:
		if err := yaml.Unmarshal(data, &f); err != nil {
			return Config{}, fmt.Errorf("%s: %w", path, err)
		}
	case os.IsNotExist(err):
		// no config.yml: run on defaults, and drop a full one at the path so
		// the operator has something to edit. A failed write (read-only fs,
		// missing dir) is not worth dying over — the defaults still work.
		log.Printf("WARN: %s not found — running on defaults", path)
		if data, err := yaml.Marshal(f); err == nil {
			if err := os.WriteFile(path, data, 0o644); err != nil {
				log.Printf("WARN: could not write default config to %s: %v", path, err)
			} else {
				log.Printf("WARN: wrote default config to %s", path)
			}
		}
	default:
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	cfg := f.toConfig()
	applyEnv(&cfg)
	if err := cfg.Rules.Validate(); err != nil {
		return Config{}, err
	}
	if cfg.NicknameMin < 1 || cfg.NicknameMax < cfg.NicknameMin {
		return Config{}, fmt.Errorf("server.nickname needs 1 <= min_len <= max_len, got %d..%d",
			cfg.NicknameMin, cfg.NicknameMax)
	}
	if cfg.LeaderboardSize < 1 {
		return Config{}, fmt.Errorf("server.leaderboard_size must be at least 1, got %d", cfg.LeaderboardSize)
	}
	if cfg.LinkCodeLen < 4 {
		return Config{}, fmt.Errorf("server.link.code_len must be at least 4, got %d", cfg.LinkCodeLen)
	}
	return cfg, nil
}

func defaultFile() file {
	r := game.Default()
	var f file
	f.Server.Port = "8080"
	f.Server.DBPath = "./thebutton.db"
	f.Server.LeaderboardSize = 20
	f.Server.Nickname.MinLen = 2
	f.Server.Nickname.MaxLen = 16
	f.Server.Link.TTL = duration{10 * time.Minute}
	f.Server.Link.CodeLen = 8
	// failed claim attempts tolerated per minute before the endpoint locks
	f.Server.Link.ClaimFailLimit = 20

	f.Game.Quota = r.Quota
	f.Game.ChanceTable = r.ChanceTable
	f.Game.MaxRisk = r.MaxRisk
	f.Game.MaxStars = r.MaxStars
	f.Game.OverflowCoinPer = r.OverflowCoinPer
	f.Game.Prestige.StarBonus = r.PrestigeStarBonus
	f.Game.Prestige.SkinCap = r.PrestigeSkinCap
	f.Game.Prestige.Rewards = r.PrestigeRewards
	f.Game.Tiers = r.Tiers
	f.Game.Rarities = r.Rarities

	f.Economy.Charm = r.Charm
	f.Economy.Headstart = r.Headstart
	f.Economy.Stamina = r.Stamina
	f.Economy.Magnet = r.Magnet
	f.Economy.Golden = r.Golden
	f.Economy.Lottery = r.Lottery
	f.Economy.Pack.Price = r.Pack.Price
	f.Economy.Pack.BonusPct = r.Pack.BonusPct
	f.Economy.Pack.TierPermille = weightMap(r.Pack.Tiers)
	f.Economy.Pack.RarityPermille = weightMap(r.Pack.Rarities)
	f.Economy.RefillPrice = r.RefillPrice
	f.Economy.RefillsPerDay = r.RefillsPerDay
	f.Economy.FuseCost = r.FuseCost
	f.Economy.DefuseYield = r.DefuseYield
	f.Economy.CardSell = r.CardSell
	f.Economy.RetiredShieldRefund = r.RetiredShieldRefund

	f.Cards = r.Cards
	return f
}

func weightMap(ws []game.Weight) map[string]int {
	m := make(map[string]int, len(ws))
	for _, w := range ws {
		m[w.Name] = w.Permille
	}
	return m
}

// orderedWeights turns a YAML map into a slice in the ladder's declared order.
// Go map iteration is randomised, so walking the map directly would make which
// entry a given roll selects vary run to run.
func orderedWeights(order []string, m map[string]int) []game.Weight {
	ws := make([]game.Weight, 0, len(m))
	seen := make(map[string]bool, len(m))
	for _, name := range order {
		if permille, ok := m[name]; ok {
			ws = append(ws, game.Weight{Name: name, Permille: permille})
			seen[name] = true
		}
	}
	// anything not on the ladder is kept so Validate can name it in the error
	for name, permille := range m {
		if !seen[name] {
			ws = append(ws, game.Weight{Name: name, Permille: permille})
		}
	}
	return ws
}

func (f file) toConfig() Config {
	tierNames := make([]string, len(f.Game.Tiers))
	for i, t := range f.Game.Tiers {
		tierNames[i] = t.Name
	}
	rng := game.RNG(game.CryptoRNG{})
	if f.Game.DevMode {
		rng = game.AlwaysWin{}
	}
	return Config{
		Port:            f.Server.Port,
		DBPath:          f.Server.DBPath,
		EventsDir:       f.Server.EventsDir,
		ShowDocs:        f.Server.ShowDocs,
		LeaderboardSize: f.Server.LeaderboardSize,
		NicknameMin:     f.Server.Nickname.MinLen,
		NicknameMax:     f.Server.Nickname.MaxLen,
		LinkTTL:         f.Server.Link.TTL.Duration,
		LinkCodeLen:     f.Server.Link.CodeLen,
		ClaimFailLimit:  f.Server.Link.ClaimFailLimit,
		DevMode:         f.Game.DevMode,
		Rules: game.Rules{
			ChanceTable:       f.Game.ChanceTable,
			MaxRisk:           f.Game.MaxRisk,
			MaxStars:          f.Game.MaxStars,
			PrestigeStarBonus: f.Game.Prestige.StarBonus,
			PrestigeSkinCap:   f.Game.Prestige.SkinCap,
			PrestigeRewards:   f.Game.Prestige.Rewards,
			OverflowCoinPer:   f.Game.OverflowCoinPer,
			Tiers:             f.Game.Tiers,
			Rarities:          f.Game.Rarities,
			Cards:             f.Cards,
			Charm:             f.Economy.Charm,
			Headstart:         f.Economy.Headstart,
			Stamina:           f.Economy.Stamina,
			Magnet:            f.Economy.Magnet,
			Golden:            f.Economy.Golden,
			Lottery:           f.Economy.Lottery,
			Pack: game.Pack{
				Price:    f.Economy.Pack.Price,
				BonusPct: f.Economy.Pack.BonusPct,
				Tiers:    orderedWeights(tierNames, f.Economy.Pack.TierPermille),
				Rarities: orderedWeights(f.Game.Rarities, f.Economy.Pack.RarityPermille),
			},
			RefillPrice:         f.Economy.RefillPrice,
			RefillsPerDay:       f.Economy.RefillsPerDay,
			FuseCost:            f.Economy.FuseCost,
			DefuseYield:         f.Economy.DefuseYield,
			CardSell:            f.Economy.CardSell,
			RetiredShieldRefund: f.Economy.RetiredShieldRefund,
			Quota:               f.Game.Quota,
			RNG:                 rng,
		},
	}
}

// applyEnv is the last and highest-precedence layer. Only the deployment-shaped
// settings are exposed here — everything else belongs in config.yml.
func applyEnv(cfg *Config) {
	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("EVENTS_DIR"); v != "" {
		cfg.EventsDir = v
	}
	if v := os.Getenv("QUOTA"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Rules.Quota = n // Validate rejects anything below 1
		}
	}
	if os.Getenv("SHOW_DOCS") != "" {
		cfg.ShowDocs = true
	}
	if os.Getenv("DEV_MODE") != "" {
		cfg.DevMode = true
		cfg.Rules.RNG = game.AlwaysWin{}
	}
}
