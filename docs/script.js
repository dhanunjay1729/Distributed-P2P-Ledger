/* Browser-only animation. Hashing/validation live in simulator-core.js. */
'use strict';
const Sim = globalThis.LedgerSim;
const NUM_NODES = 6, GOSSIP_DELAY = 500, PACKET_SPEED = 900;
const nodes = [];
let lastMinedTx = null, forkInProgress = false, syncTimer = null;
const nodesContainer = document.getElementById('nodes-container');
const svgContainer = document.getElementById('connections-svg');
const packetsContainer = document.getElementById('packets-container');
const activityLog = document.getElementById('activity-log');
const buttons = Array.from(document.querySelectorAll('.controls button'));

class Node {
    constructor(id, genesis) {
        this.id = id;
        this.name = 'Node ' + String.fromCharCode(65 + id);
        this.chain = [Sim.copy(genesis)];
        this.mempool = [];
        this.isMining = false;
        this.generation = 0;
        this.render();
    }
    render() {
        this.el = document.createElement('div');
        this.el.className = 'node glass-panel';
        this.tooltipEl = document.createElement('div');
        this.tooltipEl.className = 'node-tooltip';
        this.contentEl = document.createElement('div');
        this.contentEl.className = 'node-content';
        this.el.append(this.tooltipEl, this.contentEl);
        nodesContainer.appendChild(this.el);
        this.updateUI();
    }
    updateUI() {
        this.contentEl.replaceChildren();
        const header = document.createElement('div');
        header.className = 'node-header';
        const title = document.createElement('span');
        title.className = 'node-title';
        title.textContent = this.name;
        const status = document.createElement('span');
        status.className = 'node-status';
        status.textContent = this.isMining ? 'Mining…' : 'Idle';
        header.append(title, status);
        const indicator = document.createElement('div');
        indicator.className = 'mining-indicator';
        const progress = document.createElement('div');
        progress.className = 'mining-progress';
        indicator.appendChild(progress);
        this.contentEl.append(header, indicator);
        const section = (heading, className) => {
            const outer = document.createElement('div');
            outer.className = 'node-section';
            const label = document.createElement('h4');
            label.textContent = heading;
            const list = document.createElement('div');
            list.className = className;
            outer.append(label, list);
            this.contentEl.appendChild(outer);
            return list;
        };
        const pending = section('Mempool (' + this.mempool.length + ')', 'mempool-list');
        if (!this.mempool.length) pending.textContent = 'Empty';
        for (const id of this.mempool) {
            const row = document.createElement('div');
            row.className = 'tx-item';
            row.textContent = id;
            pending.appendChild(row);
        }
        const blocks = section('Blockchain', 'chain-list');
        for (const block of [...this.chain].reverse()) {
            const row = document.createElement('div');
            row.className = 'block-item';
            const index = document.createElement('span');
            index.textContent = 'Block #' + block.index;
            const hash = document.createElement('span');
            hash.className = 'hash-preview';
            hash.textContent = block.hash.slice(0, 8) + '…';
            row.append(index, hash);
            blocks.appendChild(row);
        }
        this.el.classList.toggle('mining', this.isMining);
    }
    showTooltip(message, color = '') {
        this.tooltipEl.textContent = message;
        this.tooltipEl.className = 'node-tooltip show ' + color + (this.y < 180 ? ' tooltip-bottom' : '');
        clearTimeout(this.tooltipTimeout);
        this.tooltipTimeout = setTimeout(() => this.tooltipEl.classList.remove('show'), 3000);
    }
    hasConfirmed(id) {
        return this.chain.some(block => block.transactions.some(tx => tx.id === id));
    }
    receiveTx(id, quiet = false) {
        if (this.hasConfirmed(id)) {
            if (!quiet) {
                this.showTooltip('Replay rejected: already confirmed', 'tooltip-red');
                logActivity(this.name + ' rejected replay of ' + id, 'log-security');
            }
            return;
        }
        if (this.mempool.includes(id) || this.mempool.length >= 1000) return;
        this.mempool.push(id);
        this.mempool.sort();
        this.updateUI();
        this.showTooltip('Message added to mempool', 'tooltip-purple');
        setTimeout(() => this.startMining(), 1800);
        setTimeout(() => getNeighbors(this.id).forEach(target => {
            animatePacket(this.id, target, 'tx', () => nodes[target].receiveTx(id, true));
        }), GOSSIP_DELAY);
    }
    stopMining() {
        this.generation++;
        this.isMining = false;
        clearTimeout(this.miningTimer);
    }
    startMining() {
        if (this.isMining || !this.mempool.length || forkInProgress) return;
        this.isMining = true;
        const generation = ++this.generation;
        this.updateUI();
        this.showTooltip('Preparing proof of work…', 'tooltip-blue');
        // Delay makes the animation readable; the hash search itself is real.
        this.miningTimer = setTimeout(async () => {
            try {
                const base = Sim.copy(this.chain);
                const block = await Sim.mine(base, this.mempool.slice(0, 100),
                    () => generation !== this.generation || forkInProgress);
                if (!block || generation !== this.generation) return;
                lastMinedTx = block.transactions[0].id;
                logActivity(this.name + ' mined block #' + block.index + ' (nonce ' + block.nonce + ')', 'log-mine');
                await this.receiveChain([...base, block], this.name);
            } catch (error) {
                this.stopMining();
                this.updateUI();
                logActivity('Mining error: ' + error.message, 'log-security');
            }
        }, 2500 + Math.random() * 4000);
    }
    async receiveChain(chain, sourceName) {
        // Snapshot first, then compare with the LIVE chain after async validation.
        const candidate = Sim.copy(chain);
        if (!await Sim.validChain(candidate) || !Sim.better(candidate, this.chain)) return false;
        const fork = candidate.length === this.chain.length;
        const pending = Sim.reconcile(this.chain, candidate, this.mempool);
        if (pending.length > 1000) return false;
        this.stopMining();
        this.mempool = pending;
        this.chain = candidate;
        this.updateUI();
        this.showTooltip(fork ? 'Fork resolved; orphan messages restored' : 'Validated chain adopted', 'tooltip-green');
        logActivity(this.name + ' adopted height ' + (candidate.length - 1) + ' from ' + sourceName +
            (fork ? ' using the smaller-hash tie-break' : ''), fork ? 'log-fork' : 'log-gossip');
        if (!forkInProgress) {
            this.broadcastChain();
            setTimeout(() => this.startMining(), 1800);
        }
        return true;
    }
    broadcastChain() {
        const snapshot = Sim.copy(this.chain);
        setTimeout(() => {
            if (forkInProgress) return;
            getNeighbors(this.id).forEach(target => animatePacket(this.id, target, 'block',
                () => nodes[target].receiveChain(snapshot, this.name)));
        }, GOSSIP_DELAY);
    }
}

