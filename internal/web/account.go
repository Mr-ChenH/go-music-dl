package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

type accountPlatformStatus struct {
	Source    string `json:"source"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
}

type accountStats struct {
	Collections       int64 `json:"collections"`
	LocalTracks       int64 `json:"local_tracks"`
	DownloadRecords   int64 `json:"download_records"`
	PlaylistDownloads int64 `json:"playlist_downloads"`
}

type accountOverview struct {
	Username           string                  `json:"username"`
	Role               string                  `json:"role"`
	AuthEnabled        bool                    `json:"auth_enabled"`
	SessionIssuedAt    *time.Time              `json:"session_issued_at"`
	SessionExpiresAt   *time.Time              `json:"session_expires_at"`
	SessionMaxAgeDays  int                     `json:"session_max_age_days"`
	ConnectedPlatforms int                     `json:"connected_platforms"`
	Platforms          []accountPlatformStatus `json:"platforms"`
	Stats              accountStats            `json:"stats"`
}

var accountSettingsProvider = core.GetWebAuthSettings

func currentAccountSession(c *gin.Context, settings core.WebAuthSettings) (*time.Time, *time.Time) {
	value, err := c.Cookie(authCookieName)
	if err != nil || !validateSessionValue(settings, value, time.Now()) {
		return nil, nil
	}
	payload, err := decodeSessionPayload(value)
	if err != nil {
		return nil, nil
	}
	issuedAt := time.Unix(payload.IssuedAt, 0)
	expiresAt := issuedAt.Add(sessionMaxAge)
	return &issuedAt, &expiresAt
}

func loadAccountStats() accountStats {
	stats := accountStats{}
	if db != nil {
		_ = db.Model(&Collection{}).Count(&stats.Collections).Error
		_ = db.Model(&LocalMusicIndex{}).Count(&stats.LocalTracks).Error
		_ = db.Model(&PlaylistDownloadTask{}).Count(&stats.PlaylistDownloads).Error
	}
	if _, total, err := core.GetDownloadRecordPage(1, 1); err == nil {
		stats.DownloadRecords = total
	}
	return stats
}

func accountPlatforms() ([]accountPlatformStatus, int) {
	cookies := core.CM.GetAll()
	sources := core.GetAllSourceNames()
	platforms := make([]accountPlatformStatus, 0, len(sources))
	connected := 0
	for _, source := range sources {
		isConnected := strings.TrimSpace(cookies[source]) != ""
		if isConnected {
			connected++
		}
		platforms = append(platforms, accountPlatformStatus{
			Source:    source,
			Name:      core.GetSourceDescription(source),
			Connected: isConnected,
		})
	}
	return platforms, connected
}

func RegisterAccountRoutes(api *gin.RouterGroup) {
	api.GET("/api/account", func(c *gin.Context) {
		settings, err := accountSettingsProvider()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取账户信息失败"})
			return
		}

		username := strings.TrimSpace(settings.Username)
		if value, ok := c.Get("AuthUsername"); ok {
			if authenticatedUsername, valid := value.(string); valid && strings.TrimSpace(authenticatedUsername) != "" {
				username = strings.TrimSpace(authenticatedUsername)
			}
		}
		authEnabled := authConfigured(settings)
		if username == "" {
			username = "本地用户"
		}
		issuedAt, expiresAt := currentAccountSession(c, settings)
		platforms, connected := accountPlatforms()

		c.JSON(http.StatusOK, accountOverview{
			Username:           username,
			Role:               "管理员",
			AuthEnabled:        authEnabled,
			SessionIssuedAt:    issuedAt,
			SessionExpiresAt:   expiresAt,
			SessionMaxAgeDays:  int(sessionMaxAge / (24 * time.Hour)),
			ConnectedPlatforms: connected,
			Platforms:          platforms,
			Stats:              loadAccountStats(),
		})
	})
}
