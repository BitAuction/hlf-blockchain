/*
SPDX-License-Identifier: Apache-2.0
*/

package auction

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// QueryAuction allows all members of the channel to read a public auction
func (s *SmartContract) QueryAuction(ctx contractapi.TransactionContextInterface, auctionID string) (*Auction, error) {

	auctionJSON, err := ctx.GetStub().GetState(auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auction object %v: %v", auctionID, err)
	}
	if auctionJSON == nil {
		return nil, fmt.Errorf("auction does not exist")
	}

	var auction *Auction
	err = json.Unmarshal(auctionJSON, &auction)
	if err != nil {
		return nil, err
	}

	return auction, nil
}

// QueryBid allows the submitter of the bid to read their bid from public state
func (s *SmartContract) QueryBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) (*FullBid, error) {

	err := verifyClientOrgMatchesPeerOrg(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client identity %v", err)
	}

	collection, err := getCollectionName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return nil, fmt.Errorf("failed to create composite key: %v", err)
	}

	bidJSON, err := ctx.GetStub().GetPrivateData(collection, bidKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get bid %v: %v", bidKey, err)
	}
	if bidJSON == nil {
		return nil, fmt.Errorf("bid %v does not exist", bidKey)
	}

	var bid *FullBid
	err = json.Unmarshal(bidJSON, &bid)
	if err != nil {
		return nil, err
	}

	// check that the client querying the bid is the bid owner
	if bid.Bidder != clientID {
		return nil, fmt.Errorf("Permission denied, client id %v is not the owner of the bid", clientID)
	}

	return bid, nil
}

// function used to get highest bid and bidder
func (s *SmartContract) GetHb(ctx contractapi.TransactionContextInterface, auctionID string) (*FullBid, error) {
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auction from public state %v", err)
	}
	if len(auction.Bids) == 0 {
		return nil, nil
	}
	var highest *FullBid
	winnerTime := time.Time{}
	for i, bid := range auction.Bids {
		if s.isHigherBid(&auction.Bids[i], highest, winnerTime) {
			highest = &auction.Bids[i]
			winnerTime = bid.Timestamp
		}
	}
	return highest, nil
}

func (s *SmartContract) isHigherBid(bid *FullBid, highest *FullBid, winnerTime time.Time) bool {
	// Check if the new bid is higher than the current highest bid
	if highest == nil || bid.Price > highest.Price {
		return true
	}
	// If the price is the same, check the timestamp
	if bid.Price == highest.Price && bid.Timestamp.Before(winnerTime) {
		return true
	}
	return false
}

func isAuctionOpenForBidding(auction *Auction) error {
	if auction.Status != "open" {
		return fmt.Errorf("auction is not open for bidding")
	}
	if auction.Timelimit.Before(time.Now().UTC()) {
		return fmt.Errorf("auction has already ended")
	}
	return nil
}

// GetAllOpenAuctions retrieves all auctions with status 'open'
func (s *SmartContract) GetAllOpenAuctions(ctx contractapi.TransactionContextInterface) ([]*Auction, error) {
	results := []*Auction{}

	// Get all keys in the ledger
	iterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get state by range: %v", err)
	}
	defer iterator.Close()

	for iterator.HasNext() {
		kv, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate: %v", err)
		}

		var auction Auction
		err = json.Unmarshal(kv.Value, &auction)
		if err != nil {
			// Not an auction object, skip
			continue
		}

		if auction.Status == "open" {
			results = append(results, &auction)
		}
	}

	return results, nil
}

// GetAllAuctionsBySeller retrieves all auctions created by a specific seller
func (s *SmartContract) GetAllAuctionsBySeller(ctx contractapi.TransactionContextInterface, sellerID string) ([]*Auction, error) {
	results := []*Auction{}

	iterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to get state by range: %v", err)
	}
	defer iterator.Close()

	for iterator.HasNext() {
		kv, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate: %v", err)
		}

		var auction Auction
		err = json.Unmarshal(kv.Value, &auction)
		if err != nil {
			continue
		}

		auctionSeller, err := s.ParseClientID(auction.Seller)
		if err != nil {
			return nil, fmt.Errorf("failed to parse auction seller: %v", err)
		}

		if auctionSeller == sellerID {
			results = append(results, &auction)
		}
	}

	return results, nil
}
