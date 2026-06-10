package main

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
)

func TestLastBrowseLocationTracksMostRecentSelection(t *testing.T) {
	var location lastBrowseLocation

	if got := location.directory(); got != "" {
		t.Fatalf("initial directory = %q, want empty", got)
	}

	first := filepath.Join("first", "family.ged")
	location.remember(first)
	if got, want := location.directory(), filepath.Dir(first); got != want {
		t.Fatalf("directory after first selection = %q, want %q", got, want)
	}

	second := filepath.Join("second", "converted.ged")
	location.remember(second)
	if got, want := location.directory(), filepath.Dir(second); got != want {
		t.Fatalf("directory after second selection = %q, want %q", got, want)
	}
}

func TestSelectMergeBaseEnablesMerge(t *testing.T) {
	entry := newHelpEntry(func() {}, false)
	check := newHelpCheck(func() {}, "Merge", nil)

	selectMergeBase(entry, check, filepath.Join("family", "base.ged"))

	if got, want := entry.Text, filepath.Join("family", "base.ged"); got != want {
		t.Fatalf("merge base = %q, want %q", got, want)
	}
	if !check.Checked {
		t.Fatal("merge checkbox was not checked")
	}
}

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

func TestHelpWidgetsHandleF1(t *testing.T) {
	tests := []struct {
		name  string
		press func(func())
	}{
		{
			name: "Entry",
			press: func(showHelp func()) {
				newHelpEntry(showHelp, false).KeyDown(&fyne.KeyEvent{Name: fyne.KeyF1})
			},
		},
		{
			name: "Check",
			press: func(showHelp func()) {
				newHelpCheck(showHelp, "", nil).TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
			},
		},
		{
			name: "Select",
			press: func(showHelp func()) {
				newHelpSelect(showHelp, nil, nil).TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
			},
		},
		{
			name: "Button",
			press: func(showHelp func()) {
				newHelpButton(showHelp, "", nil).TypedKey(&fyne.KeyEvent{Name: fyne.KeyF1})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			test.press(func() { called = true })
			if !called {
				t.Fatal("F1 did not invoke help")
			}
		})
	}
}
