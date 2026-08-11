package ledger_test

import (
	"context"
	"testing"

	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

func TestCreateAndGetInvoice(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()

	// Register traders
	issuer, _, _ := createTrader(t, s, "Issuer User", "+250700000101")
	counterparty, _, _ := createTrader(t, s, "Counterparty User", "+250700000102")

	req := ledger.CreateInvoiceRequest{
		Number:             "INV-2026-0101",
		CounterpartyUserID: counterparty.ID,
		Currency:           "RWF",
		IssueDate:          "2026-08-01",
		DueDate:            "2026-08-15",
		VATRate:            18.0,
		Terms:              "Net 15",
		Notes:              "Thank you for your business",
		Items: []ledger.InvoiceItemRequest{
			{Description: "Irish Potatoes (Tons)", Quantity: 2.5, UnitPrice: 300000},
			{Description: "Logistics Delivery", Quantity: 1.0, UnitPrice: 50000},
		},
	}

	inv, err := s.CreateInvoice(ctx, issuer.ID, req)
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	if inv.Number != req.Number {
		t.Errorf("expected number %q, got %q", req.Number, inv.Number)
	}
	if inv.IssuerUserID != issuer.ID {
		t.Errorf("expected issuer %s, got %s", issuer.ID, inv.IssuerUserID)
	}
	if inv.Status != "ISSUED" {
		t.Errorf("expected initial status to be ISSUED, got %s", inv.Status)
	}
	if len(inv.Items) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(inv.Items))
	}

	// Verify retrieval by ID
	retrieved, err := s.GetInvoice(ctx, inv.ID)
	if err != nil {
		t.Fatalf("GetInvoice failed: %v", err)
	}
	if retrieved.ID != inv.ID {
		t.Errorf("retrieved ID mismatch: %s vs %s", retrieved.ID, inv.ID)
	}

	// Verify retrieval by Number
	retrievedByNum, err := s.GetInvoiceByNumber(ctx, inv.Number)
	if err != nil {
		t.Fatalf("GetInvoiceByNumber failed: %v", err)
	}
	if retrievedByNum.ID != inv.ID {
		t.Errorf("retrieved by number ID mismatch: %s vs %s", retrievedByNum.ID, inv.ID)
	}
}

func TestCreateInvoice_Validation(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()

	issuer, _, _ := createTrader(t, s, "Issuer User", "+250700000103")
	counterparty, _, _ := createTrader(t, s, "Counterparty User", "+250700000104")

	tests := []struct {
		name      string
		req       ledger.CreateInvoiceRequest
		expectErr string
	}{
		{
			name: "missing number",
			req: ledger.CreateInvoiceRequest{
				CounterpartyUserID: counterparty.ID,
				Currency:           "RWF",
				IssueDate:          "2026-08-01",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 1, UnitPrice: 100}},
			},
			expectErr: "invoice number is required",
		},
		{
			name: "bad invoice number format",
			req: ledger.CreateInvoiceRequest{
				Number:             "INVOICE-001",
				CounterpartyUserID: counterparty.ID,
				Currency:           "RWF",
				IssueDate:          "2026-08-01",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 1, UnitPrice: 100}},
			},
			expectErr: "must match INV-YYYY-NNN",
		},
		{
			name: "missing counterparty",
			req: ledger.CreateInvoiceRequest{
				Number:    "INV-2026-0102",
				Currency:  "RWF",
				IssueDate: "2026-08-01",
				DueDate:   "2026-08-15",
				Items:     []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 1, UnitPrice: 100}},
			},
			expectErr: "counterparty_user_id is required",
		},
		{
			name: "invalid currency",
			req: ledger.CreateInvoiceRequest{
				Number:             "INV-2026-0103",
				CounterpartyUserID: counterparty.ID,
				Currency:           "INVALID",
				IssueDate:          "2026-08-01",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 1, UnitPrice: 100}},
			},
			expectErr: "invalid currency",
		},
		{
			name: "invalid date format",
			req: ledger.CreateInvoiceRequest{
				Number:             "INV-2026-0104",
				CounterpartyUserID: counterparty.ID,
				Currency:           "RWF",
				IssueDate:          "08-01-2026",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 1, UnitPrice: 100}},
			},
			expectErr: "invoice dates must be YYYY-MM-DD",
		},
		{
			name: "empty items",
			req: ledger.CreateInvoiceRequest{
				Number:             "INV-2026-0105",
				CounterpartyUserID: counterparty.ID,
				Currency:           "RWF",
				IssueDate:          "2026-08-01",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{},
			},
			expectErr: "invoice requires at least one line item",
		},
		{
			name: "zero quantity item",
			req: ledger.CreateInvoiceRequest{
				Number:             "INV-2026-0106",
				CounterpartyUserID: counterparty.ID,
				Currency:           "RWF",
				IssueDate:          "2026-08-01",
				DueDate:            "2026-08-15",
				Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 0, UnitPrice: 100}},
			},
			expectErr: "line item quantity must be > 0 and unit_price >= 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateInvoice(ctx, issuer.ID, tc.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tc.expectErr != "" && !contains(err.Error(), tc.expectErr) {
				t.Errorf("expected error containing %q, got %q", tc.expectErr, err.Error())
			}
		})
	}
}

