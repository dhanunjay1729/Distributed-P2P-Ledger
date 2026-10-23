/* Shared by the browser animation and Node regression tests. No DOM access. */
(function (root) {
    'use strict';
    const VERSION = 1, DIFFICULTY = 2;
    const copy = value => JSON.parse(JSON.stringify(value));
    const canonical = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g,
        character => '\\u' + character.charCodeAt(0).toString(16).padStart(4, '0'));

    async function hash(block) {
        // Field order and escaping match Go's versioned hash header.
        const header = {
            version: block.version, difficulty: block.difficulty,
            index: block.index, timestamp: block.timestamp,
            transactions: block.transactions, prev_hash: block.prev_hash,
            nonce: block.nonce
        };
        const digest = await root.crypto.subtle.digest('SHA-256', new TextEncoder().encode(canonical(header)));
        return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
    }
    async function genesis() {
        const block = { version: VERSION, difficulty: DIFFICULTY, index: 0,
            timestamp: 1767225600, transactions: [], prev_hash: '', nonce: 0 };
        block.hash = await hash(block);
        return block;
    }
    function transaction(id) {
        return { id, data: 'Message ' + id, timestamp: 1 };
    }
    async function mine(chain, ids, cancelled = () => false) {
        if (!ids.length || ids.length > 100) throw new Error('Expected 1–100 pending messages');
        const tip = chain[chain.length - 1];
        const block = { version: VERSION, difficulty: DIFFICULTY, index: tip.index + 1,
            timestamp: Math.max(Math.floor(Date.now() / 1000), tip.timestamp),
            transactions: ids.map(transaction), prev_hash: tip.hash, nonce: 0 };
        while (true) {
            if (cancelled()) return null;
            block.hash = await hash(block);
            if (block.hash.startsWith('0'.repeat(DIFFICULTY))) return block;
            block.nonce++;
        }
    }
    function validTransaction(tx) {
        return tx && typeof tx.id === 'string' && tx.id.trim() && new TextEncoder().encode(tx.id).length <= 128 &&
            typeof tx.data === 'string' && tx.data.trim() && new TextEncoder().encode(tx.data).length <= 4096 &&
            Number.isSafeInteger(tx.timestamp) && tx.timestamp > 0;
    }
    async function validChain(chain) {
        try {
            if (!Array.isArray(chain) || !chain.length || chain.length > 10000) return false;
            const expected = await genesis(), first = chain[0];
            if (first.version !== VERSION || first.difficulty !== DIFFICULTY || first.index !== 0 ||
                first.timestamp !== expected.timestamp || first.prev_hash !== '' || first.nonce !== 0 ||
                !Array.isArray(first.transactions) || first.transactions.length !== 0 || first.hash !== expected.hash) return false;
            const seen = new Set();
            for (let i = 1; i < chain.length; i++) {
                const block = chain[i], previous = chain[i - 1];
                if (block.version !== VERSION || block.difficulty !== DIFFICULTY || block.index !== i ||
                    block.prev_hash !== previous.hash || !Number.isSafeInteger(block.timestamp) ||
                    block.timestamp < previous.timestamp || block.timestamp > Date.now() / 1000 + 120 ||
                    !Number.isSafeInteger(block.nonce) || block.nonce < 0 ||
                    !Array.isArray(block.transactions) || !block.transactions.length || block.transactions.length > 100) return false;
                for (const tx of block.transactions) {
                    if (!validTransaction(tx) || seen.has(tx.id)) return false;
                    seen.add(tx.id);
                }
                if (typeof block.hash !== 'string' || !block.hash.startsWith('0'.repeat(DIFFICULTY)) ||
                    block.hash !== await hash(block)) return false;
            }
            return true;
        } catch (_) { return false; }
    }
    function better(candidate, current) {
        if (candidate.length !== current.length) return candidate.length > current.length;
        return candidate.length > 0 && candidate[candidate.length - 1].hash < current[current.length - 1].hash;
    }
    function reconcile(oldChain, newChain, pending) {
        const confirmed = new Set(newChain.flatMap(block => block.transactions.map(tx => tx.id)));
        return Array.from(new Set([...pending, ...oldChain.flatMap(block => block.transactions.map(tx => tx.id))]))
            .filter(id => !confirmed.has(id)).sort();
    }

    const api = { hash, genesis, mine, validChain, better, reconcile, copy, canonical };
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    else root.LedgerSim = api;
})(globalThis);
