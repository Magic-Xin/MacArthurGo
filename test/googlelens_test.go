package test

import (
	"MacArthurGo/plugins/googlelens"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type googleLensRoundTripFunc func(*http.Request) (*http.Response, error)

func (f googleLensRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGoogleLensSearch_UsesRequiredParametersAndPrioritizesPixiv(t *testing.T) {
	t.Parallel()

	client := newGoogleLensTestClient(t, func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"engine": "google_lens",
			"url":    "https://example.com/input.png?token=one",
			"type":   "visual_matches",
			"safe":   "off",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		if query.Has("hl") {
			t.Errorf("request unexpectedly contains language restriction: %s", request.URL.RawQuery)
		}
		return googleLensJSONResponse(`{
			"visual_matches": [
				{"position": 1, "title": "Other", "link": "https://example.org/post", "source": "Other", "thumbnail": "https://img.example.org/1.jpg"},
				{"position": 2, "title": "Twitter", "link": "https://x.com/user/status/1", "source": "X", "thumbnail": "https://pbs.twimg.com/media/1.jpg"},
				{"position": 5, "title": "Later Pixiv", "link": "https://www.pixiv.net/artworks/5", "source": "pixiv", "thumbnail": "https://i.pximg.net/img-original/5.jpg"},
				{"position": 3, "title": "Best Pixiv", "link": "https://www.pixiv.net/artworks/3", "source": "pixiv", "thumbnail": "https://i.pximg.net/img-original/3.jpg"}
			]
		}`), nil
	})

	result, err := client.Search(context.Background(), "https://example.com/input.png?token=one")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Position != 3 || result.Title != "Best Pixiv" || result.Thumbnail != "https://i.pximg.net/img-original/3.jpg" || result.Link != "https://www.pixiv.net/artworks/3" {
		t.Fatalf("Search() result = %#v", result)
	}
}

func TestGoogleLensSearch_PrioritizesTwitterWithoutPixiv(t *testing.T) {
	t.Parallel()

	client := newGoogleLensTestClient(t, func(*http.Request) (*http.Response, error) {
		return googleLensJSONResponse(`{
			"visual_matches": [
				{"position": 1, "title": "Other", "link": "https://example.org/post", "source": "Other", "thumbnail": "https://img.example.org/1.jpg"},
				{"position": 4, "title": "Later Twitter", "link": "https://twitter.com/user/status/4", "source": "Twitter", "thumbnail": "https://pbs.twimg.com/media/4.jpg"},
				{"position": 2, "title": "Best Twitter", "link": "https://x.com/user/status/2", "source": "X", "thumbnail": "https://pbs.twimg.com/media/2.jpg"}
			]
		}`), nil
	})

	result, err := client.Search(context.Background(), "https://example.com/input.png")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if result.Position != 2 || result.Title != "Best Twitter" {
		t.Fatalf("Search() result = %#v", result)
	}
}

func TestGoogleLensSearch_RejectsUnusableMatches(t *testing.T) {
	t.Parallel()

	client := newGoogleLensTestClient(t, func(*http.Request) (*http.Response, error) {
		return googleLensJSONResponse(`{"visual_matches":[{"position":1,"title":"Missing thumbnail","link":"https://example.com"}]}`), nil
	})
	if _, err := client.Search(context.Background(), "https://example.com/input.png"); err == nil {
		t.Fatal("Search() accepted a match without title, thumbnail, and link")
	}
}

func TestGoogleLensSearch_DoesNotLeakAPIKeyInTransportErrors(t *testing.T) {
	t.Parallel()

	client := newGoogleLensTestClient(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})
	_, err := client.Search(context.Background(), "https://example.com/input.png")
	if err == nil {
		t.Fatal("Search() returned no transport error")
	}
	if strings.Contains(err.Error(), "secret-api-key") {
		t.Fatalf("Search() leaked API key: %v", err)
	}
}

func TestGoogleLensSearch_AllowsConcurrentCalls(t *testing.T) {
	client := newGoogleLensTestClient(t, func(*http.Request) (*http.Response, error) {
		return googleLensJSONResponse(`{"visual_matches":[{"position":1,"title":"Pixiv","link":"https://www.pixiv.net/artworks/1","thumbnail":"https://i.pximg.net/1.jpg"}]}`), nil
	})

	const calls = 16
	errors := make(chan error, calls)
	var wait sync.WaitGroup
	for range calls {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := client.Search(context.Background(), "https://example.com/input.png")
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent Search() error = %v", err)
		}
	}
}

func TestGoogleLensNewClient_RequiresAPIKey(t *testing.T) {
	if _, err := googlelens.NewClient(googlelens.Config{}); err == nil {
		t.Fatal("NewClient() accepted an empty API key")
	}
}

func newGoogleLensTestClient(t *testing.T, roundTrip googleLensRoundTripFunc) *googlelens.Client {
	t.Helper()
	client, err := googlelens.NewClient(googlelens.Config{
		APIKey:  "secret-api-key",
		Timeout: time.Second,
		HTTPClient: &http.Client{
			Timeout:   time.Second,
			Transport: roundTrip,
		},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func googleLensJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
