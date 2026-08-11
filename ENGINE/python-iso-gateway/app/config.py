"""VUKA ISO 20022 gateway configuration.

Loads runtime settings from environment variables with development defaults,
mirroring the Go engine's twelve-factor style.
"""
from __future__ import annotations

import os


def _env_float(name: str, default: float) -> float:
    raw = os.getenv(name, "")
    if raw == "":
        return default
    try:
        return float(raw)
    except ValueError:
        return default


class Config:
    """Gateway settings. Values are read once at import time."""

    def __init__(self) -> None:
        # Where the Go ledger engine's gRPC bridge listens.
        self.engine_grpc_addr: str = os.getenv("VUKA_ENGINE_GRPC_ADDR", "localhost:50051")
        # Engine default FX rate used when the message omits one.
        self.fx_rate: float = _env_float("VUKA_FX_RWF_KES", 9.5)
        # ISO 20022 identifiers stamped into outbound messages.
        self.bic: str = os.getenv("VUKA_BIC", "VUKARWRW")
        self.bank_name: str = os.getenv("VUKA_BANK_NAME", "VUKA PAYMENTS LTD")
        # gRPC call timeout in seconds.
        self.grpc_timeout_s: float = _env_float("VUKA_GRPC_TIMEOUT_S", 15.0)

    @classmethod
    def load(cls) -> "Config":
        return cls()


config = Config.load()
