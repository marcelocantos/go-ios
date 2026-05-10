package dtx_test

import (
	"encoding/binary"
	"testing"
	"time"

	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureDispatcher records the messages it receives, for verifying that
// GlobalDispatcher forwards correctly to dtxConn.MessageDispatcher.
type captureDispatcher struct {
	got chan dtx.Message
}

func (c captureDispatcher) Dispatch(msg dtx.Message) {
	c.got <- msg
}

// DTX auxiliary binary format per entry: [4-byte type (null=0x0A)] [4-byte type] [value...]
// See dtxprimitivedictionary.go for the full encoding spec.

const (
	typeNull      = 0x0A
	typeByteArray = 0x02
	typeUint32    = 0x03
)

func nullKey() []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, typeNull)
	return b
}

func uint32Value(v uint32) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], typeUint32)
	binary.LittleEndian.PutUint32(b[4:8], v)
	return b
}

func bytesValue(data []byte) []byte {
	b := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(b[0:4], typeByteArray)
	binary.LittleEndian.PutUint32(b[4:8], uint32(len(data)))
	copy(b[8:], data)
	return b
}

func buildAuxEntry(value []byte) []byte {
	return append(nullKey(), value...)
}

func outputReceivedMessage(aux []byte) dtx.Message {
	return dtx.Message{
		Payload:   []interface{}{"outputReceived:fromProcess:atTime:"},
		Auxiliary: dtx.DecodeAuxiliary(aux),
	}
}

func TestDispatch_OutputReceived_EmptyArguments_NoPanic(t *testing.T) {
	// Reproduces the iOS 26 crash: device sends outputReceived with no arguments.
	// Before the fix, this panicked with: index out of range [2] with length 0
	dispatcher := dtx.NewGlobalDispatcher(make(chan dtx.Message, 1), nil)
	msg := outputReceivedMessage(nil)

	assert.NotPanics(t, func() {
		dispatcher.Dispatch(msg)
	})
}

func TestDispatch_OutputReceived_InsufficientArguments_NoPanic(t *testing.T) {
	// Only 1 argument provided, but code expects 3 (message, pid, time)
	aux := buildAuxEntry(bytesValue([]byte("hello")))
	dispatcher := dtx.NewGlobalDispatcher(make(chan dtx.Message, 1), nil)
	msg := outputReceivedMessage(aux)

	assert.NotPanics(t, func() {
		dispatcher.Dispatch(msg)
	})
}

func TestDispatch_OutputReceived_WrongArgumentType_NoPanic(t *testing.T) {
	// 3 arguments provided, but first is uint32 instead of expected []byte
	var aux []byte
	aux = append(aux, buildAuxEntry(uint32Value(42))...)
	aux = append(aux, buildAuxEntry(uint32Value(43))...)
	aux = append(aux, buildAuxEntry(uint32Value(44))...)

	dispatcher := dtx.NewGlobalDispatcher(make(chan dtx.Message, 1), nil)
	msg := outputReceivedMessage(aux)

	assert.NotPanics(t, func() {
		dispatcher.Dispatch(msg)
	})
}

// TestDispatch_Methodinvocation_ForwardedToMessageDispatcher exercises the
// iOS-17+ RSD path: BackBoard's instruments-channel callbacks (e.g.
// applicationStateNotification:) arrive on the global channel as
// Methodinvocation messages and must be forwarded to the connection's
// MessageDispatcher. Without this branch every BackBoard notification is
// silently dropped — the symptom the underlying fix addresses.
func TestDispatch_Methodinvocation_ForwardedToMessageDispatcher(t *testing.T) {
	captured := captureDispatcher{got: make(chan dtx.Message, 1)}
	conn := &dtx.Connection{MessageDispatcher: captured}
	dispatcher := dtx.NewGlobalDispatcher(make(chan dtx.Message, 1), conn)

	msg := dtx.Message{
		PayloadHeader: dtx.PayloadHeader{MessageType: dtx.Methodinvocation},
		Payload:       []interface{}{"applicationStateNotification:"},
	}
	dispatcher.Dispatch(msg)

	select {
	case got := <-captured.got:
		require.NotNil(t, got.Payload)
		require.Len(t, got.Payload, 1)
		assert.Equal(t, "applicationStateNotification:", got.Payload[0])
	case <-time.After(time.Second):
		t.Fatal("Methodinvocation was not forwarded to the connection's MessageDispatcher")
	}
}

// TestDispatch_Methodinvocation_NilMessageDispatcher_NoPanic exercises the
// degenerate case where the global-channel forward fires but no downstream
// MessageDispatcher is registered. Connection.Dispatch logs the drop
// rather than crashing; this test pins that contract.
func TestDispatch_Methodinvocation_NilMessageDispatcher_NoPanic(t *testing.T) {
	conn := &dtx.Connection{} // MessageDispatcher == nil
	dispatcher := dtx.NewGlobalDispatcher(make(chan dtx.Message, 1), conn)

	msg := dtx.Message{
		PayloadHeader: dtx.PayloadHeader{MessageType: dtx.Methodinvocation},
		Payload:       []interface{}{"applicationStateNotification:"},
	}
	assert.NotPanics(t, func() {
		dispatcher.Dispatch(msg)
	})
}
