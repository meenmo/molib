#!/usr/bin/env python3
"""
Generate Go fixture files from database curve data.

Usage:
    # Generate BGN EUR fixtures for a specific date
    python3 generate_fixtures.py --date 20251125 --source BGN --currency EUR

    # Generate LCH EUR fixtures
    python3 generate_fixtures.py --date 20251125 --source LCH --currency EUR

    # Generate BGN TIBOR fixtures
    python3 generate_fixtures.py --date 20251125 --source BGN --currency JPY

    # Generate all sources for a date
    python3 generate_fixtures.py --date 20251125 --all

Output:
    Creates Go file at: swap/basis/data/fixtures_{source}_{currency}_{YYYYMMDD}.go

Example:
    $ python3 generate_fixtures.py --date 20251125 --source BGN --currency EUR

    Generated: swap/basis/data/fixtures_bgn_eur_20251125.go

    Use in your code:
        import "github.com/meenmo/molib/swap/basis/data"

        spread, pv := basis.CalculateSpread(
            curveDate,
            10, 10,
            benchmark.EURIBOR6MFloat,
            benchmark.EURIBOR3MFloat,
            benchmark.ESTRFloat,
            data.BGNEstr_20251125,
            data.BGNEuribor6M_20251125,
            data.BGNEuribor3M_20251125,
            10_000_000.0,
        )
"""

import argparse
import sys
from datetime import datetime
from pathlib import Path
import psycopg2
from typing import Dict, List, Optional, Tuple


# Database configuration
DB_CONFIG = {
    "host": "100.127.72.74",
    "port": 1013,
    "user": "meenmo",
    "password": "04201",
    "database": "ficc",
}

# Currency to index mapping
# 'db_name' is used for database queries
# 'var_name' is used for Go variable names
CURRENCY_INDICES = {
    "EUR": {
        "ois": {"db_name": "ESTR", "var_name": "Estr"},
        "ibor_3m": {"db_name": "EURIBOR3M", "var_name": "Euribor3M"},
        "ibor_6m": {"db_name": "EURIBOR6M", "var_name": "Euribor6M"},
    },
    "JPY": {
        "ois": {"db_name": "TONAR", "var_name": "Tonar"},
        "ibor_3m": {"db_name": "TIBOR3M", "var_name": "Tibor3M"},
        "ibor_6m": {"db_name": "TIBOR6M", "var_name": "Tibor6M"},
    },
    "USD": {
        "ois": {"db_name": "SOFR", "var_name": "Sofr"},
        "ibor_3m": {"db_name": "USD_LIBOR_3M", "var_name": "UsdLibor3M"},
        "ibor_6m": {"db_name": "USD_LIBOR_6M", "var_name": "UsdLibor6M"},
    },
    "GBP": {
        "ois": {"db_name": "SONIA", "var_name": "Sonia"},
        "ibor_3m": {"db_name": "GBP_LIBOR_3M", "var_name": "GbpLibor3M"},
        "ibor_6m": {"db_name": "GBP_LIBOR_6M", "var_name": "GbpLibor6M"},
    },
}

# Allowed tenors by index (only these will be included in fixtures)
ALLOWED_TENORS = {
    "ESTR": [
        "1W",
        "2W",
        "1M",
        "2M",
        "3M",
        "4M",
        "5M",
        "6M",
        "7M",
        "8M",
        "9M",
        "10M",
        "11M",
        "1Y",
        "18M",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "11Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "40Y",
        "50Y",
    ],
    "EURIBOR3M": [
        "3M",
        "1Y",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "11Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "40Y",
        "50Y",
    ],
    "EURIBOR6M": [
        "6M",
        "1Y",
        "18M",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "11Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "40Y",
        "50Y",
    ],
    "TONAR": [
        "1W",
        "2W",
        "1M",
        "2M",
        "3M",
        "4M",
        "5M",
        "6M",
        "7M",
        "8M",
        "9M",
        "10M",
        "11M",
        "1Y",
        "15M",
        "18M",
        "21M",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "11Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "35Y",
        "40Y",
    ],
    "TIBOR3M": [
        "3M",
        "1Y",
        "18M",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "40Y",
    ],
    "TIBOR6M": [
        "6M",
        "1Y",
        "18M",
        "2Y",
        "3Y",
        "4Y",
        "5Y",
        "6Y",
        "7Y",
        "8Y",
        "9Y",
        "10Y",
        "12Y",
        "15Y",
        "20Y",
        "25Y",
        "30Y",
        "35Y",
        "40Y",
    ],
}


