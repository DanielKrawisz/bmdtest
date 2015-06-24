package bmdtest

import (
	"errors"
	"fmt"
	"sync"

	"github.com/monetas/bmutil/wire"
)

// PeerAction represents a an action to be taken by the mock peer and information
// about the expected response from the real peer.
type PeerAction struct {
	// A series of messages to send to the real peer.
	Messages []wire.Message

	// If an error is set, the interaction ends immediately.
	Err error

	// Whether the interaction is complete.
	InteractionComplete bool

	// For negative tests, we expect the peer to disconnect after the interaction is
	// complete. If disconnectExpected is set, the test fails if the real peer fails
	// to disconnect after half a second.
	DisconnectExpected bool
}

// PeerTest is a type that defines the high-level behavior of a peer.
type PeerTest interface {
	OnStart() *PeerAction
	OnMsgVersion(p *wire.MsgVersion) *PeerAction
	OnMsgVerAck(p *wire.MsgVerAck) *PeerAction
	OnMsgAddr(p *wire.MsgAddr) *PeerAction
	OnMsgInv(p *wire.MsgInv) *PeerAction
	OnMsgGetData(p *wire.MsgGetData) *PeerAction
	OnSendData(invVect []*wire.InvVect) *PeerAction
}

// OutboundHandshakePeerTester is an implementation of PeerTest that is for
// testing the handshake with an outbound peer.
type OutboundHandshakePeerTester struct {
	versionReceived   bool
	handshakeComplete bool
	response          *PeerAction
	msgAddr           wire.Message
	mutex             sync.RWMutex
}

func (peer *OutboundHandshakePeerTester) OnStart() *PeerAction {
	return nil
}

func (peer *OutboundHandshakePeerTester) VersionReceived() bool {
	peer.mutex.RLock()
	defer peer.mutex.RUnlock()

	return peer.versionReceived
}

func (peer *OutboundHandshakePeerTester) HandshakeComplete() bool {
	peer.mutex.RLock()
	defer peer.mutex.RUnlock()

	return peer.handshakeComplete
}

func (peer *OutboundHandshakePeerTester) OnMsgVersion(version *wire.MsgVersion) *PeerAction {
	if peer.VersionReceived() {
		return &PeerAction{Err: errors.New("Two version messages received")}
	}
	peer.versionReceived = true
	return peer.response
}

func (peer *OutboundHandshakePeerTester) OnMsgVerAck(p *wire.MsgVerAck) *PeerAction {
	if !peer.VersionReceived() {
		return &PeerAction{Err: errors.New("Expecting version message first.")}
	}
	peer.mutex.Lock()
	peer.handshakeComplete = true
	peer.mutex.Unlock()
	return &PeerAction{
		Messages:            []wire.Message{peer.msgAddr},
		InteractionComplete: true,
		DisconnectExpected:  false,
	}
}

