// gRPC bridge server (Phase 2).
//
// Exposes the Go ledger engine to the Python ISO 20022 gateway over gRPC.
// Every RPC maps 1:1 to a ledger.Service method; the engine remains the
// single authority for money movement. Errors are translated to canonical
// gRPC codes so the Python client can react without string matching:
//
//	InvalidArgument   — malformed request, bad amount, bad currency
//	NotFound          — unknown account / user / transfer
//	FailedPrecondition — currency mismatch, invalid account type, same account
//	InsufficientFunds — derived balance check failed
//	AlreadyExists     — idempotency key replay (client should treat as success)
//	Unavailable       — telecom adapter missing
package grpc

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"vuka/go-bridge-core/internal/grpcpb"
	"vuka/go-bridge-core/internal/ledger"
)

// Server implements grpcpb.BridgeServer by delegating to the ledger Service.
type Server struct {
	grpcpb.UnimplementedBridgeServer
	ledger *ledger.Service
}

// NewServer builds the gRPC server around the ledger engine.
func NewServer(svc *ledger.Service) *Server {
	return &Server{ledger: svc}
}

// CreateUser registers a trader and their PERSONAL + BUSINESS wallets.
func (s *Server) CreateUser(ctx context.Context, req *grpcpb.CreateUserRequest) (*grpcpb.CreateUserResponse, error) {
	currency := req.GetCurrency()
	if currency == "" {
		currency = "RWF"
	}
	user, accounts, err := s.ledger.CreateUser(ctx, ledger.CreateUserRequest{
		FullName:          req.GetFullName(),
		PhoneNumber:       req.GetPhoneNumber(),
		BusinessRegNumber: req.GetBusinessRegNumber(),
	}, currency)
	if err != nil {
		return nil, mapError(err)
	}
	resp := &grpcpb.CreateUserResponse{
		User: &grpcpb.User{
			Id:                user.ID,
			FullName:          user.FullName,
			PhoneNumber:       user.PhoneNumber,
			BusinessRegNumber: deref(user.BusinessRegNumber),
			KycStatus:         user.KYCStatus,
			CreatedAt:         user.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		},
	}
	for _, a := range accounts {
		resp.Accounts = append(resp.Accounts, accountProto(a))
	}
	return resp, nil
}

// GetBalance returns the derived balance for an account.
func (s *Server) GetBalance(ctx context.Context, req *grpcpb.GetBalanceRequest) (*grpcpb.GetBalanceResponse, error) {
	b, err := s.ledger.GetBalance(ctx, req.GetAccountId())
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcpb.GetBalanceResponse{
		AccountId: b.AccountID,
		Type:      string(b.Type),
		Currency:  b.Currency,
		Amount:    b.Amount,
	}, nil
}

