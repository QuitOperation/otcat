#!/usr/bin/env python3
"""An independent Modbus TCP server, deliberately NOT written by the
otcat author, used to test otcat's Go client against a codebase with
different assumptions, different edge-case handling, and a different
author's reading of the same specification. This is the single most
useful thing achievable without real PLC hardware: agreement between
two independently-implemented, spec-following stacks is real evidence
that otcat's client is not just consistent with its own test mock.
"""
import logging
from pymodbus.datastore import (
    ModbusSequentialDataBlock,
    ModbusDeviceContext,
    ModbusServerContext,
)
from pymodbus.server import StartTcpServer

logging.basicConfig(level=logging.WARNING)

# Seed exactly the values otcat's own test suite and paper use, so a
# human can cross-check this run's log against known-good numbers:
#   holding[0]   = 4660          (0x1234)
#   holding[10:13] = [10, 20, 30]
#   holding[200:202] = float32(3.14159) big-endian, high-word-first
#   coil[0] = True, coil[3] = True
#   discrete[7] = True
#   input[5] = 777
holding = [0] * 400
holding[0] = 0x1234
holding[10], holding[11], holding[12] = 10, 20, 30
holding[200], holding[201] = 0x4049, 0x0FDB

coils = [False] * 100
coils[0] = True
coils[3] = True

discrete = [False] * 100
discrete[7] = True

inputs = [0] * 100
inputs[5] = 777

device = ModbusDeviceContext(
    di=ModbusSequentialDataBlock(1, discrete),
    co=ModbusSequentialDataBlock(1, coils),
    hr=ModbusSequentialDataBlock(1, holding),
    ir=ModbusSequentialDataBlock(1, inputs),
)
context = ModbusServerContext(devices=device, single=True)

print("pymodbus interop server listening on 127.0.0.1:15030", flush=True)
StartTcpServer(context=context, address=("127.0.0.1", 15030))
