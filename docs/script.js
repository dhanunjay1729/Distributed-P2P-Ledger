// State
const NUM_NODES = 5;
const nodes = [];
let txCounter = 1;
let blockCounter = 1;

// DOM Elements
const nodesContainer = document.getElementById('nodes-container');
const svgContainer = document.getElementById('connections-svg');
const packetsContainer = document.getElementById('packets-container');
const activityLog = document.getElementById('activity-log');

// Setup Nodes
class Node {
    constructor(id, x, y) {
        this.id = id;
        this.name = `Node ${String.fromCharCode(65 + id)}`; // Node A, B, C...
        this.x = x;
        this.y = y;
        this.mempool = [];
        this.chain = [{ index: 0, hash: '0000genesis' }];
        this.isMining = false;
        this.miningInterval = null;
        
        this.render();
    }

    render() {
        this.el = document.createElement('div');
        this.el.className = 'node glass-panel';
        this.el.style.left = `${this.x}px`;
        this.el.style.top = `${this.y}px`;
        
        this.updateUI();
        nodesContainer.appendChild(this.el);
    }

    updateUI() {
        const mempoolHTML = this.mempool.map(tx => `<div class="tx-item">${tx}</div>`).join('') || '<div style="color:var(--text-muted)">Empty</div>';
        const chainHTML = this.chain.slice().reverse().map(b => 
            `<div class="block-item">
                <span>Block #${b.index}</span>
                <span class="hash-preview">${b.hash.substring(0,6)}...</span>
            </div>`
        ).join('');

        this.el.innerHTML = `
            <div class="node-header">
                <span class="node-title">${this.name}</span>
                <span class="node-status">${this.isMining ? 'Mining...' : 'Idle'}</span>
            </div>
            <div class="mining-indicator"><div class="mining-progress"></div></div>
            <div class="node-section">
                <h4>Mempool</h4>
                <div class="mempool-list">${mempoolHTML}</div>
            </div>
            <div class="node-section">
                <h4>Blockchain</h4>
                <div class="chain-list">${chainHTML}</div>
            </div>
        `;

        if (this.isMining) {
            this.el.classList.add('mining');
        } else {
            this.el.classList.remove('mining');
        }
    }

    receiveTx(tx) {
        if (!this.mempool.includes(tx) && !this.txInChain(tx)) {
            this.mempool.push(tx);
            this.updateUI();
            this.startMining();
            
            // Gossip to neighbors
            setTimeout(() => {
                const targets = getNeighbors(this.id);
                targets.forEach(targetId => {
                    animatePacket(this.id, targetId, 'tx', () => {
                        nodes[targetId].receiveTx(tx);
                    });
                });
            }, 300);
        }
    }

    txInChain(tx) {
        // Simplified check: we just assume if chain > 1 it might be there.
        // In this simulation, we'll just rely on mempool clearing
        return false; 
    }

    startMining() {
        if (this.isMining || this.mempool.length === 0) return;
        this.isMining = true;
        this.updateUI();

        // Random mining time between 3s and 8s
        const mineTime = Math.random() * 5000 + 3000;
        
        this.miningInterval = setTimeout(() => {
            if (!this.isMining) return; // aborted
            
            this.mineBlock();
        }, mineTime);
    }

    stopMining() {
        this.isMining = false;
        clearTimeout(this.miningInterval);
        this.updateUI();
    }

    mineBlock() {
        this.stopMining();
        
        const block = {
            index: this.chain.length,
            hash: '0000' + Math.random().toString(16).substring(2, 8),
            txs: [...this.mempool]
        };

        logActivity(`⛏️ ${this.name} successfully mined Block #${block.index}`, 'log-mine');
        
        this.el.classList.add('success');
        setTimeout(() => this.el.classList.remove('success'), 1000);

        this.receiveBlock(block, this.name);
    }

    receiveBlock(block, sourceName) {
        // Check if we already have this block index (conflict) or if it's new
        const currentHeight = this.chain.length - 1;
        
        if (block.index > currentHeight) {
            // Adopt new block
            this.chain.push(block);
            
            // Clear mempool of mined txs
            this.mempool = this.mempool.filter(tx => !block.txs.includes(tx));
            this.stopMining(); // Stop mining current batch
            this.updateUI();

            // If we have remaining txs, restart mining
            if (this.mempool.length > 0) {
                this.startMining();
            }

            // Gossip block
            setTimeout(() => {
                const targets = getNeighbors(this.id);
                targets.forEach(targetId => {
                    animatePacket(this.id, targetId, 'block', () => {
                        nodes[targetId].receiveBlock(block, this.name);
                    });
                });
            }, 400);
            
        } else if (block.index === currentHeight && block.hash !== this.chain[currentHeight].hash) {
            // FORK DETECTED!
            logActivity(`⚠️ ${this.name} detected a fork at Block #${block.index}.`, 'log-fork');
            // In a real network, we wait for the longest chain. 
            // In this sim, we'll let the "Simulate Fork" button handle the resolution natively.
        } else if (block.index > currentHeight + 1) {
            // Longest chain rule applied (resolving fork)
            logActivity(`⚖️ ${this.name} applying Longest Chain Rule. Syncing...`, 'log-gossip');
            this.chain = block.fullChainSnapshot; // Cheat for simulation
            this.mempool = [];
            this.stopMining();
            this.updateUI();
        }
    }
}

