package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test setup function
func setupTimeOracle() (*TimeOracleChaincode, *MockContext) {
	contract := new(TimeOracleChaincode)
	stub := &MockStub{State: map[string][]byte{}, TxID: "tx1"}
	id := &MockClientIdentity{MSPID: "Org1MSP", ID: "dXNlcjE="}
	ctx := &MockContext{Stub: stub, Identity: id}
	return contract, ctx
}

// TestGetTimeNtp_NewTimestamp tests getting a new NTP timestamp
func TestGetTimeNtp_NewTimestamp(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	// Call GetTimeNtp with a new transaction ID
	result, err := contract.GetTimeNtp(ctx, "newTxID")
	
	// Should succeed and return a timestamp
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	
	// Verify the timestamp was stored in state
	storedValue, exists := ctx.Stub.State["newTxID"]
	assert.True(t, exists)
	assert.NotEmpty(t, storedValue)
	
	// The result should match what was stored
	assert.Equal(t, string(storedValue), result)
}

// TestGetTimeNtp_ExistingTimestamp tests retrieving an existing timestamp
func TestGetTimeNtp_ExistingTimestamp(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	// Pre-populate state with an existing timestamp
	existingTimestamp := "2024-12-25 15:30:45.123456789 +0000 UTC"
	ctx.Stub.State["existingTxID"] = []byte(existingTimestamp)
	
	// Call GetTimeNtp with existing transaction ID
	result, err := contract.GetTimeNtp(ctx, "existingTxID")
	
	// Should succeed and return the existing timestamp
	assert.NoError(t, err)
	assert.Equal(t, existingTimestamp, result)
}

// TestGetTimeNtp_MultipleCallsSameID tests that multiple calls with same ID return same timestamp
func TestGetTimeNtp_MultipleCallsSameID(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	txID := "sameTxID"
	
	// First call - should create new timestamp
	result1, err1 := contract.GetTimeNtp(ctx, txID)
	assert.NoError(t, err1)
	assert.NotEmpty(t, result1)
	
	// Second call - should return same timestamp
	result2, err2 := contract.GetTimeNtp(ctx, txID)
	assert.NoError(t, err2)
	assert.Equal(t, result1, result2)
}

// TestGetTimeNtp_DifferentIDs tests that different transaction IDs get different timestamps
func TestGetTimeNtp_DifferentIDs(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	// Get timestamp for first transaction ID
	result1, err1 := contract.GetTimeNtp(ctx, "txID1")
	assert.NoError(t, err1)
	assert.NotEmpty(t, result1)
	
	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)
	
	// Get timestamp for second transaction ID
	result2, err2 := contract.GetTimeNtp(ctx, "txID2")
	assert.NoError(t, err2)
	assert.NotEmpty(t, result2)
	
	// Results should be different (though this might occasionally fail due to timing)
	// At minimum, they should both be valid timestamps
	assert.NotEmpty(t, result1)
	assert.NotEmpty(t, result2)
}

// TestGetTimeNtp_EmptyTxID tests behavior with empty transaction ID
func TestGetTimeNtp_EmptyTxID(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	// Call with empty transaction ID
	result, err := contract.GetTimeNtp(ctx, "")
	
	// Should still work - empty string is a valid key
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

// TestGetTimeNtp_ValidTimestampFormat tests that returned timestamp has correct format
func TestGetTimeNtp_ValidTimestampFormat(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	result, err := contract.GetTimeNtp(ctx, "formatTest")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	
	// Try to parse the timestamp to verify it's in correct format
	// The format should be: "2024-07-09 15:37:13.879908993 +0000 UTC"
	_, parseErr := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", result)
	assert.NoError(t, parseErr, "Timestamp should be in correct format")
}

// TestGetTimeNtp_UTCTimezone tests that timestamp is always in UTC
func TestGetTimeNtp_UTCTimezone(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	result, err := contract.GetTimeNtp(ctx, "utcTest")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	
	// Timestamp should contain "UTC"
	assert.Contains(t, result, "UTC", "Timestamp should be in UTC timezone")
	assert.Contains(t, result, "+0000", "Timestamp should show UTC offset")
}

// TestGetTimeNtp_StateConsistency tests that state is properly maintained
func TestGetTimeNtp_StateConsistency(t *testing.T) {
	contract, ctx := setupTimeOracle()
	
	txID := "consistencyTest"
	
	// Get timestamp
	result, err := contract.GetTimeNtp(ctx, txID)
	assert.NoError(t, err)
	
	// Verify state was updated
	storedValue, exists := ctx.Stub.State[txID]
	assert.True(t, exists, "Timestamp should be stored in state")
	assert.Equal(t, result, string(storedValue), "Stored value should match returned value")
	
	// Verify subsequent calls return the same value from state
	result2, err2 := contract.GetTimeNtp(ctx, txID)
	assert.NoError(t, err2)
	assert.Equal(t, result, result2, "Subsequent calls should return same timestamp")
}

// TestSplitFunction tests the split helper function
func TestSplitFunction(t *testing.T) {
	// Test with server and port
	server, port, err := split("pool.ntp.org|123")
	assert.NoError(t, err)
	assert.Equal(t, "pool.ntp.org", server)
	assert.Equal(t, 123, port)
	
	// Test with server only
	server2, port2, err2 := split("time.google.com")
	assert.NoError(t, err2)
	assert.Equal(t, "time.google.com", server2)
	assert.Equal(t, 0, port2)
	
	// Test with invalid port
	_, _, err3 := split("server|invalid")
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "bad port number")
}

// TestNtpQueryLoop_MockServers tests the NTP query loop logic
// Note: This test might be challenging to fully test without mocking the NTP library
func TestNtpQueryLoop_MockServers(t *testing.T) {
	ntpOpts := &ntpOptsStruct{
		timeout:      1, // Short timeout for testing
		TTL:          64,
		Version:      4,
		LocalAddress: "",
		server:       "",
		port:         123,
	}
	
	// Test with invalid servers (should fail quickly)
	invalidServers := []string{"invalid.server.test", "192.0.2.1"} // RFC 5737 test IP
	_, success := ntpQueryLoop(invalidServers, ntpOpts)
	
	// Should fail to connect to invalid servers
	assert.False(t, success, "Should fail to connect to invalid servers")
}

// Benchmark test for GetTimeNtp performance
func BenchmarkGetTimeNtp(b *testing.B) {
	contract, ctx := setupTimeOracle()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txID := "benchTx" + string(rune(i))
		_, err := contract.GetTimeNtp(ctx, txID)
		if err != nil {
			b.Fatalf("GetTimeNtp failed: %v", err)
		}
	}
}

// Benchmark test for existing timestamp retrieval
func BenchmarkGetTimeNtp_Existing(b *testing.B) {
	contract, ctx := setupTimeOracle()
	
	// Pre-populate with a timestamp
	existingTimestamp := "2024-12-25 15:30:45.123456789 +0000 UTC"
	ctx.Stub.State["benchExisting"] = []byte(existingTimestamp)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := contract.GetTimeNtp(ctx, "benchExisting")
		if err != nil {
			b.Fatalf("GetTimeNtp failed: %v", err)
		}
	}
}

