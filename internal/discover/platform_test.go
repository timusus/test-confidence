package discover

import (
	"testing"

	"github.com/timusus/test-confidence/internal/model"
)

func TestDetectAndroid(t *testing.T) {
	platform := DetectPlatform("../../testdata/projects/android-simple")
	if platform != model.Android {
		t.Errorf("expected Android, got %v", platform)
	}
}
