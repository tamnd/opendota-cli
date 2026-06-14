package opendota_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/opendota-cli/opendota"
)

// --- test data ---

const fakeHeroesJSON = `[
  {"id": 1, "name": "npc_dota_hero_antimage", "localized_name": "Anti-Mage", "primary_attr": "agi", "attack_type": "Melee", "roles": ["Carry", "Escape", "Nuker"]},
  {"id": 2, "name": "npc_dota_hero_axe", "localized_name": "Axe", "primary_attr": "str", "attack_type": "Melee", "roles": ["Initiator", "Durable", "Disabler", "Jungler"]},
  {"id": 5, "name": "npc_dota_hero_crystal_maiden", "localized_name": "Crystal Maiden", "primary_attr": "int", "attack_type": "Ranged", "roles": ["Support", "Disabler", "Nuker", "Jungler"]}
]`

const fakeHeroStatsJSON = `[
  {"id": 1, "name": "npc_dota_hero_antimage", "localized_name": "Anti-Mage", "primary_attr": "agi", "attack_type": "Melee", "roles": ["Carry", "Escape", "Nuker"], "pro_pick": 234, "pro_win": 112, "pro_ban": 45},
  {"id": 2, "name": "npc_dota_hero_axe", "localized_name": "Axe", "primary_attr": "str", "attack_type": "Melee", "roles": ["Initiator"], "pro_pick": 89, "pro_win": 41, "pro_ban": 12}
]`

const fakePlayerJSON = `{
  "profile": {
    "account_id": 232564659,
    "personaname": "Arteezy",
    "name": "Arteezy",
    "team_id": 8344818,
    "steamid": "76561198192830387",
    "profileurl": "https://steamcommunity.com/id/arteezy/"
  },
  "rank_tier": 80,
  "leaderboard_rank": 80
}`

const fakeMatchesJSON = `[
  {
    "match_id": 7890123456,
    "hero_id": 1,
    "kills": 12,
    "deaths": 2,
    "assists": 5,
    "duration": 3421,
    "game_mode": 22,
    "lobby_type": 7,
    "player_slot": 0,
    "radiant_win": true
  },
  {
    "match_id": 7890123457,
    "hero_id": 2,
    "kills": 5,
    "deaths": 8,
    "assists": 3,
    "duration": 2100,
    "game_mode": 22,
    "lobby_type": 7,
    "player_slot": 128,
    "radiant_win": true
  }
]`

const fakePlayerHeroesJSON = `[
  {"hero_id": 1, "last_played": 1700000000, "games": 123, "win": 89, "with_games": 45, "with_win": 30, "against_games": 67, "against_win": 30},
  {"hero_id": 5, "last_played": 1699000000, "games": 56, "win": 30, "with_games": 20, "with_win": 12, "against_games": 30, "against_win": 14}
]`

const fakeTeamsJSON = `[
  {"team_id": 8344818, "name": "Evil Geniuses", "tag": "EG", "wins": 234, "losses": 123, "rating": 1800.5, "last_match_time": 1700000000, "logo_url": "https://example.com/eg.png"},
  {"team_id": 1, "name": "Team Secret", "tag": "Secret", "wins": 567, "losses": 200, "rating": 1750.0, "last_match_time": 1699000000, "logo_url": "https://example.com/secret.png"}
]`

const fakeLeaguesJSON = `[
  {"leagueid": 13256, "name": "The International 2023", "tier": "premium"},
  {"leagueid": 13000, "name": "ESL One 2023", "tier": "professional"}
]`

// --- helpers ---

func newTestClient(ts *httptest.Server) *opendota.Client {
	cfg := opendota.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return opendota.NewClient(cfg)
}

func serve(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, body)
	}))
}

// --- tests ---

func TestHeroesSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeHeroesJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Heroes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestHeroesParsesItems(t *testing.T) {
	ts := serve(fakeHeroesJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Heroes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
	h := items[0]
	if h.ID != 1 {
		t.Errorf("ID = %d, want 1", h.ID)
	}
	if h.LocalizedName != "Anti-Mage" {
		t.Errorf("LocalizedName = %q, want Anti-Mage", h.LocalizedName)
	}
	if h.PrimaryAttr != "agi" {
		t.Errorf("PrimaryAttr = %q, want agi", h.PrimaryAttr)
	}
	if h.AttackType != "Melee" {
		t.Errorf("AttackType = %q, want Melee", h.AttackType)
	}
	if len(h.Roles) != 3 {
		t.Errorf("len(Roles) = %d, want 3", len(h.Roles))
	}
}

func TestHeroesLimitRespected(t *testing.T) {
	ts := serve(fakeHeroesJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Heroes(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestHeroStatsParsesPicks(t *testing.T) {
	ts := serve(fakeHeroStatsJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.HeroStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	h := items[0]
	if h.ProPick != 234 {
		t.Errorf("ProPick = %d, want 234", h.ProPick)
	}
	if h.ProWin != 112 {
		t.Errorf("ProWin = %d, want 112", h.ProWin)
	}
	if h.ProBan != 45 {
		t.Errorf("ProBan = %d, want 45", h.ProBan)
	}
}

func TestGetPlayerParsesProfile(t *testing.T) {
	ts := serve(fakePlayerJSON)
	defer ts.Close()

	c := newTestClient(ts)
	player, err := c.GetPlayer(context.Background(), 232564659)
	if err != nil {
		t.Fatal(err)
	}
	if player.AccountID != 232564659 {
		t.Errorf("AccountID = %d, want 232564659", player.AccountID)
	}
	if player.PersonaName != "Arteezy" {
		t.Errorf("PersonaName = %q, want Arteezy", player.PersonaName)
	}
	if player.RankTier != 80 {
		t.Errorf("RankTier = %d, want 80", player.RankTier)
	}
	if player.LeaderboardRank != 80 {
		t.Errorf("LeaderboardRank = %d, want 80", player.LeaderboardRank)
	}
	if player.TeamID != 8344818 {
		t.Errorf("TeamID = %d, want 8344818", player.TeamID)
	}
}

func TestPlayerMatchesParsesItems(t *testing.T) {
	ts := serve(fakeMatchesJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.PlayerMatches(context.Background(), 232564659, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	// First match: player_slot 0 (Radiant) + radiant_win = true => win
	m0 := items[0]
	if m0.MatchID != 7890123456 {
		t.Errorf("MatchID = %d, want 7890123456", m0.MatchID)
	}
	if !m0.Win {
		t.Error("Win = false, want true (radiant slot 0, radiant won)")
	}
	// Second match: player_slot 128 (Dire) + radiant_win = true => loss
	m1 := items[1]
	if m1.Win {
		t.Error("Win = true, want false (dire slot 128, radiant won)")
	}
}

func TestPlayerHeroesParsesItems(t *testing.T) {
	ts := serve(fakePlayerHeroesJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.PlayerHeroes(context.Background(), 232564659, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	ph := items[0]
	if ph.HeroID != 1 {
		t.Errorf("HeroID = %d, want 1", ph.HeroID)
	}
	if ph.Games != 123 {
		t.Errorf("Games = %d, want 123", ph.Games)
	}
	if ph.Win != 89 {
		t.Errorf("Win = %d, want 89", ph.Win)
	}
}

func TestTeamsParsesItems(t *testing.T) {
	ts := serve(fakeTeamsJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Teams(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	team := items[0]
	if team.ID != 8344818 {
		t.Errorf("ID = %d, want 8344818", team.ID)
	}
	if team.Name != "Evil Geniuses" {
		t.Errorf("Name = %q, want Evil Geniuses", team.Name)
	}
	if team.Tag != "EG" {
		t.Errorf("Tag = %q, want EG", team.Tag)
	}
	if team.Rating != 1800.5 {
		t.Errorf("Rating = %f, want 1800.5", team.Rating)
	}
}

func TestLeaguesParsesItems(t *testing.T) {
	ts := serve(fakeLeaguesJSON)
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Leagues(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	l := items[0]
	if l.ID != 13256 {
		t.Errorf("ID = %d, want 13256", l.ID)
	}
	if l.Name != "The International 2023" {
		t.Errorf("Name = %q, want The International 2023", l.Name)
	}
	if l.Tier != "premium" {
		t.Errorf("Tier = %q, want premium", l.Tier)
	}
}

func TestRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeHeroesJSON)
	}))
	defer ts.Close()

	cfg := opendota.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := opendota.NewClient(cfg)

	_, err := c.Heroes(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestNonRetryable4xx(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Heroes(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if hits != 1 {
		t.Errorf("server saw %d hits, want 1 (no retry on 404)", hits)
	}
}
