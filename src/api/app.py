

import os
from flask import Flask, Response, request, jsonify
from dotenv import load_dotenv
from pymongo import MongoClient
from bson.json_util import dumps
import ntplib
import json
from bson import ObjectId, json_util
from time import ctime
c = ntplib.NTPClient()
import random
import datetime
from aux import map_transaction_id


load_dotenv()

app = Flask(__name__)
mongo_db_url = os.environ.get("MONGO_DB_CONN_STRING")

client = MongoClient(mongo_db_url)
db = client['Bids']

counter=0

@app.route("/bids/new_bid", methods=["POST"])
def add_bid():
    _json = request.json
    print(_json, flush=True)
    json_resp = {
        'data': _json,
        'owner' : request.remote_addr,
    }
    db.Bids.insert_one(json_resp)
    json_resp_serializable = json.loads(json_util.dumps(json_resp))

    return jsonify(json_resp_serializable), 200


@app.route("/bids/new_time", methods=["POST"])
def add_sensor():
    print("debug 1" , flush=True)
    _json = request.json
    print(_json, flush=True)
    response = c.request('pool.ntp.org')
    datetime_obj = datetime.datetime.fromtimestamp(response.tx_time)
    # Format the datetime object as a string in a specific format
    formatted_time = datetime_obj.strftime("%Y-%m-%d %H:%M:%S")
    json_resp = {
        'data': _json,
        'clientIP' : request.remote_addr,
        'timestamp': formatted_time
    }
    
    db.TimeSynchronization.insert_one(json_resp)
    json_resp_serializable = json.loads(json_util.dumps(json_resp))
    print("como vamos?", flush=True)
    return jsonify(json_resp_serializable), 200

@app.route("/bids/<txid>", methods=["GET"])
def getBidTime(txid):
    # Get the requesting IP address

    # Query the database for the entry with the specified txid and different IP address
    bid_entries = db.TimeSynchronization.find({'data.txID': txid})

    timestamps = []

    # Iterate over the documents
    for document in bid_entries:
        timestamp = document["timestamp"]
        
        # Append the timestamp to the list
        timestamps.append(timestamp)
    
    print(timestamps, flush=True)

    return timestamps, 200


"""     # Extract timestamps from the retrieved documents
    timestamps = [entry['timestamp'] for entry in bid_entries]
    mapped_value = map_transaction_id(txid)
    # Select a random timestamp from the list
    random.seed(mapped_value)
    random.shuffle(timestamps)
    print(timestamps)
    return jsonify({'timestamp': timestamps[0]}), 200 """

    
    



if __name__ == '__main__':
    app.run()