package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

type Info struct {
	Title     string  `json:"title"`
	URL       string  `json:"url"`
	Thumbnail string  `json:"thumbnail"`
	Duration  float64 `json:"duration"`
	ViewCount int     `json:"view_count"`
	LikeCount int     `json:"like_count"`
	Ext       string  `json:"ext"`
}

type cacheItem struct {
	info      *Info
	expiresAt time.Time
}

type Client struct {
	binPath string
	cache   map[string]cacheItem
	mu      sync.RWMutex
	ttl     time.Duration
}

// New создает клиент. Кеш по умолчанию хранится 5 минут
func New(binPath string) *Client {
	if binPath == "" {
		binPath = "yt-dlp"
	}

	return &Client{
		binPath: binPath,
		cache:   make(map[string]cacheItem),
		ttl:     5 * time.Minute,
	}
}

// SetTTL позволяет переопределить время жизни кеша
func (c *Client) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

func (c *Client) GetInfo(ctx context.Context, url string) (*Info, error) {
	// Проверяем наличие валидного кеша
	c.mu.RLock()
	item, exists := c.cache[url]
	c.mu.RUnlock()

	if exists && time.Now().Before(item.expiresAt) {
		return item.info, nil // Cache hit
	}

	// Если кеша нет или он протух - выполняем запрос
	cmd := exec.CommandContext(ctx, c.binPath, "-j", "--no-warnings", url)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp error: %w, stderr: %s", err, stderr.String())
	}

	var info Info
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp json output: %w", err)
	}

	// Сохраняем результат в кеш
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[url] = cacheItem{
		info:      &info,
		expiresAt: time.Now().Add(c.ttl),
	}

	// Ленивая очистка протухших элементов, чтобы память не текла при долгой работе
	if len(c.cache) > 100 {
		now := time.Now()
		for k, v := range c.cache {
			if now.After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
	}

	return &info, nil
}
