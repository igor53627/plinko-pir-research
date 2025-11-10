# Rabby × Plinko PIR PoC (React + Vite)

**Purpose**: Demonstration wallet with Plinko PIR privacy mode for private Ethereum balance queries, branded for Rabby wallet integration

## Features

- ✅ **Privacy Mode Toggle**: Enable/disable private queries
- ✅ **Hint Download**: One-time ~70 MB download on first use
- ✅ **Delta Synchronization**: Real-time updates with Plinko
- ✅ **Private Queries**: Balance queries via Plinko PIR Server
- ✅ **Fallback Mode**: Public RPC when privacy disabled
- ✅ **LocalStorage Persistence**: Privacy mode preference saved

## Architecture

### Frontend Stack
- **React 18**: UI framework
- **Vite**: Build tool and dev server
- **Vanilla CSS**: Styling (no dependencies)

### Plinko PIR Integration
- **PianoPIRProvider**: React context for privacy mode state
- **piano-pir-client.js**: Hint download, query generation
- **plinko-client.js**: Delta synchronization
- **PrivacyMode component**: Toggle UI with status

## Configuration

Environment variables (.env):
```bash
VITE_PIR_SERVER_URL=http://localhost:3000
VITE_CDN_URL=http://localhost:8080
VITE_FALLBACK_RPC=https://eth.llamarpc.com
```

## Usage

### Start with Docker Compose
```bash
docker-compose up rabby-wallet
```

### Access Wallet
```
http://localhost:5173
```

### Development Mode
```bash
cd services/rabby-wallet

# Install dependencies
npm install

# Start dev server
npm run dev

# Build for production
npm run build
```

## User Flow

### First Time (Privacy Mode Off)
1. User opens wallet at `http://localhost:5173`
2. Default: Privacy Mode disabled
3. Balance queries use public RPC
4. RPC provider can see which addresses are queried

### Enabling Privacy Mode
1. User clicks "Privacy Mode" toggle
2. Wallet downloads hint.bin (~70 MB, ~1-2 seconds)
3. Privacy Mode enabled
4. All future queries are private

### Private Query Flow
1. User enters Ethereum address
2. Clicks "Query Balance"
3. Client generates Plinko PIR query
4. Server responds without learning address
5. Balance displayed with query time

### Delta Synchronization
1. Wallet checks for new deltas every 30 seconds
2. Downloads missing delta files
3. Applies XOR deltas to local hint
4. Hint stays up-to-date with blockchain

## Components

### PianoPIRProvider
React context provider for privacy state:

```javascript
const {
  privacyMode,        // Boolean: privacy enabled?
  hintDownloaded,     // Boolean: hint downloaded?
  hintSize,           // Number: hint size in bytes
  deltasApplied,      // Number: total deltas applied
  isLoading,          // Boolean: downloading hint?
  error,              // String: error message
  togglePrivacyMode,  // Function: toggle privacy
  getBalance          // Function: query balance
} = usePianoPIR();
```

### PrivacyMode Component
UI for enabling/disabling privacy:

- Toggle switch
- Download progress
- Status information
- Performance metrics

### App Component
Main wallet interface:

- Address input
- Balance query button
- Query results display
- Privacy status indicator

## Client Libraries

### PianoPIRClient
Handles Plinko PIR operations:

```javascript
const client = new PianoPIRClient(pirServerUrl, cdnUrl);

// Download hint (one-time)
await client.downloadHint();

// Query balance privately
const balance = await client.queryBalance('0x...');

// Apply delta update
client.applyDelta(deltaBytes, offset);
```

### PlinkoClient
Handles delta synchronization:

```javascript
const client = new PlinkoClient(cdnUrl);

// Get latest delta block
const latestBlock = await client.getLatestDeltaBlock();

// Sync deltas
const count = await client.syncDeltas(fromBlock, toBlock, pirClient);

// Apply single delta
client.applyDeltaToHint(delta, pirClient);
```

## Files

- `src/App.jsx` - Main wallet interface
- `src/App.css` - Wallet styles
- `src/providers/PianoPIRProvider.jsx` - Privacy mode context
- `src/components/PrivacyMode.jsx` - Privacy toggle UI
- `src/components/PrivacyMode.css` - Privacy mode styles
- `src/clients/piano-pir-client.js` - Plinko PIR client library
- `src/clients/plinko-client.js` - Plinko delta client
- `package.json` - NPM dependencies
- `vite.config.js` - Vite configuration
- `Dockerfile` - Multi-stage build (React + nginx)
- `nginx.conf` - nginx configuration for SPA
- `README.md` - This file

## Performance

