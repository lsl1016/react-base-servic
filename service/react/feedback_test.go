package react

import (
	"testing"
	"time"

	model "react-base-service/models/llm"
)

func TestParseReactRunFeedback(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
		ok    bool
	}{
		{name: "like", input: model.ReactRunFeedbackLike, want: model.ReactRunFeedbackLike, ok: true},
		{name: "dislike", input: model.ReactRunFeedbackDislike, want: model.ReactRunFeedbackDislike, ok: true},
		{name: "none", input: model.ReactRunFeedbackNone, want: model.ReactRunFeedbackNone, ok: true},
		{name: "invalid positive", input: 2, want: model.ReactRunFeedbackNone, ok: false},
		{name: "invalid negative", input: -2, want: model.ReactRunFeedbackNone, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseReactRunFeedback(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parseReactRunFeedback(%d) = (%d, %t), want (%d, %t)", tt.input, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFormatReactFeedbackTime(t *testing.T) {
	if got := formatReactFeedbackTime(nil); got != "" {
		t.Fatalf("formatReactFeedbackTime(nil) = %q, want empty", got)
	}
	var zero time.Time
	if got := formatReactFeedbackTime(&zero); got != "" {
		t.Fatalf("formatReactFeedbackTime(zero) = %q, want empty", got)
	}
	epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local)
	if got := formatReactFeedbackTime(&epoch); got != "" {
		t.Fatalf("formatReactFeedbackTime(epoch sentinel) = %q, want empty", got)
	}
	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.Local)
	if got := formatReactFeedbackTime(&ts); got != "2026-06-15 10:30:00" {
		t.Fatalf("formatReactFeedbackTime(ts) = %q, want 2026-06-15 10:30:00", got)
	}
}
