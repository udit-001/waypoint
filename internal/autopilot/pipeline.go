package autopilot

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// pipeline wires the sweep producer to the detail+curate consumer
// (WP-189): per-source sweep results flow through a size-1 channel, so
// the cycle enriches and curates one source's batch while the producer
// keeps scraping the next. The ledger backlog rides the same channel as
// the final batch.
//
// Ordering tradeoff: with per-source batches the cycle-wide caps no
// longer see one merged, date-sorted slice (WP-190's ApplyCap sorted the
// whole merged list), so "newest first" holds within a batch only. A
// posting any cap drops stays "new" in the ledger — the next cycle's
// backlog picks it up.
//
// Shared state: the atomic escalation counter IS the cap mechanism (the
// budget itself stays lock-free); the mutex guards only the escalated-URL
// map. The stop signal is the derived context the consumer cancels. The
// producer never touches the shared runRecord — it buffers its sweep
// evidence and lands it after the consumer's loop has ended.
type pipeline struct {
	batches chan []scraper.Result

	// escalated counts postings handed to detail+curate this cycle.
	escalated atomic.Int32

	mu            sync.Mutex
	escalatedURLs map[string]bool

	cancel       context.CancelFunc
	consumerDone chan struct{}
	producerDone chan struct{}
}

func newPipeline(cancel context.CancelFunc) *pipeline {
	return &pipeline{
		batches:       make(chan []scraper.Result, 1),
		escalatedURLs: map[string]bool{},
		cancel:        cancel,
		consumerDone:  make(chan struct{}),
		producerDone:  make(chan struct{}),
	}
}

// produce scrapes the selected sources, pushing each source's new
// postings as one batch, then the ledger backlog as the final batch.
// Sweep evidence lands in buf — a buffer, not the shared record — because
// the consumer writes the real record concurrently; it merges only after
// the consumer's loop has ended.
func (p *pipeline) produce(ctx context.Context, cfg CycleConfig, rec, buf *runRecord, sel *SourceSelection, sweepGated bool) {
	defer close(p.producerDone)

	swept := p.sweepAll(ctx, cfg, buf, sel, sweepGated)
	buf.PostingsNew = len(swept)

	// The backlog's exclude set carries this cycle's swept URLs — some
	// may still be sitting in the channel unprocessed, status unchanged —
	// plus whatever the consumer already escalated.
	exclude := make(map[string]bool, len(swept))
	for url := range swept {
		exclude[url] = true
	}
	for url := range p.escalatedSoFar() {
		exclude[url] = true
	}
	if batch := stageBacklog(ctx, cfg, buf, exclude); len(batch) > 0 {
		p.push(ctx, batch)
	}

	// The consumer owns the shared record while it drains the channel;
	// its loop ending (channel closed) is the handoff back.
	close(p.batches)
	<-p.consumerDone
	rec.mergeSweep(buf)
}

// sweepAll scrapes every planned source, pushing each one's new postings
// as a batch. A scraper panic is contained here — the producer goroutine
// has no outer recover, and a panicked sweep must still close the channel
// so the consumer's range ends.
func (p *pipeline) sweepAll(ctx context.Context, cfg CycleConfig, buf *runRecord, sel *SourceSelection, gated bool) (swept map[string]bool) {
	swept = map[string]bool{}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: sweep panicked: %v", r)
			buf.stageError("panic: %v", r)
		}
	}()
	if gated {
		return swept
	}

	disabled := disabledSet(cfg)
	// Plan first: the skipped rows are recorded for every scraper (they
	// carry the selection's decision to the user), while the progress
	// counter "k/n" counts only the sources this cycle actually sweeps.
	type sweepSource struct {
		s    scraper.Scraper
		name string
	}
	var plan []sweepSource
	for _, s := range cfg.Scrapers {
		name := s.Name()
		if reason, skip := skipReason(name, sel, disabled); skip {
			row := buf.outcome(name)
			row.Skipped = true
			row.Reason = reason
			continue
		}
		plan = append(plan, sweepSource{s: s, name: name})
	}
	for k, item := range plan {
		cfg.progress("sweep", fmt.Sprintf("%d/%d", k+1, len(plan)), item.s.Source())

		// Politeness: jittered delay between sources.
		jitter := time.Duration(2000+rand.Intn(1000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return swept
		case <-after(jitter):
		}

		// 30s timeout per source; the ctx is the consumer's stop signal,
		// so a spent budget aborts an in-flight scrape.
		srcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		results, err := item.s.Search(srcCtx, scraper.SearchOpts{
			Limit:  50,
			JobAge: cfg.Recency,
		})
		cancel()

		if err != nil {
			log.Printf("autopilot: sweep %s failed: %v", item.name, err)
			row := buf.outcome(item.name)
			row.Errored++
			buf.stageError("sweep %s: %v", item.name, err)
			continue
		}
		row := buf.outcome(item.name)
		row.Scraped = len(results)

		// Dedup against existing postings and jobs. Postings from earlier
		// batches are already in the ledger (each batch is added before
		// its push), so cross-source duplicates fall out here.
		var batch []scraper.Result
		for _, r := range results {
			if swept[r.URL] {
				continue
			}
			seen, _ := cfg.Store.HasPosting(r.URL)
			if seen {
				continue
			}
			tracked, _ := cfg.Store.JobExists(r.URL)
			if tracked {
				continue
			}
			r.Source = item.name
			batch = append(batch, r)
			row.New++
		}
		if len(batch) == 0 {
			continue
		}
		if err := cfg.Store.AddPostings(batch); err != nil {
			log.Printf("autopilot: sweep add postings: %v", err)
			buf.stageError("sweep: add postings: %v", err)
		}
		for _, r := range batch {
			swept[r.URL] = true
		}
		p.push(ctx, batch)
	}
	return swept
}

