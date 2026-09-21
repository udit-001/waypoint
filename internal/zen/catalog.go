package zen

// The curated free-models catalog (WP-161): a stale-while-revalidate cache
// over the curated free-models.json list, served by GET /api/zen/models and
// read by the picker (WP-163) and the connector (WP-162).
//
// Design mirrors the pi extension cache (shared seam decision, spec
// WP-160): a fresh entry answers instantly; a stale entry KEEPS answering
// (last-known-good) while one shared background conditional revalidation
// refreshes it; failures keep the stale entry and advance the timer so a
// broken CDN never becomes a busy retry loop. jsdelivr honors
// If-None-Match, so an unchanged list revalidates as a body-less 304.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// FreeModelsCDNURL is the production curated list: the free-models data
// branch served via jsdelivr (1h edge cache, honors conditional requests).
const FreeModelsCDNURL = "https://cdn.jsdelivr.net/gh/udit-001/pi-zen@data/free-models.json"

const (
	catalogTTL       = time.Hour            // matches the CDN edge cache
	catalogFetchTO   = 8 * time.Second      // bounded fetches; never hang a request
	defaultFamilyAPI = "openai-completions" // curated entries omit api for the completions family
)

// Model is one curated free model as callers see it: the id, the friendly
// display name, and the API family that endpoint routing needs (WP-162).
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	API  string `json:"api"`
}

// CatalogConfig shapes a Catalog. Accept dependencies, don't create them:
// URL/HTTPClient/Now are injection points for tests; zero values pick the
// production defaults.
type CatalogConfig struct {
	URL        string        // curated list URL; empty = FreeModelsCDNURL
	TTL        time.Duration // freshness window; 0 = 1h
	HTTPClient *http.Client
	Now        func() time.Time
}

// catalogEntry is the last-known-good list and its freshness deadline.
type catalogEntry struct {
	expiresAt time.Time
	models    []Model
	version   string // curated opencodeVersion — the wire's UA floor (WP-162)
}

// validators carry the response's conditional-request stamps (ETag /
// Last-Modified). Captured header-only from the wire response, before the
// body is parsed, so a corrupt body never poisons revalidation.
type validators struct {
	etag         string
	lastModified string
}

// Catalog serves the curated free-models list stale-while-revalidate. One
// per daemon process. The zero value is not usable; use NewCatalog.
//
// Interface: Models(ctx). Everything else — freshness, conditional
// revalidation, single-flight, failure survival — is implementation.
type Catalog struct {
	url    string
	ttl    time.Duration
	client *http.Client
	now    func() time.Time

	mu           sync.Mutex
	entry        *catalogEntry
	validators   validators
	revalidating bool

	// Shared in-flight cold fetch (single-flight on the cold path).
	coldFetching bool
	coldDone     chan struct{}
	coldModels   []Model
	coldErr      error
}

// NewCatalog builds a Catalog with the given configuration.
func NewCatalog(cfg CatalogConfig) *Catalog {
	if cfg.URL == "" {
		cfg.URL = FreeModelsCDNURL
	}
	if cfg.TTL <= 0 {
		cfg.TTL = catalogTTL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: catalogFetchTO}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Catalog{
		url:    cfg.URL,
		ttl:    cfg.TTL,
		client: cfg.HTTPClient,
		now:    cfg.Now,
	}
}

// Models returns the curated free-models list, stale-while-revalidate.
//
// A fresh cache answers without a network call. A stale cache serves the
// last-known-good list immediately and revalidates once in the background.
// A cold cache fetches synchronously; if that fetch fails there is no
// last-known-good to serve, so it returns an error — callers decide the
// fallback (the handler answers 502 and the web dropdown degrades to the
// saved selection). Concurrent cold callers share one fetch.
func (c *Catalog) Models(ctx context.Context) ([]Model, error) {
	c.mu.Lock()
	if c.entry != nil {
		models := c.entry.models
		stale := !c.entry.expiresAt.After(c.now())
		startRevalidation := stale && !c.revalidating
		if startRevalidation {
			c.revalidating = true
		}
		c.mu.Unlock()
		if startRevalidation {
			go c.revalidate()
		}
		return models, nil
	}
	if c.coldFetching {
		// Share the in-flight cold fetch instead of stampeding the CDN.
		done := c.coldDone
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		models, err := c.coldModels, c.coldErr
		c.mu.Unlock()
		return models, err
	}
	c.coldFetching = true
	c.coldDone = make(chan struct{})
	c.mu.Unlock()

	models, err := c.fetch(ctx)
	c.mu.Lock()
	c.coldModels, c.coldErr = models, err
	c.coldFetching = false
	close(c.coldDone)
	c.mu.Unlock()
	return models, err
}

// Version returns the curated opencodeVersion from the last-known-good
// list ("" before the first fetch). Passive: it never makes a network
// call — the wire's UA gate (WP-162) reads it once Models has warmed the
// cache.
func (c *Catalog) Version() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entry == nil {
		return ""
	}
	return c.entry.version
}

