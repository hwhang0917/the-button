package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	errQuotaExceeded = errors.New("daily quota exceeded")
	errNicknameTaken = errors.New("nickname taken")
)

type store struct {
	db *sql.DB
}

func openStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", path+"?_time_format=sqlite&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	// ponytail: single connection sidesteps SQLite write contention; pool tuning if traffic demands
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			token           TEXT PRIMARY KEY,
			nickname        TEXT NOT NULL DEFAULT '',
			stars           INTEGER NOT NULL DEFAULT 0,
			best_stars      INTEGER NOT NULL DEFAULT 0,
			best_at         TIMESTAMP,
			updated_at      TIMESTAMP NOT NULL,
			coins           INTEGER NOT NULL DEFAULT 0,
			shield_charges  INTEGER NOT NULL DEFAULT 0,
			charm_level     INTEGER NOT NULL DEFAULT 0,
			headstart_level INTEGER NOT NULL DEFAULT 0,
			prestige        INTEGER NOT NULL DEFAULT 0,
			talisman_tier   TEXT NOT NULL DEFAULT '',
			talisman_rarity TEXT NOT NULL DEFAULT '',
			refill_day      TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS player_quota (
			player_token TEXT NOT NULL,
			day          TEXT NOT NULL,
			count        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player_token, day)
		);
		DROP TABLE IF EXISTS quota; -- old IP-keyed quota; hourly data, disposable, no-op after first boot
		-- one-time dedupe so the unique index below can be created on old DBs;
		-- later duplicates blank out (no-op once the index exists)
		UPDATE players SET nickname = '' WHERE nickname != '' AND rowid NOT IN (
			SELECT MIN(rowid) FROM players WHERE nickname != '' GROUP BY nickname COLLATE NOCASE
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_players_nickname
			ON players(nickname COLLATE NOCASE) WHERE nickname != '';
		CREATE TABLE IF NOT EXISTS cards (
			player_token TEXT NOT NULL,
			tier         TEXT NOT NULL,
			rarity       TEXT NOT NULL,
			count        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (player_token, tier, rarity)
		);`)
	if err != nil {
		return nil, err
	}
	// bring pre-skill-shop DBs up to the canonical schema; sqlite has no
	// ADD COLUMN IF NOT EXISTS, so ignore the duplicate-column error
	// old DBs may carry an unused `earned` column from the points-rank era; harmless
	for _, ddl := range []string{
		"coins INTEGER NOT NULL DEFAULT 0",
		"shield_charges INTEGER NOT NULL DEFAULT 0",
		"charm_level INTEGER NOT NULL DEFAULT 0",
		"headstart_level INTEGER NOT NULL DEFAULT 0",
		"prestige INTEGER NOT NULL DEFAULT 0",
		"talisman_tier TEXT NOT NULL DEFAULT ''",
		"talisman_rarity TEXT NOT NULL DEFAULT ''",
		"refill_day TEXT NOT NULL DEFAULT ''",
	} {
		_, err := db.Exec("ALTER TABLE players ADD COLUMN " + ddl)
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return nil, err
		}
	}
	return &store{db: db}, nil
}

type player struct {
	Token          string
	Nickname       string
	Stars          int
	BestStars      int
	Coins          int
	ShieldCharges  int
	CharmLevel     int
	HeadstartLevel int
	Prestige       int
	TalismanTier   string
	TalismanRarity string
	RefillDay      string
}

func (s *store) getOrCreatePlayer(token string) (*player, error) {
	_, err := s.db.Exec(`INSERT INTO players (token, updated_at) VALUES (?, ?)
		ON CONFLICT (token) DO NOTHING`, token, time.Now())
	if err != nil {
		return nil, err
	}
	p := &player{Token: token}
	err = s.db.QueryRow(`SELECT nickname, stars, best_stars, coins, shield_charges, charm_level, headstart_level,
		prestige, talisman_tier, talisman_rarity, refill_day
		FROM players WHERE token = ?`, token).
		Scan(&p.Nickname, &p.Stars, &p.BestStars, &p.Coins, &p.ShieldCharges, &p.CharmLevel, &p.HeadstartLevel,
			&p.Prestige, &p.TalismanTier, &p.TalismanRarity, &p.RefillDay)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// buySkill spends coins on one unit of a skill column. The current-value pin
// keeps concurrent buys from skipping the price ladder or double-spending.
// col comes from a fixed whitelist in the handler, never from user input.
func (s *store) buySkill(token, col string, price, cur int) (bool, error) {
	q := fmt.Sprintf(`UPDATE players SET coins = coins - ?, %s = %s + 1, updated_at = ?
		WHERE token = ? AND coins >= ? AND %s = ?`, col, col, col)
	res, err := s.db.Exec(q, price, time.Now(), token, price, cur)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// consumeShield burns one charge; the guard makes two racing fails fight over
// the last charge instead of both being saved by it.
func (s *store) consumeShield(token string) (bool, error) {
	res, err := s.db.Exec(`UPDATE players SET shield_charges = shield_charges - 1
		WHERE token = ? AND shield_charges > 0`, token)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// playLottery settles a ticket in one statement: price out, prize in. The
// prize deliberately never touches `earned` — gross winnings would let bulk
// tickets buy leaderboard rank while losing coins net.
func (s *store) playLottery(token string, price, prize int) (bool, error) {
	res, err := s.db.Exec(`UPDATE players SET coins = coins - ? + ?, updated_at = ?
		WHERE token = ? AND coins >= ?`, price, prize, time.Now(), token, price)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// buyPack deducts the pack price and grants the rolled card atomically.
func (s *store) buyPack(token string, price int, tier, rarity string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE players SET coins = coins - ?, updated_at = ?
		WHERE token = ? AND coins >= ?`, price, time.Now(), token, price)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	if _, err := tx.Exec(`INSERT INTO cards (player_token, tier, rarity, count) VALUES (?, ?, ?, 1)
		ON CONFLICT (player_token, tier, rarity) DO UPDATE SET count = count + 1`, token, tier, rarity); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// refillQuota buys back the current hour's spent clicks: coins out, the hour
