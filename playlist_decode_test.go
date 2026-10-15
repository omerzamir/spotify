package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPlaylistItemDecodeKeys(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantTrackID ID
		wantNil     bool
	}{
		{
			name:        "item wins over a partial track alias",
			body:        `{"item":{"type":"track","id":"a"},"track":{"id":"x"}}`,
			wantTrackID: "a",
		},
		{
			name:        "null item falls back to track",
			body:        `{"item":null,"track":{"type":"track","id":"b"}}`,
			wantTrackID: "b",
		},
		{
			name:        "missing item falls back to track",
			body:        `{"track":{"type":"track","id":"c"}}`,
			wantTrackID: "c",
		},
		{
			name:    "neither key leaves the zero value",
			body:    `{"added_at":"2026-01-01T00:00:00Z"}`,
			wantNil: true,
		},
		{
			name:    "both null leaves the zero value",
			body:    `{"item":null,"track":null}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item PlaylistItem
			if err := json.Unmarshal([]byte(tt.body), &item); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if tt.wantNil {
				if item.Track.Track != nil || item.Track.Episode != nil {
					t.Errorf("Track = %+v, want zero", item.Track)
				}

				return
			}

			if item.Track.Track == nil || item.Track.Track.ID != tt.wantTrackID {
				t.Errorf("Track = %+v, want track %s", item.Track.Track, tt.wantTrackID)
			}
		})
	}
}

func TestPlaylistTrackDecodeKeys(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		wantID ID
	}{
		{"item wins over a partial track alias", `{"item":{"id":"a"},"track":{"id":"x"}}`, "a"},
		{"null item falls back to track", `{"item":null,"track":{"id":"b"}}`, "b"},
		{"legacy track only", `{"track":{"id":"c"}}`, "c"},
		{"neither key", `{"added_at":"2026-01-01T00:00:00Z"}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var track PlaylistTrack
			if err := json.Unmarshal([]byte(tt.body), &track); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if track.Track.ID != tt.wantID {
				t.Errorf("Track.ID = %q, want %q", track.Track.ID, tt.wantID)
			}
		})
	}
}

// The deprecated alias must not be decoded at all when "item" is usable, so a
// malformed alias (here an unknown type) cannot fail the page.
func TestPlaylistItemIgnoresMalformedTrackAlias(t *testing.T) {
	const body = `{"items":[{"item":{"type":"track","id":"a"},"track":{"id":"no-type"}}],"next":""}`

	var page PlaylistItemPage
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got := page.Items[0].Track.Track; got == nil || got.ID != "a" {
		t.Errorf("Track = %+v, want track a", got)
	}
}

func TestSimplePlaylistDecodeTracksKey(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantTotal Numeric
	}{
		{"items key", `{"id":"p","name":"Mix","items":{"href":"h","total":5}}`, 5},
		{"legacy tracks key", `{"id":"p","name":"Mix","tracks":{"href":"h","total":5}}`, 5},
		{"items wins over tracks", `{"id":"p","name":"Mix","tracks":{"total":9},"items":{"total":5}}`, 5},
		{"neither key", `{"id":"p","name":"Mix"}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p SimplePlaylist
			if err := json.Unmarshal([]byte(tt.body), &p); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if p.ID != "p" || p.Name != "Mix" {
				t.Errorf("playlist = %s %q, want p \"Mix\"", p.ID, p.Name)
			}
			if p.Tracks.Total != tt.wantTotal {
				t.Errorf("Tracks.Total = %d, want %d", p.Tracks.Total, tt.wantTotal)
			}
		})
	}
}

func TestFullPlaylistDecodeAllFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "items key",
			body: `{"id":"p","name":"Mix","owner":{"id":"me"},"followers":{"total":7},
				"items":{"total":3,"next":"n","items":[{"item":{"id":"t1"}}]}}`,
		},
		{
			name: "legacy tracks key",
			body: `{"id":"p","name":"Mix","owner":{"id":"me"},"followers":{"total":7},
				"tracks":{"total":3,"next":"n","items":[{"track":{"id":"t1"}}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p FullPlaylist
			if err := json.Unmarshal([]byte(tt.body), &p); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if p.ID != "p" || p.Name != "Mix" {
				t.Errorf("playlist = %s %q, want p \"Mix\"", p.ID, p.Name)
			}
			if p.Owner.ID != "me" {
				t.Errorf("Owner.ID = %q, want me", p.Owner.ID)
			}
			if p.Followers.Count != 7 {
				t.Errorf("Followers.Count = %d, want 7", p.Followers.Count)
			}
			if p.Tracks.Total != 3 || p.Tracks.Next != "n" {
				t.Errorf("Tracks = total %d next %q, want 3 and n", p.Tracks.Total, p.Tracks.Next)
			}
			if len(p.Tracks.Tracks) != 1 || p.Tracks.Tracks[0].Track.ID != "t1" {
				t.Errorf("Tracks.Tracks = %+v, want track t1", p.Tracks.Tracks)
			}
		})
	}
}

func TestSimplePlaylistPageStillDecodes(t *testing.T) {
	client, server := testClientFile(http.StatusOK, "test_data/current_users_playlists.txt")
	defer server.Close()

	page, err := client.CurrentUsersPlaylists(context.Background())
	if err != nil {
		t.Fatalf("CurrentUsersPlaylists: %v", err)
	}

	if len(page.Playlists) == 0 {
		t.Fatal("no playlists decoded")
	}

	if first := page.Playlists[0]; first.Name != "Core" || first.Tracks.Total != 3 {
		t.Errorf("first playlist = %q with %d tracks, want \"Core\" with 3", first.Name, first.Tracks.Total)
	}
}
