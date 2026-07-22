"""otcat: Python bindings for the otcat Go core.

A thin, typed wrapper around the real otcat/otc binary -- every read
and write in this package is executed by the same tested, fuzzed Go
Modbus TCP client the accompanying paper describes, not a Python
reimplementation of the protocol. See client.py's module docstring for
why that's a subprocess boundary rather than a cgo/ctypes one.

Quick start::

    from otcat import Client

    c = Client("127.0.0.1:502")
    v = c.read("holding:40001")
    print(v.value, v.quality)

    for v in c.watch("holding:40001", interval="500ms", count=10):
        print(v.ts, v.value)

    c.write("holding:40001", 100)  # confirm=True by default in the library

Optional extras (install with e.g. ``pip install otcat[pandas]``):

- ``otcat.pandas_ext`` -- DataFrame conversion, a rolling-window buffer
- ``otcat.alerting`` -- threshold rules with debounce, for automations
- ``otcat.audit`` -- read-only OT asset discovery / fingerprinting
- ``otcat.aio`` -- asyncio client for FastAPI and friends
"""
from .client import Client
from .exceptions import (
    ConnectionError,
    IOFailureError,
    OtcatBinaryNotFoundError,
    OtcatError,
    ProtocolError,
    Timeout,
    UsageError,
    WriteAbortedError,
)
from .models import Value, WritePlan

__all__ = [
    "Client",
    "Value",
    "WritePlan",
    "OtcatError",
    "OtcatBinaryNotFoundError",
    "UsageError",
    "ConnectionError",
    "ProtocolError",
    "WriteAbortedError",
    "IOFailureError",
    "Timeout",
]

__version__ = "1.0.0"
