package address

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/bls-signatures"
)

// Network represents which network an address belongs to.
type Network = byte

const (
	// Mainnet is the main network.
	Mainnet Network = iota
	// Testnet is the test network.
	Testnet
)

// Type represents the type of data address data holds
type Type = byte

const (
	// SECP256K1 means the address is the hash of a secp256k1 public key
	SECP256K1 Type = iota
	// ID means the address is an actor ID
	ID
	// Actor means the address is an acotr address, which is a fixed address
	Actor
	// BLS means the address is a full BLS public key
	BLS
)

const (
	// length of address data containing a hash of Secp256k1 public key
	LEN_SECP256K1 = SecpHashLength
	// length of address data containing a hash of actor address //TODO better def
	LEN_Actor = SecpHashLength
	// length of address data containing an actor ID
	LEN_ID = 8
	// length of address data containing a BLS public key
	LEN_BLS = bls.PublicKeyBytes
)

var (
	// ErrUnknownNetwork is returned when encountering an unknown network in an address.
	ErrUnknownNetwork = errors.New("unknown network")
	// ErrUnknownType is returned when encountering an unknown address type.
	ErrUnknownType = errors.New("unknown type")
	// ErrInvalidBytes is returned when encountering an invalid byte format.
	ErrInvalidBytes = errors.New("invalid bytes")
	// ErrInvalidChecksum is returned when encountering an invalid checksum.
	ErrInvalidChecksum = errors.New("invalid checksum")
)

var (
	// TODO Should please stop using this pattern
	// TestAddress is an account with some initial funds in it
	TestAddress Address
	// TODO Should probably stop using this pattern
	// TestAddress2 is an account with some initial funds in it
	TestAddress2 Address

	// NetworkAddress is the filecoin network
	NetworkAddress Address
	// StorageMarketAddress is the hard-coded address of the filecoin storage market
	StorageMarketAddress Address
	// PaymentBrokerAddress is the hard-coded address of the filecoin storage market
	PaymentBrokerAddress Address
)

func init() {
	var err error
	t := Hash([]byte("satoshi"))
	// TODO Should please stop using this pattern
	TestAddress, err = NewFromActor(Mainnet, t)
	if err != nil {
		panic(err)
	}

	t = Hash([]byte("nakamoto"))
	// TODO Should please stop using this pattern
	TestAddress2, err = NewFromActor(Mainnet, t)
	if err != nil {
		panic(err)
	}

	n := Hash([]byte("filecoin"))
	NetworkAddress, err = NewFromActor(Mainnet, n)
	if err != nil {
		panic(err)
	}

	s := Hash([]byte("storage"))
	StorageMarketAddress, err = NewFromActor(Mainnet, s)
	if err != nil {
		panic(err)
	}

	p := Hash([]byte("payments"))
	PaymentBrokerAddress, err = NewFromActor(Mainnet, p)
	if err != nil {
		panic(err)
	}
}
