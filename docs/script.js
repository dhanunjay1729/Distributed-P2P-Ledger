// State
const NUM_NODES = 6;
const nodes = [];
let lastMinedTx = null; // Track last mined TX for double-spend demo

// Timings
const PACKET_SPEED = 1800;
const GOSSIP_DELAY = 800;
const MINING_TIME_MIN = 8000;
const MINING_TIME_MAX = 25000;

// DOM Elements
const nodesContainer = document.getElementById('nodes-container');
const svgContainer = document.getElementById('connections-svg');
const packetsContainer = document.getElementById('packets-container');
const activityLog = document.getElementById('activity-log');

// Node class
class Node {
    constructor(id, x, y) {
        this.id = id;
        this.name = `Node ${String.fromCharCode(65 + id)}`;
        this.x = x;
        this.y = y;
        this.mempool = [];
        this.chain = [{ index: 0, hash: '0000genesis', txs: [] }];
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
        
        this.tooltipEl = document.createElement('div');
        this.tooltipEl.className = 'node-tooltip';
        this.el.appendChild(this.tooltipEl);

        this.contentEl = document.createElement('div');
        this.contentEl.className = 'node-content';
        this.el.appendChild(this.contentEl);
        
        this.updateUI();
        nodesContainer.appendChild(this.el);
    }

    updateUI() {
        const mempoolHTML = this.mempool.map(tx => `<div class="tx-item">${tx}</div>`).join('') || '<div style="color:var(--text-muted)">Empty</div>';
        const chainHTML = this.chain.slice().reverse().map(b => 
            `<div class="block-item">
                <span>Block #${b.index}</span>
                <span class="hash-preview">${b.hash.substring(0,8)}…</span>
            </div>`
        ).join('');

        this.contentEl.innerHTML = `
            <div class="node-header">
                <span class="node-title">${this.name}</span>
                <span class="node-status">${this.isMining ? 'Mining…' : 'Idle'}</span>
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

    showTooltip(message, duration = 3000, colorClass = '') {
        this.tooltipEl.textContent = message;
        this.tooltipEl.className = `node-tooltip show ${colorClass}`;
        
        clearTimeout(this.tooltipTimeout);
        this.tooltipTimeout = setTimeout(() => {
            this.tooltipEl.classList.remove('show');
        }, duration);
    }

    // Check if a tx is already confirmed in the blockchain
    txExistsInChain(tx) {
        for (let i = 1; i < this.chain.length; i++) {
            if (this.chain[i].txs && this.chain[i].txs.includes(tx)) {
                return true;
            }
        }
        return false;
    }

    receiveTx(tx) {
        // DOUBLE-SPEND CHECK: Reject if TX already in blockchain
        if (this.txExistsInChain(tx)) {
            this.showTooltip(`❌ REJECTED: Already in chain!`, 5000, 'tooltip-red');
            this.el.classList.add('rejected');
            setTimeout(() => this.el.classList.remove('rejected'), 1500);
            logActivity(`🚫 [Security] ${this.name} rejected double-spend of '${tx}'`, 'log-security');
            return; // Do NOT gossip it further
        }

        if (!this.mempool.includes(tx)) {
            this.mempool.push(tx);
            this.updateUI();
            this.showTooltip("1. Received TX → Mempool", 4000, 'tooltip-purple');
            
            setTimeout(() => this.startMining(), 2500);
            
            // Gossip to neighbors
            setTimeout(() => {
                this.showTooltip("Gossiping TX to peers…", 3000, 'tooltip-purple');
                const targets = getNeighbors(this.id);
                targets.forEach(targetId => {
                    animatePacket(this.id, targetId, 'tx', () => {
                        nodes[targetId].receiveTx(tx);
                    });
                });
            }, GOSSIP_DELAY);
        }
    }

    // Attempt a double-spend (for the demo button)
    attemptDoubleSpend(tx) {
        this.showTooltip(`⚠️ Double-spend attempt: ${tx}`, 5000, 'tooltip-red');
        logActivity(`🕵️ [Attack] Attacker re-submits '${tx}' to ${this.name}…`, 'log-security');
        
        // Send the TX into the network — nodes will reject it
        setTimeout(() => {
            // Gossip the fraudulent TX to all neighbors
            const targets = getNeighbors(this.id);
            targets.forEach(targetId => {
                animatePacket(this.id, targetId, 'rejected-packet', () => {
                    nodes[targetId].receiveTx(tx);
                    // Continue spreading so all nodes get a chance to reject
                    setTimeout(() => {
                        const nextTargets = getNeighbors(targetId);
                        nextTargets.forEach(nextId => {
                            animatePacket(targetId, nextId, 'rejected-packet', () => {
                                nodes[nextId].receiveTx(tx);
                            });
                        });
                    }, GOSSIP_DELAY);
                });
            });
        }, 800);
    }

    startMining() {
        if (this.isMining || this.mempool.length === 0) return;
        this.isMining = true;
        this.updateUI();
        
        this.showTooltip("2. Solving Block (PoW)…", 8000, 'tooltip-blue');

        const mineTime = Math.random() * (MINING_TIME_MAX - MINING_TIME_MIN) + MINING_TIME_MIN;
        
        this.miningInterval = setTimeout(() => {
            if (!this.isMining) return;
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

        // Track the last mined TX for double-spend demo
        if (block.txs.length > 0) {
            lastMinedTx = block.txs[0];
        }

        logActivity(`🏆 [Proof of Work] ${this.name} solved Block #${block.index}! Broadcasting…`, 'log-mine');
        this.showTooltip("3. Block Solved! Broadcasting.", 5000, 'tooltip-green');
        
        this.el.classList.add('success');
        setTimeout(() => this.el.classList.remove('success'), 1500);

        this.receiveBlock(block, this.name);
    }

