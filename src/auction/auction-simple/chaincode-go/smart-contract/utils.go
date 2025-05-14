/*
SPDX-License-Identifier: Apache-2.0
*/

package auction

import (
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"math/rand"
	"strings"

	"github.com/hyperledger/fabric-chaincode-go/pkg/statebased"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func (s *SmartContract) GetSubmittingClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
	// reference: https://github.com/hyperledger/fabric-chaincode-go/blob/main/pkg/cid/interfaces.go
	b64ID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return "", fmt.Errorf("Failed to read clientID: %v", err)
	}
	decodeID, err := base64.StdEncoding.DecodeString(b64ID)
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode clientID: %v", err)
	}

	// Extract CN from the X.509 subject
	// "x509::CN=seller_01,OU=client::CN=ca.org1.example.com,O=org1.example.com,L=Durham,ST=North Carolina,C=US"
	idStr := string(decodeID)
	if strings.HasPrefix(idStr, "x509::") {
		// Split by CN= and get the first part
		parts := strings.Split(idStr, "CN=")
		if len(parts) > 1 {
			// Get the CN value and split by comma to get just the CN
			cnParts := strings.Split(parts[1], ",")
			return cnParts[0], nil
		}
	}

	return idStr, nil
}

// setAssetStateBasedEndorsement sets the endorsement policy of a new auction
func setAssetStateBasedEndorsement(ctx contractapi.TransactionContextInterface, auctionID string, orgToEndorse string) error {

	endorsementPolicy, err := statebased.NewStateEP(nil)
	if err != nil {
		return err
	}
	err = endorsementPolicy.AddOrgs(statebased.RoleTypePeer, orgToEndorse)
	if err != nil {
		return fmt.Errorf("failed to add org to endorsement policy: %v", err)
	}
	policy, err := endorsementPolicy.Policy()
	if err != nil {
		return fmt.Errorf("failed to create endorsement policy bytes from org: %v", err)
	}
	err = ctx.GetStub().SetStateValidationParameter(auctionID, policy)
	if err != nil {
		return fmt.Errorf("failed to set validation parameter on auction: %v", err)
	}

	return nil
}

// addAssetStateBasedEndorsement adds a new organization as an endorser of the auction
func addAssetStateBasedEndorsement(ctx contractapi.TransactionContextInterface, auctionID string, orgToEndorse string) error {

	endorsementPolicy, err := ctx.GetStub().GetStateValidationParameter(auctionID)
	if err != nil {
		return err
	}

	newEndorsementPolicy, err := statebased.NewStateEP(endorsementPolicy)
	if err != nil {
		return err
	}

	err = newEndorsementPolicy.AddOrgs(statebased.RoleTypePeer, orgToEndorse)
	if err != nil {
		return fmt.Errorf("failed to add org to endorsement policy: %v", err)
	}
	policy, err := newEndorsementPolicy.Policy()
	if err != nil {
		return fmt.Errorf("failed to create endorsement policy bytes from org: %v", err)
	}
	err = ctx.GetStub().SetStateValidationParameter(auctionID, policy)
	if err != nil {
		return fmt.Errorf("failed to set validation parameter on auction: %v", err)
	}

	return nil
}

// getCollectionName is an internal helper function to get collection of submitting client identity.
func getCollectionName(ctx contractapi.TransactionContextInterface) (string, error) {

	// Get the MSP ID of submitting client identity
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("failed to get verified MSPID: %v", err)
	}

	// Create the collection name
	orgCollection := "_implicit_org_" + clientMSPID

	return orgCollection, nil
}

// verifyClientOrgMatchesPeerOrg is an internal function used to verify that client org id matches peer org id.
func verifyClientOrgMatchesPeerOrg(ctx contractapi.TransactionContextInterface) error {
	clientMSPID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed getting the client's MSPID: %v", err)
	}
	peerMSPID, err := shim.GetMSPID()
	if err != nil {
		return fmt.Errorf("failed getting the peer's MSPID: %v", err)
	}

	if clientMSPID != peerMSPID {
		return fmt.Errorf("client from org %v is not authorized to read or write private data from an org %v peer", clientMSPID, peerMSPID)
	}

	return nil
}

func contains(sli []string, str string) bool {
	for _, a := range sli {
		if a == str {
			return true
		}
	}
	return false
}

// Takes the txID and hash it to a value between 1 and 10
func encodeValue(value string) int {
	hash := crc32.ChecksumIEEE([]byte(value))
	encoded := int(hash%10 + 1)
	return encoded
}

// Receives the timestamps from all endorsing peers and the encoded txID and shuffles them
func shuffleTimestamps(timestamps []string, encodedValue int) string {
	rand.Seed(int64(encodedValue)) // Use a fixed seed value for deterministic shuffling

	// Create a copy of the timestamps slice to avoid modifying the original slice
	shuffledTimestamps := make([]string, len(timestamps))
	copy(shuffledTimestamps, timestamps)

	// Shuffle the copied slice based on the encoded values
	rand.Shuffle(len(shuffledTimestamps), func(i, j int) {
		shuffledTimestamps[i], shuffledTimestamps[j] = shuffledTimestamps[j], shuffledTimestamps[i]
	})

	return shuffledTimestamps[0]
}
