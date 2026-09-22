package soutubot

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultSimilarityThreshold = 45.0

	mainPageURL       = "https://soutubot.moe"
	searchAPIURL      = mainPageURL + "/api/search"
	defaultTimeout    = 90 * time.Second
	maxImageSize      = 20 << 20
	maxResponseSize   = 16 << 20
	webkitBoundary    = "----WebKitFormBoundary"
	boundarySuffixLen = 16
)

type Config struct {
	CloudflareBypassURL string
	ProxyURL            string
	Timeout             time.Duration
}

type Client struct {
	bypassURL string
	proxyURL  string
	timeout   time.Duration
	http      *http.Client

	mu        sync.Mutex
	userAgent string
}

type Response struct {
	Data          []Item  `json:"data"`
	ID            string  `json:"id"`
	Factor        float64 `json:"factor"`
	ImageURL      string  `json:"imageUrl"`
	SearchOption  string  `json:"searchOption"`
	ExecutionTime float64 `json:"executionTime"`
}

type Item struct {
	Source          Source   `json:"source"`
	Page            int      `json:"page"`
	Title           string   `json:"title"`
	Language        Language `json:"language"`
	PagePath        string   `json:"pagePath"`
	SubjectPath     string   `json:"subjectPath"`
	PreviewImageURL string   `json:"previewImageUrl"`
	Similarity      float64  `json:"similarity"`
	URL             string   `json:"-"`
}

type Source string

var sourceBaseURLs = map[Source]string{
	"nhentai": "https://nhentai.net",
	"ehentai": "https://e-hentai.org",
	"panda":   "https://panda.chaika.moe",
}

type Language string

const (
	LanguageChinese  Language = "cn"
	LanguageJapanese Language = "jp"
	LanguageEnglish  Language = "gb"
)

func (l Language) Emoji() string {
	switch l {
	case LanguageChinese:
		return "🇨🇳"
	case LanguageJapanese:
		return "🇯🇵"
	case LanguageEnglish:
		return "🇬🇧"
	default:
		return string(l)
	}
}

func (i Item) SourceURL() string {
	if i.URL != "" {
		return i.URL
	}
	baseURL, knownSource := sourceBaseURLs[i.Source]
	if !knownSource {
		parsed, err := url.Parse(strings.TrimSpace(string(i.Source)))
		if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return parsed.String()
		}
		return ""
	}

	path := strings.TrimSpace(i.SubjectPath)
	if path == "" {
		path = strings.TrimSpace(i.PagePath)
	}
	if path == "" {
		return baseURL
	}
	base, _ := url.Parse(baseURL)
	reference, err := url.Parse(path)
	if err != nil {
		return baseURL
	}
	// API paths are expected to be relative to the known source. Clearing an
	// unexpected host keeps malformed response data on that trusted host.
	reference.Scheme = ""
	reference.Host = ""
	return base.ResolveReference(reference).String()
}

// SelectBestMatches applies SoutuBot's overall confidence threshold and then
// returns the highest-similarity Japanese, Chinese, and English items, in that order.
func SelectBestMatches(items []Item, threshold float64) ([]Item, float64) {
	maxSimilarity := 0.0
	for _, item := range items {
		if item.Similarity > maxSimilarity {
			maxSimilarity = item.Similarity
		}
	}
	if len(items) == 0 || maxSimilarity < threshold {
		return nil, maxSimilarity
	}

	matches := make([]Item, 0, 3)
	for _, language := range []Language{LanguageJapanese, LanguageChinese, LanguageEnglish} {
		var (
			best  Item
			found bool
		)
		for _, item := range items {
			if canonicalLanguage(item.Language) == language && (!found || item.Similarity > best.Similarity) {
				best = item
				best.Language = language
				found = true
			}
		}
		if found {
			matches = append(matches, best)
		}
	}
	return matches, maxSimilarity
}