    receiveBlock(block, sourceName) {
        const currentHeight = this.chain.length - 1;
        
        if (block.index > currentHeight) {
            this.chain.push(block);
            this.mempool = this.mempool.filter(tx => !block.txs.includes(tx));
            this.stopMining();
            this.updateUI();

            if (sourceName !== this.name) {
                this.showTooltip(`4. Validated Block #${block.index} ✓`, 5000, 'tooltip-green');
            }

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
            logActivity(`⚠️ [Fork] ${this.name} received conflicting Block #${block.index}.`, 'log-fork');
            this.showTooltip("Fork! Resolving…", 5000, 'tooltip-red');
            
            if (block.hash > this.chain[currentHeight].hash) {
                logActivity(`⚖️ [Consensus] ${this.name} adopted the stronger block.`, 'log-gossip');
                this.chain[currentHeight] = block;
                this.mempool = this.mempool.filter(tx => !block.txs.includes(tx));
                this.stopMining();
                this.updateUI();
            } else {
                logActivity(`🛡️ [Consensus] ${this.name} kept its own block.`, 'log-gossip');
            }
        } else if (block.index > currentHeight + 1) {
            logActivity(`👑 [Longest Chain] ${this.name} synced to a longer chain!`, 'log-gossip');
            this.showTooltip("Longest Chain wins! Syncing…", 7000, 'tooltip-green');
            
            this.chain = block.fullChainSnapshot;
            this.mempool = [];
            this.stopMining();
            this.updateUI();
        }
    }
}

// Layout: 3 columns x 2 rows grid for guaranteed no-overlap
function initNetwork() {
    const canvasRect = document.querySelector('.network-canvas').getBoundingClientRect();
    const W = canvasRect.width;
    const H = canvasRect.height;
    
    // 3 columns, 2 rows, evenly spaced
    const cols = 3;
    const rows = 2;
    const padX = 120; // horizontal padding from edges
    const padY = 100; // vertical padding from edges
    
    const colSpacing = (W - 2 * padX) / (cols - 1);
    const rowSpacing = (H - 2 * padY) / (rows - 1);
    
    // Positions: Row 0 = [A, B, C], Row 1 = [D, E, F]
    const positions = [];
    for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
            positions.push({
                x: padX + c * colSpacing,
                y: padY + r * rowSpacing
            });
        }
    }

    for (let i = 0; i < NUM_NODES; i++) {
        nodes.push(new Node(i, positions[i].x, positions[i].y));
    }

    // Draw SVG connections
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
    const others = [];
    for (let i = 0; i < NUM_NODES; i++) {
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

    const animation = packet.animate([
        { left: `${start.x}px`, top: `${start.y}px` },
        { left: `${end.x}px`, top: `${end.y}px` }
    ], {
        duration: PACKET_SPEED,
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

// === EVENT LISTENERS ===

// Send Transaction
document.getElementById('send-tx-btn').addEventListener('click', () => {
    const txId = `tx_${Math.random().toString(36).substr(2, 4)}`;
    logActivity(`📝 [Client] Sent transaction '${txId}' into the network.`, 'log-gossip');
    
    const randomNode = nodes[Math.floor(Math.random() * NUM_NODES)];
    randomNode.receiveTx(txId);
});

// Double Spend Attack
document.getElementById('double-spend-btn').addEventListener('click', () => {
    if (!lastMinedTx) {
        logActivity(`⏳ [Info] Send a transaction first and wait for it to be mined!`);
        return;
    }
    
    logActivity(`🚨 [Attack] Attempting to double-spend '${lastMinedTx}'…`, 'log-security');
    logActivity(`💡 [Info] This TX was already confirmed in the blockchain. Every honest node should reject it.`, 'log-security');
    
    // Pick a random node to submit the fraudulent TX
    const attackNode = nodes[Math.floor(Math.random() * NUM_NODES)];
    attackNode.attemptDoubleSpend(lastMinedTx);
});

// Simulate Fork
document.getElementById('trigger-fork-btn').addEventListener('click', () => {
    logActivity(`🚨 [Simulation] Triggering a forced network split…`, 'log-fork');
    
    const nodeA = nodes[0];
    const nodeC = nodes[2];
    
    if (nodeA.mempool.length === 0) nodeA.mempool.push('tx_fork1');
    if (nodeC.mempool.length === 0) nodeC.mempool.push('tx_fork2');

    nodeA.mineBlock();
    nodeC.mineBlock();

    setTimeout(() => {
        logActivity(`⚔️ [Simulation] Network is split! Waiting for a winner…`, 'log-fork');
        
        setTimeout(() => {
            logActivity(`👑 [Simulation] Node A mined Block #${nodeA.chain.length}! Broadcasting Longest Chain…`, 'log-mine');
            const winnerBlock = {
                index: nodeA.chain.length,
                hash: '0000winner',
                txs: [],
                fullChainSnapshot: JSON.parse(JSON.stringify(nodeA.chain))
            };
            winnerBlock.fullChainSnapshot.push({ index: winnerBlock.index, hash: winnerBlock.hash });
            
            nodeA.receiveBlock(winnerBlock, nodeA.name);
        }, 8000);
        
    }, 4500);
});

// Start
window.addEventListener('load', initNetwork);
