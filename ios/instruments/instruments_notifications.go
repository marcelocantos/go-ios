package instruments

import (
	"fmt"
	"io"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	dtx "github.com/danielpaulus/go-ios/ios/dtx_codec"
	log "github.com/sirupsen/logrus"
)

type channelDispatcher struct {
	messageChannel chan dtx.Message
	closeChannel   chan struct{}
}

func ListenAppStateNotifications(device ios.DeviceEntry) (func() (map[string]interface{}, error), func() error, error) {
	conn, err := connectInstruments(device)
	if err != nil {
		return nil, nil, err
	}
	// messageChannel is buffered so that BackBoard's initial state-
	// enumeration burst (one entry per managed app, can be 30+ on
	// iPad-class devices) can land before the caller starts draining
	// via Receive(). An unbuffered channel here deadlocks the DTX
	// reader goroutine the moment the first notification arrives.
	dispatcher := channelDispatcher{messageChannel: make(chan dtx.Message, 256), closeChannel: make(chan struct{})}
	conn.AddDefaultChannelReceiver(dispatcher)
	// On iOS-17+ (RSD / dtservicehub) BackBoard's applicationStateNotification:
	// callbacks arrive on the global channel (code 0) as Methodinvocation
	// messages, not on the assigned mobilenotifications channel. Setting
	// MessageDispatcher routes the global-channel forward (see
	// GlobalDispatcher.Dispatch) into our handler.
	conn.MessageDispatcher = dispatcher
	// Pass the same populated dispatcher to the per-channel route. On the
	// iOS-17+ RSD path (com.apple.instruments.dtservicehub) BackBoard
	// dispatches applicationStateNotification: messages on the assigned
	// channel code rather than the default channel; passing a zero-value
	// channelDispatcher (with nil messageChannel) deadlocks the DTX
	// reader on `nil_chan <- msg` the moment any notification arrives.
	channel := conn.RequestChannelIdentifier(mobileNotificationsChannel, dispatcher)
	resp, err := channel.MethodCall("setApplicationStateNotificationsEnabled:", true)
	if err != nil {
		// resp.Payload may be empty when the channel-open RPC times out
		// before any response arrives — the original error path indexed
		// resp.Payload[0] unconditionally and panicked. Defend against it.
		log.Errorf("setApplicationStateNotificationsEnabled: failed: resp=%+v err=%v", resp, err)
		return nil, nil, err
	}
	log.Debugf("appstatenotifications enabled successfully: %+v", resp)
	resp, err = channel.MethodCall("setMemoryNotificationsEnabled:", true)
	if err != nil {
		log.Errorf("setMemoryNotificationsEnabled: failed: resp=%+v err=%v", resp, err)
		return nil, nil, err
	}
	log.Debugf("memory notifications enabled: %+v", resp)

	return dispatcher.Receive, dispatcher.Close, nil
}

func (dispatcher channelDispatcher) Receive() (map[string]interface{}, error) {
	for {
		select {
		case msg := <-dispatcher.messageChannel:
			selector, result, err := toMap(msg)
			if "applicationStateNotification:" == selector && err == nil {
				return result, nil
			}
			if err != nil {
				log.Debugf("error extracting message %+v, %v", msg, err)
			}
		case <-dispatcher.closeChannel:
			return map[string]interface{}{}, io.EOF
		}
	}
}

func (dispatcher *channelDispatcher) Close() error {
	select {
	case dispatcher.closeChannel <- struct{}{}:
		return nil
	case <-time.After(time.Second * 5):
		return fmt.Errorf("timeout")
	}
}

func (dispatcher channelDispatcher) Dispatch(msg dtx.Message) {
	// Defend against the zero-value dispatcher pattern: a nil channel
	// would block the DTX reader goroutine forever, taking the whole
	// connection down silently. Drop the message instead.
	if dispatcher.messageChannel == nil {
		return
	}
	select {
	case dispatcher.messageChannel <- msg:
	default:
		// Buffer full — drop rather than wedge the DTX reader.
	}
}