func FormatMatches(matches []Item) string {
	lines := make([]string, 0, 3)
	for _, language := range []Language{LanguageJapanese, LanguageChinese, LanguageEnglish} {
		var (
			match Item
			found bool
		)
		for _, candidate := range matches {
			if canonicalLanguage(candidate.Language) == language && (!found || candidate.Similarity > match.Similarity) {
				match = candidate
				match.Language = language
				found = true
			}
		}
		if !found {
			switch language {
			case LanguageJapanese:
				lines = append(lines, "未找到日文结果")
			case LanguageChinese:
				lines = append(lines, "未找到中文结果")
			case LanguageEnglish:
				lines = append(lines, "未找到英文结果")
			}
			continue
		}

		title := strings.Join(strings.Fields(match.Title), " ")
		if title == "" {
			title = "-"
		}
		source := match.SourceURL()
		if source == "" {
			source = "-"
		}
		lines = append(lines, fmt.Sprintf(
			"标题: %s | 相似度: %.2f%% | 语言: %s | 来源: %s",
			title,
			match.Similarity,
			match.Language.Emoji(),
			source,
		))
	}
	return "SoutuBot\n\n" + strings.Join(lines, "\n\n")
}

func canonicalLanguage(language Language) Language {
	switch language {
	case "ja":
		return LanguageJapanese
	case "zh":
		return LanguageChinese
	case "en":
		return LanguageEnglish
	default:
		return language
	}
}

func IsFormattedMatches(value string) bool {
	lines := strings.Split(value, "\n\n")
	if len(lines) != 4 || lines[0] != "SoutuBot" {
		return false
	}
	return isFormattedMatchOrMissing(lines[1], "未找到日文结果") &&
		isFormattedMatchOrMissing(lines[2], "未找到中文结果") &&
		isFormattedMatchOrMissing(lines[3], "未找到英文结果")
}

func isFormattedMatchOrMissing(value string, missingMessage string) bool {
	return value == missingMessage ||
		(strings.HasPrefix(value, "标题: ") &&
			strings.Contains(value, " | 相似度: ") &&
			strings.Contains(value, " | 语言: ") &&
			strings.Contains(value, " | 来源: "))
}

