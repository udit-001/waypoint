package autopilot

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/udit-001/waypoint/internal/boards"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/detail"
	"github.com/udit-001/waypoint/internal/mdsvc"
	"github.com/udit-001/waypoint/internal/scraper"
)

// BuildDetailChain constructs the detail chain with tiers 1-3.5 wired
// (board provider, scraper detailer, markdown conversion services). The
// autopilot cycle adds the direct HTTP fetcher and the shared Exa client;
// the CLI detail command uses it as-is. One construction site so the tier
// order can never drift between the two consumers.
func BuildDetailChain() *detail.Chain {
	return detail.NewChain(&boardDetailer{}, scraperDetailerFor(), nil, mdsvc.New(MdsvcStatePath()), nil)
}

// MdsvcStatePath returns the markdown-service quota-state path inside
// the waypoint data dir.
func MdsvcStatePath() string {
	dir := config.DefaultDataDir()
	if cfg, err := config.Load(); err == nil && cfg != nil && cfg.DataDir != "" {
		dir = cfg.DataDir
	}
	return filepath.Join(dir, "mdsvc-state.json")
}

// scraperDetailerFor adapts the linkedin scraper's Detailer, if registered.
func scraperDetailerFor() detail.ScraperDetailer {
	if s, ok := scraper.Get("linkedin"); ok {
		if d, ok := s.(scraper.Detailer); ok {
			return &chainScraperDetailer{d: d}
		}
	}
	return nil
}

// FindBoardURL infers the board's careers URL from a posting URL by
// looking up registered boards in the config. Exported so the CLI
// detail command shares one board-inference site.
func FindBoardURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()

	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	bf, err := config.LoadBoards(cfg)
	if err != nil {
		return ""
	}
	for _, e := range bf.Boards {
		bu, err := url.Parse(e.URL)
		if err != nil {
			continue
		}
		if bu.Hostname() == host {
			return e.URL
		}
		// Also check subdomains (e.g. boards.greenhouse.io vs job-boards.greenhouse.io).
		if strings.HasSuffix(host, "."+bu.Hostname()) || strings.HasSuffix(bu.Hostname(), "."+host) {
			return e.URL
		}
	}
	return ""
}

type boardDetailer struct{}

func (c *boardDetailer) Detail(ctx context.Context, boardURL, id string) (scraper.Result, error) {
	if boardURL == "" {
		return scraper.Result{}, fmt.Errorf("no board URL")
	}
	b := boards.Board{URL: boardURL}
	p, _, err := boards.DetectProvider(b)
	if err != nil {
		return scraper.Result{}, fmt.Errorf("detect provider: %w", err)
	}
	return p.Detail(ctx, b, id)
}

// chainScraperDetailer wraps a scraper.Detailer. Implements
// detail.ScraperDetailer.
type chainScraperDetailer struct {
	d scraper.Detailer
}

func (c *chainScraperDetailer) Detail(ctx context.Context, id string) (*scraper.Result, error) {
	return c.d.Detail(ctx, id)
}
