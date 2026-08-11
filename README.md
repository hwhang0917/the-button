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

## Config (env)

| Var | Required | Default | |
|---|---|---|---|
| `PORT` | no | `8080` | |
| `DB_PATH` | no | `./thebutton.db` | SQLite file |
| `QUOTA` | no | `5` | clicks per player per hour (resets on the clock hour) |
| `EVENTS_DIR` | no | — | anonymous NDJSON gameplay events for analytics; unset = telemetry off |

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
- RISK IT levels 1–3: odds ÷(level+1), success pays the odds back (★+round(100/chance)), card-drop odds ×(level+1).
- Tiers: Unrank → Bronze → Silver → Gold → Platinum → Diamond. First climb past your best tier refunds clicks equal to its rank.
- Successful clicks can drop collectible cards (common/rare/holo/prismatic) — tap owned cards to view them. Cards are consumable: arm one as a **talisman** (one slot; fires only inside the card's tier — common/rare +5/+10% chance on the next click, holo keeps stars on a fail, prismatic doubles the next success) or **fuse** 3 identical cards into 1 of the next rarity. Discovered cards stay in the collection even at ×0.
- **Skill shop**: sell your streak for coins (triangle value — deep streaks pay disproportionately) and buy skills: 🛡️ protection scroll (keep stars on a fail, consumable), 🍀 lucky charm (+2% success/level, cap 5), 🚀 head start (resets land at ★level, cap 3), 🎟️ 복권 (15💰 scratch ticket, up to 500💰 at ~61% payback; winnings don't count toward rank points).
- **Prestige**: at ★15 the plain sell is disabled — PRESTIGE instead converts the streak to 300/450/600 coins and promotes your star tier common → rare → holo → prismatic, then keeps counting forever (prismatic-2, prismatic-3, …) at 600 per lap. The leaderboard orders prestige tier → stars; coins are purely shop currency.
- Unique nicknames (2–16 chars, 한글/`a-z 0-9 _`, case-insensitive), asked on first visit.
- Accounts are anonymous cookie tokens — zero PII, no IPs stored. Link another device via a one-time 8-char code (player menu → link device, valid 10 minutes).

## Frontend niceties

- Pixi.js physics button: spring squash/overshoot, and embers that ignite as your odds sink (red-hot in risk mode).
- driver.js tutorial on first visit — replay it with the `?` button in the header.
- Asset preloading screen, haptic feedback (Vibration API), i18n (한국어/English), mobile-safe layout.
