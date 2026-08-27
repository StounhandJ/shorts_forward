package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/ytdlp"
)

type downloader struct {
	client *http.Client
	domain string
	yt     *ytdlp.Client
}

func New(client *http.Client, domain string, yt *ytdlp.Client) downloaders.IDownloader {
	return &downloader{
		client: client,
		domain: domain,
		yt:     yt,
	}
}

func (d downloader) Download(url string) (*downloaders.Video, error) {
	info, err := d.yt.GetInfo(context.Background(), url)
	if err != nil {
		return nil, err
	}

	mimeType := "video/mp4"
	if info.Ext != "" {
		mimeType = "video/" + info.Ext
	}

	videoURL := info.URL
	if d.domain != "" {
		videoURL = fmt.Sprintf("%s/video?src=%s", d.domain, url)
	}

	return &downloaders.Video{
		Title:        info.Title,
		VideoURL:     videoURL,
		ThumbnailURL: info.Thumbnail,
		MimeType:     mimeType,
		ViewCount:    info.ViewCount,
		LikeCount:    info.LikeCount,
		Duration:     int(info.Duration),
	}, nil
}

func (downloader) Valid(url string) bool {
	return strings.Contains(url, "instagram.com/")
}

func (d downloader) GetReader(url string) (io.ReadCloser, int64, error) {
	info, err := d.yt.GetInfo(context.Background(), url)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, info.URL, nil)
	if err != nil {
		return nil, 0, err
	}

	httpClient := d.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return resp.Body, resp.ContentLength, nil
}
