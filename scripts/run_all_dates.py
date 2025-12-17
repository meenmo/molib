#!/usr/bin/env python3
"""
Run basis swap calculations for all dates available in pricing.basis_swap.
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

def get_available_dates():
    """Get all unique valuation dates from pricing.basis_swap."""
    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    query = """
        SELECT DISTINCT valuation_date
        FROM pricing.basis_swap
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

def main():
    print("Fetching available dates from pricing.basis_swap...")
    dates = get_available_dates()

    if not dates:
        print("❌ No dates found in pricing.basis_swap")
        sys.exit(1)

    print(f"Found {len(dates)} dates\n")
    print("=" * 80)

    for i, date in enumerate(dates, 1):
        date_str = format_date_for_script(date)
        print(f"\n[{i}/{len(dates)}] Processing {date} ({date_str})...")
        print("-" * 80)

        # Run the basis_swap.sh script for this date
        try:
            result = subprocess.run(
                ["./scripts/basis_swap.sh", date_str],
                capture_output=True,
                text=True,
                timeout=120
            )

            # Extract and display only the pricing results
            lines = result.stdout.split('\n')
            in_results = False
            for line in lines:
                if "=== Running Pricing Tests ===" in line:
                    in_results = True
                    continue
                if "=== Complete! ===" in line:
                    break
                if in_results and line.strip():
                    print(line)

            if result.returncode != 0:
                print(f"⚠️  Warning: Script exited with code {result.returncode}")
                if result.stderr:
                    print(f"Error: {result.stderr}")

        except subprocess.TimeoutExpired:
            print(f"❌ Timeout processing {date}")
        except Exception as e:
            print(f"❌ Error processing {date}: {e}")

        print("-" * 80)

    print("\n" + "=" * 80)
    print(f"Completed processing {len(dates)} dates")

if __name__ == "__main__":
    main()
