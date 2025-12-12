#!/usr/bin/env python3
"""
Check TONAR availability in the database for a specific date.
"""
import sys
import psycopg2

DB_CONFIG = {
    "host": "100.127.72.74",
    "port": 1013,
    "user": "meenmo",
    "password": "04201",
    "database": "ficc",
}

def check_tonar(date_str):
    """Check TONAR availability for a date."""
    # Convert YYYYMMDD to YYYY-MM-DD
    db_date = f"{date_str[:4]}-{date_str[4:6]}-{date_str[6:]}"

    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    # Check BGN TONAR for this date
    query = """
        SELECT source, reference_index
        FROM marketdata.curves
        WHERE reference_index='TONAR' AND source='BGN' AND date=%s
    """

    print(f"Checking BGN TONAR curve for {db_date}...")
    print()

    cur.execute(query, (db_date,))
    results = cur.fetchall()

    if results:
        print(f"✅ Found BGN TONAR curve:")
        for source, index in results:
            print(f"  - {source} {index}")
    else:
        print("❌ No BGN TONAR curve found for this date")
        print()

        # Check if TONAR exists from other sources
        print("Checking TONAR from other sources...")
        query_other = """
            SELECT source, reference_index
            FROM marketdata.curves
            WHERE reference_index='TONAR' AND date=%s
            ORDER BY source
        """
        cur.execute(query_other, (db_date,))
        other_sources = cur.fetchall()

        if other_sources:
            print("Found TONAR from other sources:")
            for source, index in other_sources:
                print(f"  - {source} {index}")
        else:
            print("No TONAR data from any source for this date")

        print()
        print("Checking nearest dates with BGN TONAR...")

        # Find nearby dates with BGN TONAR
        query2 = """
            SELECT DISTINCT date
            FROM marketdata.curves
            WHERE reference_index = 'TONAR'
              AND source = 'BGN'
              AND date BETWEEN %s::date - INTERVAL '10 days' AND %s::date + INTERVAL '10 days'
            ORDER BY ABS(EXTRACT(EPOCH FROM (date - %s::date)))
            LIMIT 5
        """
        cur.execute(query2, (db_date, db_date, db_date))
        nearby = cur.fetchall()

        if nearby:
            print("Nearest dates with BGN TONAR:")
            for (date,) in nearby:
                print(f"  - {date}")
        else:
            print("No BGN TONAR data found in nearby dates either")

    print()

    # Check all Japanese curves available for this date
    print(f"All Japanese curves available for {db_date}:")
    query3 = """
        SELECT source, reference_index
        FROM marketdata.curves
        WHERE date = %s
          AND (reference_index LIKE '%TIBOR%'
               OR reference_index LIKE '%TONAR%'
               OR reference_index LIKE '%JPY%'
               OR reference_index LIKE '%TONA%')
        ORDER BY source, reference_index
    """
    cur.execute(query3, (db_date,))
    jpy_curves = cur.fetchall()

    if jpy_curves:
        for source, index in jpy_curves:
            print(f"  - {source} {index}")
    else:
        print("  No Japanese curves found for this date")

    cur.close()
    conn.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 check_tonar.py YYYYMMDD")
        sys.exit(1)

    check_tonar(sys.argv[1])
