package db

// Document-patch machinery. The writable surface of an entity is a JSON
// document: read commands emit it, set/update commands patch it, schema
// commands render its empty template. Validation and normalization live here,
// behind the same Store seam the CLI and the web route already cross, so both
// surfaces can never drift apart.
//
// The core is spec-driven (docSpec/docKey/docKeyKind below): a second entity
// (jobs — WP-116) only adds a spec. Today only the profile adapter is
// exported; the generic core stays an internal seam until a second adapter
// justifies exporting it.
//
// Doc keys mirror the READ document ('profile show --json' emits camelCase),
// so a doc round-trips: show | set --file -. Each key carries the store key
// it lands on (the Store seam is snake_case), plus its kind.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// docKeyKind is how a writable field is stored.
type docKeyKind int

const (
	// docScalar is a plain string field.
	docScalar docKeyKind = iota
	// docList is a JSON-array-of-strings field; the Store normalizes lists to
	// match form (case-fold, trim, dedupe) on write.
	docList
	// docEntries is a structured entry array (experience/education);
	// Normalize validates and serializes it to the stored JSON string.
	docEntries
	// docSalaryFloor is the [{region, amount}] special case.
	docSalaryFloor
)

// docKey declares one writable field: name is the document key (matches the
// read surface), store is the key the Store seam expects, kind decides how
// the value is normalized.
type docKey struct {
	name  string
	store string
	kind  docKeyKind
	// normalize overrides the default per-kind behavior; used by docEntries
	// (Serialize*) and docSalaryFloor.
	normalize func(raw json.RawMessage) (any, error)
}

// docSpec is the writable surface of an entity: the keys its read command
// emits, minus derived/read-only fields.
type docSpec struct {
	name string
	keys []docKey
}

// profileSpec is the profile document — the keys `profile show --json`
// emits, minus derived/read-only fields (timestamps, joined data). Doc keys
// are camelCase to match the read; store keys are the snake_case the Store
// seam expects.
var profileSpec = docSpec{
	name: "profile",
	keys: []docKey{
		{name: "name", store: "name", kind: docScalar},
		{name: "email", store: "email", kind: docScalar},
		{name: "phone", store: "phone", kind: docScalar},
		{name: "title", store: "title", kind: docScalar},
		{name: "industry", store: "industry", kind: docScalar},
		{name: "currentLocation", store: "current_location", kind: docScalar},
		{name: "visaSponsorship", store: "visa_sponsorship", kind: docScalar},
		{name: "remote", store: "remote", kind: docScalar},
		{name: "seniority", store: "seniority", kind: docScalar},
		{name: "skills", store: "skills", kind: docList},
		{name: "locationPreference", store: "location_preference", kind: docList},
		{name: "companies", store: "companies", kind: docList},
		{name: "avoidCompanies", store: "avoid_companies", kind: docList},
		{name: "keywords", store: "keywords", kind: docList},
		{name: "dealbreakers", store: "dealbreakers", kind: docList},
		{name: "salaryFloor", store: "salary_floor", kind: docSalaryFloor, normalize: normalizeSalaryFloor},
		{name: "experience", store: "experience", kind: docEntries, normalize: func(raw json.RawMessage) (any, error) { return SerializeExperience(raw) }},
		{name: "education", store: "education", kind: docEntries, normalize: func(raw json.RawMessage) (any, error) { return SerializeEducation(raw) }},
	},
}

// NormalizeProfileDocument validates and normalizes a profile patch document
// into the store-ready updates map (keys are the Store seam's snake_case
// names). Patch semantics: only keys present are changed. Unknown keys are
// rejected so a typo never silently drops an edit.
//
// currentExperience gates manual seniority: once experience carries a derived
// year signal, the level is derived, not manually assignable. Manual set is a
// placeholder for before a resume seed arrives. The gate evaluates the NEW
// experience when the doc also sets it — a doc cannot sneak a manual
// seniority past a derived level it writes in the same pass.
func NormalizeProfileDocument(doc map[string]json.RawMessage, currentExperience string) (map[string]any, error) {
	effExperience := currentExperience
	if raw, ok := doc["experience"]; ok {
		if s, err := SerializeExperience(raw); err == nil {
			effExperience = s
		}
	}
	if _, ok := doc["seniority"]; ok {
		if derived := DeriveSeniority(effExperience); derived != "" {
			return nil, fmt.Errorf("seniority derives from experience as %q — correct experience instead, or clear it first", derived)
		}
	}
	return normalizeDocument(profileSpec, doc)
}

