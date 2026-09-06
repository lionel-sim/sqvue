package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultPreservesANSI256Palette(t *testing.T) {
	theme := Default()
	if got := string(theme.DialogBackground); got != "235" {
		t.Fatalf("dialog background = %q, want 235", got)
	}
	for name, got := range map[string]struct{ got, want string }{
		"title":       {string(theme.Title.GetForeground().(lipgloss.Color)), "69"},
		"selected":    {string(theme.Selected.GetForeground().(lipgloss.Color)), "86"},
		"active row":  {string(theme.ActiveRow.GetBackground().(lipgloss.Color)), "235"},
		"active cell": {string(theme.ActiveCell.GetBackground().(lipgloss.Color)), "111"},
		"muted":       {string(theme.Muted.GetForeground().(lipgloss.Color)), "245"},
		"error":       {string(theme.Error.GetForeground().(lipgloss.Color)), "204"},
		"status":      {string(theme.Status.GetForeground().(lipgloss.Color)), "111"},
	} {
		if got.got != got.want {
			t.Errorf("%s = %q, want %q", name, got.got, got.want)
		}
	}
}

func TestByNameReturnsEachBuiltInTheme(t *testing.T) {
	for _, name := range Names() {
		got, err := ByName(name)
		if err != nil {
			t.Fatalf("ByName(%q) error = %v", name, err)
		}
		if got.Title.GetForeground() == nil || got.DialogBackground == "" {
			t.Fatalf("ByName(%q) returned an incomplete theme: %#v", name, got)
		}
	}
}

func TestByNameRejectsUnknownTheme(t *testing.T) {
	if _, err := ByName("sepia"); err == nil {
		t.Fatal("ByName accepted an unknown theme")
	}
}

func TestBuiltInThemesHaveDistinctSelectionColors(t *testing.T) {
	defaultTheme, err := ByName("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"light", "high-contrast"} {
		got, err := ByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if got.Selected.GetForeground() == defaultTheme.Selected.GetForeground() {
			t.Errorf("%s selection colour matches default", name)
		}
	}
}
