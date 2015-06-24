package bmdtest

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/monetas/bmutil/wire"
)

// The report that the MockConnection sends when its interaction is complete.
// If the error is nil, then the test has completed successfully.
// The report also includes any object messages that were sent to the peer.
type TestReport struct {
	Err      error
	DataSent []*wire.ShaHash
}

// MockPeer implements the peer.Connection interface and
// is a mock peer to be used for testing purposes.
type MockPeer struct {
	// After a message is processed, the replies are sent here.
	reply chan []wire.Message

	// A channel used to report invalid states to the test.
	report chan TestReport

	// A function that manages the sequence of steps that the test should go through.
	// This is the part that is customized for each test.
	//handle     func(wire.Message) *PeerAction
	peerTest PeerTest

	// A list of hashes of objects that have been sent to the real peer.
	objectData []*wire.ShaHash

	// A queue of messages to be received by the real peer.
	msgQueue []wire.Message

	// The current place in the queue.
	queuePlace int

	// The test ends when the timer runs out. Until the interaction is complete,
	// the timer is reset with every message received. Some time is required after
	// the mock peer is done interacting with the real one to see if the real peer
	// disconnects or not.
	timer *time.Timer

	// when interactionComplete is set, the mock peer no longer processes incoming
	// messages or resets the timer. It is just waiting to see whether the real peer
	// disconnects.
	interactionComplete bool

	// Whether the peer is expected to disconnect.
	disconnectExpected bool

	localAddr  net.Addr
	remoteAddr net.Addr

	// Whether the report has already been submitted.
	reported int32

	// A mutex for data that might be accessed by different threads.
	mutex sync.RWMutex
}

func (mock *MockPeer) ReadMessage() (wire.Message, error) {
	// If the queue is empty, get a new message from the channel.
	for mock.msgQueue == nil || mock.queuePlace >= len(mock.msgQueue) {
		mock.msgQueue = <-mock.reply
		mock.queuePlace = 0
	}

	toSend := mock.msgQueue[mock.queuePlace]

	mock.queuePlace++

	if mock.queuePlace >= len(mock.msgQueue) {
		mock.msgQueue = nil
	}

	switch t := toSend.(type) {
	case *wire.MsgGetPubKey, *wire.MsgPubKey, *wire.MsgMsg, *wire.MsgBroadcast, *wire.MsgUnknownObject, *wire.MsgObject:
		msg, _ := wire.ToMsgObject(t)
		mock.objectData = append(mock.objectData, msg.InventoryHash())
	}

	return toSend, nil
}

// WriteMessage figures out how to respond to a message once it is decyphered.
func (mock *MockPeer) WriteMessage(rmsg wire.Message) error {
	// We can keep receiving messages after the interaction is done; we just
	// ignore them.	 We are waiting to see whether the peer disconnects.
	mock.mutex.RLock()
	ic := mock.interactionComplete
	mock.mutex.RUnlock()
	if ic {
		return nil
	}

	mock.mutex.Lock()
	mock.timer.Reset(time.Second)
	mock.mutex.Unlock()
	mock.handleAction(mock.handleMessage(rmsg))
	return nil
}

func (mock *MockPeer) RequestData(invVect []*wire.InvVect) error {
	// We can keep receiving messages after the interaction is done; we just ignore them.
	// We are waiting to see whether the peer disconnects.
	mock.mutex.RLock()
	ic := mock.interactionComplete
	mock.mutex.RUnlock()
	if ic {
		return nil
	}

	mock.mutex.Lock()
	mock.timer.Reset(time.Second)
	mock.mutex.Unlock()
	mock.handleAction(mock.peerTest.OnSendData(invVect))
	return nil
}

func (mock *MockPeer) BytesWritten() uint64 {
	return 0
}

func (mock *MockPeer) BytesRead() uint64 {
	return 0
}

func (mock *MockPeer) LastWrite() time.Time {
	return time.Time{}
}

func (mock *MockPeer) LastRead() time.Time {
	return time.Time{}
}

// LocalAddr returns the localAddr field of the fake connection and satisfies
// the net.Conn interface.
func (mock *MockPeer) LocalAddr() net.Addr {
	return mock.localAddr
}

