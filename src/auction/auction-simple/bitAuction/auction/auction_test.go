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
	txID, err := contract.Bid(ctx, "auction1", 100)
	assert.NoError(t, err)
	assert.Equal(t, "tx1", txID)
}

func TestBidAfterAuctionTimelimit(t *testing.T) {
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

	// Attempt to submit a bid
	_, err := contract.Bid(ctx, "auction1", 150)

	// The bid should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auction has already ended")
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

func TestEndAuction(t *testing.T) {
	contract, ctx := setup()

	// Create timestamps for bids
	now := time.Now()
	earlierTime := now.Add(-10 * time.Minute)
	laterTime := now.Add(-5 * time.Minute)

	// Create auction object with no bids initially
	auctionObj := auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "open",
		Timelimit: now.Add(-1 * time.Hour), // Auction time limit in the past
		Bids:      []auction.FullBid{},
	}
	auctionJSON, _ := json.Marshal(auctionObj)
	ctx.Stub.State["auction1"] = auctionJSON

	// Create and store full bids using composite keys
	bid1 := auction.FullBid{
		Type:      "bid",
		Price:     100,
		Org:       "Org1MSP",
		Bidder:    "userA",
		Valid:     true,
		Timestamp: earlierTime,
	}
	bid2 := auction.FullBid{
		Type:      "bid",
		Price:     300,
		Org:       "Org1MSP",
		Bidder:    "userB",
		Valid:     true,
		Timestamp: laterTime,
	}

	// Store bids with composite keys
	fullBidKey1, _ := ctx.Stub.CreateCompositeKey("fullbid", []string{"auction1", "tx1"})
	fullBidKey2, _ := ctx.Stub.CreateCompositeKey("fullbid", []string{"auction1", "tx2"})

	fullBidJSON1, _ := json.Marshal(bid1)
	fullBidJSON2, _ := json.Marshal(bid2)

	ctx.Stub.State[fullBidKey1] = fullBidJSON1
	ctx.Stub.State[fullBidKey2] = fullBidJSON2

	// End the auction
	err := contract.EndAuction(ctx, "auction1")
	assert.NoError(t, err)

	// Check auction state updated
	endedAuctionJSON := ctx.Stub.State["auction1"]
	var endedAuction auction.Auction
	_ = json.Unmarshal(endedAuctionJSON, &endedAuction)

	assert.Equal(t, "ended", endedAuction.Status)
	assert.Equal(t, "userB", endedAuction.Winner)
	assert.Equal(t, 300, endedAuction.Price)
}

func TestRecordTimeFromOracle(t *testing.T) {
	contract, ctx := setup()
	result, err := contract.RecordTimeFromOracle(ctx, "tx1")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

// TestBidAfterAuctionEnded tests that bidding is not allowed after the auction time limit
func TestBidAfterAuctionTimeLimit(t *testing.T) {
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
func TestEndAuctionBeforeTimeLimit(t *testing.T) {
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
	err := contract.EndAuction(ctx, "auction1")

	// The operation should be rejected
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot end auction before time limit has passed")
}

// TestTimestampTieBreaking tests that when multiple bids have the same price,
// the bid with the earliest timestamp wins
func TestTimestampTieBreaking(t *testing.T) {
	contract, ctx := setup()

	// Create timestamps with clear difference
	now := time.Now()
	earlierTime := now.Add(-10 * time.Minute)
	laterTime := now.Add(-5 * time.Minute)

	// Create and store auction
	auctionObj := auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "open",
		Timelimit: now.Add(-1 * time.Hour), // Time limit in the past
		Bids:      []auction.FullBid{},     // Empty bids array
	}
	auctionJSON, _ := json.Marshal(auctionObj)
	ctx.Stub.State["auction1"] = auctionJSON

	// Create bids with same price but different timestamps
	bid1 := auction.FullBid{
		Type:      "bid",
		Price:     300,
		Org:       "Org1MSP",
		Bidder:    "userA",
		Valid:     true,
		Timestamp: laterTime, // Later bid (should lose)
	}
	bid2 := auction.FullBid{
		Type:      "bid",
		Price:     300,
		Org:       "Org1MSP",
		Bidder:    "userB",
		Valid:     true,
		Timestamp: earlierTime, // Earlier bid (should win)
	}
	bid3 := auction.FullBid{
		Type:      "bid",
		Price:     200,
		Org:       "Org1MSP",
		Bidder:    "userC",
		Valid:     true,
		Timestamp: now, // Lower price bid
	}

	// Store bids with composite keys
	fullBidKey1, _ := ctx.Stub.CreateCompositeKey("fullbid", []string{"auction1", "tx1"})
	fullBidKey2, _ := ctx.Stub.CreateCompositeKey("fullbid", []string{"auction1", "tx2"})
	fullBidKey3, _ := ctx.Stub.CreateCompositeKey("fullbid", []string{"auction1", "tx3"})

	fullBidJSON1, _ := json.Marshal(bid1)
	fullBidJSON2, _ := json.Marshal(bid2)
	fullBidJSON3, _ := json.Marshal(bid3)

	ctx.Stub.State[fullBidKey1] = fullBidJSON1
	ctx.Stub.State[fullBidKey2] = fullBidJSON2
	ctx.Stub.State[fullBidKey3] = fullBidJSON3

	// End the auction to test the tie-breaking logic
	err := contract.EndAuction(ctx, "auction1")
	assert.NoError(t, err)

	// Check the auction state to verify the winner
	endedAuctionJSON := ctx.Stub.State["auction1"]
	var endedAuction auction.Auction
	_ = json.Unmarshal(endedAuctionJSON, &endedAuction)

	// The winner should be userB who bid first with the highest price
	assert.Equal(t, "ended", endedAuction.Status)
	assert.Equal(t, "userB", endedAuction.Winner)
	assert.Equal(t, 300, endedAuction.Price)
}

// TestEndAuctionWithNoBids tests that an auction with no bids has no winner when ended
func TestEndAuctionWithNoBids(t *testing.T) {
	contract, ctx := setup()

	// Create an auction with no bids
	auctionObj := auction.Auction{
		AuctionID: "auction1",
		Seller:    "user1",
		Status:    "open",
		Timelimit: time.Now().Add(-1 * time.Hour), // Auction ended in the past
		Bids:      []auction.FullBid{},            // No bids initially
	}
	auctionJSON, _ := json.Marshal(auctionObj)
	ctx.Stub.State["auction1"] = auctionJSON

	// No need to add any bids using composite keys
	// since we're testing the case with no bids

	// End the auction
	err := contract.EndAuction(ctx, "auction1")

	// Check that the function worked correctly
	assert.NoError(t, err)

	// Verify the auction state was updated correctly
	endedAuctionJSON := ctx.Stub.State["auction1"]
	var endedAuction auction.Auction
	_ = json.Unmarshal(endedAuctionJSON, &endedAuction)

	// The auction should be ended with no winner and zero price
	assert.Equal(t, "ended", endedAuction.Status)
	assert.Equal(t, "", endedAuction.Winner)
	assert.Equal(t, 0, endedAuction.Price)
}