def get_curve_data(
    curve_date: str, source: str, reference_index: str
) -> Optional[Dict[str, float]]:
    """
    Retrieve curve quotes from database and filter by allowed tenors.

    Args:
        curve_date: Date in YYYYMMDD format (will be converted to YYYY-MM-DD for DB query)
        source: Data source (e.g., 'BGN', 'LCH')
        reference_index: Index name (e.g., 'ESTR', 'EURIBOR3M')

    Returns:
        Dictionary of tenor -> rate (filtered by allowed tenors), or None if not found
    """
    try:
        # Convert YYYYMMDD to YYYY-MM-DD for database query
        db_date = f"{curve_date[:4]}-{curve_date[4:6]}-{curve_date[6:]}"

        conn = psycopg2.connect(**DB_CONFIG)
        cur = conn.cursor()

        query = """
            SELECT quotes
            FROM marketdata.curves
            WHERE date = %s
              AND source = %s
              AND reference_index = %s
        """

        cur.execute(query, (db_date, source, reference_index))
        result = cur.fetchone()

        cur.close()
        conn.close()

        if result and result[0]:
            # Database can store quotes in different formats:
            # 1. Direct array: [{"tenor": "1Y", "rate": 1.8916, "ticker": "..."}, ...]
            # 2. Dict with nested quotes: {"quotes": [...], "source": "BGN", ...}
            # 3. Dict of tenor -> rate: {"1Y": 1.8916, "2Y": 2.5, ...}
            quotes_dict = {}
            data = result[0]

            if isinstance(data, list):
                # Format 1: Direct array
                for item in data:
                    if isinstance(item, dict) and "tenor" in item and "rate" in item:
                        quotes_dict[item["tenor"]] = item["rate"]
            elif isinstance(data, dict):
                # Check if it has a 'quotes' key (Format 2)
                if 'quotes' in data and isinstance(data['quotes'], list):
                    for item in data['quotes']:
                        if isinstance(item, dict) and "tenor" in item and "rate" in item:
                            quotes_dict[item["tenor"]] = item["rate"]
                # Otherwise assume Format 3: direct dict of tenor -> rate
                elif all(isinstance(k, str) and isinstance(v, (int, float)) for k, v in data.items() if k not in ['source', 'curve_date', 'curve_type', 'reference_index']):
                    quotes_dict = {k: v for k, v in data.items() if k not in ['source', 'curve_date', 'curve_type', 'reference_index']}

            # Filter by allowed tenors for this index
            if reference_index in ALLOWED_TENORS:
                allowed = set(ALLOWED_TENORS[reference_index])
                quotes_dict = {k: v for k, v in quotes_dict.items() if k in allowed}

            return quotes_dict

        return None

    except Exception as e:
        print(f"Error retrieving {source} {reference_index}: {e}", file=sys.stderr)
        return None


def format_tenor_key(tenor: str) -> str:
    """Format tenor key for Go map (with proper quoting)."""
    return f'"{tenor}"'


def format_rate_value(rate: float) -> str:
    """Format rate value for Go map."""
    return str(rate)


def sort_tenors(tenors: List[str]) -> List[str]:
    """
    Sort tenors in logical order (W, M, Y).

    Examples:
        1W, 2W, 1M, 2M, 3M, 6M, 1Y, 2Y, 5Y, 10Y, 30Y
    """

    def tenor_to_months(tenor: str) -> float:
        tenor = tenor.upper().strip()
        if tenor.endswith("W"):
            weeks = float(tenor[:-1])
            return weeks * 7 / 30  # Approximate weeks to months
        elif tenor.endswith("M"):
            return float(tenor[:-1])
        elif tenor.endswith("Y"):
            return float(tenor[:-1]) * 12
        return 0

    return sorted(tenors, key=tenor_to_months)