func (peer *OutboundHandshakePeerTester) OnMsgAddr(p *wire.MsgAddr) *PeerAction {
	if peer.HandshakeComplete() {
		return nil
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *OutboundHandshakePeerTester) OnMsgInv(p *wire.MsgInv) *PeerAction {
	if peer.HandshakeComplete() {
		return nil
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *OutboundHandshakePeerTester) OnMsgGetData(p *wire.MsgGetData) *PeerAction {
	if peer.HandshakeComplete() {
		return nil
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *OutboundHandshakePeerTester) OnSendData(invVect []*wire.InvVect) *PeerAction {
	if peer.HandshakeComplete() {
		return nil
	}
	return &PeerAction{Err: errors.New("Object message not allowed before handshake is complete.")}
}

func NewOutboundHandshakePeerTester(action *PeerAction, msgAddr wire.Message) *OutboundHandshakePeerTester {
	return &OutboundHandshakePeerTester{
		versionReceived:   false,
		handshakeComplete: false,
		response:          action,
		msgAddr:           msgAddr,
	}
}

// InboundHandsakePeerTester implements the PeerTest interface and is used to test
// handshakes with inbound peers and the exchange of the addr messages.
type InboundHandshakePeerTester struct {
	verackReceived    bool
	versionReceived   bool
	handshakeComplete bool
	openMsg           *PeerAction
	addrAction        *PeerAction
}

func (peer *InboundHandshakePeerTester) OnStart() *PeerAction {
	return peer.openMsg
}

func (peer *InboundHandshakePeerTester) OnMsgVersion(version *wire.MsgVersion) *PeerAction {
	if peer.versionReceived {
		return &PeerAction{Err: errors.New("Two versions received?")}
	}
	peer.versionReceived = true
	if peer.verackReceived {
		// Handshake completed successfully
		peer.handshakeComplete = true

		// Sending an addr message to make sure the peer doesn't disconnect.
		return peer.addrAction
	}
	return &PeerAction{[]wire.Message{&wire.MsgVerAck{}}, nil, false, false}
}

func (peer *InboundHandshakePeerTester) OnMsgVerAck(p *wire.MsgVerAck) *PeerAction {
	if peer.verackReceived {
		return &PeerAction{Err: errors.New("Two veracks received?")}
	}
	peer.verackReceived = true
	if peer.versionReceived {
		// Handshake completed successfully
		peer.handshakeComplete = true

		//Sending an addr message to make sure the peer doesn't disconnect.
		return peer.addrAction
	}
	// Return nothing and await a version message.
	return &PeerAction{nil, nil, false, false}
}

func (peer *InboundHandshakePeerTester) OnMsgAddr(p *wire.MsgAddr) *PeerAction {
	if peer.handshakeComplete {
		return &PeerAction{nil, nil, true, false}
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *InboundHandshakePeerTester) OnMsgInv(p *wire.MsgInv) *PeerAction {
	if peer.handshakeComplete {
		return nil
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *InboundHandshakePeerTester) OnMsgGetData(p *wire.MsgGetData) *PeerAction {
	if peer.handshakeComplete {
		return nil
	}
	return &PeerAction{Err: errors.New("Not allowed before handshake is complete.")}
}

func (peer *InboundHandshakePeerTester) OnSendData(invVect []*wire.InvVect) *PeerAction {
	if peer.handshakeComplete {
		return nil
	}
	return &PeerAction{Err: errors.New("Object message not allowed before handshake is complete.")}
}

func NewInboundHandshakePeerTester(openingMsg *PeerAction, addrAction *PeerAction) *InboundHandshakePeerTester {
	return &InboundHandshakePeerTester{
		verackReceived:    false,
		versionReceived:   false,
		handshakeComplete: false,
		openMsg:           openingMsg,
		addrAction:        addrAction,
	}
}

// DataExchangePeerTester implements the PeerTest interface and tests the exchange
// of inv messages and data.
type DataExchangePeerTester struct {
	dataSent      bool
	dataReceived  bool
	invReceived   bool
	invAction     *PeerAction
	inventory     map[wire.InvVect]*wire.MsgObject // The initial inventory of the mock peer.
	peerInventory map[wire.InvVect]struct{}        // The initial inventory of the real peer.
	requested     map[wire.InvVect]struct{}        // The messages that were requested.
}

func (peer *DataExchangePeerTester) OnStart() *PeerAction {
	return peer.invAction
}

func (peer *DataExchangePeerTester) OnMsgVersion(version *wire.MsgVersion) *PeerAction {
	return &PeerAction{Err: errors.New("This test should begin with handshake already done.")}
}

func (peer *DataExchangePeerTester) OnMsgVerAck(p *wire.MsgVerAck) *PeerAction {
	return &PeerAction{Err: errors.New("This test should begin with handshake already done.")}
}

func (peer *DataExchangePeerTester) OnMsgAddr(p *wire.MsgAddr) *PeerAction {
	return nil
}

func (peer *DataExchangePeerTester) OnMsgInv(inv *wire.MsgInv) *PeerAction {
	if peer.invReceived {
		return &PeerAction{Err: errors.New("Inv message allready received.")}
	}

	peer.invReceived = true

	if len(inv.InvList) == 0 {
		return &PeerAction{Err: errors.New("Empty inv message received.")}
	}

	if len(inv.InvList) > wire.MaxInvPerMsg {
		return &PeerAction{Err: errors.New("Excessively long inv message received.")}
	}

	// Return a get data message that requests the entries in inv which are not already known.
	i := 0
	var ok bool
	duplicate := make(map[wire.InvVect]struct{})
	newInvList := make([]*wire.InvVect, len(inv.InvList))
	for _, iv := range inv.InvList {
		if _, ok = peer.inventory[*iv]; !ok {
			if _, ok = duplicate[*iv]; ok {
				return &PeerAction{Err: errors.New("Inv with duplicates received.")}
			}
			duplicate[*iv] = struct{}{}
			newInvList[i] = iv
			peer.requested[*iv] = struct{}{}
			i++
		}
	}

	if i == 0 {
		return nil
	}

	return &PeerAction{
		Messages: []wire.Message{&wire.MsgGetData{InvList: newInvList[:i]}},
	}
}

func (peer *DataExchangePeerTester) OnMsgGetData(getData *wire.MsgGetData) *PeerAction {

	if len(getData.InvList) == 0 {
		return &PeerAction{Err: errors.New("Empty GetData message received.")}
	}

	if len(getData.InvList) > wire.MaxInvPerMsg {
		return &PeerAction{Err: errors.New("Excessively long GetData message received.")}
	}

	// The getdata message should include no duplicates and should include nothing
	// that the peer already knows, and nothing that the mock peer doesn't know.
	i := 0
	duplicate := make(map[wire.InvVect]struct{})
	messages := make([]wire.Message, len(getData.InvList))
	for _, iv := range getData.InvList {
		msg, ok := peer.inventory[*iv]
		if !ok {
			return &PeerAction{Err: errors.New("GetData asked for something we don't know.")}
		}

		if _, ok = duplicate[*iv]; ok {
			return &PeerAction{Err: errors.New("GetData with duplicates received.")}
		}

		duplicate[*iv] = struct{}{}
		messages[i] = msg
		peer.peerInventory[*iv] = struct{}{}
		i++
	}

	peer.dataSent = true

	fmt.Println("SENDING OBJECT MESSAGE")
	if peer.dataReceived {
		return &PeerAction{messages[:i], nil, true, false}
	}
	return &PeerAction{messages[:i], nil, false, false}
}

func (peer *DataExchangePeerTester) OnSendData(invVect []*wire.InvVect) *PeerAction {
	if !peer.invReceived {
		return &PeerAction{Err: errors.New("Object message not allowed before exchange of invs.")}
	}

	// The objects must have been requested.
	for _, iv := range invVect {
		if _, ok := peer.requested[*iv]; !ok {
			return &PeerAction{Err: errors.New("Object message not allowed before handshake is complete.")}
		}
		delete(peer.requested, *iv)
	}

	if len(peer.requested) == 0 {
		peer.dataReceived = true
	}

	if peer.dataReceived && peer.dataSent {
		return &PeerAction{nil, nil, true, false}
	}
	return &PeerAction{nil, nil, false, false}
}

func NewDataExchangePeerTester(inventory []*wire.MsgObject, peerInventory []*wire.MsgObject, invAction *PeerAction) *DataExchangePeerTester {
	// Catalog the initial inventories of the mock peer and real peer.
	in := make(map[wire.InvVect]*wire.MsgObject)
	pin := make(map[wire.InvVect]struct{})
	invMsg := wire.NewMsgInv()

	// Construct the real peer's inventory.
	for _, message := range inventory {
		inv := wire.InvVect{Hash: *message.InventoryHash()}
		invMsg.AddInvVect(&inv)
		in[inv] = message
	}

	// Construct the mock peer's inventory.
	for _, message := range peerInventory {
		inv := wire.InvVect{Hash: *message.InventoryHash()}
		pin[inv] = struct{}{}
	}

	dataSent := true
	dataReceived := true

	// Does the real peer have any inventory that the mock peer does not have?
	for inv := range in {
		if _, ok := pin[inv]; !ok {
			dataSent = false
			break
		}
	}

	// Does the mock peer have any inventory that the real peer does not?
	for inv := range pin {
		if _, ok := in[inv]; !ok {
			dataReceived = false
			break
		}
	}

	var inva *PeerAction
	if invAction == nil {
		if len(invMsg.InvList) == 0 {
			inva = &PeerAction{
				Messages:            []wire.Message{&wire.MsgVerAck{}},
				InteractionComplete: dataSent && dataReceived,
			}
		} else {
			inva = &PeerAction{
				Messages:            []wire.Message{&wire.MsgVerAck{}, invMsg},
				InteractionComplete: dataSent && dataReceived,
			}
		}
	} else {
		inva = invAction
	}

	return &DataExchangePeerTester{
		dataSent:      dataSent,
		dataReceived:  dataReceived,
		inventory:     in,
		peerInventory: pin,
		invAction:     inva,
		requested:     make(map[wire.InvVect]struct{}),
	}
}
