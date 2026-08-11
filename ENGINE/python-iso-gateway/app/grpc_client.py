"""Thin gRPC client over the Go ledger engine.

The gateway never mutates state itself; every ISO 20022 payment is executed
by calling the Go engine over gRPC. One instance is created at import time
using the configured engine address.
"""
from __future__ import annotations

import grpc

from app.config import config

import vuka_pb2 as pb
import vuka_pb2_grpc as pb_grpc


class EngineError(Exception):
    """Raised when the Go engine rejects an operation.

    code is the canonical gRPC status code name (e.g. FAILED_PRECONDITION).
    """

    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message


class EngineClient:
    """A stateful gRPC channel to the Go bridge engine."""

    def __init__(self, target: str | None = None, timeout: float | None = None) -> None:
        self._target = target or config.engine_grpc_addr
        self._timeout = timeout or config.grpc_timeout_s
        self._channel = grpc.insecure_channel(self._target)
        self._stub = pb_grpc.BridgeStub(self._channel)

    def close(self) -> None:
        self._channel.close()

    def _call(self, method, request):
        try:
            return method(request, timeout=self._timeout)
        except grpc.RpcError as exc:  # noqa: PERF203
            status = exc.code()
            raise EngineError(status.name, exc.details() or status.value) from exc

    # -- operations --------------------------------------------------------

    def create_user(self, full_name: str, phone_number: str, currency: str = "RWF",
                    business_reg_number: str = "") -> pb.CreateUserResponse:
        return self._call(self._stub.CreateUser, pb.CreateUserRequest(
            full_name=full_name,
            phone_number=phone_number,
            currency=currency,
            business_reg_number=business_reg_number,
        ))

    def get_balance(self, account_id: str) -> pb.GetBalanceResponse:
        return self._call(self._stub.GetBalance, pb.GetBalanceRequest(account_id=account_id))

    def fund_account(self, account_id: str, idempotency_key: str, amount: float,
                     currency: str, reference: str = "") -> pb.TransferResponse:
        return self._call(self._stub.FundAccount, pb.FundAccountRequest(
            account_id=account_id,
            idempotency_key=idempotency_key,
            amount=amount,
            currency=currency,
            reference=reference,
        ))

    def cross_border_transfer(
        self,
        idempotency_key: str,
        source_account_id: str,
        destination_account_id: str,
        amount: float,
        currency_from: str = "RWF",
        currency_to: str = "KES",
        fx_rate: float | None = None,
    ) -> pb.TransferResponse:
        return self._call(self._stub.CrossBorderTransfer, pb.CrossBorderRequest(
            idempotency_key=idempotency_key,
            source_account_id=source_account_id,
            destination_account_id=destination_account_id,
            amount=amount,
            currency_from=currency_from,
            currency_to=currency_to,
            fx_rate=fx_rate or 0,
        ))

    def get_transfer(self, transfer_id: str) -> pb.TransferResponse:
        return self._call(self._stub.GetTransfer, pb.GetTransferRequest(transfer_id=transfer_id))


# Process-wide singleton so tests and the app share one channel.
_client: EngineClient | None = None


def get_client() -> EngineClient:
    global _client
    if _client is None:
        _client = EngineClient()
    return _client


def reset_client() -> None:
    """Drop the singleton (used by tests to point at a fresh server)."""
    global _client
    if _client is not None:
        _client.close()
    _client = None