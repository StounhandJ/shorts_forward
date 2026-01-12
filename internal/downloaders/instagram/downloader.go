package instagram

import (
	"context"
	"errors"
	"io"
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
	req, err := http.NewRequestWithContext(context.TODO(), "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 YaBrowser/25.10.0.0 Safari/537.36")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.Log.Error(err)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	video, ok := extractFirstVideoURL(string(data))
	if !ok {
		return nil, errors.New("html не расшифрован")
	}

	return video, nil
}

func (downloader) Valid(url string) bool {
	return strings.Contains(url, "www.instagram.com/")
}
