#!/bin/bash
#
# Example: All-in-one fixture generation and pricing workflow
#
# Usage: ./example_workflow.sh 20251121

set -e

DATE=$1

if [ -z "$DATE" ]; then
    echo "Usage: $0 YYYYMMDD"
    echo "Example: $0 20251121"
    exit 1
fi

cd /Users/meenmo/Documents/workspace/molib

echo "=== Generating Fixtures for $DATE ==="
echo ""

# Generate BGN EUR fixtures
.venv/bin/python3 scripts/generate_fixtures.py --date $DATE --source BGN --currency EUR
printf "\n"

# Generate LCH EUR fixtures (may need different date if LCH data not available)
.venv/bin/python3 scripts/generate_fixtures.py --date $DATE --source LCH --currency EUR || echo "⚠️  LCH data not available for $DATE"
printf "\n"

# Generate BGN JPY fixtures (OIS from BGN, IBOR from BGNS)
.venv/bin/python3 scripts/generate_fixtures.py \
  --date $DATE \
  --source BGNS \
  --currency JPY \
  --ois-source BGN \
  --ibor-source BGNS
printf "\n"


echo "=== Running Pricing Tests ==="
echo ""

# Run all test cases with the newly generated fixtures
go run ./cmd/basiscalc -date $DATE

echo ""
echo "=== Complete! ==="
echo ""
echo "The fixtures have been updated to use data from $DATE"
echo "All pricing tests completed successfully"
