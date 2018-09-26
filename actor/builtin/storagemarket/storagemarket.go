package storagemarket

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MinimumPledge is the minimum amount of sectors a user can pledge.
var MinimumPledge = big.NewInt(10)

var log = logging.Logger("actor/storagemarket")

const (
	// ErrPledgeTooLow is the error code for a pledge under the MinimumPledge.
	ErrPledgeTooLow = 33
	// ErrUnknownMiner indicates a pledge under the MinimumPledge.
	ErrUnknownMiner = 34
	// ErrUnknownAsk indicates the ask for a deal could not be found.
	ErrUnknownAsk = 35
	// ErrUnknownBid indicates the bid for a deal could not be found.
	ErrUnknownBid = 36
	// ErrAskOwnerNotFound indicates the owner of an ask could not be found.
	ErrAskOwnerNotFound = 37
	// ErrNotBidOwner indicates the sender is not the owner of the bid.
	ErrNotBidOwner = 38
	// ErrInsufficientSpace indicates the bid to too big for the ask.
	ErrInsufficientSpace = 39
	// ErrInvalidSignature indicates the signature is invalid.
	ErrInvalidSignature = 40
	// ErrUnknownDeal indicates the deal id is not found.
	ErrUnknownDeal = 41
	// ErrNotDealOwner indicates someone other than the deal owner tried to commit.
	ErrNotDealOwner = 42
	// ErrDealCommitted indicates the deal is already committed.
	ErrDealCommitted = 43
	// ErrInsufficientBidFunds indicates the value of the bid message is less than the price of the space.
	ErrInsufficientBidFunds = 44
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrPledgeTooLow:         errors.NewCodedRevertErrorf(ErrPledgeTooLow, "pledge must be at least %s sectors", MinimumPledge),
	ErrUnknownMiner:         errors.NewCodedRevertErrorf(ErrUnknownMiner, "unknown miner"),
	ErrUnknownAsk:           errors.NewCodedRevertErrorf(ErrUnknownAsk, "ask id not found"),
	ErrUnknownBid:           errors.NewCodedRevertErrorf(ErrUnknownBid, "bid id not found"),
	ErrAskOwnerNotFound:     errors.NewCodedRevertErrorf(ErrAskOwnerNotFound, "cannot create a deal for someone elses ask"),
	ErrNotBidOwner:          errors.NewCodedRevertErrorf(ErrNotBidOwner, "ask id not found"),
	ErrInsufficientSpace:    errors.NewCodedRevertErrorf(ErrNotBidOwner, "not enough space in ask for bid"),
	ErrInvalidSignature:     errors.NewCodedRevertErrorf(ErrInvalidSignature, "signature failed to validate"),
	ErrUnknownDeal:          errors.NewCodedRevertErrorf(ErrUnknownDeal, "unknown deal id"),
	ErrNotDealOwner:         errors.NewCodedRevertErrorf(ErrNotDealOwner, "miner tried to commit with someone elses deal"),
	ErrDealCommitted:        errors.NewCodedRevertErrorf(ErrDealCommitted, "deal already committed"),
	ErrInsufficientBidFunds: errors.NewCodedRevertErrorf(ErrInsufficientBidFunds, "must send price * size funds to create bid"),
}

func init() {
	cbor.RegisterCborType(State{})
	cbor.RegisterCborType(struct{}{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, adding bids, asks and deals. It also exposes the
// power table used to drive filecoin consensus.
type Actor struct{}

// State is the storage market's storage.
type State struct {
	Miners *cid.Cid

	Orderbook *Orderbook

	// TotalCommitedStorage is the number of sectors that are currently committed
	// in the whole network.
	TotalCommittedStorage *big.Int
}

// NewActor returns a new storage market actor.
func NewActor() (*actor.Actor, error) {
	return actor.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL()), nil
}

// InitializeState stores the actor's initial data structure.
func (sma *Actor) InitializeState(storage exec.Storage, _ interface{}) error {
	initStorage := &State{
		Orderbook:             &Orderbook{},
		TotalCommittedStorage: big.NewInt(0),
	}
	stateBytes, err := cbor.DumpObject(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, nil)
}

var _ exec.ExecutableActor = (*Actor)(nil)

// Exports returns the actors exports.
func (sma *Actor) Exports() exec.Exports {
	return storageMarketExports
}

var storageMarketExports = exec.Exports{
	"createMiner": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Bytes, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	"addAsk": &exec.FunctionSignature{
		Params: []abi.Type{abi.AttoFIL, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addBid": &exec.FunctionSignature{
		Params: []abi.Type{abi.AttoFIL, abi.BytesAmount},
		Return: []abi.Type{abi.Integer},
	},
	"addDeal": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Integer, abi.Bytes, abi.Bytes},
		Return: []abi.Type{abi.Integer},
	},
	"updatePower": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: nil,
	},
	"getTotalStorage": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Integer},
	},
	"getAsk": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: []abi.Type{abi.Bytes},
	},
	"getBid": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: []abi.Type{abi.Bytes},
	},
	"getAllAsks": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Bytes},
	},
	"getAllBids": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Bytes},
	},
}

