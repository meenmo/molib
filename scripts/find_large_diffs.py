#!/usr/bin/env python3
"""
Find dates with spread differences > 0.1 bp.
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

def get_2025_dates():
    """Get all unique valuation dates from 2025."""
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    query = """
        SELECT DISTINCT valuation_date
        FROM pricing.basis_swap
        WHERE valuation_date >= '2025-01-01'
          AND valuation_date < '2026-01-01'
        ORDER BY valuation_date
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

def parse_output(output):
    """Parse spread output to extract differences."""
    issues = []
    lines = output.split('\n')

    for line in lines:
        if 'computed=' in line and 'diff=' in line:
            # Extract source, tenor, and diff
            parts = line.split()
            source = parts[0]  # BGN, BGNS, LCH
            indices = parts[1]  # EURIBOR3M/EURIBOR6M
            tenor = parts[2]  # 10x10

            # Extract diff value
            diff_str = [p for p in parts if p.startswith('diff=')][0]
            diff_val = float(diff_str.replace('diff=', '').replace('bp', ''))

            if abs(diff_val) > 0.1:
                issues.append({
                    'source': source,
                    'indices': indices,
                    'tenor': tenor,
                    'diff': diff_val,
                    'line': line.strip()
                })

    return issues

def main():
    print("Fetching 2025 dates from pricing.basis_swap...")
    dates = get_2025_dates()

    if not dates:
        print("❌ No dates found in 2025")
        sys.exit(1)

    print(f"Found {len(dates)} dates in 2025\n")
    print("Scanning for differences > 0.1 bp...")
    print("=" * 100)

    all_issues = []

    for i, date in enumerate(dates, 1):
        date_str = format_date_for_script(date)

        # Show progress every 50 dates
        if i % 50 == 0:
            print(f"[{i}/{len(dates)}] Processed {date}...")

        try:
            result = subprocess.run(
                ["go", "run", "./cmd/basiscalc", "-date", date_str],
                capture_output=True,
                text=True,
                timeout=30,
                cwd="/Users/meenmo/Documents/workspace/molib"
            )

            if result.returncode == 0:
                issues = parse_output(result.stdout)
                if issues:
                    for issue in issues:
                        all_issues.append({
                            'date': date,
                            'date_str': date_str,
                            **issue
                        })

        except subprocess.TimeoutExpired:
            print(f"⚠️  Timeout processing {date}")
        except Exception as e:
            print(f"⚠️  Error processing {date}: {e}")

    # Print summary
    print("\n" + "=" * 100)
    print(f"SUMMARY: Found {len(all_issues)} cases with |diff| > 0.1 bp")
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

        for date_str in sorted(by_date.keys()):
            issues = by_date[date_str]
            print(f"\n{'='*100}")
            print(f"Date: {issues[0]['date']} ({date_str})")
            print('-'*100)
            for issue in issues:
                print(f"{issue['source']:<6} {issue['indices']:<25} {issue['tenor']:<8} diff={issue['diff']:>8.4f} bp")
                print(f"  {issue['line']}")

        # Print worst offenders
        print(f"\n{'='*100}")
        print("WORST OFFENDERS (top 10 by absolute difference):")
        print('='*100)
        sorted_issues = sorted(all_issues, key=lambda x: abs(x['diff']), reverse=True)[:10]

        for i, issue in enumerate(sorted_issues, 1):
            print(f"\n{i}. {issue['date']} - {issue['source']} {issue['indices']} {issue['tenor']}")
            print(f"   Diff: {issue['diff']:>8.4f} bp")
            print(f"   {issue['line']}")
    else:
        print("\n✅ All dates have differences ≤ 0.1 bp!")

if __name__ == "__main__":
    main()
