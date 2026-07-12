package bitspilani

import (
	"testing"
)

const testHTML = `<table>
<thead><tr><th>Sr. No.</th><th>Position</th><th>Campus</th><th>Department</th><th>Last Applied Date</th><th>Detail</th><th>Link</th></tr></thead>
<tbody>
<tr>
<td>1</td>
<td><div class="question_title">JRF/Project Associate-1 <div class="new-tag"><p>New</p></div></div></td>
<td><span class="blue-tag"><a>Pilani</a></span></td>
<td><span class="yellow-tag"><a>Biological Sciences</a></span></td>
<td>28/06/2026</td>
<td><a href="https://www.bits-pilani.ac.in/careers/junior-research-fellow-project-associate-1/">View Details</a></td>
<td><a href="https://www.bits-pilani.ac.in/wp-content/uploads/adv.pdf" target="_blank">View PDF</a></td>
</tr>
<tr>
<td>2</td>
<td><div class="question_title">Assistant Professor</div></td>
<td><span class="blue-tag"><a>Goa</a></span></td>
<td><span class="yellow-tag"><a>Computer Science</a></span></td>
<td>15/07/2026</td>
<td><a href="https://www.bits-pilani.ac.in/careers/assistant-professor/">View Details</a></td>
<td></td>
</tr>
</tbody>
</table>`

func TestParseTable(t *testing.T) {
	results := parseTable(testHTML)
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}

	r := results[0]
	if r.Title != "JRF/Project Associate-1 New" {
		t.Errorf("Title: got %q", r.Title)
	}
	if r.Date != "28/06/2026" {
		t.Errorf("Date: got %q", r.Date)
	}
	if r.Metadata["campus"] != "Pilani" {
		t.Errorf("campus: got %q", r.Metadata["campus"])
	}
	if r.URL != "https://www.bits-pilani.ac.in/careers/junior-research-fellow-project-associate-1/" {
		t.Errorf("URL: got %q", r.URL)
	}
	if r.Metadata["pdf_url"] != "https://www.bits-pilani.ac.in/wp-content/uploads/adv.pdf" {
		t.Errorf("pdf_url: got %q", r.Metadata["pdf_url"])
	}

	r2 := results[1]
	if r2.Title != "Assistant Professor" {
		t.Errorf("second Title: got %q", r2.Title)
	}
	if r2.Metadata["campus"] != "Goa" {
		t.Errorf("second campus: got %q", r2.Metadata["campus"])
	}
}

func TestParseTable_empty(t *testing.T) {
	if len(parseTable("<html></html>")) != 0 {
		t.Error("expected 0")
	}
}
