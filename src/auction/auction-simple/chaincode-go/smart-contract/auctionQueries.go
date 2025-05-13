/*
SPDX-License-Identifier: Apache-2.0
*/

package auction

import (
	"encoding/json"
	"fmt"

	// "github.com/hyperledger/fabric-chaincode-go/shim" // Remove unused import
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

// function used to get highest bid and bidder
func (s *SmartContract) GetHb(ctx contractapi.TransactionContextInterface, auctionID string) (*Winner, error) {
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auction from public state %v", err)
	}
	if len(auction.Bids) == 0 {
		return &Winner{HighestBidder: "None", HighestBid: 0}, nil
	}
	highest := &Winner{}
	for _, bid := range auction.Bids {
		if bid.Price > highest.HighestBid {
			highest.HighestBidder = bid.Bidder
			highest.HighestBid = bid.Price
		}
	}
	return highest, nil
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

		if auction.Seller == sellerID {
			results = append(results, &auction)
		}
	}

	return results, nil
}
