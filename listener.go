package bmdtest

import (
	"errors"
	"net"

	"github.com/monetas/bmd/peer"
)

// MockListener implements the peer.Listener interface
type MockListener struct {
	incoming     chan peer.Connection
	disconnect   chan struct{}
	localAddr    net.Addr
	disconnected bool
}

func (ml *MockListener) Accept() (peer.Connection, error) {
	if ml.disconnected {
		return nil, errors.New("Listner disconnected.")
	}
	select {
	case <-ml.disconnect:
		return nil, errors.New("Listener disconnected.")
	case m := <-ml.incoming:
		return m, nil
	}
}

func (ml *MockListener) Close() error {
	ml.disconnect <- struct{}{}
	return nil
}

// Addr returns the listener's network address.
func (ml *MockListener) Addr() net.Addr {
	return ml.localAddr
}

func NewMockListener(localAddr net.Addr, incoming chan peer.Connection, disconnect chan struct{}) *MockListener {
	return &MockListener{
		incoming:   incoming,
		disconnect: disconnect,
		localAddr:  localAddr,
	}
}

// MockListen returns a mock listener
func MockListen(listeners []*MockListener) func(string, string) (peer.Listener, error) {
	i := 0

	return func(service, addr string) (peer.Listener, error) {
		i++
		if i > len(listeners) {
			return nil, errors.New("No mock listeners remaining.")
		}

		return listeners[i-1], nil
	}
}
