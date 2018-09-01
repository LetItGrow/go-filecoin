package chain

import (
	"fmt"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	"github.com/filecoin-project/go-filecoin/types"
)

// NewMessageForTestGetter returns a closure that returns a message unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
func NewMessageForTestGetter() func() *Message {
	i := 0
	return func() *Message {
		s := fmt.Sprintf("msg%d", i)
		i++
		return NewMessage(
			address.NewMainnet([]byte(s+"-from")),
			address.NewMainnet([]byte(s+"-to")),
			0,
			nil,
			s,
			nil)
	}
}

// NewBlockForTest returns a new block. If a parent block is provided, the returned
// block will be configured as if it were a child of that parent. The returned block
// has not been persisted into the store.
func NewBlockForTest(parent *Block, nonce uint64) *Block {
	block := &Block{
		Nonce:           types.Uint64(nonce),
		Messages:        []*SignedMessage{},
		MessageReceipts: []*MessageReceipt{},
	}

	if parent != nil {
		block.Height = parent.Height + 1
		block.StateRoot = parent.StateRoot
		block.Parents.Add(parent.Cid())
	}

	return block
}

// NewMsgs returns n messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewMsgs will return the same
// set of messages).
func NewMsgs(n int) []*Message {
	newMsg := NewMessageForTestGetter()
	msgs := make([]*Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
	}
	return msgs
}

// NewSignedMsgs returns n signed messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewSignedMsgs will return the same
// set of messages).
func NewSignedMsgs(n int, ms crypto.MockSigner) []*SignedMessage {
	newSmsg := NewSignedMessageForTestGetter(ms)
	smsgs := make([]*SignedMessage, n)
	for i := 0; i < n; i++ {
		smsgs[i] = newSmsg()
	}
	return smsgs
}

// SignMsgs returns a slice of signed messages where the original messages
// are `msgs`, if signing one of the `msgs` fails an error is returned
func SignMsgs(ms crypto.MockSigner, msgs []*Message) ([]*SignedMessage, error) {
	var smsgs []*SignedMessage
	for _, m := range msgs {
		s, err := NewSignedMessage(*m, &ms)
		if err != nil {
			return nil, err
		}
		smsgs = append(smsgs, s)
	}
	return smsgs, nil
}

// MsgCidsEqual returns true if the message cids are equal. It panics if
// it can't get their cid.
func MsgCidsEqual(m1, m2 *Message) bool {
	m1Cid, err := m1.Cid()
	if err != nil {
		panic(err)
	}
	m2Cid, err := m2.Cid()
	if err != nil {
		panic(err)
	}
	return m1Cid.Equals(m2Cid)
}

// SmsgCidsEqual returns true if the SignedMessage cids are equal. It panics if
// it can't get their cid.
func SmsgCidsEqual(m1, m2 *SignedMessage) bool {
	m1Cid, err := m1.Cid()
	if err != nil {
		panic(err)
	}
	m2Cid, err := m2.Cid()
	if err != nil {
		panic(err)
	}
	return m1Cid.Equals(m2Cid)
}

// NewMsgsWithAddrs returns a slice of `n` messages who's `From` field's are pulled
// from `a`. This method should be used when the addresses returned are to be signed
// at a later point.
func NewMsgsWithAddrs(n int, a []address.Address) []*Message {
	if n > len(a) {
		panic("cannot create more messages than there are addresess for")
	}
	newMsg := NewMessageForTestGetter()
	msgs := make([]*Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
		msgs[i].From = a[i]
	}
	return msgs
}

// SomeCid generates a Cid for use in tests where you want a Cid but don't care
// what it is.
func SomeCid() *cid.Cid {
	b := &Block{}
	return b.Cid()
}

// NewCidForTestGetter returns a closure that returns a Cid unique to that invocation.
// The Cid is unique wrt the closure returned, not globally. You can use this function
// in tests.
func NewCidForTestGetter() func() *cid.Cid {
	i := types.Uint64(31337)
	return func() *cid.Cid {
		b := &Block{Height: i}
		i++
		return b.Cid()
	}
}

// NewSignedMessageForTestGetter returns a closure that returns a SignedMessage unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
// TODO support chosing from address
func NewSignedMessageForTestGetter(ms crypto.MockSigner) func() *SignedMessage {
	i := 0
	return func() *SignedMessage {
		s := fmt.Sprintf("smsg%d", i)
		i++
		msg := NewMessage(
			ms.Addresses[0], // from needs to be an address from the signer
			address.NewMainnet([]byte(s+"-to")),
			0,
			types.NewAttoFILFromFIL(0),
			s,
			[]byte("params"))
		smsg, err := NewSignedMessage(*msg, &ms)
		if err != nil {
			panic(err)
		}
		return smsg
	}
}