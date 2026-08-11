"""VUKA ISO 20022 gateway (Phase 2).

FastAPI app that accepts inbound pacs.008 credit transfers, validates and
parses them, executes the payment through the Go engine over gRPC, and returns
a pacs.002 status report. Also exposes non-executing translate helpers and an
engine balance proxy for demos and debugging.
"""
from __future__ import annotations

from decimal import Decimal

from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse, PlainTextResponse

from app.grpc_client import EngineError, get_client
from app.iso20022.parser import ParseError, parse_pacs008
from app.iso20022.serializer import STS_ACSC, STS_RJCT, build_acsc, build_pacs002, build_rjct
from app.iso20022.validate import ValidationError, validate_payment

app = FastAPI(title="VUKA ISO 20022 Gateway", version="0.2.0")


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


@app.post("/iso20022/pacs.008")
async def iso20022_pacs008(request: Request) -> Response:
    """Execute an inbound pacs.008 credit transfer; return pacs.002 XML.

    The gateway parses and validates the message, resolves the corridor
    (RWF debtor -> KES creditor), then calls the Go engine's
    CrossBorderTransfer over gRPC. Replaying the same message_id returns the
    original execution (idempotency is enforced by the engine).
    """
    xml = (await request.body()).decode("utf-8")

    # 1. Parse.
    try:
        payment = parse_pacs008(xml)
    except ParseError as exc:
        return JSONResponse(status_code=400, content={"error": "invalid_pacs008", "detail": str(exc)})

    # 2. Validate.
    try:
        validate_payment(payment)
    except ValidationError as exc:
        rjct = build_rjct(payment.message_id, payment.reference, str(exc))
        return PlainTextResponse(content=rjct, media_type="application/xml", status_code=422)

    # 3. Execute via gRPC (idempotency key = message_id).
    try:
        engine = get_client()
        transfer = engine.cross_border_transfer(
            idempotency_key=payment.message_id,
            source_account_id=_resolve_account(engine, payment.debtor_name, payment.currency),
            destination_account_id=_resolve_account(engine, payment.creditor_name, "KES"),
            amount=float(payment.amount),
            currency_from=payment.currency,
            currency_to="KES",
        )
    except EngineError as exc:
        # Unsupported / rejected by the engine: return a pacs.002 RJCT.
        rjct = build_rjct(payment.message_id, payment.reference, f"{exc.code}: {exc.message}")
        return PlainTextResponse(content=rjct, media_type="application/xml", status_code=422)

    amount_kes = payment.amount / Decimal(str(transfer.fx_rate)) if transfer.fx_rate else payment.amount
    acsc = build_acsc(
        original_message_id=payment.message_id,
        original_end_to_end=payment.reference or payment.message_id,
        amount=amount_kes,
        currency="KES",
        fx_rate=Decimal(str(transfer.fx_rate)) if transfer.fx_rate else None,
    )
    return PlainTextResponse(content=acsc, media_type="application/xml", status_code=200)


@app.post("/iso20022/translate")
async def iso20022_translate(request: Request) -> JSONResponse:
    """Non-executing: parse + validate only (debug / integration checks)."""
    xml = (await request.body()).decode("utf-8")
    try:
        payment = parse_pacs008(xml)
        validate_payment(payment)
    except (ParseError, ValidationError) as exc:
        return JSONResponse(status_code=422, content={"error": "invalid", "detail": str(exc)})
    return JSONResponse(content={
        "message_id": payment.message_id,
        "amount": str(payment.amount),
        "currency": payment.currency,
        "debtor": payment.debtor_name,
        "creditor": payment.creditor_name,
        "reference": payment.reference,
    })


@app.post("/iso20022/from-json")
async def iso20022_from_json(request: Request) -> PlainTextResponse:
    """Non-executing: build a pacs.002 ACSC from a JSON summary (demo aid)."""
    body = await request.json()
    return PlainTextResponse(
        content=build_pacs002(
            original_message_id=body.get("message_id", ""),
            original_end_to_end=body.get("reference", ""),
            status=body.get("status", STS_ACSC),
            settlement_amount=Decimal(str(body.get("amount", 0))),
            currency=body.get("currency", "KES"),
            fx_rate=Decimal(str(body["fx_rate"])) if body.get("fx_rate") else None,
        ),
        media_type="application/xml",
    )


@app.get("/engine/balance/{account_id}")
def engine_balance(account_id: str) -> JSONResponse:
    """Proxy the Go engine's derived balance over gRPC (demo/debug)."""
    try:
        bal = get_client().get_balance(account_id)
    except EngineError as exc:
        return JSONResponse(status_code=404 if exc.code == "NOT_FOUND" else 502,
                            content={"error": exc.code, "detail": exc.message})
    return JSONResponse(content={
        "account_id": bal.account_id,
        "type": bal.type,
        "currency": bal.currency,
        "amount": bal.amount,
    })


def _resolve_account(engine, name: str, currency: str) -> str:
    """Resolve a debtor/creditor name to a VUKA BUSINESS account id.

    Prototype resolution: first match by name among accounts the engine knows
    via the balance proxy is not possible (the engine has no account-search
    RPC), so the gateway uses a lookup table seeded from the environment plus
    a deterministic fallback that maps names to accounts created at startup.
    """
    from app import registry
    return registry.resolve(name, currency)
