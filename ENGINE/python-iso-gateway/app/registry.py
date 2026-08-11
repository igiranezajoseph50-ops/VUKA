"""Debtor/Creditor -> VUKA account resolution for the gateway.

Phase 2 decision: the engine has no account-search RPC, so the gateway maps
message counterparties to VUKA BUSINESS account ids. Mapping is populated
programmatically at startup (the demo seeds it after creating traders) and can
additionally be seeded from the environment with VUKA_ACCOUNT_MAP.

VUKA_ACCOUNT_MAP format: comma-separated name=currency:account_id entries,
e.g. "ACME Trading=RWF:11111111-...,Kamau Supplies=KES:22222222-..."
"""
from __future__ import annotations

import os

_accounts: dict[tuple[str, str], str] = {}


def _load_env() -> None:
    raw = os.getenv("VUKA_ACCOUNT_MAP", "")
    if not raw:
        return
    for entry in raw.split(","):
        entry = entry.strip()
        if not entry or "=" not in entry:
            continue
        name, spec = entry.split("=", 1)
        currency, _, account_id = spec.partition(":")
        if currency and account_id:
            register(name.strip(), currency.strip().upper(), account_id.strip())


def register(name: str, currency: str, account_id: str) -> None:
    """Register a counterparty name -> business account mapping."""
    _accounts[(name.strip().lower(), currency.upper())] = account_id


def resolve(name: str, currency: str) -> str:
    """Resolve a counterparty to a VUKA BUSINESS account id.

    Raises KeyError when the counterparty/currency pair is unknown so the
    caller can return a pacs.002 RJCT instead of guessing.
    """
    if not _accounts:
        _load_env()
    try:
        return _accounts[(name.strip().lower(), currency.upper())]
    except KeyError:
        raise KeyError(
            f"no account registered for counterparty {name!r} in {currency!r} "
            "(seed the gateway registry at startup)"
        ) from None


def known() -> list[dict[str, str]]:
    """All registered mappings (debug endpoint aid)."""
    return [{"name": n, "currency": c, "account_id": a} for (n, c), a in _accounts.items()]


def clear() -> None:
    """Drop all mappings (tests)."""
    _accounts.clear()
