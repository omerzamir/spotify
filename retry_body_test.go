package spotify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