// push hands a batch to the consumer. It refuses when the pipeline is
// stopping — the batch's postings stay "new" in the ledger for the next
// cycle's backlog.
func (p *pipeline) push(ctx context.Context, batch []scraper.Result) {
	select {
	case p.batches <- batch:
	case <-ctx.Done():
	}
}

// consume drains the batches: per batch it checks the cycle budget, then
// runs detail and prefilter+curate on what survives the escalation
// bounds. It runs on Run's goroutine and owns the shared runRecord until
// its loop ends. On any stop it keeps receiving until the channel closes,
// so a producer blocked on push can always land its batch and exit.
func (p *pipeline) consume(ctx, pctx context.Context, cfg CycleConfig, rec *runRecord, gate func(string) bool) {
	draining := false
	wallRecorded := false
	expire := func() {
		if !wallRecorded {
			// Detail and curate always stop together in the consumer; the
			// once-guard keeps a multi-batch drain from recording them twice.
			wallRecorded = true
			gate("detail")
			gate("curate")
		}
		p.cancel()
		draining = true
	}
	for !draining {
		// The wall budget must also stop a consumer that is only WAITING:
		// the producer may be parked on a slow source and no batch will
		// arrive to carry the check.
		var expiry <-chan time.Time
		var timer *time.Timer
		if !cfg.Budget.Deadline.IsZero() {
			if d := time.Until(cfg.Budget.Deadline); d > 0 {
				timer = time.NewTimer(d)
				expiry = timer.C
			} else {
				expired := make(chan time.Time, 1)
				expired <- time.Now() // deadline already behind us: fire now
				expiry = expired
			}
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-expiry:
			if timer != nil {
				timer.Stop()
			}
			expire()
		case batch, ok := <-p.batches:
			if timer != nil {
				timer.Stop()
			}
			if !ok {
				return
			}
			if cfg.Budget.Expired() {
				expire()
				continue
			}
			if !p.processBatch(pctx, cfg, rec, batch) {
				// Exhausted: a cycle-wide bound is full. Not an error —
				// the postings dropped stay "new" for the next cycle.
				p.cancel()
				draining = true
			}
		}
	}
	for range p.batches { // drain: unblock the producer's pending push
	}
}

// processBatch applies the cycle-wide escalation bounds to one batch and
// runs detail + prefilter+curate on what survives. Returns false when the
// cycle is exhausted — nothing more may escalate.
func (p *pipeline) processBatch(ctx context.Context, cfg CycleConfig, rec *runRecord, batch []scraper.Result) bool {
	remaining := p.allowance(cfg) - int(p.escalated.Load())
	if remaining < 0 {
		remaining = 0
	}
	if len(batch) > remaining {
		_, name := p.bound(cfg)
		sort.SliceStable(batch, func(i, k int) bool { return batch[i].Date > batch[k].Date })
		log.Printf("autopilot: cycle bounds — escalating newest %d of %d (%s)", remaining, len(batch), name)
		batch = batch[:remaining]
	}
	if len(batch) == 0 {
		return false
	}

	// Stage 3: Detail — enrich postings with full job body.
	stageDetail(ctx, cfg, batch)
	// Stages 4+5: Prefilter + Curate — per-posting commit.
	stagePrefilterCurate(ctx, cfg, batch, rec)

	p.mu.Lock()
	for _, r := range batch {
		p.escalatedURLs[r.URL] = true
	}
	p.mu.Unlock()
	p.escalated.Add(int32(len(batch)))
	return true
}

// bound returns the tightest active escalation bound and its name: the
// user-facing Limit (a cycle-wide cap on what escalates, keeping its
// per-run meaning), the budget's scored cap, and — on the degraded path —
// the unscored cap. 0 means no bound.
func (p *pipeline) bound(cfg CycleConfig) (int, string) {
	limit, name := 0, "none"
	consider := func(n int, label string) {
		if n > 0 && (limit == 0 || n < limit) {
			limit, name = n, label
		}
	}
	consider(cfg.Limit, "run limit")
	consider(cfg.Budget.MaxScored, "scored cap")
	if cfg.ZenClient == nil {
		consider(UnscoredCapPerCycle, "unscored cap")
	}
	if limit == 0 {
		return 1<<31 - 1, name
	}
	return limit, name
}

// allowance is how many postings may still escalate this cycle.
func (p *pipeline) allowance(cfg CycleConfig) int {
	limit, _ := p.bound(cfg)
	return limit
}

// escalatedSoFar copies the consumer's escalated-URL set: notify reads it
// at the end, and the producer's backlog build checks it mid-run.
func (p *pipeline) escalatedSoFar() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make(map[string]bool, len(p.escalatedURLs))
	for url := range p.escalatedURLs {
		out[url] = true
	}
	return out
}

// mergeSweep lands the producer's buffered sweep evidence in the shared
// record. Called only after the consumer's loop has ended — the two write
// the same per-source rows and error list, just never at the same time.
func (r *runRecord) mergeSweep(buf *runRecord) {
	for source, row := range buf.outcomes {
		dst := r.outcome(source)
		dst.Scraped += row.Scraped
		dst.New += row.New
		dst.Errored += row.Errored
		dst.Shortlisted += row.Shortlisted
		dst.Dismissed += row.Dismissed
		if row.Skipped {
			dst.Skipped = true
		}
		if row.Reason != "" {
			dst.Reason = row.Reason
		}
	}
	r.StageErrors = append(r.StageErrors, buf.StageErrors...)
	r.PostingsNew = buf.PostingsNew
}
