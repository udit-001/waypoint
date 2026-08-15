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
// current is the profile the patch applies to: its lists seed append/remove
// merges, and its experience gates manual seniority. The seniority gate
// evaluates the NEW experience when the doc also sets it — a doc cannot sneak
// a manual seniority past a derived level it writes in the same pass.
func NormalizeProfileDocument(doc map[string]json.RawMessage, current Profile) (map[string]any, error) {
	effExperience := current.Experience
	if raw, ok := doc["experience"]; ok {
		if s, err := SerializeExperience(raw); err == nil {
			effExperience = s
		}
	}
	if raw, ok := doc["seniority"]; ok {
		if derived := DeriveSeniority(effExperience); derived != "" {
			var v string
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, fmt.Errorf("seniority: expected a string")
			}
			// Read-only once a level derives: only the derived value or an
			// empty one (the stored-empty round-trip case) is tolerated, and
			// both are dropped — a show|set round-trip never re-writes the
			// derived fact. A different level is a manual override, rejected.
			if v != "" && v != derived {
				return nil, fmt.Errorf("seniority derives from experience as %q — correct experience instead, or clear it first", derived)
			}
			delete(doc, "seniority")
		}
	}
	return normalizeDocument(profileSpec, doc, current)
}

// ProfileSchemaTemplate returns the empty profile document — the write schema
// for `profile set --file`, rendered as a fill-in-the-blank template.
func ProfileSchemaTemplate() string {
	return renderSchema(profileSpec)
}

// normalizeDocument validates a patch document against a spec: unknown keys
// are rejected, each value is normalized to the store-ready form.
func normalizeDocument(spec docSpec, doc map[string]json.RawMessage, current Profile) (map[string]any, error) {
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
		val, err := normalizeValue(keySpec, raw, current)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		updates[keySpec.store] = val
	}
	return updates, nil
}

// normalizeValue dispatches a raw document value by kind. Lists accept a bare
// array (replace/clear) or an op object (append/remove) merged against the
// current profile; every other kind follows its spec normalizer.
func normalizeValue(keySpec docKey, raw json.RawMessage, current Profile) (any, error) {
	if keySpec.kind == docList {
		return normalizeListDoc(raw, current, keySpec.store)
	}
	normalize := keySpec.normalize
	if normalize == nil {
		switch keySpec.kind {
		case docScalar:
			normalize = normalizeScalar
		default:
			return nil, fmt.Errorf("field %q has no normalizer", keySpec.name)
		}
	}
	return normalize(raw)
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

// normalizeListDoc handles a string-list value. A bare array replaces the list
// ([] clears it); an op object {"append":[...]} or {"remove":[...]} merges
// against the field's current stored list. The merge is idempotent: appending
// an existing value or removing a missing one is a no-op.
func normalizeListDoc(raw json.RawMessage, current Profile, storeKey string) (any, error) {
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		return normalizeList(raw) // bare array: replace / clear
	}
	add, del, err := parseListOp(raw)
	if err != nil {
		return nil, err
	}
	hasAppend, hasRemove := add != nil, del != nil
	if hasAppend && hasRemove {
		return nil, fmt.Errorf("an op object may set append or remove, not both")
	}
	if !hasAppend && !hasRemove {
		return nil, fmt.Errorf("op object needs append or remove")
	}
	fold := isListPrefKey(storeKey)
	cur := stringList(listCurrentValue(current, storeKey))
	if hasAppend {
		return mergeListAppend(cur, add, fold)
	}
	return mergeListRemove(cur, del, fold)
}

// parseListOp parses a list op object, rejecting unknown keys (a typo is never
// a silent no-op) and non-array verb values.
func parseListOp(raw json.RawMessage) (appendVals, removeVals []string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, fmt.Errorf("op object must be {append} or {remove}")
	}
	for k := range m {
		if k != "append" && k != "remove" {
			return nil, nil, fmt.Errorf("unknown list op %q (allowed: append, remove)", k)
		}
	}
	if r, ok := m["append"]; ok {
		var v []string
		if err := json.Unmarshal(r, &v); err != nil {
			return nil, nil, fmt.Errorf("append expects an array of strings")
		}
		appendVals = v
	}
	if r, ok := m["remove"]; ok {
		var v []string
		if err := json.Unmarshal(r, &v); err != nil {
			return nil, nil, fmt.Errorf("remove expects an array of strings")
		}
		removeVals = v
	}
	return appendVals, removeVals, nil
}

// listCurrentValue reads a list field's stored JSON-array string from the
// current profile, keyed by the Store seam's snake_case name.
func listCurrentValue(p Profile, storeKey string) string {
	switch storeKey {
	case "skills":
		return p.Skills
	case "location_preference":
		return p.LocationPref
	case "companies":
		return p.Companies
	case "avoid_companies":
		return p.AvoidCompanies
	case "keywords":
		return p.Keywords
	case "dealbreakers":
		return p.Dealbreakers
	}
	return ""
}

// listMatchKey maps a list value to its match key: case-folded for preference
// lists, exact for skills (which stay case-sensitive).
func listMatchKey(v string, fold bool) string {
	if fold {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return v
}

// mergeListAppend appends values that are not already present. Preference
// lists store the folded form; skills keep the original case. Empty values are
// skipped.
func mergeListAppend(cur, add []string, fold bool) (string, error) {
	merged := append([]string(nil), cur...)
	seen := make(map[string]bool, len(merged))
	for _, v := range merged {
		seen[listMatchKey(v, fold)] = true
	}
	for _, v := range add {
		k := listMatchKey(v, fold)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		if fold {
			merged = append(merged, k)
		} else {
			merged = append(merged, v)
		}
	}
	return marshalStringList(merged)
}

// mergeListRemove drops every value whose match key is in del. No-op when a
// value is absent.
func mergeListRemove(cur, del []string, fold bool) (string, error) {
	remove := make(map[string]bool, len(del))
	for _, v := range del {
		remove[listMatchKey(v, fold)] = true
	}
	out := make([]string, 0, len(cur))
	for _, v := range cur {
		if !remove[listMatchKey(v, fold)] {
			out = append(out, v)
		}
	}
	return marshalStringList(out)
}

// marshalStringList serializes a list to the stored JSON-array string. A nil
// slice serializes as "[]" — an empty list, not JSON null.
func marshalStringList(list []string) (string, error) {
	if list == nil {
		list = []string{}
	}
	b, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// docValueOp labels a list document value for set feedback: append, remove,
// replace, or clear.
func docValueOp(raw json.RawMessage) string {
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		var m map[string]json.RawMessage
		json.Unmarshal(raw, &m)
		if _, ok := m["append"]; ok {
			return "append"
		}
		if _, ok := m["remove"]; ok {
			return "remove"
		}
		return "replace"
	}
	if strings.TrimSpace(string(raw)) == "[]" {
		return "clear"
	}
	return "replace"
}

// ProfileDocOps returns storeKey → verb label for every list field present in a
// patch document, reusing the profileSpec grammar (which fields are lists).
// The CLI uses it only for 'profile set' feedback.
func ProfileDocOps(doc map[string]json.RawMessage) map[string]string {
	ops := make(map[string]string)
	for _, k := range profileSpec.keys {
		if k.kind != docList {
			continue
		}
		if raw, ok := doc[k.name]; ok {
			ops[k.store] = docValueOp(raw)
		}
	}
	return ops
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
