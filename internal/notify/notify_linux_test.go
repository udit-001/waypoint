//go:build linux

package notify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/autopilot"
)

// recordingDeliver captures what deliver was asked to send.
type recordingDeliver struct {
	titles []string
	bodies []string
	clicks int
	err    error
}

func (r *recordingDeliver) record(title, body string, onClick func()) error {
	r.titles = append(r.titles, title)
	r.bodies = append(r.bodies, body)
	if onClick != nil {
		r.clicks++
	}
	return r.err
}

func TestDesktopNotifyShortlistWiresClickAction(t *testing.T) {
	rec := &recordingDeliver{}
	orig := sendDBus
	sendDBus = rec.record
	t.Cleanup(func() { sendDBus = orig })

	d := New("http://localhost:8080/#/found")
	err := d.NotifyShortlist(context.Background(), autopilot.ShortlistNotice{
		Count: 2, TopTitle: "Platform Engineer", TopCompany: "Ramp", TopScore: 84,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.titles) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(rec.titles))
	}
	if !strings.Contains(rec.bodies[0], "Platform Engineer at Ramp (84)") {
		t.Errorf("body = %q, want top match", rec.bodies[0])
	}
	if rec.clicks != 1 {
		t.Errorf("click actions registered = %d, want 1 (OpenURL set)", rec.clicks)
	}
}

func TestDesktopNotifyWithoutURLOmitsClickAction(t *testing.T) {
	rec := &recordingDeliver{}
	orig := sendDBus
	sendDBus = rec.record
	t.Cleanup(func() { sendDBus = orig })

	d := New("")
	if err := d.NotifyShortlist(context.Background(), autopilot.ShortlistNotice{Count: 1, TopTitle: "A"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.clicks != 0 {
		t.Errorf("click actions = %d, want 0 (no OpenURL)", rec.clicks)
	}
}

func TestDesktopNotifyErrorPropagates(t *testing.T) {
	rec := &recordingDeliver{err: errors.New("no bus")}
	orig := sendDBus
	sendDBus = rec.record
	t.Cleanup(func() { sendDBus = orig })

	d := New("http://localhost:8080/#/found")
	if err := d.NotifyShortlist(context.Background(), autopilot.ShortlistNotice{Count: 1}); err == nil {
		t.Error("want error propagated to the cycle (which logs it)")
	}
}
