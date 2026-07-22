"""Pull a live register into pandas and compute rolling statistics --
the shape of a lot of predictive-maintenance and data-science work:
get real sensor data into a DataFrame with the least possible glue code.

Run against otcat-mockplc for a no-hardware demo:
    otcat-mockplc --addr 127.0.0.1:15020 &
    python examples/pandas_quickstart.py 127.0.0.1:15020
"""
import sys

from otcat import Client
from otcat.pandas_ext import watch_df


def main() -> None:
    endpoint = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1:15020"
    client = Client(endpoint, raw_address=True)

    print(f"pulling 30 samples of holding:0 from {endpoint} ...")
    df = watch_df(client, "holding:0", count=30, interval="200ms")

    # holding:0 on otcat-mockplc is a tank level, x100 fixed point
    df["percent"] = df["value"] / 100.0

    print(df[["address", "percent", "quality"]].tail(10))
    print()
    print("rolling 5-sample mean:")
    print(df["percent"].rolling(5).mean().tail(10))
    print()
    print(f"min={df['percent'].min():.2f}%  max={df['percent'].max():.2f}%  "
          f"mean={df['percent'].mean():.2f}%  std={df['percent'].std():.3f}")


if __name__ == "__main__":
    main()
