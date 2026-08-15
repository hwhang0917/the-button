# The Button

[![License](https://img.shields.io/github/license/hwhang0917/the-button)](LICENSE)
[![Version](https://img.shields.io/github/v/tag/hwhang0917/the-button?label=version)](https://github.com/hwhang0917/the-button/tags)

RNG enchant clicker (강화 시뮬레이터). One button, decreasing odds, full reset on fail.
All rolls happen server-side with `crypto/rand` — the client only renders.

**▶️ Play now: [button.runfridge.dev](https://button.runfridge.dev)**

## Build & run

```sh
cd web && npm install && npm run build && cd ..  # frontend → web/dist (embedded)
go build -o thebutton .
./thebutton
```

## Config

Settings resolve in three layers, each overriding the one before:

```
built-in defaults  ->  config.yml  ->  environment variables
```

Running with no `config.yml` is normal — you get the defaults. Copy
[`config.yml.example`](config.yml.example) to `config.yml` (or point
`CONFIG_PATH` at it) to retune **anything**: the chance table, tiers, rarities,
all 24 card effects, every price and probability, and the quota. It documents
every key and is set to the shipped defaults throughout, so it doubles as "what
is the game currently tuned to?".

An invalid config is fatal at boot — a missing card, a pack table that doesn't
total 1000‰, a card pairing `guarantee` with `mult` — rather than silently
half-applied. The server publishes the tunables the UI needs at `GET
/api/config`, so the frontend can't drift from what you configured.

> Numbers are free to change. **Renaming or adding a tier or rarity** also needs
> frontend artwork and copy (`TIER_COLORS` and `CARD_EMOJI` in
> `web/src/tiers.ts`, `cardName` in `web/src/i18n.ts`) — those are presentation
> and the TypeScript union types, not config.

Env vars win last, so a container stays tunable without mounting a file:

| Var | Required | Default | |
|---|---|---|---|
| `CONFIG_PATH` | no | `./config.yml` | absent file = built-in defaults; a path you set explicitly must exist |
| `PORT` | no | `8080` | |
| `DB_PATH` | no | `./thebutton.db` | SQLite file |
| `QUOTA` | no | `10` | clicks per player per hour (resets on the clock hour) |
| `EVENTS_DIR` | no | — | anonymous NDJSON gameplay events for analytics; unset = telemetry off |
| `DEV_MODE` | no | — | any value forces every roll to succeed (clicks at any risk) — local testing only |
| `SHOW_DOCS` | no | — | any value serves Swagger UI for the API at `/api/docs` (also `server.show_docs` in config.yml) |

## Layout

```
main.go              embed web/dist, wire everything up, serve
internal/config      defaults -> config.yml -> env, validated at boot
internal/game        the rules: tiers, rarities, cards, odds, resolve
internal/store       SQLite
internal/events      anonymous NDJSON analytics
internal/server      HTTP handlers and middleware
web/                 Vue 3 + Tailwind + PixiJS frontend
```

`main.go` stays at the root because `//go:embed` cannot reach above its own
directory, which is what keeps this a single binary.

## Dev

```sh
go run .                    # API on :8080
cd web && npm run dev       # Vite on :5173, proxies /api
```

## Analytics (DuckDB)

With `EVENTS_DIR` set, the server appends anonymous gameplay events (rolls, shop
activity, quota rejections) as daily NDJSON files — keyed by a one-way hash of the
session token, never IPs or names. Analyze them offline with [DuckDB](https://duckdb.org):

```sh
duckdb -c "SELECT type, count(*) FROM read_json('events/*.ndjson') GROUP BY type"
duckdb -c ".read analytics/queries.sql"   # starter queries: success rates, sell timing, lottery RTP, DAU
```

Telemetry is off unless `EVENTS_DIR` is set, and dropped (never blocking) under load.

## Game rules

- Success chance starts at 100% and drops each star (★15 max = win).
- Fail resets to ★0 (or your head-start floor). Stars persist; quota refreshes every clock hour.
- RISK IT levels 1–3: odds ÷(level+1), success pays the odds back (★+round(100/chance)).
- Tiers: Unrank → Bronze → Silver → Gold → Platinum → Diamond. First climb past your best tier refunds clicks equal to its rank.
- **Cards**: 24 of them, one per tier × rarity, each with its own one-shot effect. Card packs are the only source. Arm one (single slot) and it fires on your very next click, whatever tier you're standing in — then it's spent, win or lose. Rarity is the mechanic line and tier is the celestial family (별먼지 → 별조각 → 별빛 → 달빛 → 태양 → 은하) that strictly increments it — power climbs on both axes:
  - **커먼 · 행운 (60% of pulls)** — roll chance only: +2% → +4% → +6% → +8% → +10% → +12% by family.
  - **레어 · 결실 (25%)** — bonus stars on success (+1 up to +3), joined from 별조각 up by coins per star held (💰1 up to 💰3).
  - **홀로 · 수호 (12%)** — fail protection: half saves (별조각 adds 💰2 per star lost), then full keeps from 별빛, a free click at 달빛, and 1–2 rerolls at 태양/은하.
  - **프리즘 · 기적 (3%)** — a guaranteed success whose dowry grows: +0 → +1 → +2 → +3 stars, a free click at 태양, and 은하's +4 stars plus a bonus card.
  - A chance bonus lifts the roll only, shown as a `+N%` tag on the button — it never shrinks the risk-mode payout. Guaranteed-success cards deliberately settle at the safe-mode rate (+1 base): paying them at the risk-scaled rate would mint ~50 stars plus overflow coins on every deep max-risk click.
  - **Fuse** 3 identical cards into 1 of the next rarity (defuse returns only 2) — at 3% prismatic odds that's the only deterministic route to a specific joker. Discovered cards stay in the collection even at ×0.
- **Skill shop**: sell your streak for coins (triangle value — deep streaks pay disproportionately) and buy: 🍀 lucky charm (+2% success/level, cap 5), 🔋 stamina (+25% hourly clicks per level, compounding — it scales off the configured quota rather than adding a fixed number, cap 5), 🚀 head start (resets land at ★level, cap 3), 🎟️ 복권 (15💰 scratch ticket, exponential prizes up to a 2000💰 jackpot at ~81% payback; winnings don't count toward rank points), 🎴 card pack (30💰 for 1–3 random cards — always one, plus bonus rolls at 45% and 20% — opened Hearthstone-style), ⏰ time recharge (60💰, instantly refills this hour's clicks, once per day).
- **Prestige**: at ★15 the plain sell is disabled — PRESTIGE instead converts the streak to 300/450/600 coins and promotes your star tier common → rare → holo → prismatic, then keeps counting forever (prismatic-2, prismatic-3, …) at 600 per lap. The leaderboard orders prestige tier → stars; coins are purely shop currency.
- Unique nicknames (2–16 chars, 한글/`a-z 0-9 _`, case-insensitive), asked on first visit.
- Accounts are anonymous cookie tokens — zero PII, no IPs stored. Link another device via a one-time 8-char code (player menu → link device, valid 10 minutes).

## Frontend niceties

- Pixi.js physics button: spring squash/overshoot, and embers that ignite as your odds sink (red-hot in risk mode).
- driver.js tutorial on first visit — replay it with the `?` button in the header.
- Asset preloading screen, haptic feedback (Vibration API), i18n (한국어/English), mobile-safe layout.
