"""Unit tests for the pacs.008 parser."""
from decimal import Decimal
from pathlib import Path

import pytest

from app.iso20022.parser import ParseError, parse_pacs008

FIXTURE = Path(__file__).parent / "fixtures" / "pacs008_valid.xml"


def _load() -> str:
    return FIXTURE.read_text(encoding="utf-8")


def test_parses_valid_pacs008():
    p = parse_pacs008(_load())
    assert p.message_id == "VUKA-2026-08-05-001"
    assert p.debtor_name == "Amina Uwera"
    assert p.creditor_name == "Kethan Gasana"
    assert p.amount == Decimal("95000.00")
    assert p.currency == "RWF"
    assert p.reference == "INV-2026-08-05-119"
    assert p.debtor_account == "RW00VUKA0000000001"


def test_parses_different_namespace():
    # Same structure, a different namespace URI — must still parse.
    xml = _load().replace(
        "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.10",
        "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.05",
    )
    p = parse_pacs008(xml)
    assert p.creditor_name == "Kethan Gasana"


def test_rejects_malformed_xml():
    with pytest.raises(ParseError):
        parse_pacs008("<Document>not closed")


def test_rejects_missing_grphdr():
    bad = _load().replace("<GrpHdr>", "<Nope>").replace("</GrpHdr>", "</Nope>")
    with pytest.raises(ParseError):
        parse_pacs008(bad)


def test_rejects_missing_amount():
    # Remove BOTH the group total and the per-tx amount so no fallback exists.
    bad = _load()
    bad = bad.replace("<TtlIntrBkSttlmAmt Ccy=\"RWF\">95000.00</TtlIntrBkSttlmAmt>", "")
    bad = bad.replace("<IntrBkSttlmAmt Ccy=\"RWF\">95000.00</IntrBkSttlmAmt>", "")
    with pytest.raises(ParseError):
        parse_pacs008(bad)


def test_rejects_bad_amount():
    bad = _load().replace("95000.00", "abc", 1)
    with pytest.raises(ParseError):
        parse_pacs008(bad)