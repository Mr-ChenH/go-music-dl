package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/utils"
)

type DownloadProgress struct {
	DownloadedBytes int64
	TotalBytes      int64
	BytesPerSecond  float64
}

type DownloadProgressFunc func(DownloadProgress)

type DownloadedSong struct {
	Data        []byte
	Ext         string
	ContentType string
	Filename    string
	SavedPath   string
	Warning     string
	Skipped     bool // 因已存在而跳过下载
}

func DownloadSongData(song *model.Song, withCover bool, withLyrics bool) (*DownloadedSong, error) {
	return DownloadSongDataWithTemplate(song, withCover, withLyrics, DefaultDownloadFilenameTemplate)
}

func DownloadSongDataWithTemplate(song *model.Song, withCover bool, withLyrics bool, filenameTemplate string) (*DownloadedSong, error) {
	return DownloadSongDataWithTemplateProgress(song, withCover, withLyrics, filenameTemplate, nil)
}

func DownloadSongDataWithTemplateProgress(song *model.Song, withCover bool, withLyrics bool, filenameTemplate string, onProgress DownloadProgressFunc) (*DownloadedSong, error) {
	if song == nil {
		return nil, errors.New("song is nil")
	}
	if strings.TrimSpace(song.ID) == "" || strings.TrimSpace(song.Source) == "" {
		return nil, errors.New("missing song id or source")
	}

	normalized := *song
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Artist = strings.TrimSpace(normalized.Artist)
	normalized.Album = strings.TrimSpace(normalized.Album)
	if normalized.Name == "" {
		normalized.Name = "Unknown"
	}
	if normalized.Artist == "" {
		normalized.Artist = "Unknown"
	}

	audioData, contentType, err := fetchSongAudio(&normalized, onProgress)
	if err != nil {
		return nil, err
	}

	signatureExt := DetectAudioExtBySignature(audioData)
	ext := signatureExt
	if ext == "" {
		ext = DetectAudioExtByContentType(contentType)
	}
	if ext == "" {
		ext = DetectAudioExt(audioData)
	}

	var lyric string
	if withLyrics {
		if lyricFn := GetLyricFunc(normalized.Source); lyricFn != nil {
			lyric, _ = lyricFn(&normalized)
		}
	}

	var coverData []byte
	var coverMime string
	if withCover && strings.TrimSpace(normalized.Cover) != "" {
		coverData, coverMime, _ = FetchBytesWithMime(normalized.Cover, normalized.Source)
	}

	finalData := audioData
	warning := ""
	if (ext == "mp3" || ext == "flac" || ext == "m4a" || ext == "wma") && (normalized.Album != "" || lyric != "" || len(coverData) > 0) {
		embeddedData, embedErr := EmbedSongMetadata(audioData, &normalized, lyric, coverData, coverMime)
		switch {
		case embedErr == nil:
			finalData = embeddedData
		case errors.Is(embedErr, ErrFFmpegNotFound):
			warning = "ffmpeg not found, metadata embedding skipped"
		default:
			warning = "metadata embedding failed, using original audio"
		}
	}

	if ext == "" {
		ext = DetectAudioExt(finalData)
	}

	return &DownloadedSong{
		Data:        finalData,
		Ext:         ext,
		ContentType: AudioMimeByExt(ext),
		Filename:    BuildDownloadFilename(&normalized, ext, filenameTemplate),
		Warning:     warning,
	}, nil
}

func SaveSongToFile(song *model.Song, outDir string, withCover bool, withLyrics bool) (*DownloadedSong, error) {
	return SaveSongToFileWithTemplate(song, outDir, withCover, withLyrics, DefaultDownloadFilenameTemplate)
}

func SaveSongToFileWithTemplate(song *model.Song, outDir string, withCover bool, withLyrics bool, filenameTemplate string) (*DownloadedSong, error) {
	return SaveSongToFileWithTemplateProgress(song, outDir, withCover, withLyrics, filenameTemplate, nil)
}

func SaveSongToFileWithTemplateProgress(song *model.Song, outDir string, withCover bool, withLyrics bool, filenameTemplate string, onProgress DownloadProgressFunc) (*DownloadedSong, error) {
	result, err := DownloadSongDataWithTemplateProgress(song, withCover, withLyrics, filenameTemplate, onProgress)
	if err != nil {
		return nil, err
	}
	return saveDownloadedSongToFile(result, outDir)
}

