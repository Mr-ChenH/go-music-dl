package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	playlistTaskQueued      = "queued"
	playlistTaskResolving   = "resolving"
	playlistTaskDownloading = "downloading"
	playlistTaskCompleted   = "completed"
	playlistTaskPartial     = "partial"
	playlistTaskFailed      = "failed"
)

// PlaylistDownloadTask is the durable summary displayed above per-song records.
type PlaylistDownloadTask struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	PlaylistName      string    `gorm:"size:512;not null" json:"playlist_name"`
	Source            string    `gorm:"size:64;not null" json:"source"`
	ExternalID        string    `gorm:"size:512;not null" json:"external_id"`
	Status            string    `gorm:"size:32;not null;index" json:"status"`
	Total             int       `json:"total"`
	Completed         int       `json:"completed"`
	Success           int       `json:"success"`
	Skipped           int       `json:"skipped"`
	Failed            int       `json:"failed"`
	CurrentSong       string    `gorm:"size:512" json:"current_song"`
	Error             string    `gorm:"size:2048" json:"error"`
	LocalCollectionID uint      `json:"local_collection_id"`
	CreatedAt         time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type playlistDownloadRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cover       string `json:"cover"`
	Creator     string `json:"creator"`
	TrackCount  int    `json:"track_count"`
	Source      string `json:"source"`
	ExternalID  string `json:"external_id"`
	Link        string `json:"link"`
}

var playlistDownloadSlots = make(chan struct{}, 1)

var (
	playlistDownloadSongsLoader      = loadImportedCollectionSongs
	playlistDownloadSettingsProvider = core.GetWebSettings
	playlistDownloadDedupProvider    = core.LoadDownloadDedupSet
	playlistDownloadSongSaver        = core.DownloadWithDedupCheckForTask
	playlistDownloadSavedSongBuilder = savedSongFromDownloadedFile
)

func initPlaylistDownloadTasks() error {
	if db == nil {
		return errors.New("database is not initialized")
	}
	if err := db.AutoMigrate(&PlaylistDownloadTask{}); err != nil {
		return err
	}
	return db.Model(&PlaylistDownloadTask{}).
		Where("status IN ?", []string{playlistTaskQueued, playlistTaskResolving, playlistTaskDownloading}).
		Updates(map[string]interface{}{
			"status":       playlistTaskFailed,
			"current_song": "",
			"error":        "服务重启，下载任务已中断",
		}).Error
}