func TestListInvoices(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()

	traderA, _, _ := createTrader(t, s, "Trader A", "+250700000105")
	traderB, _, _ := createTrader(t, s, "Trader B", "+250700000106")

	// Create invoice from A -> B
	_, err := s.CreateInvoice(ctx, traderA.ID, ledger.CreateInvoiceRequest{
		Number:             "INV-2026-0111",
		CounterpartyUserID: traderB.ID,
		Currency:           "RWF",
		IssueDate:          "2026-08-01",
		DueDate:            "2026-08-15",
		Items:              []ledger.InvoiceItemRequest{{Description: "Item A", Quantity: 1, UnitPrice: 100}},
	})
	if err != nil {
		t.Fatalf("failed to create invoice 1: %v", err)
	}

	// Create invoice from B -> A
	_, err = s.CreateInvoice(ctx, traderB.ID, ledger.CreateInvoiceRequest{
		Number:             "INV-2026-0112",
		CounterpartyUserID: traderA.ID,
		Currency:           "RWF",
		IssueDate:          "2026-08-02",
		DueDate:            "2026-08-16",
		Items:              []ledger.InvoiceItemRequest{{Description: "Item B", Quantity: 1, UnitPrice: 200}},
	})
	if err != nil {
		t.Fatalf("failed to create invoice 2: %v", err)
	}

	// List all invoices involving Trader A
	allA, err := s.ListInvoices(ctx, traderA.ID, "")
	if err != nil {
		t.Fatalf("ListInvoices all failed: %v", err)
	}
	if len(allA) != 2 {
		t.Errorf("expected 2 invoices for A, got %d", len(allA))
	}

	// List only issued by Trader A
	issuedA, err := s.ListInvoices(ctx, traderA.ID, "issued")
	if err != nil {
		t.Fatalf("ListInvoices issued failed: %v", err)
	}
	if len(issuedA) != 1 || issuedA[0].Number != "INV-2026-0111" {
		t.Errorf("expected 1 issued invoice (INV-2026-0111), got %d", len(issuedA))
	}

	// List only received by Trader A
	receivedA, err := s.ListInvoices(ctx, traderA.ID, "received")
	if err != nil {
		t.Fatalf("ListInvoices received failed: %v", err)
	}
	if len(receivedA) != 1 || receivedA[0].Number != "INV-2026-0112" {
		t.Errorf("expected 1 received invoice (INV-2026-0112), got %d", len(receivedA))
	}
}

func TestInvoiceStatusTransitionsToPaid(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()

	// Setup Traders and Business accounts
	userA, _, bizA := createTrader(t, s, "Trader A", "+250700000107")
	userB, _, bizB := createTrader(t, s, "Trader B", "+250700000108")

	fund(t, s, bizB.ID, 10000)

	// Create invoice from A -> B (B owes A)
	inv, err := s.CreateInvoice(ctx, userA.ID, ledger.CreateInvoiceRequest{
		Number:             "INV-2026-0121",
		CounterpartyUserID: userB.ID,
		Currency:           "RWF",
		IssueDate:          "2026-08-01",
		DueDate:            "2026-08-15",
		Items:              []ledger.InvoiceItemRequest{{Description: "Item", Quantity: 10, UnitPrice: 500}},
	})
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}

	// Initially ISSUED
	retrieved, _ := s.GetInvoice(ctx, inv.ID)
	if retrieved.Status != "ISSUED" {
		t.Errorf("expected status ISSUED, got %s", retrieved.Status)
	}

	// Make transfer from B to A referencing the invoice number
	_, _, err = s.Transfer(ctx, testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      bizB.ID,
		DestinationAccountID: bizA.ID,
		Amount:               5000,
		Currency:             "RWF",
		InvoiceNumber:        inv.Number,
	})
	if err != nil {
		t.Fatalf("Transfer failed: %v", err)
	}

	// Check status now -> PAID
	retrieved, _ = s.GetInvoice(ctx, inv.ID)
	if retrieved.Status != "PAID" {
		t.Errorf("expected status PAID after successful transfer, got %s", retrieved.Status)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (s != "" && substr != "" && (len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))))
}
