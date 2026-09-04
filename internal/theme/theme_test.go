package theme

import "testing"

func TestPaletteContainsNamedANSIColors(t *testing.T) {
	want := map[string]string{
		"blue":       "69",
		"cyan":       "86",
		"gray":       "245",
		"pink":       "204",
		"light_blue": "111",
		"charcoal":   "235",
	}
	for name, value := range want {
		if got := string(Palette[name]); got != value {
			t.Errorf("Palette[%q] = %q, want %q", name, got, value)
		}
	}
}
