package auction_test

import (
	"encoding/json"
	"testing"
	"time"

	"bitAuction/auction"

	"github.com/stretchr/testify/assert"
)

func setup() (*auction.SmartContract, *MockContext) {
	contract := new(auction.SmartContract)
	stub := &MockStub{State: map[string][]byte{}, TxID: "tx1"}
	// Use base64-encoded string for ID ("user1" -> "dXNlcjE=")
	id := &MockClientIdentity{MSPID: "Org1MSP", ID: "dXNlcjE="}
	ctx := &MockContext{Stub: stub, Identity: id}
	return contract, ctx
}

func TestCreateAuction(t *testing.T) {
	contract, ctx := setup()
	timelimit := time.Now().Add(1 * time.Hour).Format(time.RFC3339Nano)
	err := contract.CreateAuction(ctx, "auction1", "Laptop", timelimit, "Desc", "http://img")
	assert.NoError(t, err)
}

func TestBid(t *testing.T) {
	contract, ctx := setup()
	ctx.Stub.State["auction1"] = []byte(`{}`)
	txID, err := contract.Bid(ctx, "auction1", 100)
	assert.NoError(t, err)
	assert.Equal(t, "tx1", txID)
}

func TestSubmitBid(t *testing.T) {
	contract, ctx := setup()
	t2 := time.Now().Add(1 * time.Hour)
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Type:      "auction",
		ItemSold:  "Laptop",
		Seller:    "user1",
		Orgs:      []string{"Org1MSP"},
		Status:    "open",
		Timelimit: t2,
		Bids:      []auction.FullBid{},
	})
	ctx.Stub.State["auction1"] = auctionJSON
	priceJSON, _ := json.Marshal(100)
	ctx.Stub.State["bid:auction1:tx1"] = priceJSON

	err := contract.SubmitBid(ctx, "auction1", "tx1")
	assert.NoError(t, err)
}

func TestCloseAuction(t *testing.T) {
	contract, ctx := setup()
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "open",
		Timelimit: time.Now().Add(-1 * time.Hour),
	})
	ctx.Stub.State["auction1"] = auctionJSON
	contract.EndAuction(ctx, "auction1")
	err := contract.CloseAuction(ctx, "auction1")
	assert.NoError(t, err)
}

func TestEndAuction(t *testing.T) {
	contract, ctx := setup()
	auctionObj := auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "closed",
		Bids: []auction.FullBid{
			{Price: 100, Bidder: "userA"},
			{Price: 300, Bidder: "userB"},
		},
	}
	auctionJSON, _ := json.Marshal(auctionObj)
	ctx.Stub.State["auction1"] = auctionJSON

	err := contract.EndAuction(ctx, "auction1")
	assert.NoError(t, err)
}

func TestRecordTimeFromOracle(t *testing.T) {
	contract, ctx := setup()
	result, err := contract.RecordTimeFromOracle(ctx, "tx1")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

// TestBidAfterAuctionEnded tests that bidding is not allowed after the auction time limit
func TestBidAfterAuctionEnded(t *testing.T) {
	contract, ctx := setup()

	// Create an auction with a past time limit
	pastTime := time.Now().Add(-1 * time.Hour) // 1 hour in the past
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Type:      "auction",
		ItemSold:  "Laptop",
		Seller:    "user1",
		Orgs:      []string{"Org1MSP"},
		Status:    "open",
		Timelimit: pastTime,
		Bids:      []auction.FullBid{},
	})
	ctx.Stub.State["auction1"] = auctionJSON

	// Create a bid in public state
	priceJSON, _ := json.Marshal(150)
	ctx.Stub.State["bid:auction1:tx1"] = priceJSON

	// Attempt to submit the bid
	err := contract.SubmitBid(ctx, "auction1", "tx1")

	// The bid should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auction has already ended")
}

// TestCloseClosedAuction tests that an already closed auction cannot be closed again
func TestCloseClosedAuction(t *testing.T) {
	contract, ctx := setup()

	// Create an auction that is already closed
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "closed", // Already closed status
		Timelimit: time.Now().Add(-1 * time.Hour),
	})
	ctx.Stub.State["auction1"] = auctionJSON
	// Attempt to close the auction again
	err := contract.CloseAuction(ctx, "auction1")

	// The operation should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot close auction that is not open")
}

// TestBidAfterAuctionClosed tests that bidding is not allowed after the auction is closed by the seller
func TestBidAfterAuctionClosed(t *testing.T) {
	contract, ctx := setup()

	// Create an auction with a future time limit but closed status
	futureTime := time.Now().Add(1 * time.Hour) // 1 hour in the future
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Type:      "auction",
		ItemSold:  "Laptop",
		Seller:    "user1",
		Orgs:      []string{"Org1MSP"},
		Status:    "closed", // Closed by seller
		Timelimit: futureTime,
		Bids:      []auction.FullBid{},
	})
	ctx.Stub.State["auction1"] = auctionJSON

	// Create a bid in public state
	priceJSON, _ := json.Marshal(200)
	ctx.Stub.State["bid:auction1:tx1"] = priceJSON

	// Attempt to submit the bid
	err := contract.SubmitBid(ctx, "auction1", "tx1")

	// The bid should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auction is not open for bidding")
}

// TestBidAfterAuctionHasEnded tests that bidding is not allowed after the auction is ended (winner determined)
func TestBidAfterAuctionHasEnded(t *testing.T) {
	contract, ctx := setup()

	// Create an auction that has been ended with a winner
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Type:      "auction",
		ItemSold:  "Laptop",
		Seller:    "user1",
		Orgs:      []string{"Org1MSP"},
		Status:    "ended", // Auction has ended with winner
		Winner:    "userB",
		Price:     300,
		Timelimit: time.Now().Add(1 * time.Hour),
		Bids: []auction.FullBid{
			{Price: 300, Bidder: "userB"},
		},
	})
	ctx.Stub.State["auction1"] = auctionJSON

	// Create a bid in public state
	priceJSON, _ := json.Marshal(350)
	ctx.Stub.State["bid:auction1:tx1"] = priceJSON

	// Attempt to submit the bid
	err := contract.SubmitBid(ctx, "auction1", "tx1")

	// The bid should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auction is not open for bidding")
}

// TestEndAlreadyEndedAuction tests that an already ended auction cannot be ended again
func TestEndAlreadyEndedAuction(t *testing.T) {
	contract, ctx := setup()

	// Create an auction that is already ended
	auctionObj := auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "ended", // Already ended
		Winner:    "userB",
		Price:     300,
		Bids: []auction.FullBid{
			{Price: 100, Bidder: "userA"},
			{Price: 300, Bidder: "userB"},
		},
	}
	auctionJSON, _ := json.Marshal(auctionObj)
	ctx.Stub.State["auction1"] = auctionJSON

	// Attempt to end the auction again (should work since the code doesn't check for ended status)
	err := contract.EndAuction(ctx, "auction1")

	// Should succeed based on current implementation which only checks for seller
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auction has already been ended")
}

// TestCloseAuctionBeforeTimeLimit tests that an auction cannot be closed before its time limit has passed
func TestCloseAuctionBeforeTimeLimit(t *testing.T) {
	contract, ctx := setup()

	// Create an auction with a future time limit
	futureTime := time.Now().Add(1 * time.Hour) // 1 hour in the future
	auctionJSON, _ := json.Marshal(auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "open",
		Timelimit: futureTime,
	})
	ctx.Stub.State["auction1"] = auctionJSON

	// Attempt to close the auction before the time limit
	err := contract.CloseAuction(ctx, "auction1")

	// The operation should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot close auction before time limit has passed")
}
