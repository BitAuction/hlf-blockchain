package auction_test

import (
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/hyperledger/fabric-chaincode-go/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/shim"
    "github.com/hyperledger/fabric-protos-go/ledger/queryresult"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Mocks ---

type MockStub struct {
	mock.Mock
	State map[string][]byte
	TxID  string
}

func (m *MockStub) PutState(key string, value []byte) error {
	m.State[key] = value
	return nil
}

func (m *MockStub) GetState(key string) ([]byte, error) {
	return m.State[key], nil
}

func (m *MockStub) GetTxID() string {
	return m.TxID
}

func (m *MockStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	joined := objectType
	for _, attr := range attributes {
		joined += ":" + attr
	}
	return joined, nil
}

// Implement all required methods for shim.ChaincodeStubInterface as needed for your tests
func (m *MockStub) DelPrivateData(collection, key string) error             { return nil }
func (m *MockStub) DelState(key string) error                               { return nil }
func (m *MockStub) GetArgs() [][]byte                                       { return [][]byte{} }
func (m *MockStub) GetArgsSlice() ([]byte, error)                           { return []byte{}, nil }
func (m *MockStub) GetBinding() ([]byte, error)                             { return []byte{}, nil }
func (m *MockStub) GetChannelID() string                                    { return "testchannel" }
func (m *MockStub) GetCreator() ([]byte, error)                             { return []byte("creator"), nil }
func (m *MockStub) GetDecorations() map[string][]byte                       { return map[string][]byte{} }
func (m *MockStub) GetFunctionAndParameters() (string, []string)            { return "", []string{} }
func (m *MockStub) SetStateValidationParameter(key string, ep []byte) error { return nil }
func (m *MockStub) GetStateValidationParameter(key string) ([]byte, error)  { return nil, nil }
func (m *MockStub) GetStateByRange(startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetStateByRangeWithPagination(startKey, endKey string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (m *MockStub) GetStateByPartialCompositeKeyWithPagination(objectType string, keys []string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (m *MockStub) SplitCompositeKey(compositeKey string) (string, []string, error) {
	return "", nil, nil
}
func (m *MockStub) GetQueryResult(query string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetQueryResultWithPagination(query string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *pb.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (m *MockStub) GetHistoryForKey(key string) (shim.HistoryQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetPrivateData(collection, key string) ([]byte, error)            { return nil, nil }
func (m *MockStub) GetPrivateDataHash(collection, key string) ([]byte, error)        { return nil, nil }
func (m *MockStub) PutPrivateData(collection string, key string, value []byte) error { return nil }
func (m *MockStub) PurgePrivateData(collection, key string) error                    { return nil }
func (m *MockStub) SetPrivateDataValidationParameter(collection, key string, ep []byte) error {
	return nil
}
func (m *MockStub) GetPrivateDataValidationParameter(collection, key string) ([]byte, error) {
	return nil, nil
}
func (m *MockStub) GetPrivateDataByRange(collection, startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetPrivateDataByPartialCompositeKey(collection, objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetPrivateDataQueryResult(collection, query string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (m *MockStub) GetSignedProposal() (*pb.SignedProposal, error)  { return nil, nil }
func (m *MockStub) GetTxTimestamp() (*timestamppb.Timestamp, error) { return nil, nil }
func (m *MockStub) SetEvent(name string, payload []byte) error      { return nil }
func (m *MockStub) GetStringArgs() []string                         { return []string{} }
func (m *MockStub) GetTransient() (map[string][]byte, error)        { return map[string][]byte{}, nil }
func (m *MockStub) InvokeChaincode(chaincodeName string, args [][]byte, channel string) pb.Response {
	if chaincodeName == "timeoracle" {
		return pb.Response{
			Status:  200,
			Message: "OK",
			Payload: []byte("2025-06-22 12:50:03.792349213 +0000 UTC"),
		}
	}
	return pb.Response{}
}


type MockStateQueryIterator struct {
	mock.Mock
	Items []queryresult.KV
	Index int
}

func (iter *MockStateQueryIterator) HasNext() bool {
	return iter.Index < len(iter.Items)
}

func (iter *MockStateQueryIterator) Next() (*queryresult.KV, error) {
	if !iter.HasNext() {
		return nil, fmt.Errorf("no more items")
	}
	item := iter.Items[iter.Index]
	iter.Index++
	return &item, nil
}

func (iter *MockStateQueryIterator) Close() error {
	return nil
}

func (m *MockStub) GetStateByPartialCompositeKey(objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	prefix := objectType
	for _, key := range keys {
		prefix += ":" + key
	}
	var items []queryresult.KV
	for k, v := range m.State {
		if strings.HasPrefix(k, prefix) {
			items = append(items, queryresult.KV{
				Key:   k,
				Value: v,
			})
		}
	}

	return &MockStateQueryIterator{Items: items, Index: 0}, nil
}



// --- End Mocks ---

type MockClientIdentity struct {
	mock.Mock
	MSPID string
	ID    string
}

func (ci *MockClientIdentity) GetMSPID() (string, error) {
	return ci.MSPID, nil
}

func (ci *MockClientIdentity) GetID() (string, error) {
	return ci.ID, nil
}

func (ci *MockClientIdentity) GetAttributeValue(attrName string) (string, bool, error) {
	return "", false, nil
}
func (ci *MockClientIdentity) AssertAttributeValue(attrName, attrValue string) error { return nil }
func (ci *MockClientIdentity) GetX509Certificate() (*x509.Certificate, error)        { return nil, nil }

// MockContext implements contractapi.TransactionContextInterface
// GetStub returns a shim.ChaincodeStubInterface, GetClientIdentity returns cid.ClientIdentity

type MockContext struct {
	mock.Mock
	Stub     *MockStub
	Identity *MockClientIdentity
}

func (m *MockContext) GetStub() shim.ChaincodeStubInterface {
	return m.Stub
}

func (m *MockContext) GetClientIdentity() cid.ClientIdentity {
	return m.Identity
}
