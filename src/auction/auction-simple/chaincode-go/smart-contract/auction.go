/*

SPDX-License-Identifier: Apache-2.0

MODIFICATION NOTICE:
FullBid has been extended with a 'valid' flag
*/

package auction

import (
	// "bytes"
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	// "io/ioutil"
	"log"
	// "net/http"

	"time"
)

type SmartContract struct {
	contractapi.Contract
}

// Auction data
type Auction struct {
	AuctionID   string    `json:"auctionID"`
	Type        string    `json:"objectType"`
	ItemSold    string    `json:"item"`
	Seller      string    `json:"seller"`
	Orgs        []string  `json:"organizations"`
	Winner      string    `json:"winner"`
	Price       int       `json:"price"`
	Status      string    `json:"status"`
	Timelimit   time.Time `json:"timelimit"`
	Description string    `json:"description"`
	PictureURL  string    `json:"pictureUrl"`
	Bids        []FullBid `json:"bids"`
}

// FullBid is the structure of a revealed bid
type FullBid struct {
	Type      string    `json:"objectType"`
	Price     int       `json:"price"`
	Org       string    `json:"org"`
	Bidder    string    `json:"bidder"`
	Valid     bool      `json:"valid"`
	Timestamp time.Time `json:"timestamp"`
}

type Winner struct {
	HighestBidder string `json:"highestbidder"`
	HighestBid    int    `json:"highestbid"`
}

const bidKeyType = "bid"

// CreateAuction creates on auction on the public channel. The identity that
// submits the transaction becomes the seller of the auction
func (s *SmartContract) CreateAuction(ctx contractapi.TransactionContextInterface, auctionID string, itemsold string, timelimit string, description string, pictureUrl string) error {

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	// get org of submitting client
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	t, err := time.Parse(time.RFC3339Nano, timelimit)
	if err != nil {
		return fmt.Errorf("invalid datetime format: %v", err)
	}

	// Create auction
	auction := Auction{
		AuctionID:   auctionID,
		Type:        "auction",
		ItemSold:    itemsold,
		Price:       0,
		Seller:      clientID,
		Orgs:        []string{clientOrgID},
		Winner:      "",
		Status:      "open",
		Timelimit:   t,
		Description: description,
		PictureURL:  pictureUrl,
		Bids:        []FullBid{},
	}

	auctionJSON, err := json.Marshal(auction)
	if err != nil {
		return err
	}

	// put auction into state
	err = ctx.GetStub().PutState(auctionID, auctionJSON)
	if err != nil {
		return fmt.Errorf("failed to put auction in public data: %v", err)
	}

	// set the seller of the auction as an endorser
	err = setAssetStateBasedEndorsement(ctx, auctionID, clientOrgID)

	// This allows any organization to endorse transactions
	// err = ctx.GetStub().SetStateValidationParameter(auctionID, nil)
	if err != nil {
		return fmt.Errorf("failed setting state based endorsement for new organization: %v", err)
	}

	return nil
}

// Bid is used to add a user's bid to the auction. The bid is stored in the public
// storage. The function returns the transaction ID so that users can identify and query their bid
func (s *SmartContract) Bid(ctx contractapi.TransactionContextInterface, auctionID string, price int) (string, error) {

	// the transaction ID is used as a unique index for the bid
	txID := ctx.GetStub().GetTxID()

	// create a composite key using the transaction ID
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return "", fmt.Errorf("failed to create composite key: %v", err)
	}

	priceJSON, _ := json.Marshal(price)
	err = ctx.GetStub().PutState(bidKey, priceJSON)

	// return the transaction ID so that the user can identify their bid
	return txID, nil
}

