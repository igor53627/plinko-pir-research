#!/usr/bin/env python3
"""
Convert balance diff parquet files into Plinko PIR artifacts.

Usage:
  python3 scripts/build_database_from_parquet.py \
      --input raw_balances \
      --output data
"""

import argparse
import struct
from pathlib import Path

import pyarrow.parquet as pq


def parse_args():
    parser = argparse.ArgumentParser(description="Build database.bin from parquet balance diffs.")
    parser.add_argument(
        "--input",
        type=Path,
        default=Path("raw_balances"),
        help="Directory containing balance_diffs_blocks-*.parquet files.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("data"),
        help="Directory where database.bin and address-mapping.bin will be written.",
    )
    return parser.parse_args()


def list_parquet_files(input_dir: Path):
    files = sorted(input_dir.glob("balance_diffs_blocks-*.parquet"))
    if not files:
        raise FileNotFoundError(f"No parquet files found in {input_dir}")
    return files


def clamp_uint64(value: int) -> int:
    return min(value, (1 << 64) - 1)


def main():
    args = parse_args()
    files = list_parquet_files(args.input)
    print(f"Found {len(files)} parquet files under {args.input}")

    balances = {}
    for idx, path in enumerate(files, start=1):
        table = pq.read_table(path, columns=["address", "balance_after"])
        addr_col = table.column("address")
        bal_col = table.column("balance_after")
        rows = len(addr_col)
        for i in range(rows):
            addr = addr_col[i].as_py()
            bal_bytes = bal_col[i].as_py()
            if addr is None or bal_bytes is None:
                continue
            balance = int.from_bytes(bal_bytes, "big", signed=False)
            balances[addr] = clamp_uint64(balance)
        print(f"[{idx}/{len(files)}] processed {path.name} ({rows} rows). Total unique addresses: {len(balances)}")

    if not balances:
        raise RuntimeError("No balances were parsed from parquet files.")

    args.output.mkdir(parents=True, exist_ok=True)
    db_path = args.output / "database.bin"
    mapping_path = args.output / "address-mapping.bin"

    sorted_items = sorted(balances.items())
    print(f"Writing {len(sorted_items)} entries to {db_path} and {mapping_path}")

    with db_path.open("wb") as db_file, mapping_path.open("wb") as map_file:
        for index, (addr, balance) in enumerate(sorted_items):
            db_file.write(struct.pack("<Q", balance))
            map_file.write(addr)
            map_file.write(struct.pack("<I", index))

    print("Done.")


if __name__ == "__main__":
    main()