function getNeighbors(id) {
    const others = nodes.map(node => node.id).filter(other => other !== id);
    for (let i = others.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [others[i], others[j]] = [others[j], others[i]];
    }
    return others.slice(0, 2);
}
function animatePacket(from, to, type, complete) {
    const packet = document.createElement('div');
    packet.className = 'packet ' + type;
    packet.style.left = nodes[from].x + 'px';
    packet.style.top = nodes[from].y + 'px';
    packetsContainer.appendChild(packet);
    const animation = packet.animate([
        { left: nodes[from].x + 'px', top: nodes[from].y + 'px' },
        { left: nodes[to].x + 'px', top: nodes[to].y + 'px' }
    ], { duration: PACKET_SPEED, easing: 'ease-in-out' });
    animation.onfinish = () => { packet.remove(); Promise.resolve(complete()).catch(error => logActivity(error.message, 'log-security')); };
}
function logActivity(message, className = '') {
    const item = document.createElement('li');
    item.textContent = message;
    item.className = className;
    activityLog.appendChild(item);
    while (activityLog.children.length > 200) activityLog.firstChild.remove();
    activityLog.scrollTop = activityLog.scrollHeight;
}
function layout() {
    const canvas = document.querySelector('.network-canvas');
    canvas.style.minHeight = '';
    const width = canvas.clientWidth, height = canvas.clientHeight;
    const compact = width < 680 || height < 700;
    const columns = width < 450 ? 1 : 2;
    const rows = Math.ceil(NUM_NODES / columns);
    canvas.classList.toggle('compact', compact);
    if (compact) canvas.style.minHeight = rows * 230 + 'px';
    else canvas.style.minHeight = '';
    nodes.forEach((node, i) => {
        if (compact) {
            node.x = width * ((i % columns) + 0.5) / columns;
            node.y = 120 + Math.floor(i / columns) * 230;
        } else {
            const angle = i * 2 * Math.PI / NUM_NODES - Math.PI / 2;
            node.x = width / 2 + (width / 2 - 115) * Math.cos(angle);
            node.y = height / 2 + (height / 2 - 145) * Math.sin(angle);
        }
        node.el.style.left = node.x + 'px';
        node.el.style.top = node.y + 'px';
    });
    svgContainer.replaceChildren();
    nodes.forEach((node, i) => nodes.slice(i + 1).forEach(other => {
        const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
        for (const [key, value] of Object.entries({ x1: node.x, y1: node.y, x2: other.x, y2: other.y })) line.setAttribute(key, value);
        line.setAttribute('class', 'connection-line');
        svgContainer.appendChild(line);
    }));
}
function synchronize() {
    if (forkInProgress) return;
    // Periodic reconciliation repairs missed fanout messages, as in the backend.
    nodes.forEach(node => getNeighbors(node.id).forEach(peer => {
        node.receiveChain(nodes[peer].chain, nodes[peer].name).catch(error => logActivity(error.message));
        nodes[peer].mempool.forEach(id => node.receiveTx(id, true));
    }));
}

