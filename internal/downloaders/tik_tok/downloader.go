package tiktok

import (
	"net/http"
	"strings"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/utils"
)

type downloader struct {
	client *http.Client
}

func New(client *http.Client) downloaders.IDownloader {
	return &downloader{
		client: client,
	}
}

func (d downloader) Download(url string) (*downloaders.Video, error) {
	metadata, err := fetchMetadata(d.client, url)
	if err != nil {
		return nil, err
	}

	return &downloaders.Video{
		Title:        metadata.Data.Title,
		VideoURL:     utils.StringNotEmptyCoalesce(metadata.Data.Hdplay, metadata.Data.Play, metadata.Data.Wmplay),
		ThumbnailURL: metadata.Data.OriginCover,
		MimeType:     "video/mp4",
	}, nil
}

func (downloader) Valid(url string) bool {
	return strings.Contains(url, "vt.tiktok.com/")
}
