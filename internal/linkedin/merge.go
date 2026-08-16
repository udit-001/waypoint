package linkedin

import "strings"

// MergeSummary describes what a merge changed — the UI renders it as an
// Added/Updated/Kept diff. Kept counts existing entries that remain in the
// profile unchanged (no LinkedIn match, or a match that changed nothing); the
// diff preview only lists what is new or changed, so kept entries are counts.
type MergeSummary struct {
	SkillsAdded       []string     `json:"skillsAdded"`
	ExperienceAdded   []Experience `json:"experienceAdded"`
	ExperienceUpdated []Experience `json:"experienceUpdated"`
	ExperienceKept    int          `json:"experienceKept"`
	EducationAdded    []Education  `json:"educationAdded"`
	EducationUpdated  []Education  `json:"educationUpdated"`
	EducationKept     int          `json:"educationKept"`
}

// Merge combines a fetched LinkedIn profile into the current one with update
// semantics — it never deletes:
//
//   - Scalars (name, headline, location): taken from the fetched profile when
//     present — LinkedIn is the source of truth for the sync.
//   - Skills: union — current skills keep their order, fetched skills are
//     appended when not already present (case-insensitive match).
//   - Experience: matched by (title, company). A matched entry gets its dates
//     replaced from LinkedIn (when the fetched entry has a start date — an
//     empty fetched end means "present", a valid value) and its description
//     replaced when LinkedIn has one. Unmatched fetched entries are appended;
//     existing entries with no LinkedIn match are kept.
//   - Education: matched by institution, same rules; degree and description
//     are replaced only when LinkedIn has a value.
//
// The returned merged Profile's Doc() emits the PATCH document. Merging into
// an empty profile yields the fetched profile unchanged — every entry lands in
// the "added" lists — so the seed and update flows share one code path.
func Merge(current, fetched Profile) (Profile, MergeSummary) {
	out := current
	var sum MergeSummary

	if fetched.Name != "" {
		out.Name = fetched.Name
	}
	if fetched.Headline != "" {
		out.Headline = fetched.Headline
	}
	if fetched.Location != "" {
		out.Location = fetched.Location
	}

	out.Skills, sum.SkillsAdded = mergeSkills(current.Skills, fetched.Skills)
	out.Exp, sum.ExperienceAdded, sum.ExperienceUpdated, sum.ExperienceKept = mergeExperience(current.Exp, fetched.Exp)
	out.Edu, sum.EducationAdded, sum.EducationUpdated, sum.EducationKept = mergeEducation(current.Edu, fetched.Edu)

	return out, sum
}

// mergeSkills unions two skill lists: current order preserved, fetched skills
// appended when not already present (case-insensitive). Returns the merged
// list and the appended subset.
func mergeSkills(cur, fetched []string) ([]string, []string) {
	seen := make(map[string]bool, len(cur)+len(fetched))
	out := make([]string, 0, len(cur)+len(fetched))
	for _, s := range cur {
		if k := strings.ToLower(strings.TrimSpace(s)); k != "" && !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	var added []string
	for _, s := range fetched {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
		added = append(added, s)
	}
	return out, added
}

// mergeExperience merges two experience lists. Current entries keep their
// order; a matched current entry is updated in place from the fetched entry
// that consumes it; unmatched current entries are kept and counted;
// unconsumed fetched entries are appended as "added".
func mergeExperience(cur, fetched []Experience) (out, added, updated []Experience, kept int) {
	used := make([]bool, len(fetched))
	out = make([]Experience, 0, len(cur)+len(fetched))
	for _, c := range cur {
		idx := -1
		for i, f := range fetched {
			if !used[i] && sameExperience(c, f) {
				idx = i
				break
			}
		}
		if idx >= 0 {
			used[idx] = true
			m := applyExperience(c, fetched[idx])
			if m != c {
				updated = append(updated, m)
			} else {
				kept++ // matched but unchanged
			}
			out = append(out, m)
		} else {
			kept++ // no LinkedIn match — kept as-is
			out = append(out, c)
		}
	}
	for i, f := range fetched {
		if !used[i] {
			added = append(added, f)
			out = append(out, f)
		}
	}
	return out, added, updated, kept
}

// mergeEducation is the education counterpart of mergeExperience; entries
// match by institution.
func mergeEducation(cur, fetched []Education) (out, added, updated []Education, kept int) {
	used := make([]bool, len(fetched))
	out = make([]Education, 0, len(cur)+len(fetched))
	for _, c := range cur {
		idx := -1
		for i, f := range fetched {
			if !used[i] && sameEducation(c, f) {
				idx = i
				break
			}
		}
		if idx >= 0 {
			used[idx] = true
			m := applyEducation(c, fetched[idx])
			if m != c {
				updated = append(updated, m)
			} else {
				kept++ // matched but unchanged
			}
			out = append(out, m)
		} else {
			kept++ // no LinkedIn match — kept as-is
			out = append(out, c)
		}
	}
	for i, f := range fetched {
		if !used[i] {
			added = append(added, f)
			out = append(out, f)
		}
	}
	return out, added, updated, kept
}

// sameExperience matches two experience entries by (title, company),
// case-insensitive and trimmed.
func sameExperience(a, b Experience) bool {
	return strings.EqualFold(strings.TrimSpace(a.Title), strings.TrimSpace(b.Title)) &&
		strings.EqualFold(strings.TrimSpace(a.Company), strings.TrimSpace(b.Company))
}

// sameEducation matches two education entries by institution,
// case-insensitive and trimmed.
func sameEducation(a, b Education) bool {
	return strings.EqualFold(strings.TrimSpace(a.Institution), strings.TrimSpace(b.Institution))
}

// applyExperience overwrites a current entry's updatable fields from a fetched
// entry. Dates are replaced as a pair only when the fetched entry has a start
// date (an empty fetched end means "present", a real value); description is
// replaced only when the fetched entry has one, so a manual description is
// never wiped by an empty LinkedIn field.
func applyExperience(cur, fetched Experience) Experience {
	if fetched.Start != "" {
		cur.Start = fetched.Start
		cur.End = fetched.End
	}
	if fetched.Description != "" {
		cur.Description = fetched.Description
	}
	return cur
}

// applyEducation is the education counterpart of applyExperience.
func applyEducation(cur, fetched Education) Education {
	if fetched.Start != "" {
		cur.Start = fetched.Start
		cur.End = fetched.End
	}
	if fetched.Degree != "" {
		cur.Degree = fetched.Degree
	}
	if fetched.Description != "" {
		cur.Description = fetched.Description
	}
	return cur
}
