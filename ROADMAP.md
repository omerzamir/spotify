# Roadmap: v3

`v2.5.0` moved this fork onto Spotify's February 2026 Web API. `v3` is a
code-quality release: a typed, `any`-free API on modern Go, one request
pipeline, iterator-based paging, and an honest surface that only exposes
endpoints Spotify still serves.

## Goals

- No `interface{}`/`any` as a type anywhere, public or internal.
- One request pipeline. `Client.get` and `Client.execute` duplicate retry
  logic today and have diverged (only `execute` rewinds bodies; neither caps
  attempts; a 202 retries forever).
- Paging with `iter.Seq2` instead of reflection (`page.go`, `NextPage`).
- Errors usable with `errors.AsType`, exposing Spotify's `reason`
  (e.g. `QUOTA_EXCEEDED`) and `Retry-After`.
- Only `golang.org/x/oauth2` as a dependency (drop testify), and a clean
  strict `golangci-lint` run with no "only new issues" escape hatch.

## Non-goals

- Wrapping every new endpoint on day one, undocumented endpoints, code
  generation, or reworking `auth/`.

## Decisions

- **Go 1.27 or newer.** v3 is expected after Go 1.28 ships, so 1.27 is the
  oldest supported release. Generic methods (1.27) are allowed.
- **One flat package**, as today. Subpackages would churn every import for
  little gain.
- **v2 stays maintained** on a `v2` branch for endpoint-compatibility and
  security fixes until six months after `v3.0.0`.
- **Retries live in the transport.** A `RetryTransport` (`http.RoundTripper`)
  honours `Retry-After`, caps attempts, rewinds request bodies and skips
  bodies it cannot rewind. The `Client` has no retry loop of its own.
- **Removed batch endpoints fall back per item automatically.** When a batch
  endpoint that Spotify removed for Development Mode (`GET /artists`,
  `/tracks`, `/albums`) fails with 403, 404 or 410, the client fetches the
  items one by one, sequentially by default so it does not trip per-account
  quotas; `WithBatchConcurrency(n)` opts into parallelism.
- **`GetPlaylistTracks` stays (deprecated).** It is the only way to read
  playlists the user neither owns nor collaborates on while Spotify still
  serves `/tracks` to an app. Every other legacy method whose replacement is
  equivalent is removed.

## Breaking changes

| Change | Migration |
|---|---|
| Module path `github.com/omerzamir/spotify/v3` | Rewrite imports. |
| `Client.NextPage`/`PreviousPage` removed | Range over the iterators, or call `PageAt(ctx, page.Next)`. |
| `Error` becomes `*APIError` | `errors.AsType[*spotify.APIError](err)`; `Status`, `Message`, `Reason`, `RetryAfter`, `Retryable()`. |
| `WithRetry(bool)` removed | `WithHTTPClient` with a `RetryTransport`, or `WithRetryPolicy(RetryPolicy{...})`. |
| Endpoints Spotify shut down in November 2024 removed (recommendations, audio features/analysis, related artists, featured playlists, category playlists) | None; Spotify no longer serves them. |
| Deprecated v2 methods with an equivalent replacement removed | Use the method named in the v2 `Deprecated:` note. |
| Batch getters return `Batch[T]` | Check `Batch.Errs` for items that could not be fetched. |
| `FullPlaylist` no longer embeds `SimplePlaylist`; both expose `Items` | Rename `.Tracks` to `.Items`; shared fields move to `PlaylistSummary`. |
| `ID.String` gets a value receiver; `Image.Download(ctx, w)` takes a context | Mostly source-compatible; pass a context. |
| testify removed from tests | Nothing for users. |

## API sketch

```go
// Paging.
type Page[T any] struct {
	Items  []T
	Total  int
	Limit  int
	Offset int
	Next   string
}

func (c *Client) PageAt[T any](ctx context.Context, url string) (*Page[T], error)

func (c *Client) AllPlaylistItems(ctx context.Context, id ID, opts ...RequestOption) iter.Seq2[PlaylistItem, error]
func (c *Client) AllSavedTracks(ctx context.Context, opts ...RequestOption) iter.Seq2[SavedTrack, error]

// Errors.
type APIError struct {
	Status     int
	Message    string
	Reason     string        // e.g. "QUOTA_EXCEEDED", "NO_ACTIVE_DEVICE"
	RetryAfter time.Duration // zero when absent
}

func (e *APIError) Error() string
func (e *APIError) Retryable() bool // 429 and 5xx

// Transport.
type RetryPolicy struct {
	MaxAttempts   int
	MaxRetryAfter time.Duration // never wait longer than this for one retry
}

func NewRetryTransport(base http.RoundTripper, policy RetryPolicy) http.RoundTripper

// Batches.
type Batch[T any] struct {
	Items []*T          // same order as the requested IDs; nil when not found
	Errs  map[ID]error  // per-item failures from the per-item fallback
}
```

## Phases

Each phase is one pull request that ends green on `go vet`,
`go test -race` and the strict `golangci-lint` config.

1. **Foundation.** Go 1.27; drop testify; strict lint config; fix the
   existing findings, including the examples. *Done when `go.mod` requires only
   `golang.org/x/oauth2` and CI lint has no exceptions.*
2. **Pipeline.** One generic `do[T]` request helper, `*APIError`,
   `RetryTransport`; retry tests with `testing/synctest`. *Done when non-test
   code has no `interface{}`/`any` (checked in CI) and 429, 202,
   context-cancel and body-rewind cases are covered.*
3. **Paging.** `Page[T]`, `PageAt`, iterators; cursor pages unified.
   *Done when the package no longer imports `reflect`.*
4. **Surface.** Remove dead endpoints and replaced methods; `Batch[T]` with
   per-item fallback; playlist type split with golden-file tests for both the
   old and new JSON; `MIGRATING.md`. *Done when `go doc` shows only the
   intentionally kept deprecation.*
5. **Release.** `/v3` module path, `v3.0.0-rc.1`, an `apidiff` report in the
   pull request, CI that builds every example.