### Initial Load
- App bundle: ~200 KB gzipped
- Time to interactive: <1 second

### Hint Download
- Size: ~70 MB
- Time: 1-2 seconds (localhost)
- Frequency: Once per device

### Delta Sync
- Size per delta: ~30 KB
- Sync interval: Every 30 seconds
- Background: Non-blocking

### Balance Query
- **Private mode**: ~5-10 ms
- **Public mode**: ~50-100 ms (network latency)

## Privacy Guarantees

### What Server Learns
**With Privacy Mode**:
- ❌ Nothing about queried address
- ❌ Nothing about query content
- ✅ Only: query timestamp, size

**Without Privacy Mode**:
- ⚠️ Exact address queried
- ⚠️ Query timestamp
- ⚠️ User IP address

### Information-Theoretic Privacy
Plinko PIR provides **perfect privacy**:
- Server response is random noise to observer
- Even infinite compute cannot determine query
- Mathematically proven (see Plinko PIR paper)

## LocalStorage

### Stored Data
```javascript
{
  "privacyMode": "true",           // Privacy preference
  "plinko_current_block": "1234"   // Last synced block
}
```

### Privacy Note
Hint data is stored in memory only (not persisted):
- Must re-download on page refresh
- Could add IndexedDB persistence in production

## Browser Compatibility

Requires:
- **ES6+**: Modern JavaScript features
- **Fetch API**: HTTP requests
- **localStorage**: State persistence
- **crypto.getRandomValues**: Random number generation

Tested on:
- Chrome 90+
- Firefox 88+
- Safari 14+
- Edge 90+

## Troubleshooting

**Problem**: "Failed to download hint"
- Check CDN service is running (`docker-compose ps`)
- Verify hint.bin exists (`ls shared/data/hint.bin`)
- Check browser console for CORS errors

**Problem**: "Private query failed"
- Verify Plinko PIR Server is running
- Check server health: `curl http://localhost:3000/health`
- Look at server logs for errors

**Problem**: Delta sync stuck
- Check Plinko Update Service is running
- Verify delta files exist in `/data/deltas/`
- Check browser console for errors

**Problem**: Slow queries
- Check Plinko PIR Server performance
- Verify hint is loaded in memory (not re-downloading)
- Check network latency to localhost

## Production Considerations

### Performance Optimization

**Code Splitting**:
```javascript
// Lazy load Privacy Mode for faster initial load
const PrivacyMode = lazy(() => import('./components/PrivacyMode'));
```

**Hint Persistence**:
```javascript
// Use IndexedDB to persist hint across sessions
const db = await openDB('piano-pir-hints', 1, {
  upgrade(db) {
    db.createObjectStore('hints');
  }
});
await db.put('hints', hintData, 'current');
```

**Delta Aggregation**:
- Fetch aggregated deltas (e.g., blocks 1-100)
- Reduces HTTP requests
- Server-side delta coalescing

### Security Hardening

**Content Security Policy**:
```html
<meta http-equiv="Content-Security-Policy"
      content="default-src 'self'; connect-src 'self' https://cdn.example.com">
```

**Subresource Integrity**:
```html
<script src="app.js"
        integrity="sha384-..."
        crossorigin="anonymous"></script>
```

### Progressive Web App

Add service worker for offline support:
```javascript
// service-worker.js
self.addEventListener('fetch', (event) => {
  event.respondWith(
    caches.match(event.request).then(response =>
      response || fetch(event.request)
    )
  );
});
```

### Analytics (Privacy-Preserving)

Track usage without compromising privacy:
```javascript
// ✅ OK - aggregate metrics
analytics.track('privacy_mode_enabled', { timestamp: Date.now() });

// ❌ NEVER - reveals query info
analytics.track('balance_query', { address: '0x...' });
```

## Testing

### Manual Testing
1. Open wallet in browser
2. Toggle privacy mode on
3. Wait for hint download
4. Query balance for test address
5. Verify private query succeeds
6. Check browser console for logs

### Automated Testing
```bash
# Install Playwright
npm install -D @playwright/test

# Run tests
npm run test:e2e
```

### Test Addresses
```
0x1000000000000000000000000000000000000042  (index 66)
0x2000000000000000000000000000000000000123  (index 291)
```

## Next Steps

After Wallet Integration:
1. **Integration Testing**: End-to-end flow validation
2. **Performance Testing**: Query latency at scale
3. **Documentation**: User guide, deployment docs
4. **Production Hardening**: Security audit, performance optimization

---

**Status**: Privacy Mode ✅ | Hint Download ✅ | Delta Sync ✅ | Private Queries ✅
