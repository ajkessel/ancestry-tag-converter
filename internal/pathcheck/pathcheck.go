package pathcheck

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureOutputDistinct rejects an output path that identifies any input file.
func EnsureOutputDistinct(outputPath string, inputPaths ...string) error {
	for _, inputPath := range inputPaths {
		same, err := sameFile(outputPath, inputPath)
		if err != nil {
			return err
		}
		if same {
			return fmt.Errorf("output file must not be the same as input file %q", inputPath)
		}
	}
	return nil
}

func sameFile(pathA, pathB string) (bool, error) {
	infoA, errA := os.Stat(pathA)
	infoB, errB := os.Stat(pathB)
	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB), nil
	}
	if errA != nil && !os.IsNotExist(errA) {
		return false, fmt.Errorf("check output path %q: %w", pathA, errA)
	}
	if errB != nil && !os.IsNotExist(errB) {
		return false, fmt.Errorf("check input path %q: %w", pathB, errB)
	}

	absA, err := filepath.Abs(pathA)
	if err != nil {
		return false, fmt.Errorf("resolve output path %q: %w", pathA, err)
	}
	absB, err := filepath.Abs(pathB)
	if err != nil {
		return false, fmt.Errorf("resolve input path %q: %w", pathB, err)
	}
	return filepath.Clean(absA) == filepath.Clean(absB), nil
}
