//go:generate easyjson extract.go
package instagram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mailru/easyjson"

	"github.com/StounhandJ/shorts_forward/internal/downloaders"
)

// videoData описывает интересующие нас поля из JSON
//
// easyjson:json
type videoData struct {
	Code          string `json:"code"`
	VideoVersions []struct {
		URL string `json:"url"`
	} `json:"video_versions"`
	ImageVersions struct {
		Candidates []struct {
			URL string `json:"url"`
		} `json:"candidates"`
	} `json:"image_versions2"`
	Caption struct {
		Text string `json:"text"`
	} `json:"caption"`
	LikeCount         int    `json:"like_count"`
	VideoDashManifest string `json:"video_dash_manifest"`
}

// easyjson:json
type videoObject struct {
	Items []videoData `json:"items"`
}

// extractFirstVideoURL ищет "video_versions":[...] в html и возвращает первый url
func extractFirstVideoURL(html string) (*downloaders.Video, bool) {
	// Ищем шаблон "key"\s*:\s*{
	re := regexp.MustCompile(`"` + regexp.QuoteMeta("xdt_api__v1__media__shortcode__web_info") + `"\s*:\s*{`)

	loc := re.FindStringIndex(html)
	if loc == nil {
		return nil, false
	}

	// индекс открывающей фигурной скобки:
	openIdx := strings.Index(html[loc[0]:loc[1]], "{")
	if openIdx == -1 {
		return nil, false
	}
	// глобальный индекс открывающей скобки
	start := loc[0] + openIdx

	// Проходим посимвольно, учитывая строки и экранирование, чтобы найти соответствующую закрывающую }
	depth := 0
	inStr := false
	escaped := false

	var end int
	for i := start; i < len(html); i++ {
		c := html[i]

		if escaped {
			// если предыдущий был '\', пропускаем обработку этого символа
			escaped = false

			continue
		}

		if c == '\\' && inStr {
			escaped = true

			continue
		}

		if c == '"' {
			inStr = !inStr

			continue
		}

		if !inStr {
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					end = i

					break
				}
			}
		}
	}

	if end == 0 {
		return nil, false
	}

	var obj videoObject
	if err := easyjson.Unmarshal([]byte(html[start:end+1]), &obj); err != nil {
		fmt.Println("json unmarshal error:", err)
	}

	if len(obj.Items) == 0 {
		return nil, false
	}

	// Найдём первый непустой url video
	var videoURL string
	for _, v := range obj.Items[0].VideoVersions {
		if strings.TrimSpace(v.URL) != "" {
			videoURL = v.URL

			break
		}
	}

	// Найдём первый непустой url img
	var thumbnailURL string
	for _, v := range obj.Items[0].ImageVersions.Candidates {
		if strings.TrimSpace(v.URL) != "" {
			thumbnailURL = v.URL

			break
		}
	}

	return &downloaders.Video{
		Title:        obj.Items[0].Caption.Text,
		VideoURL:     videoURL,
		MimeType:     "video/mp4",
		ThumbnailURL: thumbnailURL,
		Duration:     extractDurationSeconds(obj.Items[0].VideoDashManifest),
		LikeCount:    obj.Items[0].LikeCount,
		ViewCount:    0, // Доступно только через API с авторизацией
	}, true
}

func extractDurationSeconds(s string) int {
	re := regexp.MustCompile(`duration="PT([0-9]+)(?:\\.[0-9]+)?S"`)

	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}

	i, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}

	return i
}
