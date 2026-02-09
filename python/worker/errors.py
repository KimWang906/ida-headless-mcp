"""
Structured error types for IDA worker.

Design follows the "Stop Forwarding Errors, Start Designing Them" philosophy:
  - ErrorKind categorised by caller action, not by origin
  - ErrorStatus is a first-class field (permanent / temporary)
  - Every error carries operation name + arbitrary context dict

Reference: https://fast.github.io/blog/stop-forwarding-errors-start-designing-them/
"""

from enum import Enum


class ErrorKind(str, Enum):
    """Categorised by what the caller CAN DO, not by what went wrong."""

    DATABASE_CLOSED = "database_closed"
    """DB must be opened first -> call open_binary."""

    NOT_FOUND = "not_found"
    """Function / struct / enum / address does not exist -> try a different target."""

    INVALID_INPUT = "invalid_input"
    """Parameter is wrong -> fix input and retry."""

    DECOMPILER_UNAVAILABLE = "decompiler_unavailable"
    """Hex-Rays not installed or not licensed -> fall back to disassembly."""

    API_INCOMPATIBLE = "api_incompatible"
    """IDA version lacks the required API -> use alternative code path."""

    INTERNAL = "internal"
    """Unexpected internal error."""


class ErrorStatus(str, Enum):
    """Explicit retry-ability – no guessing from error types."""

    PERMANENT = "permanent"
    """Do NOT retry (wrong address, missing licence, bad input …)."""

    TEMPORARY = "temporary"
    """Safe to retry (analysis not finished, transient state …)."""


class IDAError(Exception):
    """Single flat error type for the IDA worker module.

    Inspired by Apache OpenDAL's error design: one struct with
    kind + status + context instead of scattered enum variants.
    """

    def __init__(
        self,
        kind: ErrorKind,
        message: str,
        *,
        status: ErrorStatus = ErrorStatus.PERMANENT,
        operation: str = "",
        context: dict | None = None,
    ):
        self.kind = kind
        self.status = status
        self.message = message
        self.operation = operation
        self.context = context or {}
        super().__init__(message)

    def to_dict(self) -> dict:
        """Serialise to a dict suitable for Connect-RPC JSON error details."""
        return {
            "kind": self.kind.value,
            "status": self.status.value,
            "message": self.message,
            "operation": self.operation,
            "context": self.context,
        }

    # ------------------------------------------------------------------
    # Factory methods – make adding context *easier* than skipping it.
    # ------------------------------------------------------------------

    @classmethod
    def database_closed(cls, operation: str = "") -> "IDAError":
        return cls(
            ErrorKind.DATABASE_CLOSED,
            "IDA database is not open. Call open_binary first.",
            status=ErrorStatus.PERMANENT,
            operation=operation,
        )

    @classmethod
    def not_found(
        cls, what: str, address: int = 0, *, operation: str = ""
    ) -> "IDAError":
        ctx: dict = {}
        if address:
            ctx["address"] = hex(address)
        return cls(
            ErrorKind.NOT_FOUND,
            f"{what} not found",
            status=ErrorStatus.PERMANENT,
            operation=operation,
            context=ctx,
        )

    @classmethod
    def invalid_input(
        cls, message: str, *, operation: str = "", **ctx: object
    ) -> "IDAError":
        return cls(
            ErrorKind.INVALID_INPUT,
            message,
            status=ErrorStatus.PERMANENT,
            operation=operation,
            context=dict(ctx),
        )

    @classmethod
    def decompiler_unavailable(cls, operation: str = "") -> "IDAError":
        return cls(
            ErrorKind.DECOMPILER_UNAVAILABLE,
            "Decompiler not available (Hex-Rays not installed or not licensed)",
            status=ErrorStatus.PERMANENT,
            operation=operation,
        )

    @classmethod
    def api_incompatible(
        cls, api_name: str, *, operation: str = ""
    ) -> "IDAError":
        return cls(
            ErrorKind.API_INCOMPATIBLE,
            f"API '{api_name}' not available in this IDA version",
            status=ErrorStatus.PERMANENT,
            operation=operation,
            context={"api": api_name},
        )

    @classmethod
    def internal(
        cls, message: str, *, operation: str = "", **ctx: object
    ) -> "IDAError":
        return cls(
            ErrorKind.INTERNAL,
            message,
            status=ErrorStatus.PERMANENT,
            operation=operation,
            context=dict(ctx),
        )
