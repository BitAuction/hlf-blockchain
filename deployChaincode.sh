#!/bin/bash
set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <chaincode_name> <chaincode_jar>"
    exit 1
fi

CHAINCODE_NAME=$1
CHAINCODE_JAR=$2

echo "Deploying the chaincode: $CHAINCODE_NAME..."

# ./network.sh down
# ./network.sh up createChannel

export CORE_PEER_TLS_ENABLED=true

# Install on Org1 Peer 0
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

peer lifecycle chaincode install $CHAINCODE_JAR

# Install on Org2 Peer 0
export CORE_PEER_LOCALMSPID=Org2MSP
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp
export CORE_PEER_ADDRESS=localhost:9051

peer lifecycle chaincode install $CHAINCODE_JAR

OUTPUT=$(peer lifecycle chaincode queryinstalled)
CC_PACKAGE_ID=$(echo "$OUTPUT" | grep "Package ID:" | awk -F 'Package ID: ' '{print $2}' | awk -F ', Label' '{print $1}')

export CC_PACKAGE_ID

echo "CC_PACKAGE_ID=$CC_PACKAGE_ID"

peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name $CHAINCODE_NAME --version 1.0 --package-id $CC_PACKAGE_ID --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem"

export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_ADDRESS=localhost:7051

peer lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
    --channelID mychannel --name $CHAINCODE_NAME --version 1.0 --package-id $CC_PACKAGE_ID --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem"

peer lifecycle chaincode checkcommitreadiness --channelID mychannel --name $CHAINCODE_NAME --version 1.0 --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" --output json

peer lifecycle chaincode commit -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --channelID mychannel \
    --name $CHAINCODE_NAME --version 1.0 --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
    --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
    --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt"

peer lifecycle chaincode querycommitted --channelID mychannel --name $CHAINCODE_NAME