def generate_go_map(var_name: str, quotes: Dict[str, float], indent: int = 1) -> str:
    """
    Generate Go map variable definition.

    Args:
        var_name: Variable name (e.g., 'BGNEstr_20251125')
        quotes: Dictionary of tenor -> rate
        indent: Indentation level

    Returns:
        Formatted Go code
    """
    tab = "\t" * indent
    lines = [f"{var_name} = map[string]float64{{"]

    # Sort tenors
    sorted_tenors = sort_tenors(list(quotes.keys()))

    # Add each tenor -> rate mapping (skip None values)
    for tenor in sorted_tenors:
        rate = quotes[tenor]
        if rate is not None:
            lines.append(f'{tab}"{tenor}": {rate},')

    lines.append("}")

    return "\n".join(lines)


def generate_fixture_file(
    curve_date: str,
    source: str,
    currency: str,
    ois_quotes: Dict[str, float],
    ibor_3m_quotes: Dict[str, float],
    ibor_6m_quotes: Dict[str, float],
    output_dir: Path,
    var_prefix: str = None,
) -> None:
    """
    Generate complete Go fixture file with standard names (no date suffix).

    Args:
        curve_date: Date in YYYYMMDD format
        source: Data source (e.g., 'BGN', 'LCH') - used for filename
        currency: Currency code (e.g., 'EUR', 'JPY')
        ois_quotes: OIS curve quotes
        ibor_3m_quotes: 3M IBOR quotes
        ibor_6m_quotes: 6M IBOR quotes
        output_dir: Output directory
        var_prefix: Optional prefix for variable names (defaults to source)
    """
    # Use source as default variable prefix
    if var_prefix is None:
        var_prefix = source

    # Generate standard variable names (no date suffix)
    # BGN EUR: BGNEstr, BGNEuribor3M, BGNEuribor6M
    # BGN JPY: BGNTonar, BGNTibor3M, BGNTibor6M
    # LCH EUR: LCHEstr, LCHEuribor3M, LCHEuribor6M
    indices = CURRENCY_INDICES[currency]

    ois_var = f"{var_prefix}{indices['ois']['var_name']}"
    ibor_3m_var = f"{var_prefix}{indices['ibor_3m']['var_name']}"
    ibor_6m_var = f"{var_prefix}{indices['ibor_6m']['var_name']}"

    # Determine output filename based on source and currency
    # BGN EUR -> fixtures_bgn_euribor.go
    # BGN JPY -> fixtures_bgn_tibor.go (even if IBOR from BGNS)
    # LCH EUR -> fixtures_lch_euribor.go
    if currency == "EUR":
        filename = f"fixtures_{source.lower()}_euribor.go"
    elif currency == "JPY":
        filename = f"fixtures_{source.lower()}_tibor.go"
    elif currency == "USD":
        filename = f"fixtures_{source.lower()}_usd_libor.go"
    elif currency == "GBP":
        filename = f"fixtures_{source.lower()}_gbp_libor.go"
    else:
        filename = f"fixtures_{source.lower()}_{currency.lower()}.go"

    output_path = output_dir / filename

    # Format date for comment (YYYY-MM-DD)
    formatted_date = f"{curve_date[:4]}-{curve_date[4:6]}-{curve_date[6:]}"

    # Generate Go code
    lines = [
        "package data",
        "",
        f"// {source} {currency} quotes for curve date {formatted_date}.",
        f"// Generated by generate_fixtures.py on {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        "var (",
    ]

    # OIS curve
    if ois_quotes:
        lines.append(f"\t// {indices['ois']['db_name']} OIS curve ({len(ois_quotes)} tenors)")
        lines.append(
            "\t" + generate_go_map(ois_var, ois_quotes, indent=2).replace("\n", "\n\t")
        )
        lines.append("")

    # 3M IBOR curve
    if ibor_3m_quotes:
        lines.append(f"\t// {indices['ibor_3m']['db_name']} curve ({len(ibor_3m_quotes)} tenors)")
        lines.append(
            "\t"
            + generate_go_map(ibor_3m_var, ibor_3m_quotes, indent=2).replace(
                "\n", "\n\t"
            )
        )
        lines.append("")

    # 6M IBOR curve
    if ibor_6m_quotes:
        lines.append(f"\t// {indices['ibor_6m']['db_name']} curve ({len(ibor_6m_quotes)} tenors)")
        lines.append(
            "\t"
            + generate_go_map(ibor_6m_var, ibor_6m_quotes, indent=2).replace(
                "\n", "\n\t"
            )
        )

    lines.append(")")
    lines.append("")

    # Write to file
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with open(output_path, "w") as f:
        f.write("\n".join(lines))

    print(f"✅ Generated: {output_path}")
    print(f"   - {ois_var}: {len(ois_quotes)} tenors")
    print(f"   - {ibor_3m_var}: {len(ibor_3m_quotes)} tenors")
    print(f"   - {ibor_6m_var}: {len(ibor_6m_quotes)} tenors")
    print()
    print("Use in your code:")
    print(f'    import "github.com/meenmo/molib/swap/basis/data"')
    print()
    print(f"    data.{ois_var}")
    print(f"    data.{ibor_3m_var}")
    print(f"    data.{ibor_6m_var}")


def main():
    parser = argparse.ArgumentParser(
        description="Generate Go fixture files from database curve data",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )

    parser.add_argument(
        "--date",
        required=True,
        help="Curve date in YYYYMMDD format (e.g., 20251125)",
    )

    parser.add_argument(
        "--source", help="Data source (e.g., BGN, LCH). Required unless --all is used."
    )

    parser.add_argument(
        "--currency",
        help="Currency code (e.g., EUR, JPY, USD, GBP). Required unless --all is used.",
    )

    parser.add_argument(
        "--all",
        action="store_true",
        help="Generate fixtures for all available sources for the given date",
    )

    parser.add_argument(
        "--ois-source",
        help="Separate OIS source (e.g., BGN for TONAR). If not specified, uses --source.",
    )

    parser.add_argument(
        "--ibor-source",
        help="Separate IBOR source (e.g., BGNS for TIBOR). If not specified, uses --source.",
    )

    parser.add_argument(
        "--output-dir",
        default="swap/basis/data",
        help="Output directory for fixture files (default: swap/basis/data)",
    )

    args = parser.parse_args()

    # Validate arguments
    if not args.all:
        # Either --source must be provided, OR both ois-source and ibor-source
        has_source = args.source is not None
        has_both_specific = args.ois_source is not None and args.ibor_source is not None

        if not has_source and not has_both_specific:
            parser.error("Either --source must be provided, or both --ois-source and --ibor-source must be provided")

        if not args.currency:
            parser.error("--currency is required unless --all is used")

    # Validate date format (YYYYMMDD)
    if len(args.date) != 8 or not args.date.isdigit():
        parser.error(f"Invalid date format: {args.date}. Use YYYYMMDD (e.g., 20251125)")

    try:
        # Validate it's a real date
        datetime.strptime(args.date, "%Y%m%d")
    except ValueError:
        parser.error(f"Invalid date: {args.date}. Use YYYYMMDD format")

    # Validate currency
    if args.currency and args.currency.upper() not in CURRENCY_INDICES:
        parser.error(
            f"Unsupported currency: {args.currency}. "
            f"Supported: {', '.join(CURRENCY_INDICES.keys())}"
        )

    output_dir = Path(args.output_dir)

    if args.all:
        # Generate for all available sources
        print(f"Searching for all available curves for {args.date}...")

        conn = psycopg2.connect(**DB_CONFIG)
        cur = conn.cursor()

        query = """
            SELECT DISTINCT source, reference_index
            FROM marketdata.curves
            WHERE date = %s
            ORDER BY source, reference_index
        """

        cur.execute(query, (args.date,))
        available = cur.fetchall()

        cur.close()
        conn.close()

        if not available:
            print(f"❌ No curves found for date {args.date}")
            sys.exit(1)

        print(f"Found {len(available)} curves:")
        for source, index in available:
            print(f"  - {source} {index}")
        print()

        # Group by source and currency
        groups = {}
        for source, index in available:
            # Determine currency from index
            currency = None
            for curr, indices in CURRENCY_INDICES.items():
                # Check if index matches any db_name
                for key, val in indices.items():
                    if val["db_name"] == index:
                        currency = curr
                        break
                if currency:
                    break

            if currency:
                key = (source, currency)
                if key not in groups:
                    groups[key] = []
                groups[key].append(index)

        # Generate fixture for each group
        for (source, currency), indices in groups.items():
            generate_fixture_for_source(args.date, source, currency, output_dir)

    else:
        # Generate for specific source and currency
        # Determine OIS and IBOR sources
        # If --source is not provided, use --ois-source as the default/display source
        default_source = args.source if args.source else args.ois_source

        # Determine each source with proper defaults
        if args.ois_source:
            ois_source = args.ois_source.upper()
        elif args.source:
            ois_source = args.source.upper()
        else:
            ois_source = None  # This shouldn't happen due to validation

        if args.ibor_source:
            ibor_source = args.ibor_source.upper()
        elif args.source:
            ibor_source = args.source.upper()
        else:
            ibor_source = None  # This shouldn't happen due to validation

        generate_fixture_for_source(
            args.date, default_source.upper(), args.currency.upper(), output_dir,
            ois_source=ois_source, ibor_source=ibor_source
        )


def generate_fixture_for_source(
    curve_date: str, source: str, currency: str, output_dir: Path,
    ois_source: str = None, ibor_source: str = None
) -> None:
    """Generate fixture file for a specific source and currency.

    Args:
        curve_date: Date in YYYYMMDD format
        source: Primary data source (used for display/filename)
        currency: Currency code
        output_dir: Output directory
        ois_source: Optional separate source for OIS curve (defaults to source)
        ibor_source: Optional separate source for IBOR curves (defaults to source)

    Note:
        Discounting always uses the OIS source.
    """
    # Use source as default if not specified
    if ois_source is None:
        ois_source = source
    if ibor_source is None:
        ibor_source = source

    print(f"Generating {source} {currency} fixture for {curve_date}...")
    if ois_source != source or ibor_source != source:
        print(f"  (OIS from {ois_source}, IBOR from {ibor_source})")

    # Get currency indices
    if currency not in CURRENCY_INDICES:
        print(f"❌ Unsupported currency: {currency}")
        return

    indices = CURRENCY_INDICES[currency]

    # Retrieve data from database using specified sources
    print(f"  Fetching {indices['ois']['db_name']} OIS curve from {ois_source}...")
    ois_quotes = get_curve_data(curve_date, ois_source, indices["ois"]["db_name"])

    print(f"  Fetching {indices['ibor_3m']['db_name']} curve from {ibor_source}...")
    ibor_3m_quotes = get_curve_data(curve_date, ibor_source, indices["ibor_3m"]["db_name"])

    print(f"  Fetching {indices['ibor_6m']['db_name']} curve from {ibor_source}...")
    ibor_6m_quotes = get_curve_data(curve_date, ibor_source, indices["ibor_6m"]["db_name"])

    # Check if we got any data
    if not ois_quotes and not ibor_3m_quotes and not ibor_6m_quotes:
        print(f"❌ No data found for {source} {currency} on {curve_date}")
        return

    if not ois_quotes:
        print(f"⚠️  Warning: No {indices['ois']['db_name']} OIS curve found")
        ois_quotes = {}

    if not ibor_3m_quotes:
        print(f"⚠️  Warning: No {indices['ibor_3m']['db_name']} curve found")
        ibor_3m_quotes = {}

    if not ibor_6m_quotes:
        print(f"⚠️  Warning: No {indices['ibor_6m']['db_name']} curve found")
        ibor_6m_quotes = {}

    # Generate fixture file (filename determined inside function)
    # For mixed sources (e.g., BGNS with OIS from BGN), use OIS source as var prefix
    var_prefix = ois_source if ois_source != ibor_source else source

    generate_fixture_file(
        curve_date,
        var_prefix,  # Use OIS source for both filename and variable prefix
        currency,
        ois_quotes,
        ibor_3m_quotes,
        ibor_6m_quotes,
        output_dir,
        var_prefix=var_prefix,
    )


if __name__ == "__main__":
    main()
