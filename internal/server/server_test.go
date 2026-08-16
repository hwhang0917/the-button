package server

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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

// TestGzip404StaysDecodable pins the fix for garbled 404 pages: the file
// server's error path strips Content-Encoding (Go 1.23+) while the body still
// flows through the gzip writer, so the header must be re-asserted.
func TestGzip404StaysDecodable(t *testing.T) {
	h := GzipText(http.FileServerFS(fstest.MapFS{}))
	req := httptest.NewRequest("GET", "/kdjfksdj", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip (body is compressed)", got)
	}
	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("body is not valid gzip: %v", err)
	}
	body, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !strings.Contains(string(body), "404") {
		t.Errorf("decoded body = %q, want a 404 message", body)
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
		"charm", "headstart", "stamina", "magnet", "golden", "lottery", "pack", "refillPrice", "fuseCost",
		"defuseYield", "cardSell", "nickname",
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

// TestSignupGate pins the no-anonymous-play rule at the API level: gameplay
// mutations 403 until a nickname is set, and signup itself stays open.
func TestSignupGate(t *testing.T) {
	st, err := store.Open(t.TempDir()+"/test.db", 25)
	if err != nil {
		t.Fatal(err)
	}
	srv := testServer(t)
	srv.store = st
	ts := httptest.NewServer(srv.Handler(fstest.MapFS{}))
	defer ts.Close()

	res, err := http.Post(ts.URL+"/api/click", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous click = %d, want 403", res.StatusCode)
	}
	cookies := res.Cookies()
	withCookies := func(method, path, body string) *http.Response {
		req, _ := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
		for _, c := range cookies {
			req.AddCookie(c)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := withCookies("POST", "/api/nickname", `{"name":"tester"}`); r.StatusCode != http.StatusOK {
		t.Fatalf("signup = %d, want 200", r.StatusCode)
	}
	if r := withCookies("POST", "/api/click", "{}"); r.StatusCode != http.StatusOK {
		t.Fatalf("click after signup = %d, want 200", r.StatusCode)
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
		"magnetLevel", "goldenLevel",
		"prestige", "talismanTier", "talismanRarity", "refillsLeft", "refillIn", "devMode",
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

// TestDocs pins the SHOW_DOCS gate: the docs routes exist only when enabled.
func TestDocs(t *testing.T) {
	get := func(srv *Server, path string) *http.Response {
		ts := httptest.NewServer(srv.Handler(fstest.MapFS{}))
		defer ts.Close()
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	off := testServer(t)
	if res := get(off, "/api/docs"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("docs with flag off = %d, want 404", res.StatusCode)
	}

	on := testServer(t)
	on.cfg.ShowDocs = true
	if res := get(on, "/api/docs"); res.StatusCode != http.StatusOK ||
		!strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("docs page = %d %s, want 200 text/html", res.StatusCode, res.Header.Get("Content-Type"))
	}
	res := get(on, "/api/docs/openapi.yml")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("spec = %d, want 200", res.StatusCode)
	}
	body := make([]byte, 8)
	res.Body.Read(body)
	if !strings.HasPrefix(string(body), "openapi:") {
		t.Fatalf("spec body starts with %q, want openapi:", body)
	}
}

// TestStateClampsStarsToCap: a config change (or an older ruleset) can leave
// stored stars above the current cap — the client must never see 18/15.
func TestStateClampsStarsToCap(t *testing.T) {
	got := testServer(t).stateFor(&store.Player{Stars: 18}, 0)
	if got.Stars != 15 || got.MaxStars != 15 || !got.Win {
		t.Fatalf("over-cap stars must clamp to a win at the cap, got %d/%d win=%v",
			got.Stars, got.MaxStars, got.Win)
	}
}
