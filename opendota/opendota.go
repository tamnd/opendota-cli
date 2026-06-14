// Package opendota is the library behind the opendota command line:
// the HTTP client, request shaping, and typed data models for the
// OpenDota API (https://api.opendota.com/api).
//
// OpenDota provides free public access to Dota 2 statistics including
// heroes, players, matches, professional teams, and leagues.
// No API key required for basic access. Rate limit: 60 requests/minute.
// The Client paces requests at 1100ms and retries transient failures.
package opendota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"sync"
	"time"
)

// Host is the API hostname.
const Host = "api.opendota.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for the OpenDota API.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.opendota.com/api",
		UserAgent: "opendota-cli/0.1.0 (github.com/tamnd/opendota-cli)",
		Rate:      1100 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Hero holds information about a Dota 2 hero.
type Hero struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`           // npc_dota_hero_antimage
	LocalizedName string   `json:"localized_name"` // Anti-Mage
	PrimaryAttr   string   `json:"primary_attr"`   // agi, str, int, all
	AttackType    string   `json:"attack_type"`    // Melee, Ranged
	Roles         []string `json:"roles"`
	ProPick       int      `json:"pro_pick"`
	ProWin        int      `json:"pro_win"`
	ProBan        int      `json:"pro_ban"`
}

// Player holds the profile and rank of a Dota 2 player.
type Player struct {
	AccountID       int    `json:"account_id"`
	PersonaName     string `json:"persona_name"`
	Name            string `json:"name"`             // pro player name if known
	RankTier        int    `json:"rank_tier"`        // 0=unranked, 10-19=Herald, ..., 80=Immortal
	LeaderboardRank int    `json:"leaderboard_rank"` // 0 if not on leaderboard
	TeamID          int    `json:"team_id"`
	SteamID         string `json:"steam_id"`
	ProfileURL      string `json:"profile_url"`
}

// Match holds a player's match result summary.
type Match struct {
	MatchID   int64 `json:"match_id"`
	HeroID    int   `json:"hero_id"`
	Kills     int   `json:"kills"`
	Deaths    int   `json:"deaths"`
	Assists   int   `json:"assists"`
	Duration  int   `json:"duration"`   // seconds
	GameMode  int   `json:"game_mode"`
	LobbyType int   `json:"lobby_type"`
	Win       bool  `json:"win"`
}

// PlayerHero holds a player's statistics for a specific hero.
type PlayerHero struct {
	HeroID       int   `json:"hero_id"`
	LastPlayed   int64 `json:"last_played"` // Unix timestamp
	Games        int   `json:"games"`
	Win          int   `json:"win"`
	WithGames    int   `json:"with_games"`
	WithWin      int   `json:"with_win"`
	AgainstGames int   `json:"against_games"`
	AgainstWin   int   `json:"against_win"`
}

// Team holds information about a professional Dota 2 team.
type Team struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Tag           string  `json:"tag"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Rating        float64 `json:"rating"`
	LastMatchTime int64   `json:"last_match_time"`
	LogoURL       string  `json:"logo_url"`
}

// League holds information about a Dota 2 league/tournament.
type League struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"` // premium, professional, amateur, etc.
}

// Client talks to the OpenDota API.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Heroes returns all Dota 2 heroes with basic information.
// If limit > 0, the results are truncated to that count.
func (c *Client) Heroes(ctx context.Context, limit int) ([]Hero, error) {
	body, err := c.get(ctx, c.cfg.BaseURL+"/heroes")
	if err != nil {
		return nil, err
	}
	var wire []wireHero
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode heroes: %w", err)
	}
	out := make([]Hero, 0, len(wire))
	for _, h := range wire {
		out = append(out, Hero{
			ID:            h.ID,
			Name:          h.Name,
			LocalizedName: h.LocalizedName,
			PrimaryAttr:   h.PrimaryAttr,
			AttackType:    h.AttackType,
			Roles:         h.Roles,
		})
	}
	return truncate(out, limit), nil
}

// HeroStats returns all heroes with detailed statistics including pro pick/win rates.
func (c *Client) HeroStats(ctx context.Context) ([]Hero, error) {
	body, err := c.get(ctx, c.cfg.BaseURL+"/heroStats")
	if err != nil {
		return nil, err
	}
	var wire []wireHeroStats
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode heroStats: %w", err)
	}
	out := make([]Hero, 0, len(wire))
	for _, h := range wire {
		out = append(out, Hero{
			ID:            h.ID,
			Name:          h.Name,
			LocalizedName: h.LocalizedName,
			PrimaryAttr:   h.PrimaryAttr,
			AttackType:    h.AttackType,
			Roles:         h.Roles,
			ProPick:       h.ProPick,
			ProWin:        h.ProWin,
			ProBan:        h.ProBan,
		})
	}
	return out, nil
}

// GetPlayer fetches a player's profile and rank by Steam account ID.
func (c *Client) GetPlayer(ctx context.Context, accountID int) (*Player, error) {
	u := fmt.Sprintf("%s/players/%d", c.cfg.BaseURL, accountID)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var wire wirePlayer
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode player %d: %w", accountID, err)
	}
	return &Player{
		AccountID:       wire.Profile.AccountID,
		PersonaName:     wire.Profile.PersonaName,
		Name:            wire.Profile.Name,
		RankTier:        wire.RankTier,
		LeaderboardRank: wire.LeaderboardRank,
		TeamID:          wire.Profile.TeamID,
		SteamID:         wire.Profile.SteamID,
		ProfileURL:      wire.Profile.ProfileURL,
	}, nil
}

