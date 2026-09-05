package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/go-music-dl/core"
)

func TestAccountOverviewReturnsIdentityWithoutSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := core.WebAuthSettings{
		Username:      "owner",
		PasswordHash:  "secret-password-hash",
		SessionSecret: "secret-session-key",
	}
	oldProvider := accountSettingsProvider
	oldCookies := core.CM.GetAll()
	accountSettingsProvider = func() (core.WebAuthSettings, error) { return settings, nil }
	core.CM.SetAll(map[string]string{"netease": "MUSIC_U=private-token"})
	t.Cleanup(func() {
		accountSettingsProvider = oldProvider
		core.CM.SetAll(oldCookies)
	})

	now := time.Now().Add(-time.Hour)
	session, err := createSessionValue(settings, now)
	if err != nil {
		t.Fatalf("createSessionValue: %v", err)
	}

	router := gin.New()
	api := router.Group(RoutePrefix)
	RegisterAccountRoutes(api)
	req := httptest.NewRequest(http.MethodGet, RoutePrefix+"/api/account", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: session})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var response accountOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode account response: %v", err)
	}
	if response.Username != "owner" || response.Role != "管理员" || !response.AuthEnabled {
		t.Fatalf("identity = %+v", response)
	}
	if response.SessionIssuedAt == nil || response.SessionExpiresAt == nil {
		t.Fatalf("authenticated account must expose session dates: %+v", response)
	}
	if response.ConnectedPlatforms != 1 {
		t.Fatalf("connected platforms = %d, want 1", response.ConnectedPlatforms)
	}
	body := rec.Body.String()
	for _, secret := range []string{"secret-password-hash", "secret-session-key", "private-token", "MUSIC_U"} {
		if strings.Contains(body, secret) {
			t.Fatalf("account response leaked secret %q", secret)
		}
	}
}

func TestPlatformSettingsDialogUsesFocusedNavigation(t *testing.T) {
	templateContent, err := templateFS.ReadFile("templates/partials/modals.html")
	if err != nil {
		t.Fatalf("ReadFile(modals.html): %v", err)
	}
	html := string(templateContent)
	for _, want := range []string{
		`class="settings-navigation"`,
		`data-settings-section="accounts"`,
		`id="platform-account-filter"`,
		`class="platform-account-row"`,
		`type="password" id="cookie-{{$source}}"`,
		`id="platform-connected-count"`,
		`id="settings-save-status"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("platform settings dialog missing %q", want)
		}
	}

	appContent, err := templateFS.ReadFile("templates/static/js/app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js): %v", err)
	}
	js := string(appContent)
	for _, want := range []string{
		"function switchSettingsSection(section =",
		"function filterPlatformAccounts(query =",
		"function syncPlatformAccountRows(sortRows = false)",
		"function togglePlatformCookieVisibility(button)",
		`openSystemConfig("accounts")`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("platform settings behavior missing %q", want)
		}
	}
}

func TestAccountWorkspaceIsIncludedInReactBundle(t *testing.T) {
	content, err := templateFS.ReadFile("templates/static/react/react-app.js")
	if err != nil {
		t.Fatalf("ReadFile(react-app.js): %v", err)
	}
	bundle := string(content)
	for _, want := range []string{"/api/account", "平台连接", "当前会话", "管理平台账户", "退出登录"} {
		if !strings.Contains(bundle, want) {
			t.Fatalf("React account workspace missing %q", want)
		}
	}
}

func TestAccountOverviewSupportsLocalMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldProvider := accountSettingsProvider
	accountSettingsProvider = func() (core.WebAuthSettings, error) { return core.WebAuthSettings{}, nil }
	t.Cleanup(func() { accountSettingsProvider = oldProvider })

	router := gin.New()
	api := router.Group(RoutePrefix)
	RegisterAccountRoutes(api)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, RoutePrefix+"/api/account", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var response accountOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode account response: %v", err)
	}
	if response.Username != "本地用户" || response.AuthEnabled {
		t.Fatalf("local identity = %+v", response)
	}
}
