package api

import (
	"testing"

	"github.com/danielpaulus/go-ios/ios/installationproxy"
)

func TestGetAppInfo(t *testing.T) {
	apps := []installationproxy.AppInfo{
		{
			"CFBundleIdentifier": "com.other.app",
			"CFBundleExecutable": "Other",
		},
		{
			"CFBundleIdentifier": "com.example.app",
			"CFBundleExecutable": "Example",
		},
	}
	if got := processNameForBundle(apps, "com.example.app"); got != "Example" {
		t.Fatalf("processNameForBundle = %q, want Example", got)
	}
	if got := processNameForBundle(apps, "com.missing.app"); got != "" {
		t.Fatalf("processNameForBundle missing = %q, want empty", got)
	}
}