// fetch performs the cold-cache synchronous fetch: full GET, then the
// shared validated install. Errors surface to the caller (nothing to
// serve).
func (c *Catalog) fetch(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("zen catalog: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zen catalog: fetch: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		// A 304 on a cold cache means the server thinks we have it and we
		// don't — treat as an error, there is nothing to serve.
		return nil, fmt.Errorf("zen catalog: unexpected 304 on cold cache")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("zen catalog: HTTP %d", res.StatusCode)
	}
	return c.install(res)
}

// revalidate is the single shared background revalidation: a conditional
// request with the stored validators. 304 refreshes only the freshness
// timer; 200 installs the new list; any failure keeps the stale entry and
// still advances the timer (a broken CDN must not become a busy retry
// loop). The caller serialized access via the revalidating flag.
func (c *Catalog) revalidate() {
	defer func() {
		c.mu.Lock()
		c.revalidating = false
		c.mu.Unlock()
	}()

	headers := map[string]string{"Accept": "application/json"}
	c.mu.Lock()
	if c.validators.etag != "" {
		headers["If-None-Match"] = c.validators.etag
	} else if c.validators.lastModified != "" {
		headers["If-Modified-Since"] = c.validators.lastModified
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), catalogFetchTO)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		c.keepStale()
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := c.client.Do(req)
	if err != nil {
		c.keepStale()
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		c.mu.Lock()
		if c.entry != nil {
			c.entry.expiresAt = c.now().Add(c.ttl)
		}
		c.mu.Unlock()
		return
	}
	if res.StatusCode != http.StatusOK {
		c.keepStale()
		return
	}
	if _, err := c.install(res); err != nil {
		c.keepStale()
	}
}

// install decodes, validates, and installs a 200 response as the
// last-known-good entry, capturing its conditional-request validators.
// Shared by the cold fetch and background revalidation — their policies
// differ only in error handling, which each caller keeps.
func (c *Catalog) install(res *http.Response) ([]Model, error) {
	v := captureValidators(res)
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("zen catalog: read body: %w", err)
	}
	models, version, err := parseFreeModels(raw)
	if err != nil {
		return nil, fmt.Errorf("zen catalog: %w", err)
	}

	c.mu.Lock()
	c.entry = &catalogEntry{expiresAt: c.now().Add(c.ttl), models: models, version: version}
	c.validators = v
	c.mu.Unlock()
	return models, nil
}

// keepStale advances the freshness timer without touching the list: the
// stale entry stays servable and the next revalidation waits a full TTL.
func (c *Catalog) keepStale() {
	c.mu.Lock()
	if c.entry != nil {
		c.entry.expiresAt = c.now().Add(c.ttl)
	}
	c.mu.Unlock()
}

// captureValidators reads the conditional-request stamps off a 200
// response header. Only one of the two is ever sent; Last-Modified covers
// intermediaries that strip ETags.
func captureValidators(res *http.Response) validators {
	return validators{
		etag:         res.Header.Get("ETag"),
		lastModified: res.Header.Get("Last-Modified"),
	}
}

// parseFreeModels decodes and validates the curated list. The CDN's
// defaultModel field is deliberately NOT consumed (spec WP-160, decision
// 2026-09-18): nothing in waypoint adopts it.
func parseFreeModels(raw []byte) ([]Model, string, error) {
	var file struct {
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			API  string `json:"api"`
		} `json:"models"`
		OpencodeVersion string `json:"opencodeVersion"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, "", fmt.Errorf("parse curated list: %w", err)
	}
	if len(file.Models) == 0 {
		return nil, "", fmt.Errorf("curated list has no models")
	}
	models := make([]Model, len(file.Models))
	for i, m := range file.Models {
		if m.ID == "" {
			return nil, "", fmt.Errorf("curated list entry %d has no id", i)
		}
		models[i] = Model{ID: m.ID, Name: m.Name, API: m.API}
		if models[i].Name == "" {
			models[i].Name = models[i].ID
		}
		if models[i].API == "" {
			models[i].API = defaultFamilyAPI
		}
	}
	return models, file.OpencodeVersion, nil
}

// Family returns the endpoint family for a model from the last-known-good
// list. Unknown models default to the chat-completions family — the family
// every other curated model uses. Passive: no network.
func (c *Catalog) Family(model string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entry != nil {
		for _, m := range c.entry.models {
			if m.ID == model {
				return m.API
			}
		}
	}
	return defaultFamilyAPI
}

// sharedCatalog is the process-wide catalog for client construction sites
// that don't receive one (the CLI/daemon paths): one SWR cache per
// process, lazily built against the production CDN.
var (
	sharedCatalogOnce sync.Once
	sharedCatalog     *Catalog
)

// SharedCatalog returns the process-wide catalog. Clients attach it via
// Config.Catalog so the UA version and model→family routing come from the
// curated metadata.
func SharedCatalog() *Catalog {
	sharedCatalogOnce.Do(func() {
		sharedCatalog = NewCatalog(CatalogConfig{})
		// Warm in the background: the first client request wants the fresh
		// UA version and family routing without paying the CDN latency, and
		// the SWR cache makes the warm-up free for the picker later.
		go func() {
			_, _ = sharedCatalog.Models(context.Background())
		}()
	})
	return sharedCatalog
}
