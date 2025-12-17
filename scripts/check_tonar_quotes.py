#!/usr/bin/env python3
"""
Check TONAR quotes to see if they're empty or NULL.
"""
import sys
import psycopg2
import json

DB_CONFIG = {
    "host": "100.127.72.74",
    "port": 1013,
    "user": "meenmo",
    "password": "04201",
    "database": "ficc",
}

def check_tonar_quotes(date_str):
    """Check TONAR quotes content."""
    # Convert YYYYMMDD to YYYY-MM-DD
    db_date = f"{date_str[:4]}-{date_str[4:6]}-{date_str[6:]}"

    conn = psycopg2.connect(**DB_CONFIG)
    cur = conn.cursor()

    query = """
        SELECT source, reference_index, quotes
        FROM marketdata.curves
        WHERE reference_index='TONAR' AND source='BGN' AND date=%s
    """

    cur.execute(query, (db_date,))
    result = cur.fetchone()

    if result:
        source, index, quotes = result
        print(f"Found: {source} {index}")
        print(f"Quotes type: {type(quotes)}")
        print(f"Quotes value: {quotes}")
        print()

        if quotes is None:
            print("❌ Quotes field is NULL")
        elif isinstance(quotes, list) and len(quotes) == 0:
            print("❌ Quotes field is an empty list")
        elif isinstance(quotes, list):
            print(f"✅ Quotes field contains {len(quotes)} items")
            print("First 3 quotes:")
            for item in quotes[:3]:
                print(f"  {item}")
        else:
            print(f"❌ Quotes field has unexpected type: {type(quotes)}")
    else:
        print("No BGN TONAR found for this date")

    cur.close()
    conn.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python3 check_tonar_quotes.py YYYYMMDD")
        sys.exit(1)

    check_tonar_quotes(sys.argv[1])
