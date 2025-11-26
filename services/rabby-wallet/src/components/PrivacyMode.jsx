import { usePlinkoPIR } from '../providers/PlinkoPIRProvider';
import { DATASET_STATS, DATASET_DISPLAY } from '../constants/dataset.js';
import './PrivacyMode.css';

export const PrivacyMode = () => {
  const {
    privacyMode,
    hintDownloaded,
    hintSize,
    deltasApplied,
    downloadProgress,
    isLoading,
    error,
    togglePrivacyMode
  } = usePlinkoPIR();

  const totalSnapshotMB = DATASET_DISPLAY.totalSnapshotMB;
  const databaseMB = DATASET_DISPLAY.databaseMB;
  const mappingMB = DATASET_DISPLAY.addressMappingMB;
  const hintMB = DATASET_DISPLAY.hintMB;
  const formattedAddressCount = DATASET_STATS.addressCount.toLocaleString();

  return (
    <div className="privacy-content">


      <div className="privacy-status">
        {isLoading && (
          <div className="status-loading">
            <div className="spinner"></div>
            <p>Downloading Plinko PIR snapshot + address map...</p>

            {downloadProgress && (
              <div className="progress-container">
                <div className="progress-bar">
                  <div
                    className="progress-fill"
                    style={{ width: `${downloadProgress.percent}%` }}
                  ></div>
                </div>
                <p className="progress-text">
                  {downloadProgress.stage === 'database' && `Downloading Database (${downloadProgress.percent.toFixed(0)}%)`}
                  {downloadProgress.stage === 'address_mapping' && `Downloading Address Map (${downloadProgress.percent.toFixed(0)}%)`}
                  {downloadProgress.stage === 'hint_generation' && `Generating Local Hints (${downloadProgress.percent.toFixed(0)}%)${downloadProgress.percent > 0 && downloadProgress.percent < 100 ? ` - ETA: ${Math.round((100 - downloadProgress.percent) / downloadProgress.percent * (Date.now() - (window._hintGenStart || Date.now())) / 1000)}s` : ''}`}
                </p>
              </div>
            )}

            <p className="status-hint">
              This is a one-time ~{totalSnapshotMB} MB download (snapshot database {databaseMB} MB + address-mapping.bin {mappingMB} MB)
            </p>
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
            <div className="status-header-row">
              <h3>
                {privacyMode ? '✅ Privacy Mode Enabled' : '⚠️ Privacy Mode Disabled'}
              </h3>
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

            {privacyMode ? (
              <>
                <p>Your balance queries are private and cannot be tracked by the RPC provider.</p>

                {hintDownloaded && (
                  <div className="status-details">
                    <div className="status-item">
                      <span className="label">Hint Size:</span>
                      <span className="value">{(hintSize / 1024 / 1024).toFixed(1)} MB</span>
                    </div>
                    <div className="status-item">
                      <span className="label">Deltas Applied (Account Updates):</span>
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
          <li>
            <strong>Initial Data Download</strong>: One-time ~{totalSnapshotMB} MB download
            (database {databaseMB} MB + address-mapping.bin {mappingMB} MB, hint derived locally ~{hintMB} MB)
            covering {formattedAddressCount} real Ethereum addresses
          </li>
          <li><strong>Plinko PIR Queries</strong>: Query balances without revealing which address you're interested in. The server computes a parity over a pseudorandom set of accounts.</li>
          <li><strong>Decoding Process</strong>: Your client locally reconstructs the answer by XORing the server's response with your local hint parity.</li>
          <li><strong>Incremental Updates</strong>: Each block update covers ~2,000 accounts (23.75 μs processing time)</li>
          <li><strong>Information-Theoretic Privacy</strong>: Server learns absolutely nothing about your queries</li>
        </ul>

        <p className="privacy-performance">
          <strong>Performance:</strong> ~5ms query latency | ~{totalSnapshotMB} MB one-time download | ~30 KB per block update
        </p>
      </div>
    </div>
  );
};
