package linkedin

import (
	"slices"
	"testing"
)

func TestMergeIntoEmptyProfileIsSeed(t *testing.T) {
	fetched := Profile{
		Name:     "Jane Doe",
		Headline: "Engineer",
		Location: "London",
		Skills:   []string{"Go", "React"},
		Exp:      []Experience{{Title: "Engineer", Company: "Acme", Start: "2020-01", End: ""}},
		Edu:      []Education{{Institution: "MIT", Degree: "BS", Start: "2012", End: "2016"}},
	}
	merged, sum := Merge(Profile{}, fetched)

	if merged.Name != "Jane Doe" || merged.Headline != "Engineer" || merged.Location != "London" {
		t.Errorf("merged scalars = %+v", merged)
	}
	if !slices.Equal(sum.SkillsAdded, []string{"Go", "React"}) || len(merged.Skills) != 2 {
		t.Errorf("skills = %v, added = %v", merged.Skills, sum.SkillsAdded)
	}
	if len(sum.ExperienceAdded) != 1 || sum.ExperienceKept != 0 || sum.ExperienceUpdated != nil {
		t.Errorf("exp summary = %+v", sum)
	}
	if len(sum.EducationAdded) != 1 {
		t.Errorf("edu summary = %+v", sum)
	}
	// Doc() must emit the same doc a fresh import would.
	if doc := merged.Doc(); doc["name"] != "Jane Doe" {
		t.Errorf("merged doc = %v", doc)
	}
}

func TestMergeUpdatesMatchedKeepsUnmatched(t *testing.T) {
	current := Profile{
		Name: "Jane Doe",
		Exp: []Experience{
			{Title: "Engineer", Company: "Acme", Start: "2020-01", End: "", Description: "manual note"},
			{Title: "Freelance", Company: "Side Gigs", Start: "2018-01", End: "2019-12", Description: "keep me"},
		},
		Edu: []Education{
			{Institution: "MIT", Degree: "BS CS", Start: "2012", End: "2016"},
		},
		Skills: []string{"Go", "React"},
	}
	fetched := Profile{
		Name: "Jane Doe",
		Exp: []Experience{
			{Title: "Engineer", Company: "Acme", Start: "2020-02", End: "", Description: "LinkedIn desc"},
			{Title: "Senior Engineer", Company: "Acme", Start: "2025-01", End: ""},
		},
		Edu: []Education{
			{Institution: "mit", Degree: "BS Computer Science", Start: "2012", End: "2016"},
			{Institution: "Stanford", Degree: "MS", Start: "2017", End: "2019"},
		},
		Skills: []string{"react", "AWS"},
	}
	merged, sum := Merge(current, fetched)

	// Matched role updated (dates + description from LinkedIn).
	if len(merged.Exp) != 3 {
		t.Fatalf("len(Exp) = %d, want 3: %+v", len(merged.Exp), merged.Exp)
	}
	if merged.Exp[0].Start != "2020-02" || merged.Exp[0].Description != "LinkedIn desc" {
		t.Errorf("Exp[0] = %+v, want dates+desc updated", merged.Exp[0])
	}
	// Unmatched current role kept, untouched.
	if merged.Exp[1].Title != "Freelance" || merged.Exp[1].Description != "keep me" {
		t.Errorf("Exp[1] = %+v, want kept unchanged", merged.Exp[1])
	}
	// New fetched role appended.
	if merged.Exp[2].Title != "Senior Engineer" || merged.Exp[2].Company != "Acme" {
		t.Errorf("Exp[2] = %+v, want appended new role", merged.Exp[2])
	}
	if len(sum.ExperienceAdded) != 1 || len(sum.ExperienceUpdated) != 1 || sum.ExperienceKept != 1 {
		t.Errorf("exp summary = %+v", sum)
	}

	// Education matched case-insensitively by institution; degree updated.
	if len(merged.Edu) != 2 {
		t.Fatalf("len(Edu) = %d, want 2: %+v", len(merged.Edu), merged.Edu)
	}
	if merged.Edu[0].Degree != "BS Computer Science" {
		t.Errorf("Edu[0] = %+v, want degree updated", merged.Edu[0])
	}
	if merged.Edu[1].Institution != "Stanford" {
		t.Errorf("Edu[1] = %+v, want appended", merged.Edu[1])
	}
	if len(sum.EducationAdded) != 1 || len(sum.EducationUpdated) != 1 || sum.EducationKept != 0 {
		t.Errorf("edu summary = %+v", sum)
	}

	// Skills union: "react" matches existing "React" (case-insensitive), AWS added.
	if !slices.Equal(merged.Skills, []string{"Go", "React", "AWS"}) {
		t.Errorf("merged skills = %v", merged.Skills)
	}
	if !slices.Equal(sum.SkillsAdded, []string{"AWS"}) {
		t.Errorf("skills added = %v", sum.SkillsAdded)
	}
}

func TestMergeDoesNotWipeManualFields(t *testing.T) {
	// Fetched entry has dates but no description — the manual description must
	// survive. Fetched entry with no start date must not wipe existing dates.
	current := Profile{
		Exp: []Experience{{Title: "Engineer", Company: "Acme", Start: "2020-01", End: "2024-01", Description: "hand written"}},
	}
	fetched := Profile{
		Exp: []Experience{{Title: "Engineer", Company: "Acme", Description: ""}},
	}
	merged, sum := Merge(current, fetched)

	if merged.Exp[0].Start != "2020-01" || merged.Exp[0].End != "2024-01" {
		t.Errorf("dates wiped: %+v", merged.Exp[0])
	}
	if merged.Exp[0].Description != "hand written" {
		t.Errorf("description wiped: %+v", merged.Exp[0])
	}
	// Nothing changed → no "updated" entries, nothing added, one kept.
	if len(sum.ExperienceUpdated) != 0 || len(sum.ExperienceAdded) != 0 || sum.ExperienceKept != 1 {
		t.Errorf("summary = %+v", sum)
	}
}

func TestMergeScalarsFollowLinkedIn(t *testing.T) {
	current := Profile{Name: "Old Name", Headline: "Old", Location: "Old City"}
	fetched := Profile{Name: "Jane Doe", Headline: "New Title"}
	merged, _ := Merge(current, fetched)
	if merged.Name != "Jane Doe" || merged.Headline != "New Title" || merged.Location != "Old City" {
		t.Errorf("merged = %+v", merged)
	}
}
