package reconcile

import (
	"strings"
	"testing"
)

func TestActionType_String(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionNoop, "noop"},
		{ActionCreate, "create"},
		{ActionUpdate, "update"},
		{ActionRecreate, "recreate"},
		{ActionUntracked, "untracked"},
		{ActionDuplicate, "duplicate"},
		{ActionType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.action.String()
			if got != tt.want {
				t.Errorf("ActionType(%d).String() = %q, want %q", int(tt.action), got, tt.want)
			}
		})
	}
}

func TestActionType_UnknownContainsNumber(t *testing.T) {
	got := ActionType(42).String()
	if !strings.Contains(got, "42") {
		t.Errorf("unknown action string %q should contain the numeric value", got)
	}
}