func NewClient(cfg Config) (*Client, error) {
	bypassURL := strings.TrimRight(strings.TrimSpace(cfg.CloudflareBypassURL), "/")
	parsed, err := url.Parse(bypassURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid CloudflareBypassForScraping URL %q", bypassURL)
	}

	proxyURL := strings.TrimSpace(cfg.ProxyURL)
	if proxyURL != "" {
		proxy, parseErr := url.Parse(proxyURL)
		if parseErr != nil || proxy.Host == "" || (proxy.Scheme != "http" && proxy.Scheme != "https" && proxy.Scheme != "socks4" && proxy.Scheme != "socks5") {
			return nil, fmt.Errorf("invalid SoutuBot proxy URL %q", proxyURL)
		}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		bypassURL: bypassURL,
		proxyURL:  proxyURL,
		timeout:   timeout,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (c *Client) Search(ctx context.Context, imageData []byte) (*Response, error) {
	if len(imageData) == 0 {
		return nil, errors.New("SoutuBot image data is empty")
	}
	if len(imageData) > maxImageSize {
		return nil, fmt.Errorf("SoutuBot image exceeds %d MiB", maxImageSize>>20)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// CloudflareBypassForScraping caches clearance state by host. Keep the
	// homepage user agent paired with the upload request.
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.userAgent == "" {
		if err := c.refreshCredentials(ctx); err != nil {
			return nil, err
		}
	}

	result, err := c.search(ctx, imageData, false)
	var statusErr *httpStatusError
	if err == nil || !errors.As(err, &statusErr) || (statusErr.StatusCode != http.StatusUnauthorized && statusErr.StatusCode != http.StatusForbidden) {
		return result, err
	}

	if refreshErr := c.refreshCredentials(ctx); refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	return c.search(ctx, imageData, true)
}

func (c *Client) refreshCredentials(ctx context.Context) error {
	endpoint, err := url.Parse(c.bypassURL + "/html")
	if err != nil {
		return err
	}
	query := endpoint.Query()
	query.Set("url", mainPageURL)
	if c.proxyURL != "" {
		query.Set("proxy", c.proxyURL)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("CloudflareBypassForScraping SoutuBot homepage request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()

	_, err = readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return fmt.Errorf("read CloudflareBypassForScraping SoutuBot homepage: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &httpStatusError{StatusCode: resp.StatusCode, Operation: "SoutuBot homepage"}
	}

	userAgent := strings.TrimSpace(resp.Header.Get("x-cf-bypasser-user-agent"))
	if userAgent == "" {
		return errors.New("CloudflareBypassForScraping SoutuBot homepage did not return a user agent")
	}
	c.userAgent = userAgent
	return nil
}

func (c *Client) search(ctx context.Context, imageData []byte, forceBypass bool) (*Response, error) {
	body, contentType, err := buildMultipartBody(imageData)
	if err != nil {
		return nil, fmt.Errorf("build SoutuBot request: %w", err)
	}

	target, _ := url.Parse(searchAPIURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.bypassURL+target.EscapedPath(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", mainPageURL)
	req.Header.Set("Referer", mainPageURL+"/")
	req.Header.Set("DNT", "1")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("x-hostname", target.Host)
	if c.proxyURL != "" {
		req.Header.Set("x-proxy", c.proxyURL)
	}
	if forceBypass {
		req.Header.Set("x-bypass-cache", "true")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CloudflareBypassForScraping SoutuBot search request: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()

	responseBody, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read SoutuBot response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Operation: "SoutuBot search"}
	}

	var payload struct {
		ResultID string `json:"result_id"`
		Results  []struct {
			Score        float64 `json:"score"`
			PathSegments []struct {
				SourceKey      string   `json:"source_key"`
				ExternalID     string   `json:"external_id"`
				Language       Language `json:"language"`
				RawPathSegment string   `json:"raw_path_segment"`
				SourceURL      string   `json:"source_url"`
				PageURL        string   `json:"page_url"`
				Metadata       *struct {
					Title *struct {
						Primary string `json:"primary"`
					} `json:"title"`
					Source *struct {
						Name string `json:"name"`
						ID   string `json:"id"`
					} `json:"source"`
				} `json:"metadata"`
			} `json:"path_segments"`
		} `json:"results"`
	}
	if err = json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("decode SoutuBot response: %w", err)
	}
	result := &Response{ID: payload.ResultID}
	for _, hit := range payload.Results {
		for _, segment := range hit.PathSegments {
			title := ""
			if segment.Metadata != nil {
				if segment.Metadata.Title != nil {
					title = segment.Metadata.Title.Primary
				}
				if title == "" && segment.Metadata.Source != nil {
					title = strings.TrimSpace(segment.Metadata.Source.Name + " #" + segment.Metadata.Source.ID)
				}
			}
			if title == "" {
				title = strings.TrimSpace(segment.SourceKey + " #" + segment.ExternalID)
			}
			if title == "#" {
				title = segment.RawPathSegment
			}
			link := segment.SourceURL
			if link == "" {
				link = segment.PageURL
			}
			result.Data = append(result.Data, Item{Title: title, Similarity: hit.Score, Language: canonicalLanguage(segment.Language), Source: Source(segment.SourceKey), URL: link})
		}
	}
	return result, nil
}

func buildMultipartBody(imageData []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	suffix := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, suffix); err != nil {
		return nil, "", err
	}
	boundary := webkitBoundary + base64.RawURLEncoding.EncodeToString(suffix)[:boundarySuffixLen]
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, "", err
	}

	header := textproto.MIMEHeader{}
	filename, contentType := imageFileInfo(imageData)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err = part.Write(imageData); err != nil {
		return nil, "", err
	}
	if err = writer.WriteField("factor", "1.2"); err != nil {
		return nil, "", err
	}
	if err = writer.WriteField("metadata_mode", "display"); err != nil {
		return nil, "", err
	}
	if err = writer.Close(); err != nil {
		return nil, "", err
	}
	return body, writer.FormDataContentType(), nil
}

func imageFileInfo(image []byte) (string, string) {
	contentType := http.DetectContentType(image)
	switch contentType {
	case "image/jpeg":
		return "image.jpg", contentType
	case "image/png":
		return "image.png", contentType
	case "image/gif":
		return "image.gif", contentType
	case "image/webp":
		return "image.webp", contentType
	default:
		return "image.bin", "application/octet-stream"
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d MiB", limit>>20)
	}
	return data, nil
}

func unwrapURLError(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) {
		return urlError.Err
	}
	return err
}

type httpStatusError struct {
	StatusCode int
	Operation  string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("CloudflareBypassForScraping %s returned HTTP %d", e.Operation, e.StatusCode)
}
