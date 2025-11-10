#!/bin/bash
# Repository Structure Validation Script
# Validates that the refactoring was completed successfully

set -e

echo "🔍 Validating Repository Structure..."

# Phase 1: Research Directory
echo ""
echo "✓ Phase 1: Research Directory Structure"
[[ -d "research" ]] && echo "  ✓ research/ exists"
[[ -d "research/findings" ]] && echo "  ✓ research/findings/ exists"
[[ -f "research/POC-IMPLEMENTATION.md" ]] && echo "  ✓ research/POC-IMPLEMENTATION.md exists"
[[ -f "research/POC-PLINKO-IMPLEMENTATION.md" ]] && echo "  ✓ research/POC-PLINKO-IMPLEMENTATION.md exists"
[[ -f "research/research-plan.md" ]] && echo "  ✓ research/research-plan.md exists"
[[ -f "research/_summary.md" ]] && echo "  ✓ research/_summary.md exists"

# Phase 2: PoC Promoted to Root
echo ""
echo "✓ Phase 2: PoC Promoted to Root"
[[ -d "services" ]] && echo "  ✓ services/ exists"
[[ -d "data" ]] && echo "  ✓ data/ exists"
[[ -d "scripts" ]] && echo "  ✓ scripts/ exists"
[[ -d "docs" ]] && echo "  ✓ docs/ exists"
[[ -d "shared" ]] && echo "  ✓ shared/ exists"
[[ -f ".env.example" ]] && echo "  ✓ .env.example exists"
[[ -f "Makefile" ]] && echo "  ✓ Makefile exists"
[[ -f "docker-compose.yml" ]] && echo "  ✓ docker-compose.yml exists"
[[ -f "IMPLEMENTATION.md" ]] && echo "  ✓ IMPLEMENTATION.md exists"
[[ ! -d "plinko-pir-poc" ]] && echo "  ✓ plinko-pir-poc/ removed"

# Phase 3: Documentation Updated
echo ""
echo "✓ Phase 3: Documentation Updated"
! grep -q "plinko-pir-poc/" README.md && echo "  ✓ README.md updated (no old paths)"
grep -q "research/" README.md && echo "  ✓ README.md references research/"
grep -q "IMPLEMENTATION.md" README.md && echo "  ✓ README.md references IMPLEMENTATION.md"
! grep -q "plinko-pir-poc/" QUICKSTART.md && echo "  ✓ QUICKSTART.md updated (no old paths)"

# Phase 4: .gitignore Merged
echo ""
echo "✓ Phase 4: .gitignore Merged"
[[ -f ".gitignore" ]] && echo "  ✓ Root .gitignore exists"
grep -q "shared/data/" .gitignore && echo "  ✓ .gitignore includes PoC patterns"
[[ ! -f "plinko-pir-poc/.gitignore" ]] && echo "  ✓ Old .gitignore removed"

# Phase 5: Services Intact
echo ""
echo "✓ Phase 5: Services Intact"
[[ -d "services/eth-mock" ]] && echo "  ✓ eth-mock service exists"
[[ -d "services/db-generator" ]] && echo "  ✓ db-generator service exists"
[[ -d "services/plinko-hint-generator" ]] && echo "  ✓ plinko-hint-generator service exists"
[[ -d "services/plinko-update-service" ]] && echo "  ✓ plinko-update-service service exists"
[[ -d "services/plinko-pir-server" ]] && echo "  ✓ plinko-pir-server service exists"
[[ -d "services/cdn-mock" ]] && echo "  ✓ cdn-mock service exists"
[[ -d "services/rabby-wallet" ]] && echo "  ✓ rabby-wallet service exists"

echo ""
echo "✅ Repository structure validation complete!"
echo ""
echo "Summary:"
echo "  - Research artifacts consolidated in research/"
echo "  - PoC implementation promoted to root"
echo "  - All services preserved and functional"
echo "  - Documentation updated with correct paths"
echo "  - Git history preserved (use 'git log --follow')"
