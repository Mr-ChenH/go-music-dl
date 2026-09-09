package core

import (
	"path/filepath"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestDownloadDedupIndexUsesSQLiteAndSurvivesHistoryClear(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := SaveDownloadRecord("Song\nTitle", "Artist\rName", "source", DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("SaveDownloadRecord: %v", err)
	}

	dedupSet, err := LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet: %v", err)
	}
	song := &model.Song{Name: "SongTitle", Artist: "ArtistName"}
	if !IsSongDownloaded(song, dedupSet) {
		t.Fatal("successful record should be available through the SQLite de-duplication index")
	}

	if err := ClearDownloadRecords(); err != nil {
		t.Fatalf("ClearDownloadRecords: %v", err)
	}
	records, err := GetDownloadRecords()
	if err != nil {
		t.Fatalf("GetDownloadRecords: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("download history length = %d, want 0", len(records))
	}

	dedupSet, err = LoadDownloadDedupSet()
	if err != nil {
		t.Fatalf("LoadDownloadDedupSet after clear: %v", err)
	}
	if !IsSongDownloaded(song, dedupSet) {
		t.Fatal("clearing visible history must not clear the SQLite de-duplication index")
	}
}

func TestDownloadRecordStoresPlaylistTaskContext(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := SaveDownloadRecordForTask(42, "Road Trip", "Track", "Artist", "qq", DownloadStatusSuccess, ""); err != nil {
		t.Fatalf("SaveDownloadRecordForTask: %v", err)
	}
	records, err := GetDownloadRecords()
	if err != nil {
		t.Fatalf("GetDownloadRecords: %v", err)
	}
	if len(records) != 1 || records[0].TaskID != 42 || records[0].PlaylistName != "Road Trip" {
		t.Fatalf("record task context = %#v, want task 42 and playlist name", records)
	}
}

func TestGetDownloadRecordPageForTaskKeepsPlaylistSongsTogether(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	for _, record := range []struct {
		taskID uint
		name   string
	}{
		{taskID: 7, name: "First in Seven"},
		{taskID: 8, name: "Only in Eight"},
		{taskID: 7, name: "Second in Seven"},
	} {
		if err := SaveDownloadRecordForTask(record.taskID, "Playlist", record.name, "Artist", "qq", DownloadStatusSuccess, ""); err != nil {
			t.Fatalf("SaveDownloadRecordForTask(%q): %v", record.name, err)
		}
	}

	records, total, err := GetDownloadRecordPageForTask(7, 1, 30)
	if err != nil {
		t.Fatalf("GetDownloadRecordPageForTask: %v", err)
	}
	if total != 2 || len(records) != 2 {
		t.Fatalf("task page = %d records, total %d; want 2", len(records), total)
	}
	for _, record := range records {
		if record.TaskID != 7 {
			t.Fatalf("task page contains task %d, want only task 7", record.TaskID)
		}
	}
}

func TestGetDownloadRecordPageReturnsStablePagesAndTotal(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "settings.db"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	for _, name := range []string{"First", "Second", "Third"} {
		if err := SaveDownloadRecord(name, "Artist", "qq", DownloadStatusSuccess, ""); err != nil {
			t.Fatalf("SaveDownloadRecord(%q): %v", name, err)
		}
	}

	records, total, err := GetDownloadRecordPage(2, 1)
	if err != nil {
		t.Fatalf("GetDownloadRecordPage: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(records) != 1 || records[0].Name != "Second" {
		t.Fatalf("page 2 = %#v, want Second", records)
	}
}
