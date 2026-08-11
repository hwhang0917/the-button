# The Button

RNG enchant clicker (강화 시뮬레이터). One button, decreasing odds, full reset on fail.
All rolls happen server-side with `crypto/rand` — the client only renders.

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
- Fail resets to ★0. Stars persist; quota refreshes every clock hour.
- RISK IT levels 1–3: odds ÷(level+1), success pays the odds back (★+round(100/chance)), card-drop odds ×(level+1).
- Tiers: Unrank → Bronze → Silver → Gold → Platinum → Diamond. First climb past your best tier refunds clicks equal to its rank.
- Successful clicks can drop collectible cards (common/rare/holo/prismatic).
- Accounts are anonymous cookie tokens — zero PII, no IPs stored. Link another device via a one-time 8-char code (player menu → link device, valid 10 minutes).
