package main

import (
	"database/sql"
	"errors"
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
			token      TEXT PRIMARY KEY,
			nickname   TEXT NOT NULL DEFAULT '',
			stars      INTEGER NOT NULL DEFAULT 0,
			best_stars INTEGER NOT NULL DEFAULT 0,
			best_at    TIMESTAMP,
			updated_at TIMESTAMP NOT NULL
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
	return &store{db: db}, nil
}

type player struct {
	Token     string
	Nickname  string
	Stars     int
	BestStars int
}

func (s *store) getOrCreatePlayer(token string) (*player, error) {
	_, err := s.db.Exec(`INSERT INTO players (token, updated_at) VALUES (?, ?)
		ON CONFLICT (token) DO NOTHING`, token, time.Now())
	if err != nil {
		return nil, err
	}
	p := &player{Token: token}
	err = s.db.QueryRow(`SELECT nickname, stars, best_stars FROM players WHERE token = ?`, token).
		Scan(&p.Nickname, &p.Stars, &p.BestStars)
	if err != nil {
		return nil, err
	}
	return p, nil
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

func (s *store) savePlayerStars(token string, stars int) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE players SET stars = ?, updated_at = ?,
		best_stars = MAX(best_stars, ?),
		best_at = CASE WHEN ? > best_stars THEN ? ELSE best_at END
		WHERE token = ?`, stars, now, stars, stars, now, token)
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
}

func (s *store) leaderboard(limit int) ([]rankEntry, error) {
	rows, err := s.db.Query(`SELECT nickname, stars, best_stars FROM players
		WHERE nickname != '' ORDER BY stars DESC, best_stars DESC, best_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []rankEntry{}
	for rows.Next() {
		var e rankEntry
		if err := rows.Scan(&e.Nickname, &e.Stars, &e.BestStars); err != nil {
			return nil, err
		}
		e.Tier = tierFor(e.Stars)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
