package spotify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAutoRetryResendsRequestBody guards against retrying a write with the
// body already consumed by the first attempt.
func TestAutoRetryResendsRequestBody(t *testing.T) {
	var bodies []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading body: %v", err)
		}
		bodies = append(bodies, string(body))

		if len(bodies) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"status":429,"message":"slow down"}}`)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"snapshot_id":"after-retry"}`)
	}))
	defer server.Close()

	client := &Client{http: http.DefaultClient, baseURL: server.URL + "/", autoRetry: true}

	snapshot, err := client.AddItemsToPlaylist(context.Background(), "pl", "spotify:track:a")
	if err != nil {
		t.Fatalf("AddItemsToPlaylist: %v", err)
	}

	if snapshot != "after-retry" {
		t.Errorf("snapshot = %q, want after-retry", snapshot)
	}
	if len(bodies) != 2 || bodies[0] == "" || bodies[0] != bodies[1] {
		t.Errorf("bodies = %q, want the same non-empty body sent twice", bodies)
	}
}

// TestAutoRetrySkipsUnrewindableBody checks that a request whose body cannot
// be recreated is not retried, and that the caller gets Spotify's original
// error rather than a local "cannot retry" one.
func TestAutoRetrySkipsUnrewindableBody(t *testing.T) {
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = io.Copy(io.Discard, r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"status":429,"message":"slow down"}}`)
	}))
	defer server.Close()

	client := &Client{http: http.DefaultClient, baseURL: server.URL + "/", autoRetry: true}

	// SetPlaylistImage streams its body through an io.Pipe, so the request has
	// no GetBody.
	err := client.SetPlaylistImage(context.Background(), "pl", strings.NewReader("jpeg"))

	var spotifyErr Error
	if !errors.As(err, &spotifyErr) {
		t.Fatalf("error = %v (%T), want a spotify.Error", err, err)
	}
	if spotifyErr.Status != http.StatusTooManyRequests {
		t.Errorf("Status = %d, want 429", spotifyErr.Status)
	}
	if spotifyErr.RetryAfter.IsZero() {
		t.Error("RetryAfter is zero, want it set from the Retry-After header")
	}
	if requests != 1 {
		t.Errorf("%d requests reached the server, want 1", requests)
	}
}

// TestGetWithInvalidURLReturnsError guards against touching a nil request
// when it could not be built while an Accept-Language is configured.
func TestGetWithInvalidURLReturnsError(t *testing.T) {
	client := &Client{
		http:           http.DefaultClient,
		baseURL:        "http://x/\x7f",
		acceptLanguage: "en",
	}

	if _, err := client.GetPlaylistItems(context.Background(), "pl"); err == nil {
		t.Error("error = nil, want an error for the invalid URL")
	}
}
