package youtube

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

func (d downloader) Handler(ctx *fasthttp.RequestCtx) {
	// получаем параметр src
	src := string(ctx.QueryArgs().Peek("src"))
	if src == "" {
		ctx.Response.Header.Set("Content-Type", "image/webp")
		ctx.Response.Header.Set("Content-Disposition", "inline")
		ctx.SendFile("./assets/gorila.webp")

		return
	}

	targetURL, err := url.Parse(src)
	if err != nil {
		ctx.Error("invalid src", http.StatusBadRequest)
		return
	}

	ctx.Response.Header.Add("Content-Type", "video/mp4")
	ctx.Response.Header.Set("Content-Disposition", `inline; filename="ffffe11cdc4.mp4"`)
	ctx.Response.Header.Set("Accept-Ranges", "bytes")

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

	rangeHdr := string(ctx.Request.Header.Peek("Range"))
	if rangeHdr == "" {
		// нет Range - отдаем весь файл
		ctx.Response.Header.Set("Content-Length", strconv.FormatInt(contentLength, 10))
		// Оборачиваем в readCloserOnEOF чтобы закрыть после EOF
		rc := &readCloserOnEOF{r: videoReader, c: videoReader}
		// fasthttp.SetBodyStream принимает io.Reader и int (size)
		ctx.SetBodyStream(rc, int(contentLength))
		return
	}

	// Парсим Range
	start, end, err := parseRange(rangeHdr, contentLength)
	if err != nil {
		// 416 Range Not Satisfiable
		ctx.SetStatusCode(fasthttp.StatusRequestedRangeNotSatisfiable)
		// обязательный заголовок для 416:
		ctx.Response.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", contentLength))
		_ = videoReader.Close()
		return
	}

	length := end - start + 1
	// Установим заголовки частичного контента
	ctx.SetStatusCode(fasthttp.StatusPartialContent) // 206
	ctx.Response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentLength))
	ctx.Response.Header.Set("Content-Length", strconv.FormatInt(length, 10))

	// Попытаемся использовать Seek, если поддерживается
	if seeker, ok := interface{}(videoReader).(io.Seeker); ok {
		_, seekErr := seeker.Seek(start, io.SeekStart)
		if seekErr != nil {
			// fallback: читаем и отбросим
			_, _ = io.CopyN(io.Discard, videoReader, start)
		}
		limited := io.LimitReader(videoReader, length)
		rc := &readCloserOnEOF{r: limited, c: videoReader}
		ctx.SetBodyStream(rc, int(length))
		return
	}

	// Если Seek не поддерживается - прочитаем и отбросим первые start байт (медленнее)
	if start > 0 {
		if _, err := io.CopyN(io.Discard, videoReader, start); err != nil {
			_ = videoReader.Close()
			ctx.Error("failed to skip bytes", fasthttp.StatusBadGateway)
			return
		}
	}
	limited := io.LimitReader(videoReader, length)
	rc := &readCloserOnEOF{r: limited, c: videoReader}
	ctx.SetBodyStream(rc, int(length))
}

// parseRange поддерживает одиночную запись типа:
// Range: bytes=START-END
// Range: bytes=START-
// Range: bytes=-SUFFIXLEN
func parseRange(s string, size int64) (start, end int64, err error) {
	const prefix = "bytes="
	if !strings.HasPrefix(s, prefix) {
		return 0, 0, errors.New("invalid range")
	}
	r := strings.TrimSpace(s[len(prefix):])
	if r == "" {
		return 0, 0, errors.New("empty range")
	}

	if strings.HasPrefix(r, "-") {
		// suffix: -N  => last N bytes
		n, err := strconv.ParseInt(r[1:], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, errors.New("invalid suffix range")
		}
		if n > size {
			n = size
		}
		start = size - n
		end = size - 1
		return start, end, nil
	}

	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid range format")
	}

	if parts[0] == "" {
		return 0, 0, errors.New("invalid start")
	}
	s0, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s0 < 0 {
		return 0, 0, errors.New("invalid start")
	}
	start = s0

	if parts[1] == "" {
		// START-
		end = size - 1
	} else {
		e0, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || e0 < 0 {
			return 0, 0, errors.New("invalid end")
		}
		end = e0
	}

	if start > end || start >= size {
		return 0, 0, errors.New("range unsatisfiable")
	}
	if end >= size {
		end = size - 1
	}
	return start, end, nil
}

// обёртка: закроет c при достижении EOF
type readCloserOnEOF struct {
	r io.Reader
	c io.Closer
}

func (r *readCloserOnEOF) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err == io.EOF && r.c != nil {
		_ = r.c.Close()
	}
	return n, err
}
