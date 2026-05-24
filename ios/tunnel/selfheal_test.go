package tunnel

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/danielpaulus/go-ios/ios"
)

// stubLister is an injectable deviceLister for UpdateTunnels tests.
type stubLister struct {
	mu      sync.Mutex
	devices []string
}

func (s *stubLister) set(udids ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices = udids
}

func (s *stubLister) ListDevices() (ios.DeviceList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var dl ios.DeviceList
	for _, u := range s.devices {
		var e ios.DeviceEntry
		e.Properties.SerialNumber = u
		dl.DeviceList = append(dl.DeviceList, e)
	}
	return dl, nil
}

// fakeTunnel tracks one started tunnel so the test can kill its lifeline and
// assert it was closed.
type fakeTunnel struct {
	done   chan struct{}
	closed atomic.Bool
}

// stubStarter is an injectable tunnelStarter that hands back a controllable
// Tunnel per start (Address tagged tunnel-N), so the test can verify rebuilds.
type stubStarter struct {
	mu      sync.Mutex
	starts  int
	tunnels []*fakeTunnel
}

func (s *stubStarter) StartTunnel(_ context.Context, device ios.DeviceEntry, _ PairRecordManager, _ *semver.Version, _ bool) (Tunnel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	ft := &fakeTunnel{done: make(chan struct{})}
	s.tunnels = append(s.tunnels, ft)
	return Tunnel{
		Udid:         device.Properties.SerialNumber,
		Address:      fmt.Sprintf("tunnel-%d", s.starts),
		UserspaceTUN: true,
		done:         ft.done,
		closer:       func() error { ft.closed.Store(true); return nil },
	}, nil
}

// TestUpdateTunnels_RebuildsDeadTunnel is the regression guard for the tunnel
// self-heal: a tunnel whose lifeline to the device has died must be torn down
// and rebuilt by the next reconcile, instead of persisting as a zombie that
// black-holes every new connection until the daemon restarts. A healthy
// tunnel must NOT be rebuilt, and a vanished device's tunnel must be removed.
func TestUpdateTunnels_RebuildsDeadTunnel(t *testing.T) {
	dl := &stubLister{}
	dl.set("X")
	ts := &stubStarter{}

	m := NewTunnelManager(PairRecordManager{}, true)
	m.dl = dl
	m.ts = ts
	m.getVersion = func(ios.DeviceEntry) (*semver.Version, error) {
		return semver.MustParse("17.5.0"), nil
	}
	ctx := context.Background()

	// 1. First reconcile establishes a tunnel.
	if err := m.UpdateTunnels(ctx); err != nil {
		t.Fatalf("UpdateTunnels #1: %v", err)
	}
	if ts.starts != 1 {
		t.Fatalf("starts = %d, want 1 after first reconcile", ts.starts)
	}
	if got := m.tunnels["X"]; got.Address != "tunnel-1" || !got.IsAlive() {
		t.Fatalf("after #1: tunnel = %+v, want live tunnel-1", got)
	}

	// 2. A healthy tunnel is left alone — no churn.
	if err := m.UpdateTunnels(ctx); err != nil {
		t.Fatalf("UpdateTunnels #2: %v", err)
	}
	if ts.starts != 1 {
		t.Errorf("starts = %d after healthy reconcile, want 1 (no rebuild of a live tunnel)", ts.starts)
	}

	// 3. Lifeline dies → next reconcile tears down and rebuilds.
	close(ts.tunnels[0].done)
	if err := m.UpdateTunnels(ctx); err != nil {
		t.Fatalf("UpdateTunnels #3: %v", err)
	}
	if ts.starts != 2 {
		t.Fatalf("starts = %d, want 2 (dead tunnel rebuilt)", ts.starts)
	}
	if !ts.tunnels[0].closed.Load() {
		t.Error("dead tunnel was not closed before rebuild")
	}
	if got := m.tunnels["X"]; got.Address != "tunnel-2" || !got.IsAlive() {
		t.Fatalf("after rebuild: tunnel = %+v, want live tunnel-2", got)
	}

	// 4. Device disappears → tunnel removed.
	dl.set()
	if err := m.UpdateTunnels(ctx); err != nil {
		t.Fatalf("UpdateTunnels #4: %v", err)
	}
	if _, ok := m.tunnels["X"]; ok {
		t.Error("tunnel for a vanished device was not removed")
	}
	if !ts.tunnels[1].closed.Load() {
		t.Error("removed tunnel was not closed")
	}
}