func listPlaylistDownloadTasks(limit int) ([]PlaylistDownloadTask, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var tasks []PlaylistDownloadTask
	err := db.Order("created_at DESC, id DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func updatePlaylistDownloadTask(id uint, values map[string]interface{}) {
	_ = db.Model(&PlaylistDownloadTask{}).Where("id = ?", id).Updates(values).Error
}

func playlistTaskSongLabel(song model.Song) string {
	name := strings.TrimSpace(song.Name)
	artist := strings.TrimSpace(song.Artist)
	if artist == "" {
		return name
	}
	return name + " - " + artist
}

func savedSongExtra(extra map[string]string, album, link string) string {
	values := make(map[string]string, len(extra)+2)
	for key, value := range extra {
		values[key] = value
	}
	if strings.TrimSpace(album) != "" {
		values["album"] = strings.TrimSpace(album)
	}
	if strings.TrimSpace(link) != "" {
		values["link"] = strings.TrimSpace(link)
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func savedSongFromDownloadedFile(song model.Song, result *core.DownloadedSong, downloadDir string) SavedSong {
	if result != nil && !result.Skipped && strings.TrimSpace(result.SavedPath) != "" {
		if rootAbs, err := filepath.Abs(downloadDir); err == nil {
			if track, trackErr := buildLocalMusicTrack(rootAbs, result.SavedPath); trackErr == nil {
				upsertLocalMusicIndexRow(track)
				return SavedSong{
					SongID:   track.ID,
					Source:   localMusicSource,
					Name:     song.Name,
					Artist:   song.Artist,
					Cover:    firstNonEmpty(track.Cover, song.Cover),
					Duration: firstPositive(track.Duration, song.Duration),
					Extra:    savedSongExtra(track.Extra, song.Album, song.Link),
				}
			}
		}
	}

	if row, _, err := findLocalMusicMatch(song.Name, song.Artist); err == nil && row != nil {
		cover := song.Cover
		if row.HasCover {
			cover = RoutePrefix + "/local_music/cover?id=" + row.ID
		}
		return SavedSong{
			SongID:   row.ID,
			Source:   localMusicSource,
			Name:     song.Name,
			Artist:   song.Artist,
			Cover:    cover,
			Duration: firstPositive(row.Duration, song.Duration),
			Extra:    savedSongExtra(localMusicIndexExtra(row), song.Album, song.Link),
		}
	}

	return SavedSong{
		SongID:   song.ID,
		Source:   song.Source,
		Name:     song.Name,
		Artist:   song.Artist,
		Cover:    song.Cover,
		Duration: song.Duration,
		Extra:    savedSongExtra(song.Extra, song.Album, song.Link),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func createDownloadedPlaylistCollection(req playlistDownloadRequest, songs []SavedSong) (uint, error) {
	if len(songs) == 0 {
		return 0, errors.New("没有成功下载或可复用的歌曲")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "下载的歌单"
	}

	var collectionID uint
	err := db.Transaction(func(tx *gorm.DB) error {
		candidate := name
		for suffix := 2; ; suffix++ {
			var count int64
			if err := tx.Model(&Collection{}).Where("name = ?", candidate).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				name = candidate
				break
			}
			candidate = fmt.Sprintf("%s (%d)", name, suffix)
		}

		collection := Collection{
			Name:        name,
			Description: firstNonEmpty(req.Description, "由歌单下载自动创建"),
			Cover:       req.Cover,
			Kind:        collectionKindManual,
			ContentType: collectionContentPlaylist,
			Source:      "local",
			Creator:     req.Creator,
			TrackCount:  len(songs),
		}
		if err := tx.Create(&collection).Error; err != nil {
			return err
		}
		collectionID = collection.ID
		for index := range songs {
			songs[index].CollectionID = collection.ID
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&songs).Error
	})
	return collectionID, err
}

func runPlaylistDownloadTask(taskID uint, req playlistDownloadRequest) {
	playlistDownloadSlots <- struct{}{}
	defer func() { <-playlistDownloadSlots }()
	defer func() {
		if recovered := recover(); recovered != nil {
			updatePlaylistDownloadTask(taskID, map[string]interface{}{
				"status":       playlistTaskFailed,
				"current_song": "",
				"error":        fmt.Sprintf("下载任务异常终止: %v", recovered),
			})
		}
	}()

	updatePlaylistDownloadTask(taskID, map[string]interface{}{
		"status": playlistTaskResolving,
		"error":  "",
	})
	remote := &Collection{
		Name:        req.Name,
		Description: req.Description,
		Cover:       req.Cover,
		Kind:        collectionKindImported,
		ContentType: collectionContentPlaylist,
		Source:      req.Source,
		ExternalID:  req.ExternalID,
		Link:        req.Link,
		Creator:     req.Creator,
		TrackCount:  req.TrackCount,
	}
	songs, err := playlistDownloadSongsLoader(remote)
	if err != nil || len(songs) == 0 {
		if err == nil {
			err = errors.New("歌单中没有可下载的歌曲")
		}
		updatePlaylistDownloadTask(taskID, map[string]interface{}{"status": playlistTaskFailed, "error": err.Error()})
		return
	}

	settings := playlistDownloadSettingsProvider()
	dedupSet, err := playlistDownloadDedupProvider()
	if err != nil {
		updatePlaylistDownloadTask(taskID, map[string]interface{}{"status": playlistTaskFailed, "error": err.Error()})
		return
	}
	updatePlaylistDownloadTask(taskID, map[string]interface{}{
		"status": playlistTaskDownloading,
		"total":  len(songs),
	})

	localSongs := make([]SavedSong, 0, len(songs))
	success, skipped, failed := 0, 0, 0
	for index := range songs {
		song := songs[index]
		updatePlaylistDownloadTask(taskID, map[string]interface{}{"current_song": playlistTaskSongLabel(song)})
		result, downloadErr := playlistDownloadSongSaver(
			&song,
			settings.DownloadDir,
			settings.EmbedDownload,
			settings.EmbedDownload,
			settings.DownloadFilenameTemplate,
			dedupSet,
			taskID,
			req.Name,
		)
		if downloadErr != nil {
			failed++
		} else {
			if result != nil && result.Skipped {
				skipped++
			} else {
				success++
				if result != nil && core.WebDAVConfigured(settings) {
					_ = core.UploadSongToWebDAV(settings, result.Filename, result.Data)
				}
			}
			localSongs = append(localSongs, playlistDownloadSavedSongBuilder(song, result, settings.DownloadDir))
		}
		updatePlaylistDownloadTask(taskID, map[string]interface{}{
			"completed": index + 1,
			"success":   success,
			"skipped":   skipped,
			"failed":    failed,
		})
	}

	collectionID, collectionErr := createDownloadedPlaylistCollection(req, localSongs)
	status := playlistTaskCompleted
	errText := ""
	if failed > 0 {
		status = playlistTaskPartial
	}
	if collectionErr != nil {
		status = playlistTaskFailed
		errText = "本地歌单创建失败: " + collectionErr.Error()
	}
	updatePlaylistDownloadTask(taskID, map[string]interface{}{
		"status":              status,
		"current_song":        "",
		"error":               errText,
		"local_collection_id": collectionID,
	})
	invalidateLocalMusicScanCache()
}

func RegisterPlaylistDownloadRoutes(api *gin.RouterGroup) {
	api.POST("/api/downloads/playlists", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		var req playlistDownloadRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Source = strings.TrimSpace(req.Source)
		req.ExternalID = strings.TrimSpace(req.ExternalID)
		if req.Name == "" || req.Source == "" || req.ExternalID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少歌单名称、来源或 ID"})
			return
		}
		if len(req.Name) > 500 || len(req.ExternalID) > 500 || core.GetPlaylistDetailFunc(req.Source) == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "歌单参数无效或来源不支持"})
			return
		}

		var activeCount int64
		if err := db.Model(&PlaylistDownloadTask{}).
			Where("source = ? AND external_id = ? AND status IN ?", req.Source, req.ExternalID, []string{playlistTaskQueued, playlistTaskResolving, playlistTaskDownloading}).
			Count(&activeCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "检查下载任务失败"})
			return
		}
		if activeCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "该歌单已经在下载队列中"})
			return
		}

		task := PlaylistDownloadTask{
			PlaylistName: req.Name,
			Source:       req.Source,
			ExternalID:   req.ExternalID,
			Status:       playlistTaskQueued,
			Total:        req.TrackCount,
		}
		if err := db.Create(&task).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建下载任务失败"})
			return
		}
		go runPlaylistDownloadTask(task.ID, req)
		c.JSON(http.StatusAccepted, gin.H{"status": "queued", "task_id": task.ID})
	})
}

func clearFinishedPlaylistDownloadTasks() error {
	return db.Where("status NOT IN ?", []string{playlistTaskQueued, playlistTaskResolving, playlistTaskDownloading}).Delete(&PlaylistDownloadTask{}).Error
}