document.getElementById('send-tx-btn').addEventListener('click', () => {
    const id = 'tx_' + crypto.randomUUID().slice(0, 8);
    nodes[Math.floor(Math.random() * nodes.length)].receiveTx(id);
    logActivity('Submitted ' + id, 'log-gossip');
});
document.getElementById('replay-btn').addEventListener('click', () => {
    if (!lastMinedTx) { logActivity('Submit a transaction and wait for confirmation first.'); return; }
    logActivity('Replaying ' + lastMinedTx + '. This tests ID deduplication, not spending money.', 'log-security');
    nodes.forEach(node => node.receiveTx(lastMinedTx));
});
document.getElementById('trigger-fork-btn').addEventListener('click', async () => {
    if (forkInProgress) return;
    forkInProgress = true;
    buttons.forEach(button => { button.disabled = true; });
    try {
        nodes.forEach(node => { node.stopMining(); node.updateUI(); });
        const base = Sim.copy(nodes.reduce((best, node) => Sim.better(node.chain, best) ? node.chain : best, nodes[0].chain));
        await Promise.all(nodes.map(node => node.receiveChain(base, 'fork setup')));
        logActivity('Mining two different blocks on the same parent…', 'log-fork');
        const suffix = crypto.randomUUID().slice(0, 8);
        const [a, b] = await Promise.all([Sim.mine(base, ['fork_a_' + suffix]), Sim.mine(base, ['fork_b_' + suffix])]);
        await Promise.all([nodes[0].receiveChain([...base, a], 'Node A'), nodes[2].receiveChain([...base, b], 'Node C')]);
        logActivity('Equal-height fork: both blocks are valid. The smaller tip hash breaks the tie.', 'log-fork');
        await new Promise(resolve => setTimeout(resolve, 3500));
    } catch (error) { logActivity('Fork demo error: ' + error.message, 'log-security'); }
    finally {
        forkInProgress = false;
        buttons.forEach(button => { button.disabled = false; });
        nodes.forEach(node => { node.broadcastChain(); node.startMining(); });
        synchronize();
    }
});

window.addEventListener('load', async () => {
    buttons.forEach(button => { button.disabled = true; });
    try {
        const genesis = await Sim.genesis();
        for (let i = 0; i < NUM_NODES; i++) nodes.push(new Node(i, genesis));
        layout();
        buttons.forEach(button => { button.disabled = false; });
        syncTimer = setInterval(synchronize, 5000);
        logActivity('Browser simulation ready. SHA-256 hashes are real; animation delays are simulated.');
    } catch (error) { logActivity('Cannot start: ' + error.message + '. Open via localhost or HTTPS.', 'log-security'); }
});
window.addEventListener('resize', layout);
window.addEventListener('pagehide', () => { clearInterval(syncTimer); nodes.forEach(node => node.stopMining()); });
