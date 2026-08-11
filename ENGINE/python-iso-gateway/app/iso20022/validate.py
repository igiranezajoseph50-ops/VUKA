"""Structural validation for inbound ISO 20022 payments.

The gateway does not run a full XSD engine (Phase 2 decision D6). Instead it
checks the fields the engine depends on, so a malformed message fails fast
with a clear reason instead of surfacing as an engine error.
"""
from __future__ import annotations

from decimal import Decimal

from app.iso20022.parser import Pacs008Payment

SUPPORTED_CURRENCIES = {"RWF", "KES"}


class ValidationError(ValueError):
    """Raised when a parsed payment fails business validation."""


def validate_payment(payment: Pacs008Payment) -> None:
    """Validate a parsed pacs.008 payment for the RWF->KES corridor."""
    if not payment.message_id:
        raise ValidationError("message_id (GrpHdr/MsgId) is required")

    if payment.amount <= 0:
        raise ValidationError(f"amount must be positive, got {payment.amount}")

    if payment.currency not in SUPPORTED_CURRENCIES:
        raise ValidationError(
            f"currency {payment.currency!r} not supported (expected RWF or KES)"
        )

    if payment.amount > Decimal("99999999999999.99"):
        raise ValidationError("amount exceeds the maximum allowed value")

    # Names are mandatory; accounts may be empty (resolved by the gateway).
    if not payment.debtor_name:
        raise ValidationError("debtor name is required")
    if not payment.creditor_name:
        raise ValidationError("creditor name is required")
