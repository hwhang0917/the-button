package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hwhang0917/the-button/internal/config"
	"github.com/hwhang0917/the-button/internal/game"
	"github.com/hwhang0917/the-button/internal/store"
)

// testServer is a Server with no store or logger behind it — enough for the
// handlers that only read config.
func testServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Config{
		LeaderboardSize: 20,
		NicknameMin:     2,
		NicknameMax:     16,
		LinkCodeLen:     8,
		Rules:           game.Default(),
	}
	s, err := New(cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCacheHeaders(t *testing.T) {
	h := CacheHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cases := map[string]string{
		"/assets/index-abc123.js": cacheForever,
		"/star.png":               cacheDaily,
		"/click.mp3":              cacheDaily,
		"/":                       cacheNever,
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if got := rec.Header().Get("Cache-Control"); got != want {
			t.Errorf("%s: Cache-Control = %q, want %q", path, got, want)
		}
	}
}

func TestNicknameRule(t *testing.T) {
	re := testServer(t).nicknameRe
	ok := []string{"철수", "김밥왕", "Hero_1", "버튼장인_99", "ab"}
	for _, n := range ok {
		if !re.MatchString(n) {
			t.Errorf("%q should be allowed", n)
		}
	}
	bad := []string{"a", "한", "hello world", "ㅋㅋㅋ", "가나다라마바사아자차카타파하가나다"}
	for _, n := range bad {
		if re.MatchString(n) {
			t.Errorf("%q should be rejected", n)
		}
	}
}

// TestConfigEndpoint is the contract with the frontend: every number the client
// stopped hardcoding has to survive the round trip, or the UI quotes stale
// values with no compile error to catch it.
func TestConfigEndpoint(t *testing.T) {
	s := testServer(t)
	rec := httptest.NewRecorder()
	s.handleConfig(rec, httptest.NewRequest("GET", "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"chanceTable", "maxRisk", "maxStars", "prestigeStarBonus", "prestigeSkinCap",
		"prestigeRewards", "overflowCoinPer", "tiers", "rarities", "cards",
		"charm", "headstart", "lottery", "pack", "refillPrice", "fuseCost",
		"defuseYield", "nickname",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %q — the client reads it", key)
		}
	}
	// server-side-only values must not leak into a public endpoint
	for _, key := range []string{"quota", "retiredShieldRefund", "RNG"} {
		if _, ok := got[key]; ok {
			t.Errorf("%q must stay server-side", key)
		}
	}

	r := game.Default()
	cards, ok := got["cards"].(map[string]any)
	if !ok || len(cards) != len(r.Tiers)*len(r.Rarities) {
		t.Fatalf("cards should carry all %d entries, got %d", len(r.Tiers)*len(r.Rarities), len(cards))
	}
	// omitempty must not erase a card: every one has at least one field set
	for key, v := range cards {
		if fields, _ := v.(map[string]any); len(fields) == 0 {
			t.Errorf("cards.%s serialised empty — the client would render an inert card", key)
		}
	}
	if got["maxStars"] != float64(r.MaxStars) {
		t.Errorf("maxStars = %v, want %d", got["maxStars"], r.MaxStars)
	}
}

// TestRefillIn pins refillIn to the store's hour bucket (store.bucketKey): the
// client re-polls its quota when this expires, so it must track the SERVER
// clock's rollover — the client's own top-of-hour drifts by the clock skew.
func TestRefillIn(t *testing.T) {
	got := testServer(t).stateFor(&store.Player{}, 0).RefillIn
	now := time.Now()
	want := 3600 - now.Minute()*60 - now.Second()
	// stateFor took its own time.Now(), so a second may have ticked in between
	if got < want || got > want+1 {
		t.Errorf("refillIn = %d, want %d — must count down to the top of the server's clock hour", got, want)
	}
}

// TestStateResponseKeys pins the /api/state field names the client reads.
// A bulk rename during the package split once mangled a struct tag into
// `json:"s.cfg.DevMode"`, which silently killed the DEV_MODE warning ribbon —
// nothing else in the build could catch that.
func TestStateResponseKeys(t *testing.T) {
	body, err := json.Marshal(stateResponse{})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"stars", "bestStars", "tier", "chance", "maxStars", "quotaLeft", "quota",
		"nickname", "win", "coins", "charmLevel", "headstartLevel", "staminaLevel",
		"prestige", "talismanTier", "talismanRarity", "refillUsed", "refillIn", "devMode",
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %q — the client reads it", key)
		}
	}
	if len(got) != len(want) {
		t.Errorf("state has %d fields, expected exactly %d: %v", len(got), len(want), got)
	}
}