// SubmitBid is used by the bidder to add the hash of that bid stored in private data to the
// auction. Note that this function alters the auction in private state, and needs
// to meet the auction endorsement policy. Transaction ID is used identify the bid
func (s *SmartContract) SubmitBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) error {
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction: %v", err)
	}
	if auction.Status != "open" {
		return fmt.Errorf("auction is not open for bidding")
	}

	bidder, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity: %v", err)
	}
	org, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get org: %v", err)
	}

	// get the bid from public state
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}
	priceBytes, err := ctx.GetStub().GetState(bidKey)
	if err != nil {
		return fmt.Errorf("failed to get bid from public state: %v", err)
	}
	if priceBytes == nil {
		return fmt.Errorf("bid not found in public state")
	}
	var price int
	err = json.Unmarshal(priceBytes, &price)
	if err != nil {
		return fmt.Errorf("failed to unmarshal bid: %v", err)
	}
	// check if the bid is valid
	if price <= 0 {
		return fmt.Errorf("invalid bid amount: %v", err)
	}

	body, err := s.RecordTimeFromOracle(ctx, txID)
	if err != nil {
		return fmt.Errorf("failed to read timestamp from state: %v", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("no timestamp found for transaction ID: %s", txID)
	}
	log.Printf("Successfully retrieved timestamp from state: %v", string(body))

	// Deserialize the JSON response into a TimestampResponse struct
	// var timestamps []string
	var timestamps string = body
	// err = json.Unmarshal(body, &timestamps)
	// if err != nil {
	// 	return fmt.Errorf("failed to parse API response: %v with body: %v", err, string(body))
	// }

	encodedValue := encodeValue(txID)
	shuffledTimestamps := shuffleTimestamps([]string{timestamps}, encodedValue)

	// check 3; check hash of relealed bid matches hash of private bid that was
	// added earlier. This ensures that the bid has not changed since it
	// was added to the auction

	Timestamp, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", shuffledTimestamps)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp: %v", err)
	}

	bid := FullBid{
		Type:      "bid",
		Price:     price,
		Org:       org,
		Bidder:    bidder,
		Valid:     true,
		Timestamp: Timestamp,
	}

	auction.Bids = append(auction.Bids, bid)

	auctionJSON, _ := json.Marshal(auction)
	err = ctx.GetStub().PutState(auctionID, auctionJSON)
	if err != nil {
		return fmt.Errorf("failed to update auction: %v", err)
	}
	return nil
}

// CloseAuction can be used by the seller to close the auction. This prevents
// bids from being added to the auction.
func (s *SmartContract) CloseAuction(ctx contractapi.TransactionContextInterface, auctionID string) error {

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// the auction can only be closed by the seller

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	Seller := auction.Seller
	if Seller != clientID {
		return fmt.Errorf("auction can only be closed by seller: %v", err)
	}

	Status := auction.Status
	if Status != "open" {
		return fmt.Errorf("cannot close auction that is not open")
	}

	auction.Status = string("closed")

	closedAuctionJSON, _ := json.Marshal(auction)

	err = ctx.GetStub().PutState(auctionID, closedAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to close auction: %v", err)
	}

	return nil
}

// EndAuction both changes the auction status to closed and calculates the winners
// of the auction
func (s *SmartContract) EndAuction(ctx contractapi.TransactionContextInterface, auctionID string) error {
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}
	Seller := auction.Seller
	if Seller != clientID {
		return fmt.Errorf("auction can only be ended by seller: %v", err)
	}
	// Status := auction.Status
	// if Status != "closed" {
	// 	return fmt.Errorf("can only end a closed auction")
	// }
	if len(auction.Bids) == 0 {
		return fmt.Errorf("no bids have been placed, cannot end auction: %v", err)
	}
	for _, bid := range auction.Bids {
		if bid.Price > auction.Price {
			auction.Winner = bid.Bidder
			auction.Price = bid.Price
		}
	}

	auction.Status = string("ended")
	endedAuctionJSON, _ := json.Marshal(auction)
	err = ctx.GetStub().PutState(auctionID, endedAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to end auction: %v", err)
	}
	return nil
}

// GetTimeFromOracle calls the Time Oracle chaincode and returns the current time
func (c *SmartContract) RecordTimeFromOracle(ctx contractapi.TransactionContextInterface, txID string) (string, error) {
	// Call the Time Oracle chaincode

	response := ctx.GetStub().InvokeChaincode(
		"timeoracle",
		[][]byte{[]byte("GetTimeNtp"), []byte(txID)},
		"mychannel",
	)
	log.Printf("Response from Time Oracle: %v", response)
	// Check if the response is successful
	if response.Status != 200 {
		return "", fmt.Errorf("failed to get time from Time Oracle: %s", response.Message)
	}

	log.Printf("Successfully retrieved time from timeoracle: %v", string(response.Payload))

	// Save the timestamp
	return string(response.Payload), nil
}