// PlayerMatches fetches recent matches for a player.
// If limit <= 0, defaults to 10. Max is 100.
func (c *Client) PlayerMatches(ctx context.Context, accountID, limit int) ([]Match, error) {
	n := limit
	if n <= 0 {
		n = 10
	}
	if n > 100 {
		n = 100
	}
	u := fmt.Sprintf("%s/players/%d/matches?limit=%d", c.cfg.BaseURL, accountID, n)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var wire []wireMatch
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode player matches: %w", err)
	}
	out := make([]Match, 0, len(wire))
	for _, m := range wire {
		win := (m.PlayerSlot < 128 && m.RadiantWin) || (m.PlayerSlot >= 128 && !m.RadiantWin)
		out = append(out, Match{
			MatchID:   m.MatchID,
			HeroID:    m.HeroID,
			Kills:     m.Kills,
			Deaths:    m.Deaths,
			Assists:   m.Assists,
			Duration:  m.Duration,
			GameMode:  m.GameMode,
			LobbyType: m.LobbyType,
			Win:       win,
		})
	}
	return out, nil
}

// PlayerHeroes fetches a player's hero statistics.
// If limit <= 0, defaults to 20. Max is 100.
func (c *Client) PlayerHeroes(ctx context.Context, accountID, limit int) ([]PlayerHero, error) {
	n := limit
	if n <= 0 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	u := fmt.Sprintf("%s/players/%d/heroes?limit=%d", c.cfg.BaseURL, accountID, n)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var wire []PlayerHero
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode player heroes: %w", err)
	}
	return wire, nil
}

// Teams returns professional teams, sorted by rating.
// If limit <= 0, defaults to 20. Max is 100.
func (c *Client) Teams(ctx context.Context, limit int) ([]Team, error) {
	u := fmt.Sprintf("%s/teams", c.cfg.BaseURL)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var wire []wireTeam
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode teams: %w", err)
	}
	out := make([]Team, 0, len(wire))
	for _, t := range wire {
		out = append(out, Team{
			ID:            t.TeamID,
			Name:          t.Name,
			Tag:           t.Tag,
			Wins:          t.Wins,
			Losses:        t.Losses,
			Rating:        t.Rating,
			LastMatchTime: t.LastMatchTime,
			LogoURL:       t.LogoURL,
		})
	}
	return truncate(out, limit), nil
}

// Leagues returns Dota 2 leagues.
// If limit <= 0, defaults to 20. Max is 100.
func (c *Client) Leagues(ctx context.Context, limit int) ([]League, error) {
	u := fmt.Sprintf("%s/leagues", c.cfg.BaseURL)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var wire []wireLeague
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode leagues: %w", err)
	}
	out := make([]League, 0, len(wire))
	for _, l := range wire {
		out = append(out, League{
			ID:   l.LeagueID,
			Name: l.Name,
			Tier: l.Tier,
		})
	}
	return truncate(out, limit), nil
}

// --- wire types ---

type wireHero struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	PrimaryAttr   string   `json:"primary_attr"`
	AttackType    string   `json:"attack_type"`
	Roles         []string `json:"roles"`
}

type wireHeroStats struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	PrimaryAttr   string   `json:"primary_attr"`
	AttackType    string   `json:"attack_type"`
	Roles         []string `json:"roles"`
	ProPick       int      `json:"pro_pick"`
	ProWin        int      `json:"pro_win"`
	ProBan        int      `json:"pro_ban"`
}

type wirePlayer struct {
	Profile struct {
		AccountID   int    `json:"account_id"`
		PersonaName string `json:"personaname"`
		Name        string `json:"name"`
		TeamID      int    `json:"team_id"`
		SteamID     string `json:"steamid"`
		ProfileURL  string `json:"profileurl"`
	} `json:"profile"`
	RankTier        int `json:"rank_tier"`
	LeaderboardRank int `json:"leaderboard_rank"`
}

type wireMatch struct {
	MatchID    int64 `json:"match_id"`
	HeroID     int   `json:"hero_id"`
	Kills      int   `json:"kills"`
	Deaths     int   `json:"deaths"`
	Assists    int   `json:"assists"`
	Duration   int   `json:"duration"`
	GameMode   int   `json:"game_mode"`
	LobbyType  int   `json:"lobby_type"`
	PlayerSlot int   `json:"player_slot"`
	RadiantWin bool  `json:"radiant_win"`
}

type wireTeam struct {
	TeamID        int     `json:"team_id"`
	Name          string  `json:"name"`
	Tag           string  `json:"tag"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	Rating        float64 `json:"rating"`
	LastMatchTime int64   `json:"last_match_time"`
	LogoURL       string  `json:"logo_url"`
}

type wireLeague struct {
	LeagueID int    `json:"leagueid"`
	Name     string `json:"name"`
	Tier     string `json:"tier"`
}

// --- HTTP internals ---

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	// Validate the URL is parseable (for test server overrides)
	if _, err := neturl.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("invalid url %s: %w", rawURL, err)
	}
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

func truncate[T any](s []T, limit int) []T {
	if limit > 0 && len(s) > limit {
		return s[:limit]
	}
	return s
}
