package web

import (
	"strings"
	"testing"
)

func TestCollectionSongListsUseCompactInternalViewport(t *testing.T) {
	templateContent, err := templateFS.ReadFile("templates/partials/song_list.html")
	if err != nil {
		t.Fatalf("ReadFile(song_list.html): %v", err)
	}
	if !strings.Contains(string(templateContent), "collection-song-list") {
		t.Fatal("collection song list is missing its compact layout marker")
	}

	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(appContent)
	for _, want := range []string{
		`document.querySelector(".app-sidebar")`,
		"mobileBoundary - 12 - region.getBoundingClientRect().top - paginationReserve",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("content list viewport calculation missing %q", want)
		}
	}
}

func TestActivityWorkspacesUseInternalScrollRegions(t *testing.T) {
	jsContent, err := templateFS.ReadFile("templates/static/react/react-app.js")
	if err != nil {
		t.Fatalf("ReadFile(react-app.js): %v", err)
	}
	js := string(jsContent)
	for _, want := range []string{
		"activity-workspace download-records-workspace",
		"activity-workspace playback-history-workspace",
		"activity-table-wrap activity-scroll-region",
		"download-group-list activity-scroll-region",
		"history-list activity-scroll-region",
		"全部下载歌曲记录",
		"按歌单分组的下载任务",
		"播放历史歌曲列表",
		"download-group-progress",
		"查看歌曲",
		"收起歌曲",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("activity workspace bundle missing %q", want)
		}
	}

	cssContent, err := templateFS.ReadFile("templates/static/react/react-app.css")
	if err != nil {
		t.Fatalf("ReadFile(react-app.css): %v", err)
	}
	css := string(cssContent)
	for _, want := range []string{
		".activity-workspace",
		".activity-scroll-region",
		`body[data-workspace-view=downloads]`,
		`body[data-workspace-view=history]`,
		"overflow-y:auto",
		".download-view-tabs",
		".download-group-list",
		".download-group-progress",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("activity workspace styles missing %q", want)
		}
	}
}