// bucket's count zeroed. Once per day — the refill_day pin rejects a second
// purchase — and rejected when broke or when nothing was spent.
func (s *store) refillQuota(token string, price int, day string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE players SET coins = coins - ?, refill_day = ?, updated_at = ?
		WHERE token = ? AND coins >= ? AND refill_day != ?`, price, day, time.Now(), token, price, day)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	res, err = tx.Exec(`UPDATE player_quota SET count = 0
		WHERE player_token = ? AND day = ? AND count > 0`, token, bucketKey(time.Now()))
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	return true, tx.Commit()
}

// sellStreak converts the streak to coins; the stars pin rejects a stale sell
// when another request already changed the streak.
func (s *store) sellStreak(token string, gain, toStars, fromStars int) (bool, error) {
	res, err := s.db.Exec(`UPDATE players SET coins = coins + ?, stars = ?, updated_at = ?
		WHERE token = ? AND stars = ?`, gain, toStars, time.Now(), token, fromStars)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// prestigeStreak cashes a maxed streak: big payout, unbounded prestige level
// bump (prismatic laps keep counting), reset to the floor.
func (s *store) prestigeStreak(token string, reward, toStars, fromStars int) (bool, error) {
	res, err := s.db.Exec(`UPDATE players SET coins = coins + ?, prestige = prestige + 1,
		stars = ?, updated_at = ?
		WHERE token = ? AND stars = ?`,
		reward, toStars, time.Now(), token, fromStars)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// deletePlayer wipes the player row, cards, and quota. A recreated account
// starts with fresh clicks — accepted; there is nothing else to key quota on.
func (s *store) deletePlayer(token string) error {
	for _, q := range []string{
		`DELETE FROM cards WHERE player_token = ?`,
		`DELETE FROM player_quota WHERE player_token = ?`,
		`DELETE FROM players WHERE token = ?`,
	} {
		if _, err := s.db.Exec(q, token); err != nil {
			return err
		}
	}
	return nil
}

// savePlayerStars persists a roll result; coinDelta credits an overflow
// jackpot in the same write.
func (s *store) savePlayerStars(token string, stars, coinDelta int) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE players SET stars = ?, updated_at = ?,
		coins = coins + ?,
		best_stars = MAX(best_stars, ?),
		best_at = CASE WHEN ? > best_stars THEN ? ELSE best_at END
		WHERE token = ?`, stars, now, coinDelta, stars, stars, now, token)
	return err
}

