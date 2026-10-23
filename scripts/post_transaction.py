#!/usr/bin/env python3
"""Encode transaction text safely, including quotes and newlines."""
import argparse
import json
import os
import time
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument("--port", type=int, default=int(os.environ.get("PORT", "8081")))
parser.add_argument("--id", default=os.environ.get("TX_ID", f"tx-{time.time_ns()}"))
parser.add_argument("--data", default=os.environ.get("TX_DATA", "Alice sends Bob a message"))
parser.add_argument("--timestamp", type=int, default=int(os.environ.get("TX_TS", str(int(time.time())))))
args = parser.parse_args()
body = json.dumps({"id": args.id, "data": args.data, "timestamp": args.timestamp}).encode()
request = urllib.request.Request(f"http://127.0.0.1:{args.port}/transaction", body,
                                 {"Content-Type": "application/json"})
with urllib.request.urlopen(request, timeout=5) as response:
    print(response.status, response.read().decode())
