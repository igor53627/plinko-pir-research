# Repository Guidelines
Keep contributions aligned as you extend the Plinko PIR stack.

## Project Structure & Module Organization
- `services/` hosts the Go microservices (`plinko-pir-server`, `plinko-update-service`, mocks) and the Vite Rabby wallet in `services/rabby-wallet/`. Canonical DBs are built from parquet diffs via `scripts/build_database_from_parquet.py`.
- Docker Compose is the only supported deployment path; legacy Kubernetes/VM assets have been removed.
- Canonical state lives in `data/` (e.g., `database.bin`), public artifacts live in `public-data/` (snapshot packages, delta feeds), and reproducible datasets live in `test-data/`; avoid manual edits outside the documented generators.
- `scripts/` centralizes automation used by the Makefile—extend these scripts instead of paste-in shell.

## Build, Test, and Development Commands
- `make init` primes `.env`; `make build`, `make start`, or `make up` compose the full stack.
- Use `make logs|status|health` for triage; `make stop|clean|reset` unwind environments cleanly.
- `make test` chains `scripts/test-privacy.sh` and `scripts/test-performance.sh`; call either script directly for quick loops, and run `make test-addressing` when touching networking.
- Service-specific work runs via `cd services/<service> && go test ./...`; the wallet uses `npm install && npm run dev -- --host` or `npm run test` (Vitest).

## Coding Style & Naming Conventions
- Go 1.21 code must be `gofmt`ed (tabs, K&R braces) with exported symbols documented; keep filenames aligned with their primary type (`server.go`, `prset.go`).
- React files in `services/rabby-wallet/src` stay in PascalCase, rely on functional components/hooks, and keep the existing 2-space + semicolon formatting; keep UI copy privacy-focused.
- Shell helpers already start with `#!/bin/bash` and `set -e`; retain lowercase, hyphenated filenames and log via `echo` for deterministic CI output.

## Testing Guidelines
- Mirror existing `_test.go` coverage (`services/plinko-pir-server/prset_test.go`, etc.) and pin fixtures under `test-data/` to keep diffs reviewable.
- Privacy-surface changes must extend `scripts/test-privacy.sh` so log scans fail loudly; capture new expectations inline.
- Update `scripts/test-performance.sh` when latency thresholds move, and regenerate binaries via their Go generators so `shared/data/` hashes stay reproducible.

## Commit & Pull Request Guidelines
- Follow the Conventional Commits shorthand in `git log` (`feat:`, `docs:`, `chore:`) and add scopes when helpful, e.g., `feat(server): cache decoded hints`.
- PRs should summarize user-visible impact, link to relevant notes in `docs/`, attach `make test` output, include wallet screenshots/GIFs, and call out infra touch-points when touching shared services.

## Security & Configuration Tips
- Keep secrets and RPC endpoints in local `.env` overrides; only commit sanitized defaults referenced in README/IMPLEMENTATION docs.
- Large artifacts (`data/database.bin`, `public-data/snapshots/*`, generated deltas) must be rebuilt via their generators or Make targets rather than edited by hand so downstream agents can verify hashes.
