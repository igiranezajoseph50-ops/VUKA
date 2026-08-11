"""Validation unit tests + FastAPI gateway tests (against a live engine).

The gateway tests exercise the full path: pacs.008 XML in -> parse -> validate
-> gRPC -> Go ledger -> pacs.002 XML out. They require the Go engine's gRPC
bridge to be reachable at VUKA_ENGINE_GRPC_ADDR (see Makefile gateway-test).
"""
from decimal import Decimal
from pathlib import Path

import pytest

from app.iso20022.parser import parse_pacs008
from app.iso20022.validate import ValidationError, validate_payment

FIXTURE = Path(__file__).parent / "fixtures" / "pacs008_valid.xml"


def _load() -> str:
    return FIXTURE.read_text(encoding="utf-8")


def _load_unique() -> str:
    """Fixture XML with a UNIQUE message_id so replay tests don't collide
    with transfers executed by earlier tests (idempotency key = MsgId)."""
    import time
    mid = f"VUKA-TEST-{int(time.time() * 1000)}"
    return _load().replace("VUKA-2026-08-05-001", mid)


# --- validation -----------------------------------------------------------

def test_valid_payment_passes():
    validate_payment(parse_pacs008(_load()))


def test_rejects_non_positive_amount():
    p = parse_pacs008(_load().replace("95000.00", "0.00", 1))
    with pytest.raises(ValidationError):
        validate_payment(p)


def test_rejects_unsupported_currency():
    p = parse_pacs008(_load().replace('Ccy="RWF"', 'Ccy="USD"', 1))
    with pytest.raises(ValidationError):
        validate_payment(p)


def test_rejects_missing_message_id():
    p = parse_pacs008(_load().replace("VUKA-2026-08-05-001", ""))
    with pytest.raises(ValidationError):
        validate_payment(p)


# --- end-to-end gateway (needs live Go engine) ----------------------------

gateway = pytest.importorskip("fastapi.testclient")


@pytest.fixture
def client():
    from fastapi.testclient import TestClient
    from app import registry
    from app.grpc_client import get_client
    from app.main import app

    registry.clear()
    engine = get_client()

    # Unique phones per run so the fixture is re-runnable against a live DB.
    import time
    suffix = str(int(time.time() * 1000))[-8:]

    # Seed the registry: create a RWF trader + KES trader, fund the RWF one,
    # and map the names from the fixture to their BUSINESS accounts.
    rw = engine.create_user("Amina Uwera", f"+250****{suffix}1", "RWF", "RWC-2026-0441")
    ke = engine.create_user("Kethan Gasana", f"+254****{suffix}2", "KES", "RWC-2026-0772")
    rw_biz = next(a for a in rw.accounts if a.type == "BUSINESS")
    ke_biz = next(a for a in ke.accounts if a.type == "BUSINESS")
    engine.fund_account(rw_biz.id, f"55555555-5555-4555-8555-{suffix}01", 95000, "RWF", "seed")
    registry.register("Amina Uwera", "RWF", rw_biz.id)
    registry.register("Kethan Gasana", "KES", ke_biz.id)

    with TestClient(app) as c:
        yield c, rw_biz.id, ke_biz.id


def test_gateway_executes_pacs008_to_acsc(client):
    c, rw_biz, ke_biz = client
    resp = c.post("/iso20022/pacs.008", content=_load_unique(), headers={"Content-Type": "text/xml"})
    assert resp.status_code == 200, resp.text
    assert "application/xml" in resp.headers["content-type"]
    assert "ACSC" in resp.text

    # The KES supplier balance increased by 95000 / 9.5 = 10000.
    from app.grpc_client import get_client
    bal = get_client().get_balance(ke_biz)
    assert bal.amount == 10000, bal.amount


def test_gateway_replay_is_idempotent(client):
    c, rw_biz, ke_biz = client
    xml = _load_unique()
    first = c.post("/iso20022/pacs.008", content=xml, headers={"Content-Type": "text/xml"})
    second = c.post("/iso20022/pacs.008", content=xml, headers={"Content-Type": "text/xml"})
    assert first.status_code == 200
    assert second.status_code == 200
    # Same message_id -> engine replays -> still exactly 10000 KES.
    from app.grpc_client import get_client
    bal = get_client().get_balance(ke_biz)
    assert bal.amount == 10000, bal.amount


def test_gateway_rejects_unsupported_currency(client):
    c, _, _ = client
    xml = _load().replace('Ccy="RWF"', 'Ccy="UGX"', 1)
    resp = c.post("/iso20022/pacs.008", content=xml, headers={"Content-Type": "text/xml"})
    assert resp.status_code == 422
    assert "RJCT" in resp.text


def test_gateway_translate_is_non_executing(client):
    c, rw_biz, ke_biz = client
    resp = c.post("/iso20022/translate", content=_load(), headers={"Content-Type": "text/xml"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["debtor"] == "Amina Uwera"
    assert body["amount"] == "95000.00"