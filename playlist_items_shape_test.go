package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// February 2026 shape: content under "item", "track" kept as a deprecated
// alias that may be missing fields.
const playlistItemsNewShape = `{
  "items": [
    {"added_at": "2026-03-01T00:00:00Z",
     "item":  {"type": "track", "id": "a", "external_ids": {"isrc": "ISRC-A"}},
     "track": {"type": "track", "id": "a"}},
    {"item": {"type": "episode", "id": "e", "name": "Episode"}},
    {"track": {"type": "track", "id": "legacy", "external_ids": {"isrc": "ISRC-L"}}},
    {"item": null, "track": null}
  ],
  "next": ""
}`

func TestPlaylistItemDecodesItemWithTrackFallback(t *testing.T) {
	var page PlaylistItemPage
	if err := json.Unmarshal([]byte(playlistItemsNewShape), &page); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(page.Items) != 4 {
		t.Fatalf("got %d items, want 4", len(page.Items))
	}

	first := page.Items[0]
	if first.Track.Track == nil || first.Track.Track.ID != "a" || first.Track.Track.ExternalIDs.ISRC != "ISRC-A" {
		t.Errorf("item 0 = %+v, want track a with ISRC-A taken from \"item\"", first.Track.Track)
	}
	if first.AddedAt != "2026-03-01T00:00:00Z" {
		t.Errorf("item 0 AddedAt = %q, want it decoded alongside \"item\"", first.AddedAt)
	}

	if ep := page.Items[1].Track.Episode; ep == nil || ep.ID != "e" {
		t.Errorf("item 1 episode = %+v, want episode e", ep)
	}

	if tr := page.Items[2].Track.Track; tr == nil || tr.ID != "legacy" || tr.ExternalIDs.ISRC != "ISRC-L" {
		t.Errorf("item 2 = %+v, want legacy track from \"track\"", tr)
	}

	if unavailable := page.Items[3].Track; unavailable.Track != nil || unavailable.Episode != nil {
		t.Errorf("item 3 = %+v, want both nil for unavailable content", unavailable)
	}
}

func TestFullPlaylistDecodesItemsWithTracksFallback(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "new items key",
			body: `{"id":"p","name":"Mix","items":{"total":1,"items":[{"item":{"id":"t1","external_ids":{"isrc":"I1"}}}]}}`,
		},
		{
			name: "legacy tracks key",
			body: `{"id":"p","name":"Mix","tracks":{"total":1,"items":[{"track":{"id":"t1","external_ids":{"isrc":"I1"}}}]}}`,
		},
		{
			name: "items wins over tracks",
			body: `{"id":"p","name":"Mix","tracks":{"total":9,"items":[]},"items":{"total":1,"items":[{"item":{"id":"t1","external_ids":{"isrc":"I1"}}}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p FullPlaylist
			if err := json.Unmarshal([]byte(tt.body), &p); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if p.ID != "p" || p.Name != "Mix" {
				t.Errorf("playlist = %s %q, want p \"Mix\" (embedded fields must still decode)", p.ID, p.Name)
			}
			if p.Tracks.Total != 1 || len(p.Tracks.Tracks) != 1 {
				t.Fatalf("Tracks = %+v, want 1 track", p.Tracks)
			}
			if tr := p.Tracks.Tracks[0].Track; tr.ID != "t1" || tr.ExternalIDs.ISRC != "I1" {
				t.Errorf("track = %s %q, want t1 I1", tr.ID, tr.ExternalIDs.ISRC)
			}
		})
	}
}

func TestGetPlaylistItemsUsesItemsEndpoint(t *testing.T) {
	client, server := testClientString(http.StatusOK, playlistItemsNewShape, func(req *http.Request) {
		if req.URL.Path != "/playlists/pl/items" {
			t.Errorf("path = %s, want /playlists/pl/items", req.URL.Path)
		}
	})
	defer server.Close()

	page, err := client.GetPlaylistItems(context.Background(), "pl")
	if err != nil {
		t.Fatalf("GetPlaylistItems: %v", err)
	}
	if page.Items[0].Track.Track == nil || page.Items[0].Track.Track.ExternalIDs.ISRC != "ISRC-A" {
		t.Errorf("first item = %+v, want ISRC-A", page.Items[0].Track.Track)
	}
}
