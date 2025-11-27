# molib Quick Reference

One-page reference for common tasks.

## 🚀 Setup (First Time Only)

```bash
cd /Users/meenmo/Documents/workspace/molib

# Install Python dependencies
uv pip install psycopg2-binary

# Or use setup script
./scripts/setup.sh
```

## 📊 Generate Fixtures and Run Tests

```bash
# All-in-one: Generate fixtures + run tests (recommended)
./scripts/update_fixtures_and_test.sh 20251201

# Or generate individually
.venv/bin/python3 scripts/generate_fixtures.py --date 20251201 --source BGN --currency EUR
.venv/bin/python3 scripts/generate_fixtures.py --date 20251201 --source BGN --currency JPY
.venv/bin/python3 scripts/generate_fixtures.py --date 20251201 --source LCH --currency EUR
```

**Note**: Fixtures are generated with standard names (`fixtures_bgn_euribor.go`, `fixtures_bgn_tibor.go`, `fixtures_lch_euribor.go`) and overwrite existing files. The date is only recorded in the comment.

## 🧪 Run Tests

```bash
# Run all test cases
go run ./cmd/basiscalc

# Test specific structure
go run ./cmd/basiscalc-flex \
  --forward 10 \
  --tenor 10

# Test different provider/currency
go run ./cmd/basiscalc-flex \
  --provider LCH \
  --currency EUR

go run ./cmd/basiscalc-flex \
  --provider BGN \
  --currency JPY \
  --forward 1 \
  --tenor 4
```

## 🔍 Check Available Dates

```bash
# See what dates are in database
PGPASSWORD='04201' psql -h 100.127.72.74 -p 1013 -U meenmo -d ficc -c "
SELECT DISTINCT curve_date
FROM marketdata.curves
WHERE source='BGN' AND curve_date >= '2025-11-01'
ORDER BY curve_date DESC
LIMIT 10;
"
```

## 📝 Common Workflows

### Test with New Date

```bash
# All-in-one command: Generate fixtures and run tests
./scripts/update_fixtures_and_test.sh 20251201

# Or do it step by step:
# 1. Generate fixtures (overwrites existing)
.venv/bin/python3 scripts/generate_fixtures.py --date 20251201 --source BGN --currency EUR

# 2. Run tests
go run ./cmd/basiscalc
```

### Test Multiple Dates

```bash
# Test with different dates
for date in 20251125 20251126 20251127; do
    echo "Testing with date: $date"
    ./scripts/update_fixtures_and_test.sh $date
    echo ""
done
```

## 📚 Documentation

| Topic | File |
|-------|------|
| Quick Start | `docs/QUICK_START.md` |
| Fixture Generation | `docs/FIXTURE_GENERATION.md` ⭐ |
| Testing Guide | `docs/TESTING_GUIDE.md` |
| Data Flow | `docs/DATA_FLOW.md` |
| Project Overview | `README.md` |

## ✅ Current Test Results

| Test Case | Result | Error | Status |
|-----------|--------|-------|--------|
| BGN EUR 10x10 | -4.072 bp | 0.05 bp | ✅ Excellent |
| BGN EUR 10x20 | -4.973 bp | 0.03 bp | ✅ Excellent |
| LCH EUR 10x10 | -3.598 bp | 0.27 bp | ✅ Great |
| LCH EUR 10x20 | -4.076 bp | 0.35 bp | ✅ Great |
| TIBOR 1x4 | -2.190 bp | 0.10 bp | 🟡 Good |
| TIBOR 2x3 | -2.393 bp | 0.12 bp | 🟡 Good |

All EUR cases < 0.4 bp! ✅

## 🔧 Troubleshooting

| Issue | Solution |
|-------|----------|
| `ModuleNotFoundError: psycopg2` | `uv pip install psycopg2-binary` |
| "No data found" | Check date exists in database |
| Database connection error | Check network access to 100.127.72.74:1013 |
| File won't compile | `go build ./swap/basis/data` to check syntax |

## 📞 Need Help?

1. Read `docs/FIXTURE_GENERATION.md`
2. Check `docs/TESTING_GUIDE.md`
3. Review example in `README.md`
