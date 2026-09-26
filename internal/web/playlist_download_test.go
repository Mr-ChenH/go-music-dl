package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func setupPlaylistDownloadTestDB(t *testing.T) {
	t.Helper()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(t.TempDir(), "settings.db"))
	resetCollectionStateForTest()
	t.Cleanup(resetCollectionStateForTest)
	InitDB()
}

func TestInitPlaylistDownloadTasksMarksInterruptedTasksFailed(t *testing.T) {
	setupPlaylistDownloadTestDB(t)

	task := PlaylistDownloadTask{
		PlaylistName: "Interrupted",
		Source:       "qq",
		ExternalID:   "playlist-1",
		Status:       playlistTaskDownloading,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := initPlaylistDownloadTasks(); err != nil {
		t.Fatalf("initPlaylistDownloadTasks: %v", err)
	}
	if err := db.First(&task, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if task.Status != playlistTaskFailed || !strings.Contains(task.Error, "服务重启") {
		t.Fatalf("interrupted task = %+v, want failed restart state", task)
	}
}

func TestCreateDownloadedPlaylistCollectionCreatesEditableLocalPlaylist(t *testing.T) {
	setupPlaylistDownloadTestDB(t)

	request := playlistDownloadRequest{Name: "Downloaded Mix", Description: "Source playlist", Cover: "https://example.com/cover.jpg"}
	collectionID, err := createDownloadedPlaylistCollection(request, []SavedSong{{
		SongID: "local-file-id",
		Source: localMusicSource,
		Name:   "Track",
		Artist: "Artist",
	}})
	if err != nil {
		t.Fatalf("createDownloadedPlaylistCollection: %v", err)
	}
	collection, err := loadCollection(strconv.FormatUint(uint64(collectionID), 10))
	if err != nil {
		var stored Collection
		if dbErr := db.First(&stored, collectionID).Error; dbErr != nil {
			t.Fatalf("load created collection: %v (original load error: %v)", dbErr, err)
		}
		collection = &stored
	}
	if !collection.isManual() || collection.normalizedSource() != "local" || !collection.editable() {
		t.Fatalf("created collection is not an editable local playlist: %+v", collection)
	}
	var songs []SavedSong
	if err := db.Where("collection_id = ?", collectionID).Find(&songs).Error; err != nil {
		t.Fatalf("load saved songs: %v", err)
	}
	if len(songs) != 1 || songs[0].Source != localMusicSource {
		t.Fatalf("saved songs = %+v, want one local song", songs)
	}
}

func TestRunPlaylistDownloadTaskCompletesAndCreatesCollection(t *testing.T) {
	setupPlaylistDownloadTestDB(t)
	defer resetCollectionStateForTest()

	originalLoader := playlistDownloadSongsLoader
	originalSettings := playlistDownloadSettingsProvider
	originalDedup := playlistDownloadDedupProvider
	originalSaver := playlistDownloadSongSaver
	originalBuilder := playlistDownloadSavedSongBuilder
	t.Cleanup(func() {
		playlistDownloadSongsLoader = originalLoader
		playlistDownloadSettingsProvider = originalSettings
		playlistDownloadDedupProvider = originalDedup
		playlistDownloadSongSaver = originalSaver
		playlistDownloadSavedSongBuilder = originalBuilder
	})
	playlistDownloadSongsLoader = func(*Collection) ([]model.Song, error) {
		return []model.Song{
			{ID: "one", Source: "qq", Name: "One", Artist: "Artist"},
			{ID: "two", Source: "qq", Name: "Two", Artist: "Artist"},
		}, nil
	}
	playlistDownloadSettingsProvider = func() core.WebSettings {
		return core.WebSettings{DownloadDir: filepath.Join(filepath.Dir(core.ConfigDBPath()), "downloads")}
	}
	playlistDownloadDedupProvider = func() (core.DownloadDedupIndex, error) { return core.DownloadDedupIndex{}, nil }
	playlistDownloadSongSaver = func(song *model.Song, _ string, _, _ bool, _ string, _ core.DownloadDedupIndex, taskID uint, playlistName string, onProgress core.DownloadProgressFunc) (*core.DownloadedSong, error) {
		if taskID == 0 || playlistName != "Downloaded Mix" {
			t.Fatalf("download context = task %d playlist %q", taskID, playlistName)
		}
		if onProgress == nil {
			t.Fatal("playlist saver missing progress callback")
		}
		onProgress(core.DownloadProgress{DownloadedBytes: 2 << 20, TotalBytes: 8 << 20, BytesPerSecond: 1 << 20})
		var running PlaylistDownloadTask
		if err := db.First(&running, taskID).Error; err != nil {
			t.Fatalf("reload running task: %v", err)
		}
		if running.CurrentSong == "" || running.CurrentIndex == 0 || running.CurrentBytes != 2<<20 || running.CurrentTotalBytes != 8<<20 || running.CurrentSpeed != 1<<20 || running.StartedAt == nil {
			t.Fatalf("running task missing live progress: %+v", running)
		}
		return &core.DownloadedSong{}, nil
	}
	playlistDownloadSavedSongBuilder = func(song model.Song, _ *core.DownloadedSong, _ string) SavedSong {
		return SavedSong{SongID: song.ID, Source: localMusicSource, Name: song.Name, Artist: song.Artist}
	}

	task := PlaylistDownloadTask{PlaylistName: "Downloaded Mix", Source: "qq", ExternalID: "remote-id", Status: playlistTaskQueued}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	runPlaylistDownloadTask(task.ID, playlistDownloadRequest{Name: task.PlaylistName, Source: task.Source, ExternalID: task.ExternalID})
	if err := db.First(&task, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if task.Status != playlistTaskCompleted || task.Completed != 2 || task.Success != 2 || task.LocalCollectionID == 0 {
		t.Fatalf("completed task = %+v", task)
	}
	collection, err := loadCollection(strconv.FormatUint(uint64(task.LocalCollectionID), 10))
	if err != nil {
		t.Fatalf("load generated collection: %v", err)
	}
	if !collection.isManual() || countSavedSongs(collection.ID) != 2 {
		t.Fatalf("generated collection = %+v, songs=%d", collection, countSavedSongs(collection.ID))
	}
}

func TestRunPlaylistDownloadTaskRecoversProviderPanic(t *testing.T) {
	setupPlaylistDownloadTestDB(t)
	defer resetCollectionStateForTest()

	originalLoader := playlistDownloadSongsLoader
	t.Cleanup(func() { playlistDownloadSongsLoader = originalLoader })
	playlistDownloadSongsLoader = func(*Collection) ([]model.Song, error) {
		panic("provider crashed")
	}

	task := PlaylistDownloadTask{PlaylistName: "Broken", Source: "qq", ExternalID: "remote-id", Status: playlistTaskQueued}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	runPlaylistDownloadTask(task.ID, playlistDownloadRequest{Name: task.PlaylistName, Source: task.Source, ExternalID: task.ExternalID})
	if err := db.First(&task, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if task.Status != playlistTaskFailed || !strings.Contains(task.Error, "provider crashed") {
		t.Fatalf("panic task = %+v, want failed status", task)
	}
	select {
	case playlistDownloadSlots <- struct{}{}:
		<-playlistDownloadSlots
	default:
		t.Fatal("panic must release playlist download queue slot")
	}
}

func TestPlaylistDownloadTaskRecordsEndpointKeepsTasksSeparate(t *testing.T) {
	setupPlaylistDownloadTestDB(t)
	gin.SetMode(gin.TestMode)
	if err := core.ClearDownloadRecords(); err != nil {
		t.Fatalf("clear download records: %v", err)
	}

	task := PlaylistDownloadTask{PlaylistName: "Grouped Mix", Cover: "https://example.com/cover.jpg", Source: "qq", ExternalID: "mix-1", Status: playlistTaskCompleted, Total: 2, Completed: 2}
	other := PlaylistDownloadTask{PlaylistName: "Other Mix", Source: "netease", ExternalID: "mix-2", Status: playlistTaskCompleted, Total: 1, Completed: 1}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other task: %v", err)
	}
	if err := core.SaveDownloadRecordForTask(task.ID, task.PlaylistName, "One", "Artist", "qq", core.DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("save task record: %v", err)
	}
	if err := core.SaveDownloadRecordForTask(other.ID, other.PlaylistName, "Other", "Artist", "netease", core.DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("save other record: %v", err)
	}

	router := gin.New()
	api := router.Group(RoutePrefix)
	RegisterPlaylistDownloadRoutes(api)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, RoutePrefix+"/api/downloads/playlists/"+strconv.FormatUint(uint64(task.ID), 10)+"/records", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Records []core.DownloadRecord `json:"records"`
		Total   int64                 `json:"total"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 1 || len(response.Records) != 1 || response.Records[0].TaskID != task.ID {
		t.Fatalf("task records response = %+v, want only task %d", response, task.ID)
	}
}

func TestPlaylistDownloadUIExposesTaskEntryAndProgress(t *testing.T) {
	for _, path := range []string{
		"templates/partials/song_list.html",
		"templates/partials/playlist_grid.html",
		"templates/partials/playlist_source_tabs.html",
	} {
		content, err := templateFS.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		if !strings.Contains(string(content), "startPlaylistDownloadFromButton") {
			t.Fatalf("%s missing playlist download action", path)
		}
	}

	appJS, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	for _, want := range []string{"/api/downloads/playlists", "歌单下载已加入队列", "musicdl:workspace-refresh"} {
		if !strings.Contains(string(appJS), want) {
			t.Fatalf("app.js missing %q", want)
		}
	}

	reactBundle, err := templateFS.ReadFile("templates/static/react/react-app.js")
	if err != nil {
		t.Fatalf("ReadFile(react-app.js): %v", err)
	}
	for _, want := range []string{"playlist_tasks", "歌单任务", "正在下载", "实时速率", "current_speed", "current_total_bytes", "打开我的歌单", "/api/downloads/playlists/"} {
		if !strings.Contains(string(reactBundle), want) {
			t.Fatalf("React bundle missing %q", want)
		}
	}
}
