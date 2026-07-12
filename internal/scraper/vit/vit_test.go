package vit

import (
	"encoding/json"
	"testing"
)

func TestParseZwayamJSON(t *testing.T) {
	raw := `[{"_index":"es-g3-v1","_type":"_doc","_source":{
		"jobTitle":"Lab Assistant - VIT Business School [VITBS]",
		"jobCode":10853,
		"jobUrl":"lab-assistant-vit-business-school-vitbs-vellore-tamil-nadu-india-lab-assistant-2025072915445027",
		"location":"Vellore, Tamil Nadu, India",
		"experienceUIField":"2-4 years",
		"skillSet":"lab assistant, Inventorying stock, Troubleshooting",
		"mandatorySkills":["lab assistant","Inventorying stock","Troubleshooting"],
		"positionsRequired":1,
		"createdDate":1753784094000
	},"_id":"671377","sort":[0,1780637381670],"_score":null}]`

	var jobs []struct {
		Source zwayamJob `json:"_source"`
	}
	if err := jsonUnmarshal([]byte(raw), &jobs); err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	s := jobs[0].Source
	if s.JobTitle != "Lab Assistant - VIT Business School [VITBS]" {
		t.Errorf("Title: got %q", s.JobTitle)
	}
	if s.JobCode != 10853 {
		t.Errorf("JobCode: got %d", s.JobCode)
	}
	if s.ExperienceUI != "2-4 years" {
		t.Errorf("Experience: got %q", s.ExperienceUI)
	}
	if len(s.MandatorySkills) != 3 {
		t.Errorf("Skills count: got %d", len(s.MandatorySkills))
	}
}

func TestParseZwayamJSON_empty(t *testing.T) {
	var jobs []struct {
		Source zwayamJob `json:"_source"`
	}
	if err := jsonUnmarshal([]byte("[]"), &jobs); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0, got %d", len(jobs))
	}
}

func jsonUnmarshal(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}
