#!/usr/bin/env python3
"""
Find dates with spread differences > 1.0 bp for dates BEFORE 2025.
This version REGENERATES FIXTURES for each date before testing.
"""
import sys
import subprocess
import psycopg2
from datetime import datetime

DB_CONFIG = {
    "host": "100.127.72.74",
    "port": 1013,
    "user": "meenmo",
    "password": "04201",
    "database": "ficc",
}

def get_pre2025_dates():
    """Get all unique valuation dates before 2025."""
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    query = """
        SELECT DISTINCT valuation_date
        FROM pricing.basis_swap
        WHERE valuation_date < '2025-01-01'
        ORDER BY valuation_date DESC
    """

    cur.execute(query)
    dates = [row[0] for row in cur.fetchall()]

    cur.close()
    conn.close()

    return dates

def format_date_for_script(date):
    """Convert date object to YYYYMMDD format."""
    if isinstance(date, str):
        date = datetime.strptime(date, "%Y-%m-%d").date()
    return date.strftime("%Y%m%d")

def regenerate_fixtures(date_str):
    """Regenerate fixtures for a specific date."""
    try:
        # Run the fixture generation script
        result = subprocess.run(
            ["bash", "./scripts/basis_swap.sh", date_str],
            capture_output=True,
            text=True,
            timeout=120,
            cwd="/Users/meenmo/Documents/workspace/molib"
        )
        return result.returncode == 0
    except Exception as e:
        print(f"⚠️  Error regenerating fixtures for {date_str}: {e}")
        return False

def parse_output(output):
    """Parse spread output to extract differences."""
    issues = []
    lines = output.split('\n')

    for line in lines:
        if 'computed=' in line and 'diff=' in line:
            # Extract source, tenor, and diff
            parts = line.split()
            if len(parts) < 3:
                continue

            source = parts[0]  # BGN, BGNS, LCH
            indices = parts[1]  # EURIBOR3M/EURIBOR6M
            tenor = parts[2]  # 10x10

            # Extract diff value
            diff_str = [p for p in parts if p.startswith('diff=')][0]
            diff_val = float(diff_str.replace('diff=', '').replace('bp', ''))

            if abs(diff_val) > 1.0:
                # Extract computed and database values
                computed_str = [p for p in parts if p.startswith('computed=')][0]
                computed = float(computed_str.replace('computed=', '').replace('bp', ''))

                database_str = [p for p in parts if p.startswith('database=')][0]
                database = float(database_str.replace('database=', '').replace('bp', ''))

                issues.append({
                    'source': source,
                    'indices': indices,
                    'tenor': tenor,
                    'computed': computed,
                    'database': database,
                    'diff': diff_val,
                    'line': line.strip()
                })

    return issues

def main():
    print("Fetching dates BEFORE 2025 from pricing.basis_swap...")
    dates = get_pre2025_dates()

    if not dates:
        print("❌ No dates found before 2025")
        sys.exit(1)

    print(f"Found {len(dates)} dates before 2025")

    # Show date range
    if dates:
        print(f"Date range: {dates[-1]} to {dates[0]}\n")

    print("Scanning for differences > 1.0 bp...")
    print("NOTE: This will regenerate fixtures for EACH date (will take a while)")
    print("=" * 100)

    all_issues = []
    dates_tested = 0
    dates_skipped = 0

    for i, date in enumerate(dates, 1):
        date_str = format_date_for_script(date)

        # Show progress for every date
        print(f"\n[{i}/{len(dates)}] Processing {date} ({date_str})...")

        # Regenerate fixtures for this date
        print(f"  → Regenerating fixtures...")
        if not regenerate_fixtures(date_str):
            print(f"  ⚠️  Failed to regenerate fixtures, skipping")
            dates_skipped += 1
            continue

        # Run the calculation
        print(f"  → Running calculation...")
        try:
            result = subprocess.run(
                ["go", "run", "./cmd/basiscalc", "-date", date_str],
                capture_output=True,
                text=True,
                timeout=30,
                cwd="/Users/meenmo/Documents/workspace/molib"
            )

            if result.returncode == 0:
                dates_tested += 1
                issues = parse_output(result.stdout)
                if issues:
                    print(f"  ⚠️  Found {len(issues)} cases with |diff| > 1.0 bp")
                    for issue in issues:
                        all_issues.append({
                            'date': date,
                            'date_str': date_str,
                            **issue
                        })
                else:
                    print(f"  ✅ All differences ≤ 1.0 bp")
            else:
                print(f"  ⚠️  Calculation failed: {result.stderr[:100]}")
                dates_skipped += 1

        except subprocess.TimeoutExpired:
            print(f"  ⚠️  Timeout")
            dates_skipped += 1
        except Exception as e:
            print(f"  ⚠️  Error: {e}")
            dates_skipped += 1

    # Print summary
    print("\n" + "=" * 100)
    print(f"SUMMARY:")
    print(f"  Total dates found: {len(dates)}")
    print(f"  Dates tested: {dates_tested}")
    print(f"  Dates skipped: {dates_skipped}")
    print(f"  Cases with |diff| > 1.0 bp: {len(all_issues)}")
    print("=" * 100)

    if all_issues:
        # Group by date
        by_date = {}
        for issue in all_issues:
            date_str = issue['date_str']
            if date_str not in by_date:
                by_date[date_str] = []
            by_date[date_str].append(issue)

        print(f"\nDates with issues: {len(by_date)}\n")

        for date_str in sorted(by_date.keys(), reverse=True):
            issues = by_date[date_str]
            print(f"\n{'='*100}")
            print(f"Date: {issues[0]['date']} ({date_str})")
            print('-'*100)
            for issue in issues:
                print(f"{issue['source']:<6} {issue['indices']:<25} {issue['tenor']:<8}")
                print(f"  Computed: {issue['computed']:>8.3f} bp | Database: {issue['database']:>8.3f} bp | Diff: {issue['diff']:>8.3f} bp")

        # Print worst offenders
        print(f"\n{'='*100}")
        print("WORST OFFENDERS (top 10 by absolute difference):")
        print('='*100)
        sorted_issues = sorted(all_issues, key=lambda x: abs(x['diff']), reverse=True)[:10]

        for i, issue in enumerate(sorted_issues, 1):
            print(f"\n{i}. {issue['date']} - {issue['source']} {issue['indices']} {issue['tenor']}")
            print(f"   Computed: {issue['computed']:>8.3f} bp | Database: {issue['database']:>8.3f} bp")
            print(f"   Diff: {issue['diff']:>8.3f} bp")
    else:
        print("\n✅ All dates have differences ≤ 1.0 bp!")

if __name__ == "__main__":
    main()
