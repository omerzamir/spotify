package spotify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestPlaylistEditRequestBodies pins the exact JSON sent by the playlist edit
// methods, including the "tracks" vs "items" key of the remove endpoints.
func TestPlaylistEditRequestBodies(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantMethod string
		wantPath   string
		wantBody   string
		call       func(*Client) (string, error)
	}{
		{
			name:       "AddItemsToPlaylist",
			status:     http.StatusCreated,
			wantMethod: http.MethodPost,
			wantPath:   "/playlists/pl/items",
			wantBody:   `{"uris":["spotify:track:a","spotify:episode:b"]}`,
			call: func(c *Client) (string, error) {
				return c.AddItemsToPlaylist(context.Background(), "pl", "spotify:track:a", "spotify:episode:b")
			},
		},
		{
			name:       "AddTracksToPlaylist",
			status:     http.StatusCreated,
			wantMethod: http.MethodPost,
			wantPath:   "/playlists/pl/tracks",
			wantBody:   `{"uris":["spotify:track:a"]}`,
			call: func(c *Client) (string, error) {
				return c.AddTracksToPlaylist(context.Background(), "pl", "a")
			},
		},
		{
			name:       "RemoveItemsFromPlaylist",
			status:     http.StatusOK,
			wantMethod: http.MethodDelete,
			wantPath:   "/playlists/pl/items",
			wantBody:   `{"items":[{"uri":"spotify:track:a"}]}`,
			call: func(c *Client) (string, error) {
				return c.RemoveItemsFromPlaylist(context.Background(), "pl", "spotify:track:a")
			},
		},
		{
			name:       "RemoveTracksFromPlaylist",
			status:     http.StatusOK,
			wantMethod: http.MethodDelete,
			wantPath:   "/playlists/pl/tracks",
			wantBody:   `{"tracks":[{"uri":"spotify:track:a"}]}`,
			call: func(c *Client) (string, error) {
				return c.RemoveTracksFromPlaylist(context.Background(), "pl", "a")
			},
		},
		{
			name:       "RemoveItemsFromPlaylistOpt",
			status:     http.StatusOK,
			wantMethod: http.MethodDelete,
			wantPath:   "/playlists/pl/items",
			wantBody:   `{"items":[{"uri":"spotify:track:a","positions":[0,3]}],"snapshot_id":"snap"}`,
			call: func(c *Client) (string, error) {
				return c.RemoveItemsFromPlaylistOpt(context.Background(), "pl",
					[]ItemToRemove{NewItemToRemove("spotify:track:a", []int{0, 3})}, "snap")
			},
		},
		{
			name:       "ReplacePlaylistItems",
			status:     http.StatusCreated,
			wantMethod: http.MethodPut,
			wantPath:   "/playlists/pl/items",
			wantBody:   `{"uris":["spotify:track:a"]}`,
			call: func(c *Client) (string, error) {
				return c.ReplacePlaylistItems(context.Background(), "pl", "spotify:track:a")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotBody, gotContentType string

			client, server := testClientString(tt.status, `{"snapshot_id":"new"}`, func(req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Errorf("reading body: %v", err)
				}
				gotMethod, gotPath, gotBody = req.Method, req.URL.Path, strings.TrimSpace(string(body))
				gotContentType = req.Header.Get("Content-Type")
			})
			defer server.Close()

			snapshot, err := tt.call(client)
			if err != nil {
				t.Fatalf("call: %v", err)
			}

			if snapshot != "new" {
				t.Errorf("snapshot = %q, want %q", snapshot, "new")
			}
			if gotMethod != tt.wantMethod || gotPath != tt.wantPath {
				t.Errorf("request = %s %s, want %s %s", gotMethod, gotPath, tt.wantMethod, tt.wantPath)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body = %s, want %s", gotBody, tt.wantBody)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
		})
	}
}
