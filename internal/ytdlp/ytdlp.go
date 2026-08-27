package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL            = 5 * time.Minute
	defaultDownloadTTL    = 15 * time.Minute
	maxStderrSize         = 64 * 1024
	maxMetadataCacheItems = 100
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

type downloadCacheItem struct {
	dir       string
	path      string
	size      int64
	expiresAt time.Time
	refs      int
}

type downloadCall struct {
	done   chan struct{}
	cancel context.CancelFunc

	err error
}

// Client — потокобезопасный клиент yt-dlp.
//
// Metadata cache хранит JSON-метаданные.
// Download cache хранит физические файлы на диске.
//
// Для одного URL одновременно выполняется только один download.
// Остальные вызовы используют уже скачанный файл.
type Client struct {
	binPath string

	mu sync.Mutex

	cache map[string]cacheItem

	downloadCache map[string]*downloadCacheItem
	downloads     map[string]*downloadCall

	ttl         time.Duration
	downloadTTL time.Duration
	closed      bool
}

// FileStream оборачивает *os.File.
//
// Close:
//   - закрывает файловый descriptor;
//   - освобождает ссылку на cached file;
//   - удаляет физический файл, когда он больше никому не нужен
//     и его TTL истёк либо клиент закрыт.
type FileStream struct {
	*os.File
	Size int64

	release func() error
	once    sync.Once
}

// Close безопасен для многократного вызова.
func (fs *FileStream) Close() error {
	var result error

	fs.once.Do(func() {
		var closeErr error

		if fs.File != nil {
			closeErr = fs.File.Close()
		}

		var releaseErr error

		if fs.release != nil {
			releaseErr = fs.release()
		}

		result = errors.Join(closeErr, releaseErr)
	})

	return result
}

// New создаёт клиент.
//
// По умолчанию:
//   - metadata TTL = 5 минут;
//   - download TTL = 5 минут;
//   - максимальное время одного физического download = 15 минут.
func New(binPath string) *Client {
	if binPath == "" {
		binPath = "yt-dlp"
	}

	return &Client{
		binPath:       binPath,
		cache:         make(map[string]cacheItem),
		downloadCache: make(map[string]*downloadCacheItem),
		downloads:     make(map[string]*downloadCall),
		ttl:           defaultTTL,
		downloadTTL:   defaultDownloadTTL,
	}
}

// SetTTL меняет TTL metadata cache.
//
// Значение <= 0 фактически отключает кеширование metadata.
func (c *Client) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl < 0 {
		ttl = 0
	}

	c.ttl = ttl
}

// SetDownloadTTL меняет TTL кеша скачанных файлов.
//
// Значение <= 0 означает, что файл не будет считаться cache hit
// для следующего вызова после его выдачи.
func (c *Client) SetDownloadTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl < 0 {
		ttl = 0
	}

	c.downloadTTL = ttl
}

// Close останавливает активные downloads и освобождает кеш.
//
// Уже выданные FileStream продолжают работать.
// После их Close() соответствующие временные файлы будут удалены.
func (c *Client) Close() error {
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()

		return nil
	}

	c.closed = true

	var paths []string

	for urlKey, item := range c.downloadCache {
		delete(c.downloadCache, urlKey)

		if item.refs == 0 {
			paths = append(paths, item.dir)
		}
	}

	for _, call := range c.downloads {
		if call.cancel != nil {
			call.cancel()
		}
	}

	c.cache = make(map[string]cacheItem)

	c.mu.Unlock()

	var result error

	for _, path := range paths {
		result = errors.Join(result, os.RemoveAll(path))
	}

	return result
}

// GetInfo получает metadata.
//
// URL приводится к нормализованному виду.
// Playlist запрещён.
// Пользовательский/system yt-dlp config отключён.
func (c *Client) GetInfo(
	ctx context.Context,
	rawURL string,
) (*Info, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}

	targetURL, err := normalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()

		return nil, errors.New("ytdlp client is closed")
	}

	item, exists := c.cache[targetURL]

	if exists && now.Before(item.expiresAt) {
		info := cloneInfo(item.info)

		c.mu.Unlock()

		return info, nil
	}

	if exists {
		delete(c.cache, targetURL)
	}

	binPath := c.binPath
	ttl := c.ttl

	c.mu.Unlock()

	cmd := exec.CommandContext(
		ctx,
		binPath,
		"--ignore-config",
		"--no-playlist",
		"--no-warnings",
		"-j",
		"--",
		targetURL,
	)

	var stdout limitedBuffer
	var stderr limitedBuffer

	stdout.limit = maxStderrSize
	stderr.limit = maxStderrSize

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"yt-dlp metadata failed: %w: %s",
			err,
			stderr.String(),
		)
	}

	var info Info

	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, fmt.Errorf(
			"failed to parse yt-dlp JSON output: %w",
			err,
		)
	}

	cachedInfo := cloneInfo(&info)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return cloneInfo(&info), nil
	}

	c.cache[targetURL] = cacheItem{
		info:      cachedInfo,
		expiresAt: time.Now().Add(ttl),
	}

	if len(c.cache) > maxMetadataCacheItems {
		now := time.Now()

		for key, cached := range c.cache {
			if !now.Before(cached.expiresAt) {
				delete(c.cache, key)
			}
		}
	}

	return cloneInfo(&info), nil
}

