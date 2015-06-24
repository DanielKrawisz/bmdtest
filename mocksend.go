package bmdtest

import (
	"github.com/monetas/bmd/peer"
	"github.com/monetas/bmutil/wire"
)

type MockSend struct {
	conn         *MockPeer
	msgQueue     chan wire.Message
	requestQueue chan []*wire.InvVect
	quit         chan struct{}
}

func (msq *MockSend) QueueMessage(msg wire.Message) error {
	msq.msgQueue <- msg
	return nil
}

func (msq *MockSend) QueueDataRequest(inv []*wire.InvVect) error {
	msq.requestQueue <- inv
	return nil
}

func (msq *MockSend) QueueInventory([]*wire.InvVect) error {
	return nil
}

// Start ignores its input here because we need a MockConnection, which has some
// extra functions that the regular Connection does not have.
func (msq *MockSend) Start(conn peer.Connection) {
	go msq.handler()
}

func (msq *MockSend) Running() bool {
	return true
}

func (msq *MockSend) Stop() {
	close(msq.quit)
}

// Must be run as a go routine.
func (msq *MockSend) handler() {
out:
	for {
		select {
		case <-msq.quit:
			break out
		case inv := <-msq.requestQueue:
			msq.conn.RequestData(inv)
		case msg := <-msq.msgQueue:
			msq.conn.WriteMessage(msg)
		}
	}
}

func NewMockSend(mockConn *MockPeer) *MockSend {
	return &MockSend{
		conn:         mockConn,
		msgQueue:     make(chan wire.Message, 1),
		requestQueue: make(chan []*wire.InvVect, 1),
		quit:         make(chan struct{}),
	}
}
