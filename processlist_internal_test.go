package main

import (
	"errors"
	"testing"
)

// TestProcessListHandlesServiceOpenError exercises the ordering bug in
// processList: it used to defer service.Close() before checking whether
// instruments.NewDeviceInfoService succeeded. When it failed (service ==
// nil), the deferred Close() call was bound to a nil receiver at defer time
// — if that defer ever ran, (*instruments.DeviceInfoService)(nil).Close()
// dereferences a nil field and panics. buildProcessList extracts the
// check-then-close ordering into a unit the panic can be tested against
// without a real device connection.
func TestProcessListHandlesServiceOpenError(t *testing.T) {
	wantErr := errors.New("failed opening deviceInfoService")

	_, err := buildProcessList(nil, wantErr, false)

	if !errors.Is(err, wantErr) {
		t.Fatalf("buildProcessList error = %v, want %v", err, wantErr)
	}
}
