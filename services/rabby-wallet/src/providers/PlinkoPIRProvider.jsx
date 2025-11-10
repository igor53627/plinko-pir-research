import { createContext, useContext, useState, useEffect, useRef } from 'react';
import { PlinkoPIRClient } from '../clients/plinko-pir-client.js';
import { PlinkoClient } from '../clients/plinko-client.js';

const PlinkoPIRContext = createContext(null);

export const usePlinkoPIR = () => {
  const context = useContext(PlinkoPIRContext);
  if (!context) {
    throw new Error('usePlinkoPIR must be used within PlinkoPIRProvider');
  }
  return context;
};

export const PlinkoPIRProvider = ({ children }) => {
  const [privacyMode, setPrivacyMode] = useState(false);

  const [hintDownloaded, setHintDownloaded] = useState(false);
  const [hintSize, setHintSize] = useState(0);
  const [deltasApplied, setDeltasApplied] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);
  const [rabbyDetected, setRabbyDetected] = useState(false);

  // Create persistent client instances using useRef
  const pirClientRef = useRef(null);
  const plinkoClientRef = useRef(null);

  // Initialize clients once
  if (!pirClientRef.current) {
    pirClientRef.current = new PlinkoPIRClient(
      import.meta.env.VITE_PIR_SERVER_URL || 'http://localhost:3000',
      import.meta.env.VITE_CDN_URL || 'http://localhost:8080'
    );
  }

  if (!plinkoClientRef.current) {
    plinkoClientRef.current = new PlinkoClient(
      import.meta.env.VITE_CDN_URL || 'http://localhost:8080'
    );
  }

  const pirClient = pirClientRef.current;
  const plinkoClient = plinkoClientRef.current;

  // Detect Rabby wallet on mount
  useEffect(() => {
    if (typeof window !== 'undefined' && window.ethereum?.isRabby) {
      console.log('🦊 Rabby wallet detected');
      setRabbyDetected(true);
    }
  }, []);

  // Toggle privacy mode
  const togglePrivacyMode = async () => {
    const newMode = !privacyMode;

    if (newMode && !hintDownloaded) {
      // First time enabling privacy mode - download hint
      setIsLoading(true);
      setError(null);

      try {
        console.log('📥 Downloading Plinko PIR hints...');
        const startTime = performance.now();

        await pirClient.downloadHint();

        const elapsed = performance.now() - startTime;
        const size = pirClient.getHintSize();

        console.log(`✅ Hint downloaded: ${(size / 1024 / 1024).toFixed(1)} MB in ${(elapsed / 1000).toFixed(2)}s`);

        setHintDownloaded(true);
        setHintSize(size);
        setPrivacyMode(true);
        localStorage.setItem('privacyMode', 'true');
      } catch (err) {
        console.error('❌ Failed to download hint:', err);
        setError(err.message);
        setPrivacyMode(false);
      } finally {
        setIsLoading(false);
      }
    } else {
      // Just toggle
      setPrivacyMode(newMode);
      localStorage.setItem('privacyMode', String(newMode));
    }
  };

  // Sync deltas periodically
  useEffect(() => {
    if (!privacyMode || !hintDownloaded) return;

    const syncDeltas = async () => {
      try {
        const latestBlock = await plinkoClient.getLatestDeltaBlock();
        const currentBlock = plinkoClient.getCurrentBlock();

        if (latestBlock > currentBlock) {
          console.log(`🔄 Syncing deltas from block ${currentBlock + 1} to ${latestBlock}...`);

          const startTime = performance.now();
          const count = await plinkoClient.syncDeltas(currentBlock + 1, latestBlock, pirClient);
          const elapsed = performance.now() - startTime;

          console.log(`✅ Applied ${count} deltas in ${elapsed.toFixed(1)}ms`);
          setDeltasApplied(prev => prev + count);
        }
      } catch (err) {
        console.error('⚠️ Delta sync failed:', err);
        // Non-fatal - continue with stale hints
      }
    };

    // Sync immediately, then every 30 seconds
    syncDeltas();
    const interval = setInterval(syncDeltas, 30000);

    return () => clearInterval(interval);
  }, [privacyMode, hintDownloaded]);

  // Query balance with privacy
  const getBalance = async (address) => {
    if (!privacyMode || !hintDownloaded) {
      // Fallback to public RPC
      console.log('📡 Using PUBLIC RPC (Privacy Mode disabled)');
      const balance = await fetchBalancePublic(address);
      return { balance, visualization: null };
    }

    try {
      console.log('🔒 Querying balance with Plinko PIR (PRIVATE mode)');
      const startTime = performance.now();

      // Use real private PIR query (FullSet)
      const result = await pirClient.queryBalancePrivate(address);

      const elapsed = performance.now() - startTime;
      console.log(`✅ Private query completed in ${elapsed.toFixed(1)}ms`);

      return result; // { balance, visualization }
    } catch (err) {
      console.error('⚠️ Private query failed, falling back to public RPC:', err);
      const balance = await fetchBalancePublic(address);
      return { balance, visualization: null };
    }
  };

  // Fallback to public RPC
  const fetchBalancePublic = async (address) => {
    const fallbackRPC = import.meta.env.VITE_FALLBACK_RPC || 'http://localhost:8545';

    console.log(`🌐 Public RPC URL: ${fallbackRPC}`);
    console.log(`📍 Querying address: ${address}`);

    const response = await fetch(fallbackRPC, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jsonrpc: '2.0',
        method: 'eth_getBalance',
        params: [address, 'latest'],
        id: 1
      })
    });

    const data = await response.json();
    const balance = BigInt(data.result);
    console.log(`💰 Public RPC returned: ${balance.toString()} wei (${Number(balance) / 1e18} ETH)`);
    return balance;
  };

  const value = {
    privacyMode,
    hintDownloaded,
    hintSize,
    deltasApplied,
    isLoading,
    error,
    rabbyDetected,
    togglePrivacyMode,
    getBalance
  };

  return (
    <PlinkoPIRContext.Provider value={value}>
      {children}
    </PlinkoPIRContext.Provider>
  );
};