// FundAccount credits an external cash-in to an account (idempotent).
func (s *Server) FundAccount(ctx context.Context, req *grpcpb.FundAccountRequest) (*grpcpb.TransferResponse, error) {
	t, err := s.ledger.FundAccount(ctx, req.GetAccountId(), req.GetIdempotencyKey(), ledger.FundRequest{
		Amount:    req.GetAmount(),
		Currency:  req.GetCurrency(),
		Reference: req.GetReference(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return transferProto(t, false), nil
}

// Transfer moves value between two SAME-currency accounts (idempotent).
func (s *Server) Transfer(ctx context.Context, req *grpcpb.TransferRequest) (*grpcpb.TransferResponse, error) {
	t, replayed, err := s.ledger.Transfer(ctx, req.GetIdempotencyKey(), ledger.TransferRequest{
		SourceAccountID:      req.GetSourceAccountId(),
		DestinationAccountID: req.GetDestinationAccountId(),
		Amount:               req.GetAmount(),
		Currency:             req.GetCurrency(),
		InvoiceNumber:        req.GetInvoiceNumber(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return transferProto(t, replayed), nil
}

// CrossBorderTransfer moves value RWF -> KES through FX settlement legs.
func (s *Server) CrossBorderTransfer(ctx context.Context, req *grpcpb.CrossBorderRequest) (*grpcpb.TransferResponse, error) {
	from := req.GetCurrencyFrom()
	if from == "" {
		from = "RWF"
	}
	to := req.GetCurrencyTo()
	if to == "" {
		to = "KES"
	}
	t, replayed, err := s.ledger.CrossBorderTransfer(ctx, req.GetIdempotencyKey(), ledger.CrossBorderRequest{
		SourceAccountID:      req.GetSourceAccountId(),
		DestinationAccountID: req.GetDestinationAccountId(),
		Amount:               req.GetAmount(),
		CurrencyFrom:         from,
		CurrencyTo:           to,
		FxRate:               req.GetFxRate(),
		InvoiceNumber:        req.GetInvoiceNumber(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return transferProto(t, replayed), nil
}

// GetTransfer fetches a transfer by id. Returns NOT_FOUND if unknown.
func (s *Server) GetTransfer(ctx context.Context, req *grpcpb.GetTransferRequest) (*grpcpb.TransferResponse, error) {
	t, err := s.ledger.GetTransfer(ctx, req.GetTransferId())
	if err != nil {
		return nil, mapError(err)
	}
	return transferProto(t, false), nil
}

// ReverseTransfer reverses a SUCCESS transfer.
func (s *Server) ReverseTransfer(ctx context.Context, req *grpcpb.ReverseRequest) (*grpcpb.TransferResponse, error) {
	t, err := s.ledger.ReverseTransfer(ctx, req.GetTransferId(), req.GetReason())
	if err != nil {
		return nil, mapError(err)
	}
	return transferProto(t, false), nil
}

// accountProto converts a ledger.Account to its protobuf form.
func accountProto(a *ledger.Account) *grpcpb.Account {
	return &grpcpb.Account{
		Id:        a.ID,
		UserId:    a.UserID,
		Type:      string(a.Type),
		Currency:  a.Currency,
		CreatedAt: a.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// transferProto converts a ledger.Transfer to its protobuf form.
func transferProto(t *ledger.Transfer, replayed bool) *grpcpb.TransferResponse {
	resp := &grpcpb.TransferResponse{
		Id:                  t.ID,
		IdempotencyKey:      t.IdempotencyKey,
		InvoiceNumber:       deref(t.InvoiceNumber),
		SourceAccountId:     t.SourceAccountID,
		DestinationAccountId: t.DestinationAccountID,
		Amount:              t.Amount,
		Currency:            t.Currency,
		Status:              string(t.Status),
		ExternalReference:   deref(t.ExternalReference),
		FailureReason:       deref(t.FailureReason),
		CreatedAt:           t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           t.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Replayed:            replayed,
	}
	if t.FxRate != nil {
		resp.FxRate = *t.FxRate
	}
	return resp
}

// deref flattens an optional string pointer for protobuf ("" == unset).
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// mapError translates ledger sentinels to canonical gRPC status codes.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ledger.ErrAccountNotFound),
		errors.Is(err, ledger.ErrUserNotFound),
		errors.Is(err, ledger.ErrTransferNotFound),
		errors.Is(err, ledger.ErrSettlementAccountNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ledger.ErrInsufficientFunds):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ledger.ErrCurrencyMismatch),
		errors.Is(err, ledger.ErrInvalidAccountType),
		errors.Is(err, ledger.ErrSameAccount):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, ledger.ErrDuplicateKey):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ledger.ErrAdapterUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, ledger.ErrInvalidAmount),
		errors.Is(err, ledger.ErrInvalidFxRate),
		errors.Is(err, ledger.ErrTransferNotSuccess),
		errors.Is(err, ledger.ErrIdempotencyKeyRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, fmt.Sprintf("ledger: %v", err))
	}
}