// CreateMiner creates a new miner with the a pledge of the given amount of sectors. The
// miners collateral is set by the value in the message.
func (sma *Actor) CreateMiner(vmctx exec.VMContext, pledge *big.Int, publicKey []byte, pid peer.ID) (_ address.Address, _ uint8, err error) {
	ctx := context.Background()
	ctx = log.Start(ctx, "CreateMiner")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		if pledge.Cmp(MinimumPledge) < 0 {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, Errors[ErrPledgeTooLow]
		}

		addr, err := vmctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		minerInitializationParams := miner.NewState(vmctx.Message().From, publicKey, pledge, pid, vmctx.Message().Value)
		if err := vmctx.CreateNewActor(addr, types.MinerActorCodeCid, minerInitializationParams); err != nil {
			return nil, err
		}

		_, _, err = vmctx.Send(addr, "", vmctx.Message().Value, nil)
		if err != nil {
			return nil, err
		}

		state.Miners, err = actor.SetKeyValue(ctx, vmctx.Storage(), state.Miners, addr.String(), true)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not set miner key value for lookup with CID: %s", state.Miners)
		}

		return addr, nil
	})
	if err != nil {
		return address.Address{}, errors.CodeError(err), err
	}

	log.SetTag(ctx, "miner", map[string]interface{}{
		"pledge":    pledge.String(),
		"colateral": vmctx.Message().Value.String(),
		"publicKey": string(publicKey),
		"peerID":    pid.Pretty(),
		"address":   ret.(address.Address),
	})
	return ret.(address.Address), 0, nil
}

// AddAsk adds an ask order to the orderbook. Must be called by a miner created
// by this storage market actor.
func (sma *Actor) AddAsk(vmctx exec.VMContext, price *types.AttoFIL, size *types.BytesAmount) (askid *big.Int, retval uint8, err error) {
	ctx := context.Background()

	ctx = log.Start(ctx, "AddAsk")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		// method must be called by a miner that was created by this storage market actor.
		miner := vmctx.Message().From

		miners, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Miners)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with CID: %s", state.Miners)
		}

		_, err = miners.Find(ctx, miner.String())
		if err != nil {
			if err == hamt.ErrNotFound {
				return nil, Errors[ErrUnknownMiner]
			}
			return nil, errors.FaultErrorWrapf(err, "could lookup miner with address: %s", miner)
		}

		askID := state.Orderbook.NextSAskID
		state.Orderbook.NextSAskID++
		ask := &Ask{
			ID:    askID,
			Price: price,
			Size:  size,
			Owner: miner,
		}

		state.Orderbook.StorageAsks, err = actor.SetKeyValue(ctx, vmctx.Storage(), state.Orderbook.StorageAsks, keyFromID(askID), ask)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not set ask with askID, %d, into lookup", askID)
		}

		log.SetTag(ctx, "ask", ask)
		return big.NewInt(0).SetUint64(askID), nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	askID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return askID, 0, nil
}

// AddBid adds a bid order to the orderbook. Can be called by anyone. The
// message must contain the appropriate amount of funds to be locked up for the
// bid.
func (sma *Actor) AddBid(vmctx exec.VMContext, price *types.AttoFIL, size *types.BytesAmount) (bidid *big.Int, retval uint8, err error) {
	ctx := context.Background()

	ctx = log.Start(ctx, "AddBid")
	defer func() {
		log.FinishWithErr(ctx, err)
	}()

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {

		lockedFunds := price.CalculatePrice(size)
		if vmctx.Message().Value.LessThan(lockedFunds) {
			return nil, Errors[ErrInsufficientBidFunds]
		}

		bidID := state.Orderbook.NextBidID
		state.Orderbook.NextBidID++

		bid := &Bid{
			ID:    bidID,
			Price: price,
			Size:  size,
			Owner: vmctx.Message().From,
		}
		state.Orderbook.Bids, err = actor.SetKeyValue(ctx,
			vmctx.Storage(), state.Orderbook.Bids, keyFromID(bidID), bid)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not set bid with bidID, %d, into lookup", bidID)
		}

		log.SetTag(ctx, "bid", bid)
		return big.NewInt(0).SetUint64(bidID), nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	bidID, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, errors.NewRevertErrorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return bidID, 0, nil
}

// VerifyDealSignature checks if the given deal and signature match to the provided address.
func VerifyDealSignature(deal *Deal, sig types.Signature, addr address.Address) bool {
	dealBytes, err := deal.Marshal()
	if err != nil {
		return false
	}

	return types.VerifySignature(dealBytes, addr, sig)
}

// SignDeal signs the given deal using the provided address.
func SignDeal(deal *Deal, signer types.Signer, addr address.Address) (types.Signature, error) {
	dealBytes, err := deal.Marshal()
	if err != nil {
		return nil, err
	}

	return signer.SignBytes(dealBytes, addr)
}

