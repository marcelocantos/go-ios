package appservice

import (
	"errors"
	"strings"
	"testing"
)

func TestGetErrorDecodesCoreDeviceError(t *testing.T) {
	resp := map[string]interface{}{
		"CoreDevice.error": map[string]interface{}{
			"code":   int64(10004),
			"domain": "com.apple.dt.CoreDeviceError",
			"userInfo": map[string]interface{}{
				"NSLocalizedDescription": "The process identifier of the launched application could not be determined. It may have already terminated.",
			},
			"userInfoWithNSSecureCoding": []byte{1, 2, 3},
		},
	}
	err := getError(resp)
	var cde *CoreDeviceError
	if !errors.As(err, &cde) {
		t.Fatalf("getError = %v, want *CoreDeviceError", err)
	}
	if cde.Code != 10004 || cde.Domain != "com.apple.dt.CoreDeviceError" {
		t.Fatalf("decoded %+v", cde)
	}
	if strings.Contains(err.Error(), "[98 112") || !strings.Contains(err.Error(), "could not be determined") {
		t.Fatalf("unhelpful message: %q", err.Error())
	}
}

func TestGetErrorNilWithoutErrorField(t *testing.T) {
	if err := getError(map[string]interface{}{"CoreDevice.output": map[string]interface{}{}}); err != nil {
		t.Fatalf("getError = %v, want nil", err)
	}
}
