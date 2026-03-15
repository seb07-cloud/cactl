package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/spf13/viper"
)

func TestNewRenderer_JSON(t *testing.T) {
	r := NewRenderer("json", false)
	if _, ok := r.(*JSONRenderer); !ok {
		t.Errorf("NewRenderer(\"json\") returned %T, want *JSONRenderer", r)
	}
}

func TestNewRenderer_Human(t *testing.T) {
	r := NewRenderer("human", true)
	if _, ok := r.(*HumanRenderer); !ok {
		t.Errorf("NewRenderer(\"human\") returned %T, want *HumanRenderer", r)
	}
}

func TestNewRenderer_Default(t *testing.T) {
	r := NewRenderer("", false)
	if _, ok := r.(*HumanRenderer); !ok {
		t.Errorf("NewRenderer(\"\") returned %T, want *HumanRenderer", r)
	}
}

// captureStdout captures stdout output during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestHumanRenderer_Success(t *testing.T) {
	r := &HumanRenderer{useColor: false}
	out := captureStdout(t, func() { r.Success("done") })
	if !strings.Contains(out, "OK done") {
		t.Errorf("Success output = %q, want to contain %q", out, "OK done")
	}
}

func TestHumanRenderer_Error(t *testing.T) {
	r := &HumanRenderer{useColor: false}
	out := captureStdout(t, func() { r.Error("failed") })
	if !strings.Contains(out, "ERROR failed") {
		t.Errorf("Error output = %q, want to contain %q", out, "ERROR failed")
	}
}

func TestHumanRenderer_Info(t *testing.T) {
	r := &HumanRenderer{useColor: false}
	out := captureStdout(t, func() { r.Info("note") })
	if !strings.Contains(out, "INFO note") {
		t.Errorf("Info output = %q, want to contain %q", out, "INFO note")
	}
}

func TestHumanRenderer_Warn(t *testing.T) {
	r := &HumanRenderer{useColor: false}
	out := captureStdout(t, func() { r.Warn("careful") })
	if !strings.Contains(out, "WARN careful") {
		t.Errorf("Warn output = %q, want to contain %q", out, "WARN careful")
	}
}

func TestHumanRenderer_Print(t *testing.T) {
	r := &HumanRenderer{useColor: false}
	out := captureStdout(t, func() { r.Print("hello") })
	if !strings.Contains(out, "hello") {
		t.Errorf("Print output = %q, want to contain %q", out, "hello")
	}
}

func TestHumanRenderer_WithColor(t *testing.T) {
	r := &HumanRenderer{useColor: true}
	out := captureStdout(t, func() { r.Success("done") })
	if !strings.Contains(out, colorGreen) {
		t.Errorf("colored Success should contain ANSI green code")
	}
	out = captureStdout(t, func() { r.Error("fail") })
	if !strings.Contains(out, colorRed) {
		t.Errorf("colored Error should contain ANSI red code")
	}
	out = captureStdout(t, func() { r.Info("note") })
	if !strings.Contains(out, colorBlue) {
		t.Errorf("colored Info should contain ANSI blue code")
	}
	out = captureStdout(t, func() { r.Warn("warn") })
	if !strings.Contains(out, colorYellow) {
		t.Errorf("colored Warn should contain ANSI yellow code")
	}
}

func TestJSONRenderer_AllLevels(t *testing.T) {
	r := &JSONRenderer{}

	tests := []struct {
		method func(string)
		level  string
		msg    string
	}{
		{r.Success, "success", "ok"},
		{r.Error, "error", "bad"},
		{r.Info, "info", "note"},
		{r.Warn, "warn", "careful"},
		{r.Print, "info", "plain"},
	}

	for _, tt := range tests {
		out := captureStdout(t, func() { tt.method(tt.msg) })
		var m jsonMessage
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &m); err != nil {
			t.Errorf("json.Unmarshal(%q) error = %v", out, err)
			continue
		}
		if m.Level != tt.level {
			t.Errorf("level = %q, want %q", m.Level, tt.level)
		}
		if m.Message != tt.msg {
			t.Errorf("message = %q, want %q", m.Message, tt.msg)
		}
	}
}

func TestFormatApplied(t *testing.T) {
	got := FormatApplied(reconcile.ActionCreate, "my-policy", false)
	if !strings.Contains(got, "Applied:") || !strings.Contains(got, "my-policy") {
		t.Errorf("FormatApplied = %q, want to contain 'Applied:' and 'my-policy'", got)
	}
}

func TestFormatApplySummary(t *testing.T) {
	tests := []struct {
		name             string
		created, updated int
		recreated        int
		wantContains     string
	}{
		{"no changes", 0, 0, 0, "no changes"},
		{"created only", 2, 0, 0, "2 created"},
		{"mixed", 1, 2, 1, "1 created"},
		{"updated only", 0, 3, 0, "3 updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatApplySummary(tt.created, tt.updated, tt.recreated, false)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("FormatApplySummary(%d,%d,%d) = %q, want to contain %q",
					tt.created, tt.updated, tt.recreated, got, tt.wantContains)
			}
		})
	}
}

func TestShouldUseColor_PipedOutput(t *testing.T) {
	// In test environment, stdout is not a terminal
	v := newTestViper()
	got := ShouldUseColor(v)
	if got {
		t.Error("ShouldUseColor() = true in piped/test environment, want false")
	}
}

func TestShouldUseColor_NoColorFlag(t *testing.T) {
	v := newTestViper()
	v.Set("no-color", true)
	if ShouldUseColor(v) {
		t.Error("ShouldUseColor() = true with no-color flag, want false")
	}
}

func TestShouldUseColor_CIFlag(t *testing.T) {
	v := newTestViper()
	v.Set("ci", true)
	if ShouldUseColor(v) {
		t.Error("ShouldUseColor() = true with ci flag, want false")
	}
}

func newTestViper() *viper.Viper {
	v := viper.New()
	return v
}