// DownloadToTemp скачивает файл во временную директорию
// и возвращает отдельный FileStream.
//
// Для одного URL:
//   - первый вызов выполняет физический download;
//   - параллельные вызовы ждут этот же download;
//   - последующие вызовы в TTL открывают тот же файл.
//
// Каждый FileStream имеет собственный file descriptor.
func (c *Client) DownloadToTemp(
	ctx context.Context,
	rawURL string,
) (*FileStream, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}

	targetURL, err := normalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	for {
		cleanupPaths := make([]string, 0, 4)

		c.mu.Lock()

		if c.closed {
			c.mu.Unlock()

			return nil, errors.New("ytdlp client is closed")
		}

		now := time.Now()

		// Удаляем старые cache entries,
		// которые больше никому не нужны.
		cleanupPaths = append(
			cleanupPaths,
			c.cleanupExpiredDownloadsLocked(now)...,
		)

		// ------------------------------------------------------------
		// 1. Cache hit.
		// ------------------------------------------------------------

		if item, ok := c.downloadCache[targetURL]; ok {
			if now.Before(item.expiresAt) {
				item.refs++

				c.mu.Unlock()

				cleanupErrors := removeDirs(cleanupPaths)

				stream, openErr := c.openCachedFile(
					targetURL,
					item,
				)

				if cleanupErrors != nil {
					// Ошибка cleanup не должна ломать успешную выдачу.
					// Здесь намеренно игнорируем её.
					_ = cleanupErrors
				}

				if openErr != nil {
					return nil, openErr
				}

				return stream, nil
			}
		}

		// ------------------------------------------------------------
		// 2. Уже идёт download этого URL.
		// ------------------------------------------------------------

		if call, ok := c.downloads[targetURL]; ok {
			done := call.done

			c.mu.Unlock()

			_ = removeDirs(cleanupPaths)

			select {
			case <-done:
				if call.err != nil {
					return nil, call.err
				}

				// Download закончен.
				// Возвращаемся наверх и открываем cache.
				continue

			case <-ctx.Done():
				return nil, fmt.Errorf(
					"waiting for download: %w",
					ctx.Err(),
				)
			}
		}

		// ------------------------------------------------------------
		// 3. Мы первый — создаём shared download.
		// ------------------------------------------------------------

		downloadCtx, cancel := context.WithTimeout(
			context.Background(),
			c.downloadTTL,
		)

		call := &downloadCall{
			done:   make(chan struct{}),
			cancel: cancel,
		}

		c.downloads[targetURL] = call

		binPath := c.binPath

		c.mu.Unlock()

		_ = removeDirs(cleanupPaths)

		// ------------------------------------------------------------
		// 4. Физический download.
		//
		// ВАЖНО:
		// используем отдельный context, а не ctx вызывающего HTTP-запроса.
		// Иначе отмена первого caller'а убьёт shared download
		// для всех остальных.
		// ------------------------------------------------------------

		item, err := c.downloadFile(
			downloadCtx,
			binPath,
			targetURL,
		)

		cancel()

		// ------------------------------------------------------------
		// 5. Публикуем результат.
		// ------------------------------------------------------------

		c.mu.Lock()

		delete(c.downloads, targetURL)

		call.err = err
		if err == nil {
			// Старый expired item может ещё использоваться
			// другими FileStream, поэтому просто заменяем map entry.
			c.downloadCache[targetURL] = item
		}

		close(call.done)

		if err != nil {
			c.mu.Unlock()

			return nil, err
		}

		// Мы тоже становимся одним из readers.
		item.refs++

		c.mu.Unlock()

		return c.openCachedFile(
			targetURL,
			item,
		)
	}
}

// downloadFile выполняет один физический download.
//
// Файл создаётся внутри отдельной директории 0700.
// Поэтому нет необходимости делать:
//
//	CreateTemp -> Close -> Remove -> yt-dlp
//
// что устраняет race window с путём.
func (c *Client) downloadFile(
	ctx context.Context,
	binPath string,
	targetURL string,
) (*downloadCacheItem, error) {
	tmpDir, err := os.MkdirTemp("", "ytdlp-cache-*")
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create temp directory: %w",
			err,
		)
	}

	cleanupOnError := true

	defer func() {
		if cleanupOnError {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	outputPath := filepath.Join(
		tmpDir,
		"video.mp4",
	)

	cmd := exec.CommandContext(
		ctx,
		binPath,
		"--ignore-config",
		"--no-playlist",
		"--no-warnings",
		"--quiet",
		"--no-progress",
		"--force-overwrites",
		"-o",
		outputPath,
		"--",
		targetURL,
	)

	// yt-dlp не должен использовать stdout как application output.
	cmd.Stdout = io.Discard

	var stderr limitedBuffer
	stderr.limit = maxStderrSize
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"yt-dlp download failed: %w: %s",
			err,
			stderr.String(),
		)
	}

	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf(
			"downloaded file does not exist: %w: %s",
			err,
			stderr.String(),
		)
	}

	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf(
			"downloaded path is not a regular file",
		)
	}

	if stat.Size() <= 0 {
		return nil, fmt.Errorf(
			"downloaded file is empty: %s",
			stderr.String(),
		)
	}

	c.mu.Lock()
	ttl := c.ttl
	c.mu.Unlock()

	cleanupOnError = false

	return &downloadCacheItem{
		dir:       tmpDir,
		path:      outputPath,
		size:      stat.Size(),
		expiresAt: time.Now().Add(ttl),
	}, nil
}