func saveDownloadedSongToFile(result *DownloadedSong, outDir string) (*DownloadedSong, error) {
	if result == nil {
		return nil, errors.New("download result is nil")
	}

	targetDir := strings.TrimSpace(outDir)
	if targetDir == "" {
		targetDir = DefaultWebDownloadDir
	}
	targetDir = filepath.Clean(targetDir)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, err
	}

	fileName := sanitizeDownloadRelativePath(result.Filename)
	filePath := filepath.Join(targetDir, fileName)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filePath, result.Data, 0644); err != nil {
		return nil, err
	}

	result.Filename = fileName
	result.SavedPath = filePath
	return result, nil
}

func BuildDownloadFilename(song *model.Song, ext string, filenameTemplate string) string {
	template := strings.TrimSpace(filenameTemplate)
	if template == "" {
		template = DefaultDownloadFilenameTemplate
	}
	ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))

	name := "Unknown"
	artist := "Unknown"
	album := ""
	source := ""
	id := ""
	if song != nil {
		if strings.TrimSpace(song.Name) != "" {
			name = strings.TrimSpace(song.Name)
		}
		if strings.TrimSpace(song.Artist) != "" {
			artist = strings.TrimSpace(song.Artist)
		}
		album = strings.TrimSpace(song.Album)
		source = strings.TrimSpace(song.Source)
		id = strings.TrimSpace(song.ID)
	}
	name = sanitizeDownloadTemplateValue(name, "Unknown")
	artist = sanitizeDownloadTemplateValue(artist, "Unknown")
	album = sanitizeDownloadTemplateValue(album, "")
	source = sanitizeDownloadTemplateValue(source, "")
	id = sanitizeDownloadTemplateValue(id, "")

	hasExtToken := strings.Contains(template, "{ext}")
	rendered := strings.NewReplacer(
		"{name}", name,
		"{artist}", artist,
		"{album}", album,
		"{source}", source,
		"{id}", id,
		"{ext}", ext,
	).Replace(template)
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		rendered = strings.TrimSpace(DefaultDownloadFilenameTemplate)
		rendered = strings.NewReplacer("{name}", name, "{artist}", artist, "{album}", album, "{source}", source, "{id}", id, "{ext}", ext).Replace(rendered)
	}
	if !hasExtToken && ext != "" {
		rendered += "." + ext
	}

	return sanitizeDownloadRelativePath(rendered)
}

func sanitizeDownloadRelativePath(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	parts := strings.Split(name, "/")
	safeParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizeDownloadPathSegment(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		safeParts = append(safeParts, part)
	}
	if len(safeParts) == 0 {
		return "download"
	}
	return filepath.Join(safeParts...)
}

func sanitizeDownloadTemplateValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	value = sanitizeDownloadPathSegment(value)
	if value == "" {
		return fallback
	}
	return value
}

func sanitizeDownloadPathSegment(value string) string {
	value = strings.Trim(value, " .")
	if value == "" {
		return ""
	}
	return strings.Trim(utils.SanitizeFilename(value), " .")
}

// FetchDecryptedSodaAudio 下载并解密 soda（汽水）加密音频流，返回明文音频字节。
func FetchDecryptedSodaAudio(song *model.Song) ([]byte, error) {
	return fetchDecryptedSodaAudio(song, nil)
}

func fetchDecryptedSodaAudio(song *model.Song, onProgress DownloadProgressFunc) ([]byte, error) {
	cookie := CM.Get("soda")
	sodaInst := soda.New(cookie)
	info, err := sodaInst.GetDownloadInfo(song)
	if err != nil {
		return nil, err
	}

	encryptedData, _, err := FetchBytesWithMimeProgress(info.URL, "soda", onProgress)
	if err != nil {
		return nil, err
	}

	// 汽水 SEO 路径返回明文 m4a（无 play_auth），只需下载无需解密。
	if strings.TrimSpace(info.PlayAuth) == "" {
		return encryptedData, nil
	}
	return soda.DecryptAudio(encryptedData, info.PlayAuth)
}

func fetchSongAudio(song *model.Song, onProgress DownloadProgressFunc) ([]byte, string, error) {
	if song.Source == "soda" {
		finalData, err := fetchDecryptedSodaAudio(song, onProgress)
		if err != nil {
			return nil, "", err
		}
		return finalData, "", nil
	}

	dlFunc := GetDownloadFunc(song.Source)
	if dlFunc == nil {
		return nil, "", fmt.Errorf("unsupported source: %s", song.Source)
	}

	urlStr, err := dlFunc(song)
	if err != nil {
		return nil, "", err
	}
	if urlStr == "" {
		return nil, "", errors.New("empty download url")
	}

	return FetchBytesWithMimeProgress(urlStr, song.Source, onProgress)
}
