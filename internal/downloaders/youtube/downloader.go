package youtube

import (
	"errors"
	"net/http"
	"strings"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
	"github.com/kkdai/youtube/v2"
)

type downloader struct {
	client *youtube.Client
}

func New(client *http.Client) downloaders.IDownloader {
	return &downloader{
		client: &youtube.Client{
			HTTPClient: client,
		},
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

	// videoReader, _, err := d.client.GetStream(youtubeVideo, &formats[0])
	// if err != nil {
	// 	return nil, err
	// }

	videoURL, err := d.client.GetStreamURL(youtubeVideo, &formats[0])
	if err != nil {
		return nil, err
	}

	if len(youtubeVideo.Thumbnails) == 0 {
		return nil, errors.New("не найдено ThumbnailURL")
	}

	return &downloaders.Video{
		Title: youtubeVideo.Title,
		// VideoReader:  &videoReader,
		VideoURL:     videoURL,
		ThumbnailURL: youtubeVideo.Thumbnails[len(youtubeVideo.Thumbnails)-1].URL,
		MimeType:     "video/mp4",
	}, nil
}

func (downloader) Valid(url string) bool {
	return strings.Contains(url, "youtube.com/")
}
