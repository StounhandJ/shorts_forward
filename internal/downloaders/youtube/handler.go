package youtube

import (
	"net/http"
	"net/url"

	"github.com/StounhandJ/shorts_forward/internal/utils"
	"github.com/valyala/fasthttp"
)

func (d downloader) Handler(ctx *fasthttp.RequestCtx) {
	utils.Log.Info(string(ctx.Request.RequestURI()))
	// получаем параметр src
	src := string(ctx.QueryArgs().Peek("src"))
	if src == "" {
		ctx.Error("missing src query", http.StatusBadRequest)
		return
	}

	targetURL, err := url.Parse(src)
	if err != nil {
		ctx.Error("invalid src", http.StatusBadRequest)
		return
	}

	youtubeVideo, err := d.client.GetVideo(targetURL.String())
	if err != nil {
		ctx.Error("error get video", http.StatusBadGateway)
		return
	}

	formats := youtubeVideo.Formats.WithAudioChannels().Type("video/mp4")
	if len(formats) == 0 {
		ctx.Error("not found video", http.StatusBadGateway)
		return
	}

	videoReader, contentLength, err := d.client.GetStream(youtubeVideo, &formats[0])
	if err != nil {
		ctx.Error("get video stream", http.StatusBadGateway)
		return
	}

	ctx.Response.Header.Add("Content-Type", "video/mp4")

	ctx.SetBodyStream(videoReader, int(contentLength))
}
