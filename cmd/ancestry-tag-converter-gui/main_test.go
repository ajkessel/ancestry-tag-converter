package main

import (
	"os"
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

func TestValidateGEDCOMFile(t *testing.T) {
	tmpdir := t.TempDir()

	tests := []struct {
		name     string
		setup    func(path string) error
		wantErr  bool
		errMatch string
	}{
		{
			name: "nonexistent file",
			setup: func(path string) error {
				return nil // file doesn't exist
			},
			wantErr:  true,
			errMatch: "does not exist",
		},
		{
			name: "empty file",
			setup: func(path string) error {
				return os.WriteFile(path, []byte{}, 0644)
			},
			wantErr:  true,
			errMatch: "empty",
		},
		{
			name: "not GEDCOM",
			setup: func(path string) error {
				return os.WriteFile(path, []byte("This is just random text, not GEDCOM"), 0644)
			},
			wantErr:  true,
			errMatch: "does not appear to be a valid GEDCOM file",
		},
		{
			name: "valid GEDCOM",
			setup: func(path string) error {
				return os.WriteFile(path, []byte("0 HEAD\n1 SOUR Test\n0 TRLR"), 0644)
			},
			wantErr: false,
		},
		{
			name: "GEDCOM with BOM",
			setup: func(path string) error {
				return os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, []byte("0 HEAD\n1 SOUR Test")...), 0644)
			},
			wantErr: false,
		},
		{
			name: "directory instead of file",
			setup: func(path string) error {
				return os.Mkdir(path, 0755)
			},
			wantErr:  true,
			errMatch: "directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpdir, tt.name+".ged")
			if err := tt.setup(path); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			err := validateGEDCOMFile(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateGEDCOMFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMatch != "" && (err == nil || !contains(err.Error(), tt.errMatch)) {
				t.Fatalf("validateGEDCOMFile() error = %v, want to contain %q", err, tt.errMatch)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
