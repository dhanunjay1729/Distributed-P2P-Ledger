### Running API Tests

To run all tests for the project, use the following command in your terminal:

```bash
make test
```

Exact problem statement 

Simplified P2P Ledger via Gossip and Consensus
The 6-Container Setup
You will deploy 6 identical Docker containers (Node_A through Node_F). Each runs a simple Python (Flask) or Node.js web server.

Shared Logic: Every node has the exact same code.

Storage: Each node stores a local ledger.json file.

Peer List: Every node knows the addresses of the other 5 nodes via a simple environment variable or a peers.txt file.

1. Simplified Task: The "HTTP Gossip" Protocol
Instead of low-level binary networking, we use standard HTTP POST requests.

Trigger: When a user sends a new message (e.g., "Alice pays Bob $10") to Node_A.

The Gossip Rule: Node_A saves the message locally and picks 2 random peers from its list. It sends the message to them.

The Termination Rule: When a peer receives a message, it checks if it already exists in its local mempool. If Yes, it does nothing. If No, it saves it and forwards it to 2 other random peers.

Result: Within milliseconds, all 6 nodes have the message without a central server.

2. Simplified Task: Turn-Based Consensus
To avoid "Mining" (Proof of Work), we use a Validation Rule based on the Previous Hash.

Block Structure: * Index, Data, Timestamp, Previous_Hash, and Current_Hash.

The Validation Rule: When a node receives a "New Block" from a peer, it only accepts it if:

The Previous_Hash field matches the Current_Hash of its own last local block.

The Index is exactly +1 of its own last index.

The Agreement: If these two things match, the node appends the block and considers it "Truth."

3. Simplified Task: Conflict Resolution (The Tie-Breaker)
What if Node_A and Node_F create a new block at the exact same time?

The Rule: The Longest Chain Wins.

The Implementation: If Node_B receives a block that doesn't fit its current hash (a "fork"), it requests the entire chain from the sender.

The Logic: If the sender's chain is longer (contains more blocks), Node_B deletes its local ledger.json and replaces it with the sender's version. This ensures the whole cluster eventually matches.








WORK 

PERSON A → Node Core + Storage
Goal:
Build a single-node working API

Tasks:
1. Project Setup
Initialize Go module

Folder structure:

/cmd/node
/internal/api
/internal/models
/internal/storage
2. Define Model
type Transaction struct {
    ID        string `json:"id"`
    Data      string `json:"data"`
    Timestamp int64  `json:"timestamp"`
}
3. Storage Layer
File: ledger.json

Functions:

LoadTransactions()
SaveTransaction(tx Transaction)
TransactionExists(id string)
4. API Handlers
Using Gin or net/http:

POST /transaction

Validate JSON

Save transaction

Return success

GET /transactions

Return all stored transactions

Deliverable:
→ You can POST manually and see it stored