# The Button

RNG enchant clicker (강화 시뮬레이터). One button, decreasing odds, full reset on fail.
All rolls happen server-side with `crypto/rand` — the client only renders.

## Build & run

```sh
cd web && npm install && npm run build && cd ..  # frontend → web/dist (embedded)
go build -o thebutton .
IP_SALT=change-me ./thebutton
```

## Config (env)

| Var | Required | Default | |
|---|---|---|---|
| `IP_SALT` | yes | — | salt for hashing client IPs (only the hash is stored) |
| `PORT` | no | `8080` | |
| `DB_PATH` | no | `./thebutton.db` | SQLite file |
| `QUOTA` | no | `5` | clicks per IP per hour (resets on the clock hour) |

## Dev

```sh
IP_SALT=dev go run .        # API on :8080
cd web && npm run dev       # Vite on :5173, proxies /api
```

## Game rules

- Success chance starts at 100% and drops each star (★15 max = win).
- Fail resets to ★0. Stars persist; quota refreshes every clock hour.
- RISK IT: half odds, ★+2 on success, doubled card-drop chance.
- Tiers: Unrank → Bronze → Silver → Gold → Platinum → Diamond.
- Successful clicks can drop collectible cards (common/rare/holo/prismatic).
