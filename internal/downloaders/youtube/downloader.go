package youtube

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/StounhandJ/shorts_forward/internal/ytdlp"
)

type downloader struct {
	domain string
	yt     *ytdlp.Client
}

func New(domain string, yt *ytdlp.Client) downloaders.IDownloader {
	return &downloader{
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
	return strings.Contains(url, "youtube.com/") || strings.Contains(url, "youtu.be/")
}

func (d downloader) GetReader(url string) (io.ReadCloser, int64, error) {
	// Скачиваем во временный файл и получаем точно измеренный размер
	stream, err := d.yt.DownloadToTemp(context.Background(), url)
	if err != nil {
		return nil, 0, err
	}

	return stream, stream.Size, nil
}
