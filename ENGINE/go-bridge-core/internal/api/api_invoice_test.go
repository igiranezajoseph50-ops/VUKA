package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"vuka/go-bridge-core/internal/ledger"
)

func TestInvoiceAPI_CreateListGet(t *testing.T) {
	srv := newServer(t)

	// Create user A (issuer) and user B (counterparty)
	cuA, _ := setupTrader(t, srv, "IssuerTrader", "+250700000201", "RWF", 0)
	cuB, _ := setupTrader(t, srv, "ReceiverTrader", "+250700000202", "RWF", 0)

	// Create an invoice via HTTP API
	req := ledger.CreateInvoiceRequest{
		Number:             "INV-2026-0201",
		CounterpartyUserID: cuB.User.ID,
		Currency:           "RWF",
		IssueDate:          "2026-08-01",
		DueDate:            "2026-08-15",
		VATRate:            18,
		Items: []ledger.InvoiceItemRequest{
			{Description: "Goods", Quantity: 1, UnitPrice: 1000},
		},
	}

	rec := doJSON(t, srv, "POST", "/api/users/"+cuA.User.ID+"/invoices", nil, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/users/{id}/invoices failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	var created ledger.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created invoice: %v", err)
	}

	if created.Number != req.Number {
		t.Errorf("expected number %q, got %q", req.Number, created.Number)
	}
	if created.IssuerUserID != cuA.User.ID {
		t.Errorf("expected issuer %s, got %s", cuA.User.ID, created.IssuerUserID)
	}

	// Get the invoice by ID via HTTP API
	rec = doJSON(t, srv, "GET", "/api/invoices/"+created.ID, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/invoices/{id} failed: status=%d", rec.Code)
	}

	var retrieved ledger.Invoice
	if err := json.Unmarshal(rec.Body.Bytes(), &retrieved); err != nil {
		t.Fatalf("failed to decode retrieved invoice: %v", err)
	}
	if retrieved.ID != created.ID {
		t.Errorf("invoice ID mismatch: %s vs %s", retrieved.ID, created.ID)
	}

	// List invoices for User A (issued)
	rec = doJSON(t, srv, "GET", "/api/users/"+cuA.User.ID+"/invoices?direction=issued", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/users/{id}/invoices?direction=issued failed: status=%d", rec.Code)
	}

	var listResp struct {
		Invoices []ledger.Invoice `json:"invoices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(listResp.Invoices) != 1 || listResp.Invoices[0].ID != created.ID {
		t.Errorf("expected 1 issued invoice with ID %s, got %d", created.ID, len(listResp.Invoices))
	}

	// List invoices for User B (received)
	rec = doJSON(t, srv, "GET", "/api/users/"+cuB.User.ID+"/invoices?direction=received", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/users/{id}/invoices?direction=received failed: status=%d", rec.Code)
	}

	listResp.Invoices = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(listResp.Invoices) != 1 || listResp.Invoices[0].ID != created.ID {
		t.Errorf("expected 1 received invoice with ID %s, got %d", created.ID, len(listResp.Invoices))
	}
}
