/*

SPDX-License-Identifier: Apache-2.0

MODIFICATION NOTICE:
FullBid has been extended with a 'valid' flag
*/

package auction

import (
	// "bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	// "io/ioutil"
	"log"
	// "net/http"
	"regexp"
	"time"
)

type SmartContract struct {
	contractapi.Contract
}

// Auction data
type Auction struct {
	Type         string             `json:"objectType"`
	ItemSold     string             `json:"item"`
	Seller       string             `json:"seller"`
	Orgs         []string           `json:"organizations"`
	PrivateBids  map[string]BidHash `json:"privateBids"`
	RevealedBids map[string]FullBid `json:"revealedBids"`
	Winner       string             `json:"winner"`
	Price        int                `json:"price"`
	Status       string             `json:"status"`
	Timelimit    time.Time          `json:"timelimit"`
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

// BidHash is the structure of a private bid
type BidHash struct {
	Org       string    `json:"org"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

type BidData struct {
	AuctionID string `json:"auctionID"`
	Org       string `json:"org"`
	Endorser  string `json:"endorser"`
	TxID      string `json:"txID"`
}

type Bidstore struct {
	TxID string `json:"txID"`
	Org  string `json:"Org"`
}

type TimestampResponse struct {
	Timestamps []string `json:"timestamps"`
}

const bidKeyType = "bid"

// CreateAuction creates on auction on the public channel. The identity that
// submits the transaction becomes the seller of the auction
func (s *SmartContract) CreateAuction(ctx contractapi.TransactionContextInterface, auctionID string, itemsold string, timelimit string) error {

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
		return fmt.Errorf("Invalid datetime format: %v", err)
	}

	// Create auction
	bidders := make(map[string]BidHash)
	revealedBids := make(map[string]FullBid)

	// TODO: Add functionality to add a starting price for an item
	auction := Auction{
		Type:         "auction",
		ItemSold:     itemsold,
		Price:        0,
		Seller:       clientID,
		Orgs:         []string{clientOrgID},
		PrivateBids:  bidders,
		RevealedBids: revealedBids,
		Winner:       "",
		Status:       "open",
		Timelimit:    t,
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
	if err != nil {
		return fmt.Errorf("failed setting state based endorsement for new organization: %v", err)
	}

	return nil
}

// Bid is used to add a user's bid to the auction. The bid is stored in the private
// data collection on the peer of the bidder's organization. The function returns
// the transaction ID so that users can identify and query their bid
func (s *SmartContract) Bid(ctx contractapi.TransactionContextInterface, auctionID string) (string, error) {

	// get bid from transient map
	transientMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return "", fmt.Errorf("error getting transient: %v", err)
	}

	BidJSON, ok := transientMap["bid"]
	if !ok {
		return "", fmt.Errorf("bid key not found in the transient map")
	}

	// get the implicit collection name using the bidder's organization ID
	collection, err := getCollectionName(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	// the bidder has to target their peer to store the bid
	err = verifyClientOrgMatchesPeerOrg(ctx)
	if err != nil {
		return "", fmt.Errorf("Cannot store bid on this peer, not a member of this org: Error %v", err)
	}

	// the transaction ID is used as a unique index for the bid
	txID := ctx.GetStub().GetTxID()

	// create a composite key using the transaction ID
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return "", fmt.Errorf("failed to create composite key: %v", err)
	}

	// // get the MSP ID of the bidder's org
	// clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get client MSP ID: %v", err)
	// }

	// apiData := Bidstore{
	// 	TxID: txID,
	// 	Org:  clientOrgID,
	// }

	//API additions
	// payload, err := json.Marshal(apiData)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to serialize data: %v", err)
	// }
	// TODO: Make this point to our backend or our pub-sub entity, to make faster reads :)
	// url := "http://flask-app:5000/bids/new_bid" // Replace with the actual URL of your Flask app

	// // Make an HTTP POST request to your Flask app
	// resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	// if err != nil {
	// 	return "", fmt.Errorf("failed to make API call to Flask app: %v", err)
	// }
	// defer resp.Body.Close()

	//Finish API Additions

	// put the bid into the organization's implicit data collection
	err = ctx.GetStub().PutPrivateData(collection, bidKey, BidJSON)
	if err != nil {
		return "", fmt.Errorf("failed input price into collection: %v", err)
	}

	// return the trannsaction ID so that the uset can identify their bid
	return txID, nil
}

// SubmitBid is used by the bidder to add the hash of that bid stored in private data to the
// auction. Note that this function alters the auction in private state, and needs
// to meet the auction endorsement policy. Transaction ID is used identify the bid
func (s *SmartContract) SubmitBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) error {

	creatorBytes, err := ctx.GetStub().GetCreator()
	if err != nil {
		return fmt.Errorf("failed to get endorser identity: %v", err)
	}
	re := regexp.MustCompile("-----BEGIN CERTIFICATE-----[^ ]+-----END CERTIFICATE-----\n")
	match := re.FindStringSubmatch(string(creatorBytes))
	pemBlock, _ := pem.Decode([]byte(match[0]))
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to get endorser identity: %v", cert)
	}
	// endorser := cert.Subject.CommonName

	collection, err := getCollectionName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get implicit collection name: %v", err)
	}
	// use the transaction ID passed as a parameter to create composite bid key
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}
	// get the MSP ID of the bidder's org
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client MSP ID: %v", err)
	}
	log.Printf("Breaks 1")
	// get the auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// the auction needs to be open for users to add their bid
	Status := auction.Status
	if Status != "open" {
		return fmt.Errorf("cannot join closed or ended auction")
	}

	// get the hash of the bid stored in private data collection
	bidHash, err := ctx.GetStub().GetPrivateDataHash(collection, bidKey)
	if err != nil {
		return fmt.Errorf("failed to read bid bash from collection: %v", err)
	}
	if bidHash == nil {
		return fmt.Errorf("bid hash does not exist: %s", bidKey)
	}

	//Transaction timestamp
	txTimestamp, err := ctx.GetStub().GetTxTimestamp()
	ts := time.Unix(txTimestamp.GetSeconds(), int64(txTimestamp.GetNanos())).UTC()
	if err != nil {
		return fmt.Errorf("failed to retrieve transaction timestamp: %v", err)
	}

	err = fmt.Errorf("Submitting bid in time: %v, txID: %s", ts, txID)
	log.Println(err)

	// store the hash along with the bidder's organization
	NewHash := BidHash{
		Org:       clientOrgID,
		Hash:      fmt.Sprintf("%x", bidHash),
		Timestamp: ts,
	}

	bidders := make(map[string]BidHash)
	bidders = auction.PrivateBids
	bidders[bidKey] = NewHash
	auction.PrivateBids = bidders

	/* 	// Get ID of submitting client identity
	   	clientID, err := s.GetSubmittingClientIdentity(ctx)
	   	if err != nil {
	   		return fmt.Errorf("failed to get client identity %v", err)
	   	} */

	// Add the bidding organization to the list of participating organizations if it is not already

	log.Printf("Breaks 2")
	Orgs := auction.Orgs
	if !(contains(Orgs, clientOrgID)) {
		newOrgs := append(Orgs, clientOrgID)
		auction.Orgs = newOrgs

		err = addAssetStateBasedEndorsement(ctx, auctionID, clientOrgID)
		if err != nil {
			return fmt.Errorf("failed setting state based endorsement for new organization: %v", err)
		}
	}

	// apiData := BidData{
	// 	AuctionID: auctionID,
	// 	Org:       clientOrgID,
	// 	Endorser:  endorser,
	// 	TxID:      txID,
	// }

	//API additions
	// payload, err := json.Marshal(apiData)
	// if err != nil {
	// 	return fmt.Errorf("failed to serialize data: %v", err)
	// }


	// url := "http://flask-app:5000/bids/new_time" // Replace with the actual URL of your Flask app
	// // Make an HTTP POST request to your Flask app
	// resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	// if err != nil {
	// 	return fmt.Errorf("failed to make API call to Flask app: %v", err)
	// }
	// defer resp.Body.Close()node revealBid.js org1 bidder1 Auction

	//Finish API Additions

	newAuctionJSON, _ := json.Marshal(auction)
	// timestampJSON, _ := json.Marshal(bidder_timestamp)
	// Read the response body
	// bodyBytes, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return fmt.Errorf("failed to read response body: %v", err)
	// }

	// Convert the response body to a string and print it
	// responseBody := string(bodyBytes)
	// log.Printf("Breaks 4: " + responseBody)

	err = ctx.GetStub().PutState(auctionID, newAuctionJSON)
	if err != nil {
		return fmt.Errorf("failed to update auction: %v", err)
	}

	// TODO: Get 5 timestamps from the Time Oracle chaincode
	// bidder_timestamp, err := s.GetTimeFromOracle(ctx)
	err, _ = s.RecordTimeFromOracle(ctx, txID)
	if err != nil {
		return fmt.Errorf("failed to get time from Time Oracle: %v", err)
	}

	// err = ctx.GetStub().PutState(txID, timestampJSON)
	// if err != nil {
	// 	return fmt.Errorf("failed to save timestamp %v: %v", timestampJSON, err)
	// }
	// fmt.Printf("Successfully saved timestamp %v for transaction ID %s", timestampJSON, txID)

	log.Printf("Breaks 5")
	return nil
}

// RevealBid is used by a bidder to reveal their bid
func (s *SmartContract) RevealBid(ctx contractapi.TransactionContextInterface, auctionID string, txID string) error {

	// get bid from transient map
	transientMap, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient: %v", err)
	}

	transientBidJSON, ok := transientMap["bid"]
	if !ok {
		return fmt.Errorf("bid key not found in transient map")
	}

	// get implicit collection name of organization ID
	collection, err := getCollectionName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get implicit collection name: %v", err)
	}

	// use transaction ID to create composit bid key
	bidKey, err := ctx.GetStub().CreateCompositeKey(bidKeyType, []string{auctionID, txID})
	if err != nil {
		return fmt.Errorf("failed to create composite key: %v", err)
	}

	// get bid hash of bid if private bid on the public ledger
	bidHash, err := ctx.GetStub().GetPrivateDataHash(collection, bidKey)
	if err != nil {
		return fmt.Errorf("failed to read bid bash from collection: %v", err)
	}
	if bidHash == nil {
		return fmt.Errorf("bid hash does not exist: %s", bidKey)
	}

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// Complete a series of three checks before we add the bid to the auction

	// check 1: check that the auction is closed. We cannot reveal a
	// bid to an open auction
	/* 	Status := auction.Status
	   	if Status != "closed" {
	   		return fmt.Errorf("cannot reveal bid for open or ended auction")
	   	} */

	// check 2: check that hash of revealed bid matches hash of private bid
	// on the public ledger. This checks that the bidder is telling the truth
	// about the value of their bid

	hash := sha256.New()
	hash.Write(transientBidJSON)
	// calculatedBidJSONHash := hash.Sum(nil)

	// // verify that the hash of the passed immutable properties matches the on-chain hash
	// if !bytes.Equal(calculatedBidJSONHash, bidHash) {
	// 	return fmt.Errorf("hash %x for bid JSON %s does not match hash in auction: %x",
	// 		calculatedBidJSONHash,
	// 		transientBidJSON,
	// 		bidHash,
	// 	)
	// }

	// url := "http://flask-app:5000/bids/" + txID // Replace with the actual URL of your Flask app

	// Make an HTTP GET request to your Flask app
	// resp, err := http.Get(url)
	// if err != nil {
	// 	return fmt.Errorf("failed to make API call to Flask app: %v", err)
	// }
	// defer resp.Body.Close()

	// Read the response body
	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return fmt.Errorf("failed to read response body: %v", err)
	// }
	// if resp.StatusCode != http.StatusOK {
	// 	return fmt.Errorf("API call to the Flask app failed with status code: %d", resp.StatusCode)
	// }

	err, body := s.RecordTimeFromOracle(ctx, txID)
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
	if err != nil {
		return fmt.Errorf("failed to parse API response: %v with body: %v", err, string(body))
	}

	encodedValue := encodeValue(txID)
	shuffledTimestamps := shuffleTimestamps([]string{timestamps}, encodedValue)

	// check 3; check hash of relealed bid matches hash of private bid that was
	// added earlier. This ensures that the bid has not changed since it
	// was added to the auction

	bidders := auction.PrivateBids
	privateBidHashString := bidders[bidKey].Hash
	Timestamp, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", shuffledTimestamps)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp: %v", err)
	}

	onChainBidHashString := fmt.Sprintf("%x", bidHash)
	if privateBidHashString != onChainBidHashString {
		return fmt.Errorf("hash %s for bid JSON %s does not match hash in auction: %s, bidder must have changed bid",
			privateBidHashString,
			transientBidJSON,
			onChainBidHashString,
		)
	}

	// we can add the bid to the auction if all checks have passed
	type transientBidInput struct {
		Price     int    `json:"price"`
		Org       string `json:"org"`
		Bidder    string `json:"bidder"`
		Valid     bool   `json:"valid"`
		Timestamp string `json:"timestamp"`
	}

	// unmarshal bid input
	var bidInput transientBidInput
	err = json.Unmarshal(transientBidJSON, &bidInput)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Get ID of submitting client identity
	// clientID, err := s.GetSubmittingClientIdentity(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to get client identity %v", err)
	// }

	// marshal transient parameters and ID and MSPID into bid object
	NewBid := FullBid{
		Type:      bidKeyType,
		Price:     bidInput.Price,
		Org:       bidInput.Org,
		Bidder:    bidInput.Bidder,
		Valid:     bidInput.Valid,
		Timestamp: Timestamp,
	}

	// check 4: make sure that the transaction is being submitted is the bidder
	// if bidInput.Bidder != clientID {
	// 	return fmt.Errorf("Permission denied, client id %v is not the owner of the bid", clientID)
	// }

	NewBid.Valid = true

	revealedBids := make(map[string]FullBid)
	revealedBids = auction.RevealedBids
	revealedBids[bidKey] = NewBid
	auction.RevealedBids = revealedBids

	newAuctionJSON, _ := json.Marshal(auction)

	// put auction with bid added back into state
	err = ctx.GetStub().PutState(auctionID, newAuctionJSON)
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

	// get auction from public state
	auction, err := s.QueryAuction(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction from public state %v", err)
	}

	// Check that the auction is being ended by the seller

	// get ID of submitting client
	clientID, err := s.GetSubmittingClientIdentity(ctx)
	if err != nil {
		return fmt.Errorf("failed to get client identity %v", err)
	}

	Seller := auction.Seller
	if Seller != clientID {
		return fmt.Errorf("auction can only be ended by seller: %v", err)
	}

	Status := auction.Status
	if Status != "closed" {
		return fmt.Errorf("Can only end a closed auction")
	}

	// get the list of revealed bids
	revealedBidMap := auction.RevealedBids
	if len(auction.RevealedBids) == 0 {
		return fmt.Errorf("No bids have been revealedd, cannot end auction: %v", err)
	}

	// determine the highest bid
	for _, bid := range revealedBidMap {
		if bid.Price > auction.Price {
			auction.Winner = bid.Bidder
			auction.Price = bid.Price
		}
	}
	// TODO: Unneccessary in our case. Remove it.
	// check if there is a winning bid that has yet to be revealed
	err = checkForHigherBid(ctx, auction.Price, auction.RevealedBids, auction.PrivateBids)
	if err != nil {
		return fmt.Errorf("Cannot end auction: %v", err)
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
func (c *SmartContract) RecordTimeFromOracle(ctx contractapi.TransactionContextInterface, txID string)  (error, string) {
	// Call the Time Oracle chaincode
	response := ctx.GetStub().InvokeChaincode("timeoracle", [][]byte{[]byte("GetTimeNtp")}, "mychannel")

	// Check if the response is successful
	if response.Status != 200 {
		return fmt.Errorf("failed to get time from Time Oracle: %s", response.Message), ""
	}

	log.Printf("Successfully retrieved time from timeoracle: %v", string(response.Payload))

	// Save the timestamp
	return nil, string(response.Payload)
}