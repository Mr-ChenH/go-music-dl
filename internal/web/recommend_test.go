package web

import (
	"errors"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

func resetKugouRecommendCache() {
	kugouRecommendCache.Lock()
	kugouRecommendCache.playlists = nil
	kugouRecommendCache.fetchedAt = time.Time{}
	kugouRecommendCache.Unlock()
}

func TestKugouRecommendationsRetryAndCacheSuccess(t *testing.T) {
	resetKugouRecommendCache()
	t.Cleanup(resetKugouRecommendCache)

	now := time.Now()
	calls := 0
	playlists, err := loadKugouRecommendedPlaylistsWithRetry(func() ([]model.Playlist, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("temporary kugou failure")
		}
		return []model.Playlist{{ID: "recovered", Source: "kugou"}}, nil
	}, []time.Duration{0, 0}, now)
	if err != nil {
		t.Fatalf("loadKugouRecommendedPlaylistsWithRetry() error = %v", err)
	}
	if calls != 3 || len(playlists) != 1 || playlists[0].ID != "recovered" {
		t.Fatalf("unexpected retry result: calls=%d playlists=%+v", calls, playlists)
	}

	playlists[0].ID = "mutated"
	cached, err := loadKugouRecommendedPlaylistsWithRetry(func() ([]model.Playlist, error) {
		t.Fatal("fresh cache should avoid another upstream request")
		return nil, nil
	}, nil, now.Add(time.Minute))
	if err != nil || len(cached) != 1 || cached[0].ID != "recovered" {
		t.Fatalf("unexpected cached result: playlists=%+v err=%v", cached, err)
	}
}

func TestKugouRecommendationsUseRecentStaleCacheAfterRetriesFail(t *testing.T) {
	resetKugouRecommendCache()
	t.Cleanup(resetKugouRecommendCache)

	now := time.Now()
	kugouRecommendCache.Lock()
	kugouRecommendCache.playlists = []model.Playlist{{ID: "last-success", Source: "kugou"}}
	kugouRecommendCache.fetchedAt = now.Add(-kugouRecommendCacheTTL - time.Minute)
	kugouRecommendCache.Unlock()

	calls := 0
	playlists, err := loadKugouRecommendedPlaylistsWithRetry(func() ([]model.Playlist, error) {
		calls++
		return nil, errors.New("kugou unavailable")
	}, []time.Duration{0, 0}, now)
	if err != nil {
		t.Fatalf("stale cache fallback returned error: %v", err)
	}
	if calls != 3 || len(playlists) != 1 || playlists[0].ID != "last-success" {
		t.Fatalf("unexpected stale fallback: calls=%d playlists=%+v", calls, playlists)
	}
}

func TestKugouRecommendationsRejectExpiredStaleCache(t *testing.T) {
	resetKugouRecommendCache()
	t.Cleanup(resetKugouRecommendCache)

	now := time.Now()
	kugouRecommendCache.Lock()
	kugouRecommendCache.playlists = []model.Playlist{{ID: "expired", Source: "kugou"}}
	kugouRecommendCache.fetchedAt = now.Add(-kugouRecommendStaleLimit - time.Minute)
	kugouRecommendCache.Unlock()

	playlists, err := loadKugouRecommendedPlaylistsWithRetry(func() ([]model.Playlist, error) {
		return nil, errors.New("kugou unavailable")
	}, nil, now)
	if err == nil || len(playlists) != 0 {
		t.Fatalf("expired cache should not hide the upstream failure: playlists=%+v err=%v", playlists, err)
	}
}
