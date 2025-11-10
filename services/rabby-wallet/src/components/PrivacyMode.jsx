import { usePlinkoPIR } from '../providers/PlinkoPIRProvider';
import './PrivacyMode.css';

export const PrivacyMode = () => {
  const {
    privacyMode,
    hintDownloaded,
    hintSize,
    deltasApplied,
    isLoading,
    error,
    togglePrivacyMode
  } = usePlinkoPIR();

  return (
    <div className="privacy-mode">
      <div className="privacy-header">
        <h2>🔒 Privacy Mode</h2>
        <label className="toggle-switch">
          <input
            type="checkbox"
            checked={privacyMode}
            onChange={togglePrivacyMode}
            disabled={isLoading}
          />
          <span className="slider"></span>
        </label>
      </div>

      <div className="privacy-status">
        {isLoading && (
          <div className="status-loading">
            <div className="spinner"></div>
            <p>Downloading Plinko PIR hints...</p>
            <p className="status-hint">This is a one-time ~70 MB download</p>
          </div>
        )}

        {error && (
          <div className="status-error">
            <p>❌ Error: {error}</p>
            <p className="status-hint">Falling back to public RPC</p>
          </div>
        )}

        {!isLoading && !error && (
          <div className={`status-info ${privacyMode ? 'enabled' : 'disabled'}`}>
            {privacyMode ? (
              <>
                <h3>✅ Privacy Mode Enabled</h3>
                <p>Your balance queries are private and cannot be tracked by the RPC provider.</p>

                {hintDownloaded && (
                  <div className="status-details">
                    <div className="status-item">
                      <span className="label">Hint Size:</span>
                      <span className="value">{(hintSize / 1024 / 1024).toFixed(1)} MB</span>
                    </div>
                    <div className="status-item">
                      <span className="label">Deltas Applied:</span>
                      <span className="value">{deltasApplied}</span>
                    </div>
                    <div className="status-item">
                      <span className="label">Technology:</span>
                      <span className="value">Plinko PIR</span>
                    </div>
                  </div>
                )}
              </>
            ) : (
              <>
                <h3>⚠️ Privacy Mode Disabled</h3>
                <p>Your balance queries are sent to a public RPC provider who can see which addresses you query.</p>
                <button onClick={togglePrivacyMode} className="enable-button">
                  Enable Privacy Mode
                </button>
              </>
            )}
          </div>
        )}
      </div>

      <div className="privacy-info">
        <h4>How Privacy Mode Works:</h4>
        <ul>
          <li><strong>Initial Hint Download</strong>: One-time ~70 MB download covering 8.4M accounts (2^23 entries)</li>
          <li><strong>Plinko PIR Queries</strong>: Query balances without revealing which address you're interested in</li>
          <li><strong>Incremental Updates</strong>: Each block update covers ~2,000 accounts (23.75 μs processing time)</li>
          <li><strong>Information-Theoretic Privacy</strong>: Server learns absolutely nothing about your queries</li>
        </ul>

        <p className="privacy-performance">
          <strong>Performance:</strong> ~5ms query latency | ~70 MB one-time download | ~30 KB per block update
        </p>
      </div>
    </div>
  );
};
