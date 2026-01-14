//go:generate easyjson api.go
package tiktok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	netUrl "net/url"

	"github.com/StounhandJ/shorts_forward/internal/utils"
	easyjson "github.com/mailru/easyjson"
)

const (
	BaseUrl = "https://tikwm.com/api/"
)

var (
	ErrRateLimit = errors.New("rate limit exceeded")
	ErrParse     = errors.New("parse error")
	ErrUnknown   = errors.New("unknown error")
)

func fetchMetadata(client *http.Client, postUrl string) (ApiResponse, error) {
	postUrl = fmt.Sprintf("%s?url=%s", BaseUrl, netUrl.QueryEscape(postUrl))

	req, err := http.NewRequestWithContext(context.TODO(), "GET", postUrl, nil)
	if err != nil {
		return ApiResponse{}, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 YaBrowser/25.10.0.0 Safari/537.36")
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return ApiResponse{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			utils.Log.Error(err)
		}
	}()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ApiResponse{}, err
	}

	var data ApiResponse

	err = easyjson.Unmarshal(b, &data)
	if err != nil {
		return ApiResponse{}, err
	}

	if data.Code != 0 {
		switch {
		case strings.HasPrefix(data.Msg, "Free Api Limit"):
			return data, ErrRateLimit
		case strings.HasPrefix(data.Msg, "Url parsing is failed"):
			return data, ErrParse
		default:
			return data, ErrUnknown
		}
	}

	return data, nil
}

// easyjson:json
type ApiResponse struct {
	Code          int     `json:"code,omitempty"`
	Msg           string  `json:"msg"`
	ProcessedTime float64 `json:"processed_time,omitempty"`
	Data          struct {
		ID             string `json:"id,omitempty"`
		Region         string `json:"region,omitempty"`
		Title          string `json:"title,omitempty"`
		Cover          string `json:"cover,omitempty"`
		AiDynamicCover string `json:"ai_dynamic_cover,omitempty"`
		OriginCover    string `json:"origin_cover,omitempty"`
		Duration       int    `json:"duration,omitempty"`
		Play           string `json:"play,omitempty"`
		Hdplay         string `json:"hdplay,omitempty"`
		Wmplay         string `json:"wmplay,omitempty"`
		Size           int    `json:"size,omitempty"`
		WmSize         int    `json:"wm_size,omitempty"`
		HdSize         int    `json:"hd_size,omitempty"`
		Music          string `json:"music,omitempty"`
		MusicInfo      struct {
			ID       string `json:"id,omitempty"`
			Title    string `json:"title,omitempty"`
			Play     string `json:"play,omitempty"`
			Cover    string `json:"cover,omitempty"`
			Author   string `json:"author,omitempty"`
			Original bool   `json:"original,omitempty"`
			Duration int    `json:"duration,omitempty"`
			Album    string `json:"album,omitempty"`
		} `json:"music_info,omitempty"`
		PlayCount     int `json:"play_count,omitempty"`
		DiggCount     int `json:"digg_count,omitempty"`
		CommentCount  int `json:"comment_count,omitempty"`
		ShareCount    int `json:"share_count,omitempty"`
		DownloadCount int `json:"download_count,omitempty"`
		CollectCount  int `json:"collect_count,omitempty"`
		CreateTime    int `json:"create_time,omitempty"`
		Anchors       []struct {
			Actions      []any  `json:"actions,omitempty"`
			AnchorStrong any    `json:"anchor_strong,omitempty"`
			ComponentKey string `json:"component_key,omitempty"`
			Description  string `json:"description,omitempty"`
			Extra        string `json:"extra,omitempty"`
			GeneralType  int    `json:"general_type,omitempty"`
			Icon         struct {
				Height    int      `json:"height,omitempty"`
				URI       string   `json:"uri,omitempty"`
				URLList   []string `json:"url_list,omitempty"`
				URLPrefix any      `json:"url_prefix,omitempty"`
				Width     int      `json:"width,omitempty"`
			} `json:"icon,omitempty"`
			ID        string `json:"id,omitempty"`
			Keyword   string `json:"keyword,omitempty"`
			LogExtra  string `json:"log_extra,omitempty"`
			Schema    string `json:"schema,omitempty"`
			Thumbnail struct {
				Height    int      `json:"height,omitempty"`
				URI       string   `json:"uri,omitempty"`
				URLList   []string `json:"url_list,omitempty"`
				URLPrefix any      `json:"url_prefix,omitempty"`
				Width     int      `json:"width,omitempty"`
			} `json:"thumbnail,omitempty"`
			Type int `json:"type,omitempty"`
		} `json:"anchors,omitempty"`
		AnchorsExtras string `json:"anchors_extras,omitempty"`
		IsAd          bool   `json:"is_ad,omitempty"`
		CommerceInfo  struct {
			AdvPromotable          bool `json:"adv_promotable,omitempty"`
			AuctionAdInvited       bool `json:"auction_ad_invited,omitempty"`
			BrandedContentType     int  `json:"branded_content_type,omitempty"`
			WithCommentFilterWords bool `json:"with_comment_filter_words,omitempty"`
		} `json:"commerce_info,omitempty"`
		CommercialVideoInfo string `json:"commercial_video_info,omitempty"`
		ItemCommentSettings int    `json:"item_comment_settings,omitempty"`
		MentionedUsers      string `json:"mentioned_users,omitempty"`
		Author              struct {
			ID       string `json:"id,omitempty"`
			UniqueID string `json:"unique_id,omitempty"`
			Nickname string `json:"nickname,omitempty"`
			Avatar   string `json:"avatar,omitempty"`
		} `json:"author,omitempty"`
		Images []string `json:"images,omitempty"`
	} `json:"data,omitempty"`
}
