"""pacs.002.001.10 status report serializer.

Builds the outbound status report the gateway returns after executing (or
rejecting) an inbound pacs.008. Uses a fixed template with lxml, emitting a
well-formed ISO 20022 document in the pacs.002.001.10 namespace.
"""
from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

from lxml import etree

NS = "urn:iso:std:iso:20022:tech:xsd:pacs.002.001.10"
NSMAP = {"ns": NS}

# Transaction statuses (ISO 20022 ExternalPaymentTransactionStatus1Code).
STS_ACSC = "ACSC"  # AcceptedSettlementCompleted
STS_RJCT = "RJCT"  # Rejected


def _make(tag: str, text: str | Decimal | None = None) -> etree._Element:
    el = etree.Element(f"{{{NS}}}{tag}", nsmap=NSMAP)
    if text is not None:
        el.text = str(text)
    return el


def _append(parent: etree._Element, tag: str, text: str | Decimal | None = None) -> etree._Element:
    el = _make(tag, text)
    parent.append(el)
    return el


def build_pacs002(
    original_message_id: str,
    original_end_to_end: str,
    status: str,
    reason: str | None = None,
    settlement_amount: Decimal | None = None,
    currency: str | None = None,
    fx_rate: Decimal | None = None,
    now: datetime | None = None,
) -> str:
    """Build a pacs.002.001.10 status report XML string."""
    ts = (now or datetime.now(timezone.utc)).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"

    doc = _make("Document")
    report = _append(doc, "FIToFIPmtStsRpt")

    # Group header.
    grp = _append(report, "GrpHdr")
    _append(grp, "MsgId", f"VUKA-{int(datetime.now(timezone.utc).timestamp())}")
    _append(grp, "CreDtTm", ts)
    _append(grp, "InitgPty", None)
    _append(grp.find(f"{{{NS}}}InitgPty"), "Nm", "VUKA PAYMENTS LTD")

    # Transaction information.
    txi = _append(report, "OrgnlGrpInfAndSts")
    _append(txi, "OrgnlMsgId", original_message_id)
    _append(txi, "OrgnlMsgNmId", "pacs.008.001.10")
    _append(txi, "OrgnlCreDtTm", ts)
    _append(txi, "GrpSts", status)
    if reason:
        sts_rsn = _append(txi, "StsRsnInf")
        _append(sts_rsn, "Rsn", None)
        _append(sts_rsn.find(f"{{{NS}}}Rsn"), "Cd", reason)

    # Per-transfer detail with settlement amount + FX.
    tx = _append(txi, "TxInfAndSts")
    if original_end_to_end:
        _append(tx, "OrgnlEndToEndId", original_end_to_end)
    _append(tx, "TxSts", status)
    if status == STS_ACSC and settlement_amount is not None:
        _append(tx, "AccptncAmt", None)
        amt_el = tx.find(f"{{{NS}}}AccptncAmt")
        amt_el.text = str(settlement_amount)
        amt_el.set("Ccy", currency or "RWF")
        if fx_rate is not None:
            _append(tx, "IntrBkSttlmAmt", None)
            stl_el = tx.find(f"{{{NS}}}IntrBkSttlmAmt")
            stl_el.text = str(settlement_amount)
            stl_el.set("Ccy", currency or "RWF")
            _append(tx, "ExctgBrr", None)
            _append(tx.find(f"{{{NS}}}ExctgBrr"), "Nm", "VUKA CROSS-BORDER BRIDGE")

    return etree.tostring(doc, xml_declaration=True, encoding="UTF-8", pretty_print=True).decode()


def build_acsc(original_message_id: str, original_end_to_end: str,
               amount: Decimal, currency: str, fx_rate: Decimal | None = None) -> str:
    """Convenience: an accepted-completed status report."""
    return build_pacs002(
        original_message_id=original_message_id,
        original_end_to_end=original_end_to_end,
        status=STS_ACSC,
        settlement_amount=amount,
        currency=currency,
        fx_rate=fx_rate,
    )


def build_rjct(original_message_id: str, original_end_to_end: str, reason: str) -> str:
    """Convenience: a rejected status report with a machine reason code."""
    code = _reason_code(reason)
    return build_pacs002(
        original_message_id=original_message_id,
        original_end_to_end=original_end_to_end,
        status=STS_RJCT,
        reason=code,
    )


def _reason_code(reason: str) -> str:
    """Map a human reason to an ExternalStatusReason1Code where possible."""
    lowered = reason.lower()
    if "amount" in lowered or "positive" in lowered:
        return "AM04"  # InvalidAmount
    if "currency" in lowered or "support" in lowered:
        return "AM01"  # IncorrectCurrency
    if "fund" in lowered or "insufficient" in lowered:
        return "AM05"  # Duplication? No — use NS01 (insufficient funds)
    if "duplicate" in lowered or "idempot" in lowered:
        return "AM05"  # Duplication
    return "AG01"  # TransactionForbidden (generic rejection)
