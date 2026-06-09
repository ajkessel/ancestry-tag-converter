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
