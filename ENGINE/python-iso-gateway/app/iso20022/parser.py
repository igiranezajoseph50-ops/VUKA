"""pacs.008.001.10 inbound parser.

Extracts the fields the VUKA engine needs from a customer credit transfer
message using lxml. Only the subset relevant to the Rwanda->Kenya corridor is
mapped (see PHASE2-TODO.md §6). The gateway is deliberately tolerant of the
XML namespace used by the sender — we resolve local names, not prefixes.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal

from lxml import etree

NS_PACS_008 = "urn:iso:std:iso:20022:tech:xsd:pacs.008.001.10"


@dataclass
class Pacs008Payment:
    """Normalized internal representation of an inbound pacs.008."""

    message_id: str
    created_at: str
    amount: Decimal
    currency: str
    debtor_name: str
    debtor_account: str
    creditor_name: str
    creditor_account: str
    reference: str = ""
    raw: str = ""


class ParseError(ValueError):
    """Raised when the XML is not a usable pacs.008."""


def _local(tag: str) -> str:
    """Strip namespace prefix: {uri}MsgId -> MsgId."""
    if tag.startswith("{"):
        return tag.split("}", 1)[1]
    return tag.rsplit(":", 1)[-1]


def _text(node, *path: str) -> str | None:
    """Return the text of the first child matching the local-name path."""
    cur = node
    for name in path:
        found = None
        for child in cur:
            if _local(child.tag) == name:
                found = child
                break
        if found is None:
            return None
        cur = found
    return (cur.text or "").strip() or None


def parse_pacs008(xml: str) -> Pacs008Payment:
    """Parse an inbound pacs.008.001.10 XML document into a payment."""
    try:
        root = etree.fromstring(xml.encode("utf-8"))
    except etree.XMLSyntaxError as exc:
        raise ParseError(f"malformed XML: {exc}") from exc

    # Root must be a pacs.008 document (any namespace version we accept).
    root_local = _local(root.tag)
    if root_local not in ("Document", "pacs.008.001.10"):
        raise ParseError(f"expected pacs.008 Document, got <{root_local}>")

    # Locate the FIToFICstmrCdtTrf element (sits under Document).
    fitofi = None
    for child in root:
        if _local(child.tag) in ("FIToFICstmrCdtTrf", "FIToFICstmrCdtTrfV10"):
            fitofi = child
            break
    if fitofi is None:
        raise ParseError("pacs.008: missing FIToFICstmrCdtTrf")

    grphdr = None
    txinf = None
    for child in fitofi:
        name = _local(child.tag)
        if name == "GrpHdr":
            grphdr = child
        elif name == "CdtTrfTxInf":
            txinf = child

    if grphdr is None:
        raise ParseError("pacs.008: missing GrpHdr")
    if txinf is None:
        raise ParseError("pacs.008: missing CdtTrfTxInf")

    message_id = _text(grphdr, "MsgId") or ""
    created_at = _text(grphdr, "CreDtTm") or ""

    # Total amount + currency.
    total_amt = _text(grphdr, "TtlIntrBkSttlmAmt")
    total_ccy = ""
    for child in grphdr:
        if _local(child.tag) == "TtlIntrBkSttlmAmt":
            total_ccy = child.get("Ccy") or ""
            break
    amount_raw = total_amt
    currency = total_ccy
    if not amount_raw:
        # Fall back to the per-transaction InterbankSettlementAmount.
        amount_raw = _text(txinf, "IntrBkSttlmAmt")
        for child in txinf:
            if _local(child.tag) == "IntrBkSttlmAmt":
                currency = child.get("Ccy") or ""
                break
    if not amount_raw:
        raise ParseError("pacs.008: no settlement amount found")
    if not currency:
        raise ParseError("pacs.008: no currency found")

    try:
        amount = Decimal(amount_raw)
    except Exception as exc:  # noqa: BLE001
        raise ParseError(f"pacs.008: invalid amount {amount_raw!r}") from exc

    # Debtor + Creditor (name + account).
    debtor_name = _text(txinf, "Dbtr", "Nm") or ""
    debtor_account = _text(txinf, "DbtrAcct", "Id", "IBAN") or ""
    creditor_name = _text(txinf, "Cdtr", "Nm") or ""
    creditor_account = _text(txinf, "CdtrAcct", "Id", "IBAN") or ""
    reference = _text(txinf, "InstrForCdtrAgt", "InstrInf") or ""

    if not debtor_name or not creditor_name:
        raise ParseError("pacs.008: debtor and creditor names are required")

    return Pacs008Payment(
        message_id=message_id,
        created_at=created_at,
        amount=amount,
        currency=currency,
        debtor_name=debtor_name,
        debtor_account=debtor_account,
        creditor_name=creditor_name,
        creditor_account=creditor_account,
        reference=reference,
        raw=xml,
    )
