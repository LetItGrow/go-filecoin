package address

import (
	"testing"

	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestSetRoundtrip(t *testing.T) {
	assert := assert.New(t)
	addrGetter := NewForTestGetter()

	addrs := make([]Address, 10)
	for i := range addrs {
		addrs[i] = addrGetter()
	}

	set := Set{}

	for _, addr := range addrs {
		set[addr] = struct{}{}
	}

	bytes, err := cbor.DumpObject(set)
	assert.NoError(err)

	var setBack Set
	assert.NoError(cbor.DecodeInto(bytes, &setBack))

	assert.Equal(len(addrs), len(setBack))
	for _, addr := range addrs {
		_, ok := setBack[addr]
		assert.True(ok)
	}
}