func (s *store) setNickname(token, nickname string) error {
	// friendly pre-check; the unique index backstops the lookup-to-update race
	var taken bool
	err := s.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM players WHERE nickname = ? COLLATE NOCASE AND token != ?)`,
		nickname, token).Scan(&taken)
	if err != nil {
		return err
	}
	if taken {
		return errNicknameTaken
	}
	_, err = s.db.Exec(`UPDATE players SET nickname = ?, updated_at = ? WHERE token = ?`,
		nickname, time.Now(), token)
	return err
}

// bucketKey is the quota bucket for a moment in server-local time; quota
// refreshes at the top of each clock hour.
func bucketKey(t time.Time) string {
	return t.Format("2006-01-02T15")
}

// consumeQuota spends one click for the current hour, or errQuotaExceeded if none left.
// Returns clicks remaining after the spend.
func (s *store) consumeQuota(token string, limit int) (int, error) {
	res, err := s.db.Exec(`INSERT INTO player_quota (player_token, day, count) VALUES (?, ?, 1)
		ON CONFLICT (player_token, day) DO UPDATE SET count = count + 1 WHERE count < ?`,
		token, bucketKey(time.Now()), limit)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, errQuotaExceeded
	}
	used, err := s.quotaUsed(token)
	if err != nil {
		return 0, err
	}
	return limit - used, nil
}

// grantQuota hands back bonus clicks in the current hour bucket; the count may
// go negative, which just means extra headroom until the next refill.
func (s *store) grantQuota(token string, n int) error {
	_, err := s.db.Exec(`UPDATE player_quota SET count = count - ? WHERE player_token = ? AND day = ?`,
		n, token, bucketKey(time.Now()))
	return err
}

func (s *store) quotaUsed(token string) (int, error) {
	var used int
	err := s.db.QueryRow(`SELECT count FROM player_quota WHERE player_token = ? AND day = ?`,
		token, bucketKey(time.Now())).Scan(&used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return used, err
}

func (s *store) addCard(token, tier, rarity string) error {
	_, err := s.db.Exec(`INSERT INTO cards (player_token, tier, rarity, count) VALUES (?, ?, ?, 1)
		ON CONFLICT (player_token, tier, rarity) DO UPDATE SET count = count + 1`,
		token, tier, rarity)
	return err
}

// armTalisman consumes one copy of a card and arms it as the player's single
// talisman slot. Card rows are never deleted — a row at count 0 stays as the
// permanent "discovered" marker for the collection.
func (s *store) armTalisman(token, tier, rarity string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE cards SET count = count - 1
		WHERE player_token = ? AND tier = ? AND rarity = ? AND count >= 1`, token, tier, rarity)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	res, err = tx.Exec(`UPDATE players SET talisman_tier = ?, talisman_rarity = ?, updated_at = ?
		WHERE token = ? AND talisman_tier = ''`, tier, rarity, time.Now(), token)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	return true, tx.Commit()
}

func (s *store) clearTalisman(token string) error {
	_, err := s.db.Exec(`UPDATE players SET talisman_tier = '', talisman_rarity = '' WHERE token = ?`, token)
	return err
}

// cancelTalisman disarms the slot and refunds the card copy.
func (s *store) cancelTalisman(token string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var tier, rarity string
	err = tx.QueryRow(`SELECT talisman_tier, talisman_rarity FROM players
		WHERE token = ? AND talisman_tier != ''`, token).Scan(&tier, &rarity)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(`INSERT INTO cards (player_token, tier, rarity, count) VALUES (?, ?, ?, 1)
		ON CONFLICT (player_token, tier, rarity) DO UPDATE SET count = count + 1`, token, tier, rarity); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE players SET talisman_tier = '', talisman_rarity = '' WHERE token = ?`, token); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// defuseCard breaks one card into defuseYield copies of the rarity below.
func (s *store) defuseCard(token, tier, rarity, lower string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE cards SET count = count - 1
		WHERE player_token = ? AND tier = ? AND rarity = ? AND count >= 1`, token, tier, rarity)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	if _, err := tx.Exec(`INSERT INTO cards (player_token, tier, rarity, count) VALUES (?, ?, ?, ?)
		ON CONFLICT (player_token, tier, rarity) DO UPDATE SET count = count + ?`,
		token, tier, lower, defuseYield, defuseYield); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// fuseCards burns 3 copies of a card into 1 of the next rarity, same tier.
func (s *store) fuseCards(token, tier, rarity, next string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE cards SET count = count - 3
		WHERE player_token = ? AND tier = ? AND rarity = ? AND count >= 3`, token, tier, rarity)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return false, nil
	}
	if _, err := tx.Exec(`INSERT INTO cards (player_token, tier, rarity, count) VALUES (?, ?, ?, 1)
		ON CONFLICT (player_token, tier, rarity) DO UPDATE SET count = count + 1`, token, tier, next); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

type ownedCard struct {
	Tier   string `json:"tier"`
	Rarity string `json:"rarity"`
	Count  int    `json:"count"`
}

func (s *store) getCards(token string) ([]ownedCard, error) {
	rows, err := s.db.Query(`SELECT tier, rarity, count FROM cards WHERE player_token = ?`, token)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []ownedCard{}
	for rows.Next() {
		var c ownedCard
		if err := rows.Scan(&c.Tier, &c.Rarity, &c.Count); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

type rankEntry struct {
	Nickname  string `json:"nickname"`
	Stars     int    `json:"stars"`
	BestStars int    `json:"bestStars"`
	Tier      string `json:"tier"`
	Prestige  int    `json:"prestige"`
}

func (s *store) leaderboard(limit int) ([]rankEntry, error) {
	rows, err := s.db.Query(`SELECT nickname, stars, best_stars, prestige FROM players
		WHERE nickname != ''
		ORDER BY prestige DESC, stars DESC, best_stars DESC, best_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []rankEntry{}
	for rows.Next() {
		var e rankEntry
		if err := rows.Scan(&e.Nickname, &e.Stars, &e.BestStars, &e.Prestige); err != nil {
			return nil, err
		}
		e.Tier = tierFor(e.Stars)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
