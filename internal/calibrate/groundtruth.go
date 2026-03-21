package calibrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CouplingLevel represents a human judgment of structural coupling.
type CouplingLevel string

const (
	CouplingHigh   CouplingLevel = "high"
	CouplingMedium CouplingLevel = "medium"
	CouplingLow    CouplingLevel = "low"
)

// Annotation is a single human-annotated test file judgment.
type Annotation struct {
	File               string        `yaml:"file"`
	StructuralCoupling CouplingLevel `yaml:"structural_coupling"`
	Behavioral         *bool         `yaml:"behavioral"`        // is this primarily behavioral?
	Fragile            *bool         `yaml:"fragile"`           // would this break on refactoring?
	HasAntiPatterns    *bool         `yaml:"has_anti_patterns"` // does it contain anti-patterns?
	HasTautologies     *bool         `yaml:"has_tautologies"`   // does it contain tautological tests?
	MockPlacement      *string       `yaml:"mock_placement"`    // "boundary", "internal", or "mixed"
	Notes              string        `yaml:"notes"`
}

// GroundTruth is the top-level calibration file structure.
type GroundTruth struct {
	Annotations []Annotation `yaml:"annotations"`
}

// LoadGroundTruth reads and parses a calibration YAML file.
func LoadGroundTruth(path string) (*GroundTruth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading calibration file: %w", err)
	}

	var gt GroundTruth
	if err := yaml.Unmarshal(data, &gt); err != nil {
		return nil, fmt.Errorf("parsing calibration file: %w", err)
	}

	if len(gt.Annotations) == 0 {
		return nil, fmt.Errorf("calibration file has no annotations")
	}

	// Validate
	for i, a := range gt.Annotations {
		if a.File == "" {
			return nil, fmt.Errorf("annotation %d: file is required", i)
		}
		if a.StructuralCoupling != "" {
			switch a.StructuralCoupling {
			case CouplingHigh, CouplingMedium, CouplingLow:
				// ok
			default:
				return nil, fmt.Errorf("annotation %d (%s): structural_coupling must be high, medium, or low", i, a.File)
			}
		}
		if a.MockPlacement != nil {
			switch *a.MockPlacement {
			case "boundary", "internal", "mixed":
				// ok
			default:
				return nil, fmt.Errorf("annotation %d (%s): mock_placement must be boundary, internal, or mixed", i, a.File)
			}
		}
	}

	return &gt, nil
}

// FindCalibrationFile looks for a calibration file in the scan path.
func FindCalibrationFile(scanPath string) string {
	candidate := filepath.Join(scanPath, ".confidence-calibration.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// ResolveAnnotationPath converts an annotation's relative file path to match
// against a FileAnalysis path. Both are normalized for comparison.
func ResolveAnnotationPath(annotationFile, scanPath string) string {
	if filepath.IsAbs(annotationFile) {
		return filepath.Clean(annotationFile)
	}
	return filepath.Clean(filepath.Join(scanPath, annotationFile))
}

// MatchesFile checks if a file analysis path matches an annotation path.
// Uses suffix matching so annotations with relative paths work against
// absolute analysis paths.
func MatchesFile(analysisPath, annotationResolved string) bool {
	a := filepath.Clean(analysisPath)
	b := filepath.Clean(annotationResolved)
	if a == b {
		return true
	}
	return strings.HasSuffix(a, string(filepath.Separator)+b) ||
		strings.HasSuffix(b, string(filepath.Separator)+a)
}