// ProfileSchemaTemplate returns the empty profile document — the write schema
// for `profile set --file`, rendered as a fill-in-the-blank template.
func ProfileSchemaTemplate() string {
	return renderSchema(profileSpec)
}

// normalizeDocument validates a patch document against a spec: unknown keys
// are rejected, each value is normalized to the store-ready form.
func normalizeDocument(spec docSpec, doc map[string]json.RawMessage) (map[string]any, error) {
	if len(doc) == 0 {
		return nil, fmt.Errorf("no fields provided")
	}
	byName := make(map[string]docKey, len(spec.keys))
	for _, k := range spec.keys {
		byName[k.name] = k
	}
	updates := make(map[string]any, len(doc))
	for key, raw := range doc {
		keySpec, ok := byName[key]
		if !ok {
			return nil, fmt.Errorf("unknown %s field %q", spec.name, key)
		}
		normalize := keySpec.normalize
		if normalize == nil {
			switch keySpec.kind {
			case docScalar:
				normalize = normalizeScalar
			case docList:
				normalize = normalizeList
			default:
				return nil, fmt.Errorf("field %q has no normalizer", key)
			}
		}
		val, err := normalize(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		updates[keySpec.store] = val
	}
	return updates, nil
}

// renderSchema builds the empty document from a spec. Entry objects take
// their shape from the entry structs' json tags — the struct is the single
// source of the entry shape, so the schema can never drift from what
// Serialize* accepts.
func renderSchema(spec docSpec) string {
	doc := make(map[string]any, len(spec.keys))
	for _, k := range spec.keys {
		switch k.kind {
		case docScalar:
			doc[k.name] = ""
		case docList:
			doc[k.name] = []string{}
		case docEntries:
			doc[k.name] = []map[string]string{emptyEntryTemplate(k.name)}
		case docSalaryFloor:
			// An empty list — a template must round-trip through set, and an
			// empty {region, amount} entry would fail validation. The entry
			// shape is documented in 'profile schema --help' and the skill.
			doc[k.name] = []any{}
		}
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b)
}

// emptyEntryTemplate renders {"<json tag>": ""} for a structured entry struct.
func emptyEntryTemplate(key string) map[string]string {
	var v any
	switch key {
	case "experience":
		v = ExperienceEntry{}
	case "education":
		v = EducationEntry{}
	default:
		return nil
	}
	t := reflect.TypeOf(v)
	out := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			out[name] = ""
		}
	}
	return out
}

// normalizeScalar validates a plain-string field.
func normalizeScalar(raw json.RawMessage) (any, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("expected a string")
	}
	return s, nil
}

// normalizeList validates a JSON-array-of-strings field and serializes it to
// the stored JSON-array string. The Store seam normalizes lists to match form
// on write.
func normalizeList(raw json.RawMessage) (any, error) {
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("expected an array of strings")
	}
	b, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// normalizeSalaryFloor validates a [{region, amount}] list. Currency is a
// derived fact, never accepted from the client.
func normalizeSalaryFloor(raw json.RawMessage) (any, error) {
	var entries []struct {
		Region string `json:"region"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("expected an array of {region, amount}")
	}
	floors := make([]SalaryFloor, 0, len(entries))
	for _, e := range entries {
		region := strings.TrimSpace(e.Region)
		if region == "" {
			return nil, fmt.Errorf("region is required")
		}
		if e.Amount <= 0 {
			return nil, fmt.Errorf("amount must be a positive number")
		}
		floors = append(floors, SalaryFloor{Region: region, Amount: e.Amount})
	}
	return SalaryFloorToJSON(floors)
}
