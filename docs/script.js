// State
const NUM_NODES = 6; // Increased to 6 nodes
const nodes = [];

// Timings (Balanced for readability without causing endless network collisions)
const PACKET_SPEED = 1800; // Travel time
const GOSSIP_DELAY = 800; // Delay before forwarding
const MINING_TIME_MIN = 8000; 
const MINING_TIME_MAX = 25000;

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
        this.tooltipTimeout = null;
        
        this.render();
    }

    render() {
        this.el = document.createElement('div');
        this.el.className = 'node glass-panel';
        this.el.style.left = `${this.x}px`;
        this.el.style.top = `${this.y}px`;
        
        // Add tooltip element
        this.tooltipEl = document.createElement('div');
        this.tooltipEl.className = 'node-tooltip';
        this.el.appendChild(this.tooltipEl);

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

        // We update the innerHTML of specific sections to not destroy the tooltip
        let header = this.el.querySelector('.node-header');
        if (!header) {
            this.el.insertAdjacentHTML('afterbegin', `
                <div class="node-header">
                    <span class="node-title">${this.name}</span>
                    <span class="node-status">${this.isMining ? 'Mining...' : 'Idle'}</span>
                </div>
                <div class="mining-indicator"><div class="mining-progress"></div></div>
                <div class="node-section">
                    <h4>Mempool (Waiting Room)</h4>
                    <div class="mempool-list"></div>
                </div>
                <div class="node-section">
                    <h4>Blockchain (Permanent)</h4>
                    <div class="chain-list"></div>
                </div>
            `);
        } else {
            this.el.querySelector('.node-status').textContent = this.isMining ? 'Mining...' : 'Idle';
            this.el.querySelector('.mempool-list').innerHTML = mempoolHTML;
            this.el.querySelector('.chain-list').innerHTML = chainHTML;
        }

        if (this.isMining) {
            this.el.classList.add('mining');
        } else {
            this.el.classList.remove('mining');
        }
    }

    showTooltip(message, duration = 3000, colorClass = '') {
        this.tooltipEl.textContent = message;
        this.tooltipEl.className = `node-tooltip show ${colorClass}`;
        
        clearTimeout(this.tooltipTimeout);
        this.tooltipTimeout = setTimeout(() => {
            this.tooltipEl.classList.remove('show');
        }, duration);
    }

    receiveTx(tx) {
        if (!this.mempool.includes(tx) && !this.txInChain(tx)) {
            this.mempool.push(tx);
            this.updateUI();
            this.showTooltip("1. Received TX. Adding to Mempool.", 4000, 'tooltip-purple');
            
            // Start mining if we aren't already
            setTimeout(() => this.startMining(), 2500);
            
            // Gossip to neighbors (Slowed down for visibility)
            setTimeout(() => {
                this.showTooltip("Gossiping TX to peers...", 3000, 'tooltip-purple');
                const targets = getNeighbors(this.id);
                targets.forEach(targetId => {
                    animatePacket(this.id, targetId, 'tx', () => {
                        nodes[targetId].receiveTx(tx);
                    });
                });
            }, GOSSIP_DELAY);
        }
    }

    txInChain(tx) {
        return false; 
    }

    startMining() {
        if (this.isMining || this.mempool.length === 0) return;
        this.isMining = true;
        this.updateUI();
        
        this.showTooltip("2. Grinding CPU to solve Block (PoW)...", 8000, 'tooltip-blue');

        // Random mining time
        const mineTime = Math.random() * (MINING_TIME_MAX - MINING_TIME_MIN) + MINING_TIME_MIN;
        
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
        this.showTooltip("3. Block Solved! Sending to network.", 5000, 'tooltip-green');
        
        this.el.classList.add('success');
        setTimeout(() => this.el.classList.remove('success'), 1500);

        this.receiveBlock(block, this.name);
    }

    receiveBlock(block, sourceName) {
        const currentHeight = this.chain.length - 1;
        
        if (block.index > currentHeight) {
            // Adopt new block
            this.chain.push(block);
            
            // Clear mempool of mined txs
            this.mempool = this.mempool.filter(tx => !block.txs.includes(tx));
            this.stopMining(); // Stop mining current batch because block is solved
            this.updateUI();

            if (sourceName !== this.name) {
                this.showTooltip(`4. Validated Block #${block.index}. Saved to chain!`, 5000, 'tooltip-green');
            }

            // If we have remaining txs, restart mining
            if (this.mempool.length > 0) {
                setTimeout(() => this.startMining(), 3000);
            }

            // Gossip block to peers
            setTimeout(() => {
                const targets = getNeighbors(this.id);
                targets.forEach(targetId => {
                    animatePacket(this.id, targetId, 'block', () => {
                        nodes[targetId].receiveBlock(block, this.name);
                    });
                });
            }, GOSSIP_DELAY);
            
        } else if (block.index === currentHeight && block.hash !== this.chain[currentHeight].hash) {
            // FORK DETECTED!
            logActivity(`⚠️ ${this.name} detected a fork at Block #${block.index}.`, 'log-fork');
            this.showTooltip("Fork detected! Resolving...", 5000, 'tooltip-red');
            
            // Simulator Tie-Breaker: In a real network, nodes wait for the next block to decide the longest chain.
            // If the user didn't explicitly trigger a fork, we automatically tie-break via hash comparison 
            // so the simulation doesn't get permanently split.
            if (block.hash > this.chain[currentHeight].hash) {
                logActivity(`⚖️ ${this.name} resolved fork: Adopted stronger block ${block.hash.substring(0,6)}`, 'log-gossip');
                this.chain[currentHeight] = block;
                this.mempool = this.mempool.filter(tx => !block.txs.includes(tx));
                this.stopMining();
                this.updateUI();
            } else {
                logActivity(`🛡️ ${this.name} resolved fork: Rejected weaker block ${block.hash.substring(0,6)}`, 'log-gossip');
            }
        } else if (block.index > currentHeight + 1) {
            // Longest chain rule applied (resolving fork)
            logActivity(`⚖️ ${this.name} applying Longest Chain Rule. Syncing...`, 'log-gossip');
            this.showTooltip("Longest Chain won! Discarding old chain.", 7000, 'tooltip-green');
            
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
    // Pick 2 random peers to gossip to
    const others = [];
    for (let i=0; i<NUM_NODES; i++) {
        if (i !== nodeId) others.push(i);
    }
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
        duration: PACKET_SPEED, // Slower travel time
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
    
    const nodeA = nodes[0];
    const nodeC = nodes[2];
    
    if (nodeA.mempool.length === 0) nodeA.mempool.push('tx_fork1');
    if (nodeC.mempool.length === 0) nodeC.mempool.push('tx_fork2');

    nodeA.mineBlock();
    nodeC.mineBlock();

    setTimeout(() => {
        logActivity(`⚔️ Network is split! Wait for next block to resolve...`, 'log-fork');
        
        // Force Node A to mine the NEXT block to win the longest chain
        setTimeout(() => {
            logActivity(`👑 Node A mined Block #${nodeA.chain.length}! Broadcasting Longest Chain...`, 'log-mine');
            const winnerBlock = {
                index: nodeA.chain.length,
                hash: '0000winner',
                txs: [],
                fullChainSnapshot: JSON.parse(JSON.stringify(nodeA.chain))
            };
            winnerBlock.fullChainSnapshot.push({ index: winnerBlock.index, hash: winnerBlock.hash });
            
            nodeA.receiveBlock(winnerBlock, nodeA.name);
        }, 8000); // Wait 8 seconds so user can read the fork tooltips
        
    }, 4500);
});

// Start
window.addEventListener('load', initNetwork);
