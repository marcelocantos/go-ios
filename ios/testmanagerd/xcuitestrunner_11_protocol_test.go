package testmanagerd

import "testing"

func TestXcode11HandshakeUsesNegotiatedProtocolVersion(t *testing.T) {
	got := xcode11HandshakeProtocolVersion(36)
	if got == 25 {
		t.Fatal("handshake still uses hardcoded protocolVersion 25, want the negotiated reply")
	}
	if got != 36 {
		t.Fatalf("xcode11HandshakeProtocolVersion(36) = %d, want 36", got)
	}
}
