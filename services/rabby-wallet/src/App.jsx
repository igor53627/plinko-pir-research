import { useState } from 'react';
import { PlinkoPIRProvider, usePlinkoPIR } from './providers/PlinkoPIRProvider';
import { PrivacyMode } from './components/PrivacyMode';
import './App.css';

function WalletDemo() {
  const [address, setAddress] = useState('0x1000000000000000000000000000000000000042');
  const [balance, setBalance] = useState(null);
  const [visualization, setVisualization] = useState(null);
  const [isQuerying, setIsQuerying] = useState(false);
  const [queryTime, setQueryTime] = useState(null);

  const { getBalance, privacyMode } = usePlinkoPIR();

  const handleQueryBalance = async () => {
    if (!address) return;

    setIsQuerying(true);
    setBalance(null);
    setVisualization(null);
    setQueryTime(null);

    try {
      const startTime = performance.now();
      const result = await getBalance(address);
      const elapsed = performance.now() - startTime;

      setBalance(result.balance);
      setVisualization(result.visualization);
      setQueryTime(elapsed);
    } catch (err) {
      console.error('Query failed:', err);
      alert('Failed to query balance: ' + err.message);
    } finally {
      setIsQuerying(false);
    }
  };

  const formatBalance = (wei) => {
    if (!wei) return '0';
    const eth = Number(wei) / 1e18;
    return eth.toLocaleString(undefined, { maximumFractionDigits: 4 });
  };

  return (
    <div className="wallet-demo">
      <header className="wallet-header">
        <h1>🔐 Rabby × Plinko PIR PoC</h1>
        <p>Demonstration of Private Information Retrieval for Ethereum</p>
      </header>

      <PrivacyMode />

      <div className="balance-query">
        <h2>Query Balance</h2>

        <div className="query-form">
          <input
            type="text"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="Enter Ethereum address (0x...)"
            className="address-input"
          />

          <button
            onClick={handleQueryBalance}
            disabled={isQuerying || !address}
            className="query-button"
          >
            {isQuerying ? 'Querying...' : 'Query Balance'}
          </button>
        </div>

        {balance !== null && (
          <div className="balance-result">
            <h3>Balance Result</h3>
            <div className="balance-value">
              <span className="amount">{formatBalance(balance)}</span>
              <span className="unit">ETH</span>
            </div>

            <div className="query-details">
              <div className="detail-item">
                <span className="label">Query Time:</span>
                <span className="value">{queryTime?.toFixed(1)} ms</span>
              </div>
              <div className="detail-item">
                <span className="label">Method:</span>
                <span className="value">
                  {privacyMode ? '🔒 Plinko PIR (Private)' : '📡 Public RPC'}
                </span>
              </div>
              <div className="detail-item">
                <span className="label">Privacy:</span>
                <span className={`value ${privacyMode ? 'private' : 'public'}`}>
                  {privacyMode ? 'Server learned nothing ✅' : 'RPC knows your address ⚠️'}
                </span>
              </div>
            </div>

            {visualization && (
              <div className="pir-visualization">
                <h3>🔐 Plinko PIR Decoding Process</h3>

                <div className="viz-flow">
                  <div className="viz-step">
                    <div className="step-number">1</div>
                    <div className="step-content">
                      <h4>PRF Key Generation</h4>
                      <div className="code-block">
                        <code>[{visualization.prfKey.join(', ')}]</code>
                      </div>
                      <p className="step-desc">16 random bytes sent to server</p>
                    </div>
                  </div>

                  <div className="viz-arrow">↓</div>

                  <div className="viz-step">
                    <div className="step-number">2</div>
                    <div className="step-content">
                      <h4>PRF Set Expansion</h4>
                      <p className="step-desc" style={{marginTop: 0, marginBottom: '0.75rem'}}>
                        <strong>How it works:</strong> The database has {(visualization.chunkSize * visualization.setSize).toLocaleString()} entries split into {visualization.setSize} chunks of {visualization.chunkSize.toLocaleString()} entries each.
                        The PRF key acts like a magic dice that picks exactly <strong>one random entry from each chunk</strong>.
                        This gives us {visualization.setSize} specific indices. The server will XOR these {visualization.setSize} entries together and send back one combined value.
                      </p>
                      <div className="info-grid-viz">
                        <div>
                          <strong>Formula:</strong> index[i] = i × {visualization.chunkSize.toLocaleString()} + PRF(key, i) mod {visualization.chunkSize.toLocaleString()}
                        </div>
                        <div>
                          <strong>Total Indices Selected:</strong> {visualization.prfSetSize} out of {(visualization.chunkSize * visualization.setSize).toLocaleString()} entries ({((visualization.prfSetSize / (visualization.chunkSize * visualization.setSize)) * 100).toFixed(3)}% of database)
                        </div>
                        <div>
                          <strong>Your Target Index:</strong> {visualization.targetIndex.toLocaleString()} (in Chunk {visualization.targetChunk})
                        </div>
                        <div>
                          <strong>Example Indices Selected:</strong> [{visualization.prfSetSample.join(', ')}, ...]
                        </div>
                      </div>
                      <p className="step-desc" style={{marginTop: '0.75rem', fontStyle: 'italic'}}>
                        💡 <strong>Why this is private:</strong> The server sees these {visualization.prfSetSize} random indices, but has no way to know which one you actually care about!
                      </p>
                    </div>
                  </div>

                  <div className="viz-arrow">↓</div>

                  <div className="viz-step">
                    <div className="step-number">3</div>
                    <div className="step-content">
                      <h4>Server Response</h4>
                      <div className="xor-values">
                        <div className="xor-item">
                          <strong>Server Parity (XOR result):</strong>
                          <code>{visualization.serverParity} wei</code>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.9em', color: '#666'}}>
                            ≈ {(BigInt(visualization.serverParity) / BigInt(10**18)).toString()} ETH (truncated)
                          </span>
                        </div>
                      </div>
                      <p className="step-desc">XOR of {visualization.prfSetSize} database entries (includes your balance + {visualization.prfSetSize - 1} others)</p>
                    </div>
                  </div>

                  <div className="viz-arrow">↓</div>

                  <div className="viz-step">
                    <div className="step-number">4</div>
                    <div className="step-content">
                      <h4>Client-Side Verification</h4>
                      <p className="step-desc" style={{marginTop: 0, marginBottom: '0.75rem'}}>
                        <strong>How verification works:</strong> You have a local copy of the database (the "hint").
                        Using the same PRF key, you XOR the same {visualization.prfSetSize} indices from your local hint.
                        Then you compare your result with what the server sent. If they match (delta = 0), your hint is perfectly synchronized with the server's database!
                        If they don't match, the delta tells you what changed.
                      </p>
                      <div className="xor-values">
                        <div className="xor-item">
                          <strong>Hint Parity (your local XOR):</strong>
                          <code>{visualization.hintParity} wei</code>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.9em', color: '#666'}}>
                            ≈ {(BigInt(visualization.hintParity) / BigInt(10**18)).toString()} ETH (truncated)
                          </span>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.85em', color: '#555'}}>
                            This is the XOR of the same {visualization.prfSetSize} entries from your local hint file
                          </span>
                        </div>
                        <div className="xor-item">
                          <strong>Delta (server ⊕ hint):</strong>
                          <code>{visualization.delta} wei</code>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.9em', color: '#666'}}>
                            {visualization.delta === '0' ? '✅ Hint is up to date!' : '⚠️ Hint has updates'}
                          </span>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.85em', color: '#555'}}>
                            Delta = {visualization.serverParity} ⊕ {visualization.hintParity} = {visualization.delta}
                          </span>
                        </div>
                        <div className="xor-item">
                          <strong>Your balance at index {visualization.targetIndex}:</strong>
                          <code>{visualization.hintValue} wei</code>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.9em', color: '#666'}}>
                            = {formatBalance(BigInt(visualization.hintValue))} ETH
                          </span>
                          <span style={{display: 'block', marginTop: '0.25rem', fontSize: '0.85em', color: '#555'}}>
                            Read directly from hint[{visualization.targetIndex}] - no server interaction needed!
                          </span>
                        </div>
                      </div>
                      <p className="step-desc" style={{marginTop: '0.75rem', fontStyle: 'italic'}}>
                        💡 <strong>The magic:</strong> You can verify the server's response is correct without revealing which entry you care about.
                        The server sent you XOR of {visualization.prfSetSize} random entries, but you only need the value at index {visualization.targetIndex}!
                      </p>
                    </div>
                  </div>

                  <div className="viz-arrow">↓</div>

                  <div className="viz-step viz-final">
                    <div className="step-number">✓</div>
                    <div className="step-content">
                      <h4>Balance Extracted</h4>
                      <div className="final-balance">
                        <code>balance = hint[{visualization.targetIndex}] {visualization.delta !== '0' ? `⊕ delta (${visualization.delta})` : '(hint up to date)'}</code>
                      </div>
                      <div className="balance-value-viz">
                        <strong>{formatBalance(balance)} ETH</strong>
                      </div>
                      <div className="step-desc" style={{marginTop: '1rem'}}>
                        <strong style={{color: '#10b981'}}>✅ Privacy Achieved:</strong>
                        <ul style={{marginTop: '0.5rem', marginBottom: 0, paddingLeft: '1.5rem', textAlign: 'left'}}>
                          <li>Server computed XOR of {visualization.prfSetSize} random database entries</li>
                          <li>Server has NO IDEA you're interested in index {visualization.targetIndex}!</li>
                          <li>You got your balance by reading directly from your local hint</li>
                          <li>Delta = {visualization.delta} confirms your hint is synchronized with server</li>
                        </ul>
                      </div>
                    </div>
                  </div>
                </div>

                <div className="pir-stats">
                  <div className="stat-item">
                    <strong>Database Size:</strong> {(visualization.chunkSize * visualization.setSize).toLocaleString()} entries
                  </div>
                  <div className="stat-item">
                    <strong>Chunk Size:</strong> {visualization.chunkSize.toLocaleString()}
                  </div>
                  <div className="stat-item">
                    <strong>Privacy Set:</strong> {visualization.prfSetSize} entries queried
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="info-section">
        <h2>About This PoC</h2>
        <div className="info-grid">
          <div className="info-card">
            <h3>🎯 Goal</h3>
            <p>Demonstrate private Ethereum balance queries using Plinko PIR with Rabby Wallet</p>
          </div>

          <div className="info-card">
            <h3>⚡ Performance</h3>
            <p>
              <strong>Query:</strong> ~5ms latency<br />
              <strong>Updates:</strong> 23.75 μs per 2,000 accounts<br />
              <strong>Hint:</strong> ~70 MB one-time download
            </p>
          </div>

          <div className="info-card">
            <h3>🔬 Scale</h3>
            <p>
              <strong>Database:</strong> 8,388,608 accounts (2^23)<br />
              <strong>Technology:</strong> Plinko PIR<br />
              <strong>Privacy:</strong> Information-theoretic
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

function App() {
  return (
    <PlinkoPIRProvider>
      <WalletDemo />
    </PlinkoPIRProvider>
  );
}

export default App;