// RemoteAddr returns the remoteAddr field of the fake connection and satisfies
// the net.Conn interface.
func (mock *MockPeer) RemoteAddr() net.Addr {
	return mock.remoteAddr
}

func (mock *MockPeer) Close() {
	mock.ConnectionClosed()
}

func (mock *MockPeer) Connected() bool {
	return true
}

func (mock *MockPeer) Connect() error {
	return errors.New("Already connected.")
}

func (mock *MockPeer) handleMessage(msg wire.Message) *PeerAction {
	switch msg.(type) {
	case *wire.MsgVersion:
		return mock.peerTest.OnMsgVersion(msg.(*wire.MsgVersion))
	case *wire.MsgVerAck:
		return mock.peerTest.OnMsgVerAck(msg.(*wire.MsgVerAck))
	case *wire.MsgAddr:
		return mock.peerTest.OnMsgAddr(msg.(*wire.MsgAddr))
	case *wire.MsgInv:
		return mock.peerTest.OnMsgInv(msg.(*wire.MsgInv))
	case *wire.MsgGetData:
		return mock.peerTest.OnMsgGetData(msg.(*wire.MsgGetData))
	default:
		return nil
	}
}

func (mock *MockPeer) handleAction(action *PeerAction) {
	// A nil response means to do nothing.
	if action == nil {
		return
	}

	// If an error is returned, immediately end the interaction.
	if action.Err != nil {
		mock.Done(action.Err)
		return
	}

	mock.mutex.Lock()
	mock.interactionComplete = mock.interactionComplete || action.InteractionComplete

	mock.disconnectExpected = mock.disconnectExpected || action.DisconnectExpected
	mock.mutex.Unlock()

	if action.Messages != nil {

		mock.reply <- action.Messages
	}
}

// ConnectionClosed is called when the real peer closes the connection to the mock peer.
func (mock *MockPeer) ConnectionClosed() {
	mock.mutex.RLock()
	ic := mock.interactionComplete
	mock.mutex.RUnlock()
	if !ic &&
		(!mock.disconnectExpected ||
			(mock.msgQueue != nil && mock.queuePlace < len(mock.msgQueue))) {
		mock.Done(errors.New("Connection closed prematurely."))
	}
	mock.Done(nil)
}

// Done stops the server and ends the test.
func (mock *MockPeer) Done(err error) {
	if atomic.AddInt32(&mock.reported, 1) > 1 {
		// The report has already been submitted.
		return
	}
	mock.mutex.Lock()
	mock.timer.Stop()
	mock.interactionComplete = true
	mock.mutex.Unlock()
	mock.report <- TestReport{err, mock.objectData}
}

// Start loads the mock peer's initial action if there is one.
func (mock *MockPeer) BeginTest() {
	action := mock.peerTest.OnStart()

	if action == nil {
		return
	}

	mock.mutex.Lock()
	mock.interactionComplete = mock.interactionComplete || action.InteractionComplete

	mock.disconnectExpected = mock.disconnectExpected || action.DisconnectExpected
	mock.mutex.Unlock()

	if action.Messages == nil {
		return
	}

	if mock.msgQueue == nil {
		mock.msgQueue = action.Messages
		mock.queuePlace = 0
	}
}

func NewMockPeer(localAddr, remoteAddr net.Addr, report chan TestReport, peerTest PeerTest) *MockPeer {
	mock := &MockPeer{
		localAddr:           localAddr,
		remoteAddr:          remoteAddr,
		report:              report,
		peerTest:            peerTest,
		interactionComplete: false,
		disconnectExpected:  false,
		reply:               make(chan []wire.Message),
		objectData:          make([]*wire.ShaHash, 0),
	}

	mock.mutex.Lock()
	mock.timer = time.AfterFunc(time.Millisecond*100, func() {
		mock.mutex.RLock()
		ic := mock.interactionComplete
		mock.mutex.RUnlock()
		if !ic {
			if mock.disconnectExpected {
				mock.Done(errors.New("Peer stopped interacting when it was expected to disconnect."))
			} else {
				mock.Done(errors.New("Peer stopped interacting (without disconnecting) before the interaction was complete."))
			}
		} else {
			mock.Done(nil)
		}
	})
	mock.mutex.Unlock()

	mock.BeginTest()

	return mock
}
