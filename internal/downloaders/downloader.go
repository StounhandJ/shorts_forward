package downloaders

import (
	"github.com/StounhandJ/shorts_forward/internal/utils"
)

type IDownloader interface {
	Download(url string) (*Video, error)
	Valid(url string) bool
}

type Video struct {
	Title        string
	VideoURL     string
	ThumbnailURL string
	MimeType     string
	Duration     int
	ViewCount    int
	LikeCount    int
}

func (v Video) MainInfo() string {
	var result string
	if v.ViewCount != 0 {
		result = utils.FormatBigInt(v.ViewCount) + "👁️ "
	}

	if v.LikeCount != 0 {
		result += utils.FormatBigInt(v.LikeCount) + "🤍"
	}

	return result
}
