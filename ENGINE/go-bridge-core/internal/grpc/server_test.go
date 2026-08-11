package grpc_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	bridgegrpc "vuka/go-bridge-core/internal/grpc"
	"vuka/go-bridge-core/internal/grpcpb"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

// newGRPCServer spins a real in-process gRPC server over the ledger engine.
func newGRPCServer(t *testing.T) (grpcpb.BridgeClient, *ledger.Service) {
	t.Helper()
	pool := testutil.TestPool(t)
	idem := idempotency.NewStore(pool)
	svc := ledger.NewService(pool, idem, testutil.CorridorRegistry())
	svc.SetDefaultFxRate(9.5)
	if err := svc.EnsureSettlementAccounts(context.Background(), "RWF", "KES"); err != nil {
		t.Fatalf("EnsureSettlementAccounts: %v", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	grpcpb.RegisterBridgeServer(gs, bridgegrpc.NewServer(svc))
	go gs.Serve(lis) //nolint:errcheck
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return grpcpb.NewBridgeClient(conn), svc
}

func TestGRPC_CreateUser_And_GetBalance(t *testing.T) {
	client, _ := newGRPCServer(t)

	u, err := client.CreateUser(context.Background(), &grpcpb.CreateUserRequest{
		FullName: "Amina Uwera", PhoneNumber: "+250****0081",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(u.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(u.Accounts))
	}
	var business *grpcpb.Account
	for _, a := range u.Accounts {
		if a.Type == "BUSINESS" {
			business = a
		}
	}
	if business == nil {
		t.Fatal("no BUSINESS account")
	}

	bal, err := client.GetBalance(context.Background(), &grpcpb.GetBalanceRequest{AccountId: business.Id})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.Amount != 0 {
		t.Errorf("initial balance=%.2f, want 0", bal.Amount)
	}
}

func TestGRPC_CrossBorder_LiveCorridor(t *testing.T) {
	client, _ := newGRPCServer(t)

	// Rwanda importer (RWF) + Kenya supplier (KES).
	rw, err := client.CreateUser(context.Background(), &grpcpb.CreateUserRequest{
		FullName: "RW Importer", PhoneNumber: "+250****0082", Currency: "RWF",
	})
	if err != nil {
		t.Fatalf("CreateUser RWF: %v", err)
	}
	ke, err := client.CreateUser(context.Background(), &grpcpb.CreateUserRequest{
		FullName: "KE Supplier", PhoneNumber: "+254****0083", Currency: "KES",
	})
	if err != nil {
		t.Fatalf("CreateUser KES: %v", err)
	}
	rwBiz := findBusiness(t, rw.Accounts)
	keBiz := findBusiness(t, ke.Accounts)

	if _, err := client.FundAccount(context.Background(), &grpcpb.FundAccountRequest{
		AccountId: rwBiz.Id, IdempotencyKey: "33333333-3333-4333-8333-333333333301",
		Amount: 190000, Currency: "RWF", Reference: "MOMO-IN-1",
	}); err != nil {
		t.Fatalf("FundAccount: %v", err)
	}

	// Cross-border RWF -> KES.
	tf, err := client.CrossBorderTransfer(context.Background(), &grpcpb.CrossBorderRequest{
		IdempotencyKey:       "33333333-3333-4333-8333-333333333302",
		SourceAccountId:      rwBiz.Id,
		DestinationAccountId: keBiz.Id,
		Amount:               95000,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
		InvoiceNumber:        "INV-GRPC-1",
	})
	if err != nil {
		t.Fatalf("CrossBorderTransfer: %v", err)
	}
	if tf.Status != "SUCCESS" {
		t.Errorf("status=%s, want SUCCESS", tf.Status)
	}
	if tf.FxRate != 9.5 {
		t.Errorf("fx_rate=%.2f, want 9.5", tf.FxRate)
	}
	if tf.ExternalReference == "" {
		t.Error("expected external reference")
	}

	// Replay the SAME key: same transfer id, no double charge.
	tf2, err := client.CrossBorderTransfer(context.Background(), &grpcpb.CrossBorderRequest{
		IdempotencyKey:       "33333333-3333-4333-8333-333333333302",
		SourceAccountId:      rwBiz.Id,
		DestinationAccountId: keBiz.Id,
		Amount:               95000,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
		InvoiceNumber:        "INV-GRPC-1",
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if tf2.Id != tf.Id {
		t.Errorf("replay returned different transfer %s vs %s", tf2.Id, tf.Id)
	}
	if !tf2.Replayed {
		t.Error("expected Replayed=true on second call")
	}

	keBal, err := client.GetBalance(context.Background(), &grpcpb.GetBalanceRequest{AccountId: keBiz.Id})
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if keBal.Amount != 10000 { // 95000 / 9.5
		t.Errorf("KES balance=%.2f, want 10000 (no double charge)", keBal.Amount)
	}
}

func TestGRPC_ErrorMapping(t *testing.T) {
	client, _ := newGRPCServer(t)

	// Unknown account -> NotFound.
	_, err := client.GetBalance(context.Background(), &grpcpb.GetBalanceRequest{
		AccountId: "00000000-0000-4000-8000-000000000000",
	})
	if status.Code(err) != codes.NotFound {
		t.Errorf("expected NotFound, got %v", status.Code(err))
	}

	// Same-account transfer -> FailedPrecondition.
	rw, _ := client.CreateUser(context.Background(), &grpcpb.CreateUserRequest{
		FullName: "T", PhoneNumber: "+250****0084", Currency: "RWF",
	})
	biz := findBusiness(t, rw.Accounts)
	_, err = client.Transfer(context.Background(), &grpcpb.TransferRequest{
		IdempotencyKey:       "44444444-4444-4444-8444-444444444401",
		SourceAccountId:      biz.Id,
		DestinationAccountId: biz.Id,
		Amount:               100,
		Currency:             "RWF",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", status.Code(err))
	}

	// Missing idempotency key -> InvalidArgument.
	_, err = client.Transfer(context.Background(), &grpcpb.TransferRequest{
		SourceAccountId:      biz.Id,
		DestinationAccountId: "00000000-0000-4000-8000-000000000001",
		Amount:               100,
		Currency:             "RWF",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func findBusiness(t *testing.T, accounts []*grpcpb.Account) *grpcpb.Account {
	t.Helper()
	for _, a := range accounts {
		if a.Type == "BUSINESS" {
			return a
		}
	}
	t.Fatal("no BUSINESS account")
	return nil
}
