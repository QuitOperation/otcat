from __future__ import annotations

import pytest

from otcat import Client, ConnectionError as OtcatConnectionError
from otcat import ProtocolError, UsageError, WriteAbortedError
from otcat.exceptions import OtcatBinaryNotFoundError


def test_read_holding_uint16(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    v = c.read("holding:100")
    assert v.value == 0x1234
    assert v.type == "uint16"
    assert v.good


def test_read_float32(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True, type="float32")
    v = c.read("holding:200:2")
    assert abs(v.value - 3.14159) < 1e-4
    assert v.raw == [0x4049, 0x0FDB]


def test_read_coil(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    v = c.read("coil:0")
    assert v.value is True
    assert v.type == "bool"


def test_read_array(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    v = c.read("holding:0:3")
    assert isinstance(v.value, list)
    assert len(v.value) == 3
    assert v.type == "uint16[]"


def test_read_many(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    values = c.read_many(["holding:100", "coil:0", "input:5"])
    assert len(values) == 3
    assert values[0].value == 0x1234


def test_write_default_confirm_true(mockplc, otcat_env):
    # library default differs from the CLI default -- see client.py's
    # module docstring -- so a plain write() here must NOT raise.
    c = Client(mockplc, raw_address=True)
    c.write("holding:50", 999)
    assert c.read("holding:50").value == 999


def test_write_confirm_false_is_refused(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    with pytest.raises(WriteAbortedError):
        c.write("holding:50", 999, confirm=False)


def test_dry_run_touches_no_network(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    plan = c.dry_run("holding:100", 777)
    assert plan.registers == [777]
    assert plan.driver == "modbus"
    # confirm it really didn't write anything
    assert c.read("holding:100").value == 0x1234


def test_watch_yields_n_values(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    values = list(c.watch("holding:0", interval="50ms", count=5))
    assert len(values) == 5
    for v in values:
        assert v.address == "holding:0"


def test_watch_break_terminates_subprocess_cleanly(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    n = 0
    for _v in c.watch("holding:0", interval="30ms", count=0):
        n += 1
        if n >= 3:
            break
    assert n == 3


def test_illegal_address_raises_protocol_error(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    with pytest.raises(ProtocolError) as exc_info:
        c.read("holding:65530:10")
    assert exc_info.value.exception_code == "0x02"


def test_bad_spec_raises_usage_error(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    with pytest.raises(UsageError):
        c.read("not-a-real-table:0")


def test_unreachable_host_raises_connection_error(otcat_env):
    c = Client("127.0.0.1:1", raw_address=True, timeout=1.0)
    with pytest.raises(OtcatConnectionError):
        c.read("holding:0")


def test_binary_not_found_is_a_clear_error(monkeypatch):
    monkeypatch.delenv("OTCAT_BINARY", raising=False)
    monkeypatch.setattr("shutil.which", lambda _name: None)
    from otcat import _binary
    # also neutralize the bundled-binary path, since this test
    # environment legitimately ships one for linux-amd64 and that
    # correctly takes priority over a $PATH search -- this test is
    # specifically about the "nothing at all is available" case.
    monkeypatch.setattr(_binary, "_bundled_path", lambda: None)
    with pytest.raises(OtcatBinaryNotFoundError) as exc_info:
        _binary.find_binary()
    assert "otcat" in str(exc_info.value) or "Tried" in str(exc_info.value)


def test_write_multi_value_csv(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True)
    c.write("holding:60", "1,2,3,4,5")
    v = c.read("holding:60:5")
    assert v.value == [1, 2, 3, 4, 5]


def test_repr_is_useful(mockplc, otcat_env):
    c = Client(mockplc, raw_address=True, type="float32")
    assert "float32" in repr(c)
    assert mockplc in repr(c)
