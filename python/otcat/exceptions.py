"""otcat's Python exceptions mirror the Go CLI's exit-code contract
(see the main project's README, "Exit codes" table) one-for-one, so
catching a specific class tells you exactly what class of failure
happened without parsing error text.
"""
from __future__ import annotations


class OtcatError(Exception):
    """Base class for every error this package raises."""


class OtcatBinaryNotFoundError(OtcatError):
    """No otcat/otc binary could be located."""

    def __init__(self, tried: list[str]):
        self.tried = tried
        msg = (
            "could not find an otcat binary. Tried, in order:\n  - "
            + "\n  - ".join(tried)
            + "\n\nFix: install the Go binary (see "
            "https://github.com/QuitOperation/otcat#install--build) and "
            "ensure it's on $PATH, or set OTCAT_BINARY to its exact path."
        )
        super().__init__(msg)


class UsageError(OtcatError):
    """Exit code 1: bad arguments, malformed address spec, bad literal.
    Almost always a bug in the calling code, not a transient condition."""


class ConnectionError(OtcatError):  # noqa: A001 - deliberately shadows builtin, mirrors Go's naming
    """Exit code 2: dial failure, timeout, connection refused."""


class ProtocolError(OtcatError):
    """Exit code 3: the device returned a real Modbus exception
    (illegal address, illegal value, etc). See .exception_code."""

    def __init__(self, message: str, exception_code: str | None = None):
        super().__init__(message)
        self.exception_code = exception_code


class WriteAbortedError(OtcatError):
    """Exit code 4: a write was refused because it was not confirmed.
    This is otcat's safety gate working as designed, not a bug --
    catching this and silently retrying with confirm=True defeats the
    entire point of the gate. See client.py's module docstring."""


class IOFailureError(OtcatError):
    """Exit code 5: e.g. a broken output pipe, or an otherwise
    unclassified failure."""


class Timeout(OtcatError):
    """Raised by watch()/read() when a client-side Python timeout
    (distinct from otcat's own --timeout) elapses -- e.g. a watch
    generator that produced no value for longer than expected."""


_EXIT_CODE_MAP = {
    1: UsageError,
    2: ConnectionError,
    3: ProtocolError,
    4: WriteAbortedError,
    5: IOFailureError,
}


def error_for_exit_code(code: int, stderr: str) -> OtcatError:
    cls = _EXIT_CODE_MAP.get(code, OtcatError)
    message = stderr.strip() or f"otcat exited with code {code}"
    if cls is ProtocolError:
        # otcat's protocol-error stderr looks like:
        #   otcat: read holding:0: modbus: unit 1 function 0x03: illegal data address (0x02)
        exc_code = None
        if "(0x" in message:
            exc_code = message.rsplit("(0x", 1)[-1].rstrip(")")
            exc_code = "0x" + exc_code
        return ProtocolError(message, exception_code=exc_code)
    return cls(message)
