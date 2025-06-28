package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	// "os"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/ntp"
)

type ntpOptsStruct struct {

	// Timeout determines how long the client waits for a response from the
	// server before failing with a timeout error.
	timeout time.Duration

	// TTL specifies the maximum number of IP hops before the query datagram
	// is dropped by the network. See also: https://en.wikipedia.org/wiki/Time_to_live#IP_packets .
	TTL int

	// Version of the NTP protocol to use. Defaults to 4.
	Version int

	// LocalAddress contains the local IP address to use when creating a
	// connection to the remote NTP server. This may be useful when the local
	// system has more than one IP address. This address should not contain
	// a port number.
	LocalAddress string

	// Address of the remote NTP server.
	server string

	// Port indicates the port used to reach the remote NTP server.
	port int
}

// ntpResult holds the result of an NTP query
type ntpResult struct {
	time   time.Time
	server string
	err    error
}

// TimeOracleChaincode provides functions to get the current time from trusted NTP/NTS sources
type TimeOracleChaincode struct {
	contractapi.Contract
}

// split takes string in format "server|port" and returns server, port and error.
func split(str string) (string, int, error) {
	var (
		server = ""
		port   = 0
		err    error
	)

	fields := strings.Split(str, "|")

	for i, data := range fields {
		switch i {
		case 0:
			server = data
		case 1:
			port, err = strconv.Atoi(data)

			if err != nil {
				return server, port, fmt.Errorf("bad port number: %v", err)
			}
		}
	}

	return server, port, nil
}

// queryNTP queries a single NTP server and sends the result to the channel
func queryNTP(serverStr string, ntpOpts *ntpOptsStruct, resultCh chan<- ntpResult, wg *sync.WaitGroup) {
	defer wg.Done()

	result := ntpResult{server: serverStr}

	log.Printf("Processing NTP server: %s", serverStr)

	server, port, err := split(serverStr)
	if err != nil {
		result.err = fmt.Errorf("failed to parse server %s: %v", serverStr, err)
		resultCh <- result
		return
	}

	options := ntp.QueryOptions{
		Timeout:      ntpOpts.timeout * time.Second,
		TTL:          ntpOpts.TTL,
		Port:         port,
		Version:      ntpOpts.Version,
		LocalAddress: ntpOpts.LocalAddress,
	}

	response, err := ntp.QueryWithOptions(server, options)
	if err != nil || response == nil {
		result.err = fmt.Errorf("query failed for %s: %v", serverStr, err)
		log.Printf("error in the GetTimeNtp(): %s", err)
		resultCh <- result
		return
	}

	if err := response.Validate(); err != nil {
		result.err = fmt.Errorf("validation failed for %s: %v", serverStr, err)
		log.Printf("error in the GetTimeNtp(): %s", err)
		resultCh <- result
		return
	}

	result.time = time.Now().Add(response.ClockOffset).UTC()
	resultCh <- result
}

// ntpQueryLoop sends requests to all NTP servers in parallel and waits for all responses.
// Returns all successful times and a boolean indicating if any were successful.
func ntpQueryLoop(NTPs []string, ntpOpts *ntpOptsStruct) ([]time.Time, bool) {
	var wg sync.WaitGroup
	resultCh := make(chan ntpResult, len(NTPs))

	// Start goroutines for each NTP server
	for _, serverStr := range NTPs {
		wg.Add(1)
		go queryNTP(serverStr, ntpOpts, resultCh, &wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultCh)

	// Collect successful results
	var times []time.Time
	var successfulServers []string
	var failedServers []string

	for result := range resultCh {
		if result.err != nil {
			log.Printf("Failed to get time from %s: %v", result.server, result.err)
			failedServers = append(failedServers, result.server)
		} else {
			times = append(times, result.time)
			successfulServers = append(successfulServers, result.server)
		}
	}

	log.Printf("Successful NTP servers: %v", successfulServers)
	if len(failedServers) > 0 {
		log.Printf("Failed NTP servers: %v", failedServers)
	}

	return times, len(times) > 0
}

// GetTimeNtp returns the timestamp from one of NTP server in format: yyyy-mm-dd hh:mm:ss.nnnnnnnnn +0000 UTC.
// For example: "2024-07-09 15:37:13.879908993 +0000 UTC"
// In case of failure to connect to any of the servers:
// the following is logged: "Reach end of file";
// returns an error with the text "Failed to get response from NTP servers, see log file".
// The log also stores information about the reasons for the unsuccessful receipt of data from the NTP server.
func (cc *TimeOracleChaincode) GetTimeNtp(ctx contractapi.TransactionContextInterface, txID string) (string, error) {
	stub := ctx.GetStub()

	existing, err := stub.GetState(txID)
	if err != nil {
		return "", fmt.Errorf("failed to get state: %s", err.Error())
	}
	if existing != nil {
		log.Printf("Timestamp with txID %s already exists with value: %s", txID, string(existing))
		return string(existing), nil
	}

	var ntpOpts = ntpOptsStruct{
		timeout:      1,
		TTL:          128,
		Version:      4,
		LocalAddress: "",
		server:       "",
		port:         123,
	}

	NTPs := []string{
		"time.google.com",
		"time1.google.com",
		"time2.google.com",
		"time3.google.com",
		"time4.google.com",
	}

	if TimeList, result := ntpQueryLoop(NTPs, &ntpOpts); result {
		log.Printf("Successfully received time from NTP servers: %v", TimeList)
		accurateTime := TimeList[rand.Intn(len(TimeList))]
		jsonTimeStamp, err := json.Marshal(accurateTime.String())
		if err != nil {
			return "", fmt.Errorf("failed to marshal response payload: %s", err.Error())
		}

		err = stub.PutState(txID, []byte(accurateTime.String()))
		if err != nil {
			return "", fmt.Errorf("failed to save timestamp: %s", err.Error())
		}

		log.Printf("Saved tx: %s", txID)
		log.Printf("Saved timestamp: %s", jsonTimeStamp)

		return accurateTime.String(), nil
	}

	return "", fmt.Errorf("Failed to get response from NTP servers, see log file")
}

func main() {
	log.Printf("Starting TimeOracleChaincode...")
	chaincode, err := contractapi.NewChaincode(&TimeOracleChaincode{})
	if err != nil {
		log.Panicf("Error creating TimeOracleChaincode: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting TimeOracleChaincode: %v", err)
	}
}
