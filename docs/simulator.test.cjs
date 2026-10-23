const { test } = require('node:test');
const assert = require('node:assert/strict');
if (!globalThis.crypto) globalThis.crypto = require('node:crypto').webcrypto;
const sim = require('./simulator-core.js');

test('valid work is accepted, complete transaction tampering is rejected', async () => {
    const genesis = await sim.genesis();
    const block = await sim.mine([genesis], ['a']);
    assert.equal(await sim.validChain([genesis, block]), true);
    for (const field of ['data', 'timestamp']) {
        const altered = sim.copy(block);
        altered.transactions[0][field] = field === 'data' ? 'tampered' : 2;
        assert.notEqual(await sim.hash(altered), block.hash);
        assert.equal(await sim.validChain([genesis, altered]), false);
    }
});
test('missing parents, fabricated work and genesis tampering are rejected', async () => {
    const genesis = await sim.genesis(), block = await sim.mine([genesis], ['a']);
    const gap = sim.copy(block); gap.index = 3;
    assert.equal(await sim.validChain([genesis, gap]), false);
    const forged = sim.copy(block); forged.hash = '0000winner';
    assert.equal(await sim.validChain([genesis, forged]), false);
    const altered = sim.copy(genesis); altered.timestamp++;
    assert.equal(await sim.validChain([altered]), false);
});
test('fork rule is stable and orphan messages return to pending', async () => {
    const genesis = await sim.genesis();
    const a = await sim.mine([genesis], ['a']), b = await sim.mine([genesis], ['b']);
    const old = [genesis, a], candidate = [genesis, b];
    assert.equal(sim.better(candidate, old), b.hash < a.hash);
    assert.equal(sim.better(old, candidate), a.hash < b.hash);
    assert.deepEqual(sim.reconcile(old, candidate, ['b', 'pending']), ['a', 'pending']);
    const replay = await sim.mine(old, ['a']);
    assert.equal(await sim.validChain([...old, replay]), false);
});
test('obsolete mining can be cancelled', async () => {
    assert.equal(await sim.mine([await sim.genesis()], ['a'], () => true), null);
});

test('Go and browser use identical protocol hashes, including Unicode and HTML escaping', async () => {
    const chain = require('../internal/testdata/protocol-v1.json');
    for (const block of chain) assert.equal(await sim.hash(block), block.hash);
    assert.equal(await sim.validChain(chain), true);
});
