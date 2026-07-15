package db

import "time"

// expiringSoonDays is the window for the --expiring-soon filter.
const expiringSoonDays = 7

// ListOpts controls job listing dispatch across search, filter, and default.
type ListOpts struct {
	Search       string
	Status       string
	Category     string
	Expired      bool
	ExpiringSoon bool
}

// ListJobs dispatches to SearchJobs, FilterJobs, or GetJobs based on opts,
// then applies Go-level deadline filtering (--expired / --expiring-soon).
//
// Deadline filtering uses string comparison because dates in the jobs table
// are normalized to YYYY-MM-DD (by dates.NormalizeDate at promote/add time),
// and ISO dates sort correctly as strings.
func ListJobs(s Store, opts ListOpts) ([]Job, error) {
	jobs, err := listJobsFromStore(s, opts)
	if err != nil {
		return nil, err
	}

	if !opts.Expired && !opts.ExpiringSoon {
		return jobs, nil
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	soonCutoff := now.AddDate(0, 0, expiringSoonDays).Format("2006-01-02")

	filtered := jobs[:0]
	for _, j := range jobs {
		if j.Date == "" {
			continue
		}
		isExpired := j.Date < today
		isExpiringSoon := j.Date >= today && j.Date <= soonCutoff
		if (opts.Expired && isExpired) || (opts.ExpiringSoon && isExpiringSoon) {
			filtered = append(filtered, j)
		}
	}
	return filtered, nil
}

func listJobsFromStore(s Store, opts ListOpts) ([]Job, error) {
	if opts.Search != "" {
		return s.SearchJobs(opts.Search, opts.Status, opts.Category)
	}
	if opts.Status != "" || opts.Category != "" {
		return s.FilterJobs(opts.Status, opts.Category)
	}
	return s.GetJobs()
}
