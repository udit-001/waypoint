package db

// ListOpts controls job listing dispatch across search, filter, and default.
type ListOpts struct {
	Search   string
	Status   string
	Category string
}

// ListJobs dispatches to SearchJobs, FilterJobs, or GetJobs based on opts.
// Single source of truth for the dispatch — both CLI and server call this
// instead of duplicating the switch.
func ListJobs(s Store, opts ListOpts) ([]Job, error) {
	if opts.Search != "" {
		return s.SearchJobs(opts.Search, opts.Status, opts.Category)
	}
	if opts.Status != "" || opts.Category != "" {
		return s.FilterJobs(opts.Status, opts.Category)
	}
	return s.GetJobs()
}
