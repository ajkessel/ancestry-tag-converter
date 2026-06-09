package pathcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureOutputDistinct(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.ged")
	if err := os.WriteFile(input, []byte("0 HEAD\n0 TRLR\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("different path", func(t *testing.T) {
		if err := EnsureOutputDistinct(filepath.Join(dir, "output.ged"), input); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("same path", func(t *testing.T) {
		if err := EnsureOutputDistinct(input, input); err == nil {
			t.Fatal("expected identical paths to be rejected")
		}
	})

	t.Run("hard link", func(t *testing.T) {
		output := filepath.Join(dir, "hard-link.ged")
		if err := os.Link(input, output); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
		if err := EnsureOutputDistinct(output, input); err == nil {
			t.Fatal("expected hard link to input to be rejected")
		}
	})

	t.Run("second input", func(t *testing.T) {
		mergeBase := filepath.Join(dir, "base.ged")
		if err := os.WriteFile(mergeBase, []byte("0 HEAD\n0 TRLR\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := EnsureOutputDistinct(mergeBase, input, mergeBase); err == nil {
			t.Fatal("expected output matching second input to be rejected")
		}
	})
}
