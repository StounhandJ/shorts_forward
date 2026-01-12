package downloaders

import "io"

type IDownloader interface {
	Download(url string) (*Video, error)
	Valid(url string) bool
}

type Video struct {
	Title        string
	VideoURL     string
	ThumbnailURL string
	MimeType     string
	VideoReader  *io.ReadCloser
}
