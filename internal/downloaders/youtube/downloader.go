package youtube

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/kkdai/youtube/v2"
)

type downloader struct {
	client *youtube.Client
	domen  string
}

func New(client *http.Client, domen string) *downloader {
	return &downloader{
		client: &youtube.Client{
			HTTPClient: client,
		},
		domen: domen,
	}
}

func (d downloader) Download(url string) (*downloaders.Video, error) {
	youtubeVideo, err := d.client.GetVideo(url)
	if err != nil {
		return nil, err
	}

	formats := youtubeVideo.Formats.WithAudioChannels().Type("video/mp4")
	if len(formats) == 0 {
		return nil, errors.New("не найдено VideoURL")
	}

	if len(youtubeVideo.Thumbnails) == 0 {
		return nil, errors.New("не найдено ThumbnailURL")
	}

	return &downloaders.Video{
		Title:        youtubeVideo.Title,
		VideoURL:     fmt.Sprintf("%s/?src=%s", d.domen, url),
		ThumbnailURL: youtubeVideo.Thumbnails[len(youtubeVideo.Thumbnails)-1].URL,
		MimeType:     "video/mp4",
		ViewCount:    youtubeVideo.Views,
		LikeCount:    0, // Нельзя получить через API
		Duration:     int(youtubeVideo.Duration / 1000000000),
	}, nil
}

func (downloader) Valid(url string) bool {
	return strings.Contains(url, "youtube.com/")
}
