#!/bin/bash
set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <chaincode_name> <chaincode_jar>"
    exit 1
fi

CHAINCODE_NAME=$1
CHAINCODE_JAR=$2
PEER_CMD="../bin/peer"

echo "Deploying the chaincode: $CHAINCODE_NAME..."

export CORE_PEER_TLS_ENABLED=true

# Install on Org1 Peer 0
for ORG in 1 2 3 4; do
    export CORE_PEER_LOCALMSPID=Org${ORG}MSP
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org${ORG}.example.com/peers/peer0.org${ORG}.example.com/tls/ca.crt
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org${ORG}.example.com/users/Admin@org${ORG}.example.com/msp
    export CORE_PEER_ADDRESS=localhost:$((7051 + (ORG-1) * 2000))
    echo "Installing chaincode on peer0.org${ORG}..."
    $PEER_CMD lifecycle chaincode install $CHAINCODE_JAR

done

OUTPUT=$($PEER_CMD lifecycle chaincode queryinstalled)
CC_PACKAGE_ID=$(echo "$OUTPUT" | grep "Package ID:" | awk -F 'Package ID: ' '{print $2}' | awk -F ', Label' '{print $1}')

export CC_PACKAGE_ID
echo "CC_PACKAGE_ID=$CC_PACKAGE_ID"

# Approve for all 4 Orgs
for ORG in 1 2 3 4; do
    export CORE_PEER_LOCALMSPID=Org${ORG}MSP
    export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org${ORG}.example.com/users/Admin@org${ORG}.example.com/msp
    export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org${ORG}.example.com/peers/peer0.org${ORG}.example.com/tls/ca.crt
    export CORE_PEER_ADDRESS=localhost:$((7051 + (ORG-1) * 2000))

    $PEER_CMD lifecycle chaincode approveformyorg -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com \
        --channelID mychannel --name $CHAINCODE_NAME --version 1.0 --package-id $CC_PACKAGE_ID --sequence 1 --tls \
        --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem"

done

$PEER_CMD lifecycle chaincode checkcommitreadiness --channelID mychannel --name $CHAINCODE_NAME --version 1.0 --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" --output json

$PEER_CMD lifecycle chaincode commit -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --channelID mychannel \
    --name $CHAINCODE_NAME --version 1.0 --sequence 1 --tls \
    --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" \
    --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" \
    --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" \
    --peerAddresses localhost:11051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org3.example.com/peers/peer0.org3.example.com/tls/ca.crt" \
    --peerAddresses localhost:13051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org4.example.com/peers/peer0.org4.example.com/tls/ca.crt"

$PEER_CMD lifecycle chaincode querycommitted --channelID mychannel --name $CHAINCODE_NAME