// UpdatePower is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing or willful removal). The delta is in number of sectors.
func (sma *Actor) UpdatePower(vmctx exec.VMContext, delta *big.Int) (uint8, error) {
	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miner := vmctx.Message().From
		ctx := context.Background()

		miners, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Miners)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with CID: %s", state.Miners)
		}

		_, err = miners.Find(ctx, miner.String())
		if err != nil {
			if err == hamt.ErrNotFound {
				return nil, Errors[ErrUnknownMiner]
			}
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with address: %s", miner)
		}

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(state.TotalCommittedStorage, delta)

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetTotalStorage returns the total amount of proven storage in the system.
func (sma *Actor) GetTotalStorage(vmctx exec.VMContext) (*big.Int, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		return state.TotalCommittedStorage, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	count, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, fmt.Errorf("expected *BytesAmount to be returned, but got %T instead", ret)
	}

	return count, 0, nil
}

// GetAsk returns the ask on the orderbook for the given askID.
func (sma *Actor) GetAsk(vmctx exec.VMContext, askID *big.Int) ([]byte, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()

		asks, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Orderbook.StorageAsks)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for asks with CID: %s", state.Orderbook.StorageAsks)
		}
		ask, err := asks.Find(ctx, strconv.FormatUint(askID.Uint64(), 36))
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not find ask with askID: %d", askID.Uint64())
		}

		return actor.MarshalStorage(ask)
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	ask, ok := ret.([]byte)
	if !ok {
		return nil, 1, fmt.Errorf("expected []bytes to be returned, but got %T instead", ret)
	}

	return ask, 0, nil
}

// GetBid returns the bid on the orderbook for the given bidID.
func (sma *Actor) GetBid(vmctx exec.VMContext, bidID *big.Int) ([]byte, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()

		bids, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Orderbook.Bids)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for bids with CID: %s", state.Orderbook.Bids)
		}
		bid, err := bids.Find(ctx, strconv.FormatUint(bidID.Uint64(), 36))
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not find bid with askID: %d", bidID.Uint64())
		}

		return actor.MarshalStorage(bid)
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	bid, ok := ret.([]byte)
	if !ok {
		return nil, 1, fmt.Errorf("expected []bytes to be returned, but got %T instead", ret)
	}

	return bid, 0, nil
}

// GetAllAsks returns all asks on the orderbook.
// TODO limit number of results
func (sma *Actor) GetAllAsks(vmctx exec.VMContext) ([]byte, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()

		askLookup, err := actor.LoadTypedLookup(ctx, vmctx.Storage(), state.Orderbook.StorageAsks, &Ask{})
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for asks with CID: %s", state.Orderbook.StorageAsks)
		}

		askValues, err := askLookup.Values(ctx)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not retrieve ask values from storage market")
		}

		asks := AskSet{}

		// translate kvs to AskSet.
		for _, kv := range askValues {
			id, err := idFromKey(kv.Key)
			if err != nil {
				return nil, errors.FaultErrorWrap(err, "Invalid key in orderbook.asks")
			}

			ask, ok := kv.Value.(*Ask)
			if !ok {
				return nil, errors.NewFaultError("Expected Ask from ask lookup")
			}
			asks[id] = ask
		}

		return actor.MarshalStorage(asks)
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	asks, ok := ret.([]byte)
	if !ok {
		return nil, 1, fmt.Errorf("expected []bytes to be returned, but got %T instead", ret)
	}

	return asks, 0, nil
}

// GetAllBids returns all bids on the orderbook.
// TODO limit number of results
func (sma *Actor) GetAllBids(vmctx exec.VMContext) ([]byte, uint8, error) {
	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()

		bidLookup, err := actor.LoadTypedLookup(ctx, vmctx.Storage(), state.Orderbook.Bids, &Bid{})
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for bids with CID: %s", state.Orderbook.Bids)
		}

		bidValues, err := bidLookup.Values(ctx)
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not retrieve bid values from storage market")
		}

		bids := BidSet{}

		// translate kvs to BidSet.
		for _, kv := range bidValues {
			id, err := idFromKey(kv.Key)
			if err != nil {
				return nil, errors.FaultErrorWrap(err, "Invalid key in orderbook.bids")
			}

			bid, ok := kv.Value.(*Bid)
			if !ok {
				return nil, errors.NewFaultError("Expected Bid from bid lookup")
			}
			bids[id] = bid
		}

		return actor.MarshalStorage(bids)
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	bids, ok := ret.([]byte)
	if !ok {
		return nil, 1, fmt.Errorf("expected []bytes to be returned, but got %T instead", ret)
	}

	return bids, 0, nil
}

func keyFromID(askID uint64) string {
	return strconv.FormatUint(askID, 36)
}

func idFromKey(key string) (uint64, error) {
	return strconv.ParseUint(key, 36, 64)
}
