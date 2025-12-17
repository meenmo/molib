#!/usr/bin/env python3
"""
Extract curve quotes for a new curve date from the database.
Usage: python3 extract_curves_new_date.py 2025-11-25
"""

import sys
import psycopg2

def extract_curves(curve_date):
    conn = psycopg2.connect(
        host='100.127.72.74',
        port=1013,
        user='meenmo',
        password='04201',
        database='ficc'
    )

    cur = conn.cursor()

    # Extract BGN ESTR
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='BGN' AND reference_index='ESTR'
    """, (curve_date,))
    bgn_estr = cur.fetchone()

    # Extract BGN EURIBOR3M
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='BGN' AND reference_index='EURIBOR3M'
    """, (curve_date,))
    bgn_3m = cur.fetchone()

    # Extract BGN EURIBOR6M
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='BGN' AND reference_index='EURIBOR6M'
    """, (curve_date,))
    bgn_6m = cur.fetchone()

    # Extract LCH ESTR
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='LCH' AND reference_index='ESTR'
    """, (curve_date,))
    lch_estr = cur.fetchone()

    # Extract LCH EURIBOR3M
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='LCH' AND reference_index='EURIBOR3M'
    """, (curve_date,))
    lch_3m = cur.fetchone()

    # Extract LCH EURIBOR6M
    cur.execute("""
        SELECT quotes
        FROM marketdata.curves
        WHERE curve_date=%s AND source='LCH' AND reference_index='EURIBOR6M'
    """, (curve_date,))
    lch_6m = cur.fetchone()

    cur.close()
    conn.close()

    print(f"// Curve data for {curve_date}")
    print()

    if bgn_estr:
        print("var BGNEstr = map[string]float64{")
        for tenor, rate in sorted(bgn_estr[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")
        print()

    if bgn_3m:
        print("var BGNEuribor3M = map[string]float64{")
        for tenor, rate in sorted(bgn_3m[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")
        print()

    if bgn_6m:
        print("var BGNEuribor6M = map[string]float64{")
        for tenor, rate in sorted(bgn_6m[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")
        print()

    if lch_estr:
        print("var LCHEstr = map[string]float64{")
        for tenor, rate in sorted(lch_estr[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")
        print()

    if lch_3m:
        print("var LCHEuribor3M = map[string]float64{")
        for tenor, rate in sorted(lch_3m[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")
        print()

    if lch_6m:
        print("var LCHEuribor6M = map[string]float64{")
        for tenor, rate in sorted(lch_6m[0].items()):
            print(f'    "{tenor}": {rate},')
        print("}")

if __name__ == '__main__':
    if len(sys.argv) != 2:
        print("Usage: python3 extract_curves_new_date.py YYYY-MM-DD")
        sys.exit(1)

    curve_date = sys.argv[1]
    extract_curves(curve_date)
