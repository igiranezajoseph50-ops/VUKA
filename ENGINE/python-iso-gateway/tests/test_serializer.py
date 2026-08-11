"""Unit tests for the pacs.002 serializer (round-trip via lxml)."""
from decimal import Decimal

from lxml import etree

from app.iso20022.serializer import (
    STS_ACSC,
    STS_RJCT,
    build_acsc,
    build_pacs002,
    build_rjct,
)


def test_build_acsc_has_valid_structure():
    xml = build_acsc("VUKA-2026-08-05-001", "INV-119", Decimal("10000"), "KES", Decimal("9.5"))
    root = etree.fromstring(xml.encode())
    assert root.tag.endswith("Document")
    # Traverse by local name.
    local = {etree.QName(el).localname: el for el in root.iter()}
    assert local["TxSts"].text == STS_ACSC
    assert local["OrgnlMsgId"].text == "VUKA-2026-08-05-001"
    assert local["AccptncAmt"] is not None


def test_build_acsc_carries_converted_amount_and_currency():
    xml = build_acsc("M1", "E2E", Decimal("10000"), "KES", Decimal("9.5"))
    root = etree.fromstring(xml.encode())
    amt = root.find(".//{*}AccptncAmt")
    assert amt is not None
    assert amt.get("Ccy") == "KES"
    assert amt.text == "10000"


def test_build_rjct_has_status_and_reason():
    xml = build_rjct("M1", "E2E", "Insuff Funds")
    root = etree.fromstring(xml.encode())
    local = {etree.QName(el).localname: el for el in root.iter()}
    assert local["TxSts"].text == STS_RJCT
    assert local["Cd"].text  # machine reason code present


def test_build_pacs002_with_created_at_is_deterministic_shape():
    from datetime import datetime, timezone
    xml = build_pacs002(
        "M1", "E2E", STS_ACSC,
        settlement_amount=Decimal("5000"),
        currency="KES",
        now=datetime(2026, 8, 5, 12, 0, 0, tzinfo=timezone.utc),
    )
    root = etree.fromstring(xml.encode())
    cre = root.find(".//{*}CreDtTm")
    assert cre is not None
    assert cre.text.startswith("2026-08-05")


def test_reason_code_mapping():
    # Amount-related rejection -> AM04 (InvalidAmount).
    xml = build_rjct("m", "e", "amount must be positive")
    assert "AM04" in xml
    # Currency rejection -> AM01 (IncorrectCurrency).
    xml2 = build_rjct("m", "e", "currency USD not supported")
    assert "AM01" in xml2