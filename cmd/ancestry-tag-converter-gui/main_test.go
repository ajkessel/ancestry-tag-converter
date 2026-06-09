package main

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestHelpKeyboardShortcut(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		key      fyne.KeyName
		modifier fyne.KeyModifier
	}{
		{
			name: "Windows",
			goos: "windows",
			key:  fyne.KeyF1,
		},
		{
			name: "Linux",
			goos: "linux",
			key:  fyne.KeyF1,
		},
		{
			name:     "macOS",
			goos:     "darwin",
			key:      fyne.KeySlash,
			modifier: fyne.KeyModifierSuper | fyne.KeyModifierShift,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shortcut := helpKeyboardShortcut(test.goos)
			if shortcut.KeyName != test.key {
				t.Errorf("key = %q, want %q", shortcut.KeyName, test.key)
			}
			if shortcut.Modifier != test.modifier {
				t.Errorf("modifier = %v, want %v", shortcut.Modifier, test.modifier)
			}
		})
	}
}
