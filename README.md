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

## Dev

```sh
go run .                    # API on :8080
cd web && npm run dev       # Vite on :5173, proxies /api
```

## Game rules

- Success chance starts at 100% and drops each star (★15 max = win).
- Fail resets to ★0 (or your head-start floor). Stars persist; quota refreshes every clock hour.
- RISK IT levels 1–3: odds ÷(level+1), success pays the odds back (★+round(100/chance)), card-drop odds ×(level+1).
- Tiers: Unrank → Bronze → Silver → Gold → Platinum → Diamond. First climb past your best tier refunds clicks equal to its rank.
- Successful clicks can drop collectible cards (common/rare/holo/prismatic) — tap owned cards to view them.
- **Skill shop**: sell your streak for coins (triangle value — deep streaks pay disproportionately; selling at ★15 restarts a won game) and buy skills: 🛡️ protection scroll (keep stars on a fail, consumable), 🍀 lucky charm (+2% success/level, cap 5), 🚀 head start (resets land at ★level, cap 3).
- Unique nicknames (3–16 chars, `a-z 0-9 _`, case-insensitive), asked on first visit.
- Accounts are anonymous cookie tokens — zero PII, no IPs stored. Link another device via a one-time 8-char code (player menu → link device, valid 10 minutes).

## Frontend niceties

- Pixi.js physics button: spring squash/overshoot, and embers that ignite as your odds sink (red-hot in risk mode).
- driver.js tutorial on first visit — replay it with the `?` button in the header.
- Asset preloading screen, haptic feedback (Vibration API), i18n (한국어/English), mobile-safe layout.
