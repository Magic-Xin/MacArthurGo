package googlelens

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	serpapi "github.com/serpapi/serpapi-golang"
)

type Config struct {
	APIKey     string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type Client struct {
	client *serpapi.SerpApiClient
}

type Result struct {
	Position  int
	Title     string
	Thumbnail string
	Link      string
}

type visualMatch struct {
	Result
	source   string
	priority int
	index    int
}

func NewClient(config Config) (*Client, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, errors.New("SerpApi API key is required")
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	setting := serpapi.NewSerpApiClientSetting(apiKey)
	setting.Engine = "google_lens"
	setting.Persistent = true
	setting.Timeout = timeout
	apiClient := serpapi.NewClient(setting)
	if config.HTTPClient != nil {
		apiClient.HttpSearch = config.HTTPClient
	} else {
		transport := http.DefaultTransport
		if defaultTransport, ok := transport.(*http.Transport); ok {
			transport = defaultTransport.Clone()
		}
		apiClient.HttpSearch = &http.Client{Timeout: timeout, Transport: transport}
	}
	return &Client{client: &apiClient}, nil
}

func (c *Client) Search(ctx context.Context, imageURL string) (Result, error) {
	imageURL = validHTTPURL(imageURL)
	if imageURL == "" {
		return Result{}, errors.New("image URL must use http or https")
	}

	apiClient := *c.client
	apiClient.HttpSearch = clientWithContext(c.client.HttpSearch, ctx)
	response, err := apiClient.Search(map[string]string{
		"url":  imageURL,
		"type": "visual_matches",
		"safe": "off",
	})
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) {
			err = urlError.Err
		}
		return Result{}, fmt.Errorf("SerpApi Google Lens request failed: %w", err)
	}
	return selectBestMatch(response)
}

func clientWithContext(client *http.Client, ctx context.Context) *http.Client {
	copy := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	copy.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return transport.RoundTrip(request.WithContext(ctx))
	})
	return &copy
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func selectBestMatch(response map[string]interface{}) (Result, error) {
	rawMatches, ok := response["visual_matches"].([]interface{})
	if !ok || len(rawMatches) == 0 {
		return Result{}, errors.New("SerpApi Google Lens returned no visual matches")
	}

	best := visualMatch{priority: math.MaxInt, index: math.MaxInt, Result: Result{Position: math.MaxInt}}
	for index, rawMatch := range rawMatches {
		data, ok := rawMatch.(map[string]interface{})
		if !ok {
			continue
		}
		match := visualMatch{
			Result: Result{
				Position:  resultPosition(data["position"], index+1),
				Title:     stringValue(data["title"]),
				Thumbnail: validHTTPURL(stringValue(data["thumbnail"])),
				Link:      validHTTPURL(stringValue(data["link"])),
			},
			source: stringValue(data["source"]),
			index:  index,
		}
		if match.Title == "" || match.Thumbnail == "" || match.Link == "" {
			continue
		}
		match.priority = sourcePriority(match.Link, match.source)
		if match.priority < best.priority ||
			(match.priority == best.priority && match.Position < best.Position) ||
			(match.priority == best.priority && match.Position == best.Position && match.index < best.index) {
			best = match
		}
	}
	if best.priority == math.MaxInt {
		return Result{}, errors.New("SerpApi Google Lens returned no usable visual match")
	}
	return best.Result, nil
}

func sourcePriority(link, source string) int {
	host := ""
	if parsed, err := url.Parse(link); err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	source = strings.ToLower(strings.TrimSpace(source))
	switch {
	case isDomain(host, "pixiv.net"), source == "pixiv", strings.HasPrefix(source, "pixiv "):
		return 0
	case isDomain(host, "twitter.com"), isDomain(host, "x.com"), source == "twitter", source == "x", strings.HasPrefix(source, "twitter "):
		return 1
	default:
		return 2
	}
}

func isDomain(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func resultPosition(value interface{}, fallback int) int {
	position := fallback
	switch number := value.(type) {
	case float64:
		if number > 0 && number <= math.MaxInt {
			position = int(number)
		}
	case int:
		if number > 0 {
			position = number
		}
	case string:
		if parsed, err := strconv.Atoi(number); err == nil && parsed > 0 {
			position = parsed
		}
	}
	return position
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func validHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.String()
}