// openCachedFile открывает новый FD для cached file.
//
// item.refs уже увеличен вызывающим кодом.
func (c *Client) openCachedFile(
	urlKey string,
	item *downloadCacheItem,
) (*FileStream, error) {
	file, err := os.Open(item.path)
	if err != nil {
		var cleanupPath string

		c.mu.Lock()

		if item.refs > 0 {
			item.refs--
		}

		current, exists := c.downloadCache[urlKey]

		if !exists || current != item {
			if item.refs == 0 {
				cleanupPath = item.dir
			}
		} else if item.refs == 0 && !time.Now().Before(item.expiresAt) {
			delete(c.downloadCache, urlKey)

			cleanupPath = item.dir
		}

		c.mu.Unlock()

		if cleanupPath != "" {
			_ = os.RemoveAll(cleanupPath)
		}

		return nil, fmt.Errorf(
			"failed to open cached file: %w",
			err,
		)
	}

	return &FileStream{
		File: file,
		Size: item.size,
		release: func() error {
			return c.releaseDownloadedFile(urlKey, item)
		},
	}, nil
}

// releaseDownloadedFile освобождает одну ссылку на файл.
func (c *Client) releaseDownloadedFile(
	urlKey string,
	item *downloadCacheItem,
) error {
	var cleanupPath string

	c.mu.Lock()

	if item.refs > 0 {
		item.refs--
	}

	current, exists := c.downloadCache[urlKey]

	// Пока cache entry жив и TTL не истёк,
	// физический файл оставляем.
	if exists &&
		current == item &&
		!c.closed &&
		time.Now().Before(item.expiresAt) {
		c.mu.Unlock()

		return nil
	}

	// Если это текущий item — удаляем его из cache.
	if exists && current == item {
		delete(c.downloadCache, urlKey)
	}

	// Старый item с refs == 0 больше никому не нужен.
	if item.refs == 0 {
		cleanupPath = item.dir
	}

	c.mu.Unlock()

	if cleanupPath == "" {
		return nil
	}

	if err := os.RemoveAll(cleanupPath); err != nil {
		return fmt.Errorf(
			"failed to remove cached download: %w",
			err,
		)
	}

	return nil
}

// cleanupExpiredDownloadsLocked удаляет expired entries,
// которые больше не используются.
//
// Caller должен владеть c.mu.
func (c *Client) cleanupExpiredDownloadsLocked(
	now time.Time,
) []string {
	var paths []string

	for key, item := range c.downloadCache {
		if now.Before(item.expiresAt) {
			continue
		}

		if item.refs != 0 {
			// FileStream всё ещё используется.
			// Он будет удалён через releaseDownloadedFile().
			continue
		}

		delete(c.downloadCache, key)

		paths = append(paths, item.dir)
	}

	return paths
}

// normalizeURL валидирует входной URL.
//
// Для server-side downloader сознательно разрешены только HTTP/HTTPS.
// Это снижает риск использования компонента как произвольного
// protocol/file fetcher.
func normalizeURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" {
		return "", errors.New("empty URL")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf(
			"invalid URL: %w",
			err,
		)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf(
			"unsupported URL scheme %q: only http and https are allowed",
			u.Scheme,
		)
	}

	if u.Host == "" {
		return "", errors.New("URL host is empty")
	}

	// Не принимаем user:password@host.
	// Для API-сервиса credentials в URL обычно не нужны
	// и могут случайно попасть в cache/logs.
	if u.User != nil {
		return "", errors.New(
			"URLs with embedded credentials are not allowed",
		)
	}

	return u.String(), nil
}

func cloneInfo(info *Info) *Info {
	if info == nil {
		return nil
	}

	return new(*info)
}

// removeDirs удаляет директории после выхода из mutex.
func removeDirs(paths []string) error {
	var result error

	for _, path := range paths {
		if path == "" {
			continue
		}

		if err := os.RemoveAll(path); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}

// limitedBuffer ограничивает объём stderr/stdout,
// который может накопиться в памяти.
type limitedBuffer struct {
	buf   bytes.Buffer
	limit int64
	total int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.total += int64(len(p))

	if b.limit <= 0 {
		return len(p), nil
	}

	remaining := b.limit - int64(b.buf.Len())

	if remaining > 0 {
		if int64(len(p)) <= remaining {
			_, _ = b.buf.Write(p)
		} else {
			_, _ = b.buf.Write(p[:remaining])
		}
	}

	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *limitedBuffer) String() string {
	result := b.buf.String()

	if b.total > int64(b.buf.Len()) {
		result += "\n[output truncated]"
	}

	return result
}