// Initialization
function initNetwork() {
    const canvasRect = document.querySelector('.network-canvas').getBoundingClientRect();
    const centerX = canvasRect.width / 2;
    const centerY = canvasRect.height / 2;
    const radius = Math.min(centerX, centerY) - 120;

    // Create Nodes in a circle
    for (let i = 0; i < NUM_NODES; i++) {
        const angle = (i * 2 * Math.PI) / NUM_NODES - Math.PI / 2;
        const x = centerX + radius * Math.cos(angle);
        const y = centerY + radius * Math.sin(angle);
        nodes.push(new Node(i, x, y));
    }

    // Draw SVG connections (fully connected graph for simplicity)
    let svgHTML = '';
    for (let i = 0; i < NUM_NODES; i++) {
        for (let j = i + 1; j < NUM_NODES; j++) {
            svgHTML += `<line x1="${nodes[i].x}" y1="${nodes[i].y}" x2="${nodes[j].x}" y2="${nodes[j].y}" class="connection-line" />`;
        }
    }
    svgContainer.innerHTML = svgHTML;
}

// Helpers
function getNeighbors(nodeId) {
    // In this sim, everyone is connected to everyone else, but we'll gossip to 2 random peers to simulate fanout
    const others = [0, 1, 2, 3, 4].filter(id => id !== nodeId);
    // Shuffle and pick 2
    return others.sort(() => 0.5 - Math.random()).slice(0, 2);
}

function animatePacket(fromId, toId, type, onComplete) {
    const start = nodes[fromId];
    const end = nodes[toId];
    
    const packet = document.createElement('div');
    packet.className = `packet ${type}`;
    packet.style.left = `${start.x}px`;
    packet.style.top = `${start.y}px`;
    packetsContainer.appendChild(packet);

    // Animate using Web Animations API
    const animation = packet.animate([
        { left: `${start.x}px`, top: `${start.y}px` },
        { left: `${end.x}px`, top: `${end.y}px` }
    ], {
        duration: 800,
        easing: 'ease-in-out'
    });

    animation.onfinish = () => {
        packet.remove();
        onComplete();
    };
}

function logActivity(msg, className = '') {
    const li = document.createElement('li');
    li.textContent = msg;
    if (className) li.className = className;
    activityLog.appendChild(li);
    activityLog.scrollTop = activityLog.scrollHeight;
}

// Event Listeners
document.getElementById('send-tx-btn').addEventListener('click', () => {
    const txId = `tx_${Math.random().toString(36).substr(2, 4)}`;
    logActivity(`📨 Client sent transaction ${txId}`, 'log-gossip');
    
    // Pick a random node to receive the tx
    const randomNode = nodes[Math.floor(Math.random() * NUM_NODES)];
    randomNode.receiveTx(txId);
});

document.getElementById('trigger-fork-btn').addEventListener('click', () => {
    logActivity(`🚨 Simulating network fork...`, 'log-fork');
    
    // Force Node A and Node C to mine blocks at the exact same time
    const nodeA = nodes[0];
    const nodeC = nodes[2];
    
    // Give them a dummy tx if empty
    if (nodeA.mempool.length === 0) nodeA.mempool.push('tx_fork1');
    if (nodeC.mempool.length === 0) nodeC.mempool.push('tx_fork2');

    // Force mine
    nodeA.mineBlock();
    nodeC.mineBlock();

    setTimeout(() => {
        logActivity(`⚔️ Network is split! Wait for next block to resolve...`, 'log-fork');
        
        // Force Node A to mine the NEXT block quickly to win the longest chain
        setTimeout(() => {
            logActivity(`👑 Node A mined Block #${nodeA.chain.length}! Broadcasting Longest Chain...`, 'log-mine');
            const winnerBlock = {
                index: nodeA.chain.length,
                hash: '0000winner',
                txs: [],
                fullChainSnapshot: JSON.parse(JSON.stringify(nodeA.chain)) // Inject full chain for sim
            };
            winnerBlock.fullChainSnapshot.push({ index: winnerBlock.index, hash: winnerBlock.hash });
            
            nodeA.receiveBlock(winnerBlock, nodeA.name);
        }, 2000);
        
    }, 1500);
});

// Start
window.addEventListener('load', initNetwork);
