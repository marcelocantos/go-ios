//go:build !fast
// +build !fast

package instruments_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/stretchr/testify/require"
)

// TestListenAppStateNotifications_DeliversBackBoardEnumeration is the
// real-device guard for the iOS-17+ RSD fix: subscribing must deliver the
// initial state-enumeration burst BackBoard sends on connect, including
// at least one entry with state_description == "Running".
//
// Requires: a paired iOS device (iOS 17+ for the RSD code path that the
// fix targets), `ios tunnel start` (or `--userspace`) running on the
// host, and the developer image mounted on the device. Set IOS_UDID to
// the device's serial.
func TestListenAppStateNotifications_DeliversBackBoardEnumeration(t *testing.T) {
	udid := os.Getenv("IOS_UDID")
	if udid == "" {
		t.Skip("IOS_UDID not set; skipping real-device test")
	}

	dev, err := deviceWithRsd(t, udid)
	require.NoError(t, err)

	recv, closeFn, err := instruments.ListenAppStateNotifications(dev)
	require.NoError(t, err, "ListenAppStateNotifications should subscribe without error")
	defer closeFn()

	type result struct {
		data map[string]interface{}
		err  error
	}
	results := make(chan result, 64)
	go func() {
		for {
			data, rerr := recv()
			results <- result{data: data, err: rerr}
			if rerr != nil {
				return
			}
		}
	}()

	deadline := time.After(5 * time.Second)
	count := 0
	sawRunning := false
collect:
	for {
		select {
		case <-deadline:
			break collect
		case r := <-results:
			require.NoError(t, r.err)
			count++
			// BackBoard enumerates per-app entries; each "result" map carries
			// state_description directly (single-app frame) or via a payload
			// list (multi-app frame). Walk both shapes.
			if sd, _ := r.data["state_description"].(string); sd == "Running" {
				sawRunning = true
			}
		}
	}

	require.Greater(t, count, 0,
		"BackBoard delivered zero applicationStateNotification: events in 5s — "+
			"the iOS-17+ RSD dispatch path is broken (the bug this test guards against)")
	require.True(t, sawRunning,
		"received %d notifications but none had state_description=Running; "+
			"BackBoard should always have at least one running app on a live device", count)
}

// deviceWithRsd resolves a DeviceEntry through the userspace-tunnel
// agent so the resulting entry has its Rsd provider populated and DTX
// requests route through dtservicehub on iOS 17+. Mirrors what the
// `ios` CLI does when it sees `--tunnel-info-host`/`--tunnel-info-port`
// or auto-discovers the local agent.
func deviceWithRsd(t *testing.T, udid string) (ios.DeviceEntry, error) {
	t.Helper()
	dev, err := ios.GetDevice(udid)
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	host := envDefault("IOS_TUNNEL_INFO_HOST", "127.0.0.1")
	port := envIntDefault(t, "IOS_TUNNEL_INFO_PORT", 60105)
	info, err := tunnel.TunnelInfoForDevice(udid, host, port)
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	dev.UserspaceTUN = info.UserspaceTUN
	dev.UserspaceTUNHost = host
	dev.UserspaceTUNPort = info.UserspaceTUNPort

	rsdService, err := ios.NewWithAddrPortDevice(info.Address, info.RsdPort, dev)
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	defer rsdService.Close()
	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	enriched, err := ios.GetDeviceWithAddress(udid, info.Address, rsdProvider)
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	enriched.UserspaceTUN = dev.UserspaceTUN
	enriched.UserspaceTUNHost = dev.UserspaceTUNHost
	enriched.UserspaceTUNPort = dev.UserspaceTUNPort
	return enriched, nil
}

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envIntDefault(t *testing.T, k string, def int) int {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s=%q is not an integer: %v", k, v, err)
	}
	return n
}
