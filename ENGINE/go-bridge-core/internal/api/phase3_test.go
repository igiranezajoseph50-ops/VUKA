package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

// ---------------------------------------------------------------------------
// CORS

func TestCORS_Preflight_AllowedOrigin(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/transfers", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status=%d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin=%q, want http://localhost:5173", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Allow-Methods=%q, want POST", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Idempotency-Key") {
		t.Errorf("Allow-Headers=%q, want Idempotency-Key", got)
	}
}

func TestCORS_Preflight_DisallowedOrigin(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodOptions, "/api/transfers", nil)
	req.Header.Set("Origin", "http://evil.example")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("preflight status=%d, want 403", rr.Code)
	}
}

func TestCORS_ActualRequest_GetsAllowHeader(t *testing.T) {
	srv := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin=%q, want http://localhost:5173", got)
	}
}

// ---------------------------------------------------------------------------
// Transfer history

func TestListUserTransfers_NewestFirst_AndFiltered(t *testing.T) {
	srv := newServer(t)

	// Trader A (RWF business) sends two transfers to trader B.
	cu1, b1 := setupTrader(t, srv, "HistSrc", "+250****0201", "RWF", 500000)
	cu2, b2 := setupTrader(t, srv, "HistDst", "+250****0202", "RWF", 0)

	tf1 := createTransferReq(t, srv, b1, b2, 1000, "INV-H-1")
	time.Sleep(5 * time.Millisecond) // ensure created_at ordering
	tf2 := createTransferReq(t, srv, b1, b2, 2000, "INV-H-2")
	_ = tf1
	_ = tf2

	// Full history for u1: funding + 2 transfers, newest first.
	// NOTE: setupTrader funds 500000 via a transfer row, so u1 sees 3.
	rec := doJSON(t, srv, "GET", "/api/users/"+cu1.User.ID+"/transfers", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Transfers []ledger.Transfer `json:"transfers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Transfers) != 3 {
		t.Fatalf("expected 3 transfers (1 fund + 2 sends), got %d", len(body.Transfers))
	}
	if body.Transfers[0].Amount != 2000 { // newest first
		t.Errorf("first transfer amount=%.0f, want 2000", body.Transfers[0].Amount)
	}

	// Filter by status: SUCCESS only -> all 3.
	rec = doJSON(t, srv, "GET", "/api/users/"+cu1.User.ID+"/transfers?status=SUCCESS", nil, nil)
	body.Transfers = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Transfers) != 3 {
		t.Errorf("expected 3 SUCCESS transfers, got %d", len(body.Transfers))
	}

	// Filter by account: transfers touching b1 only -> 3.
	rec = doJSON(t, srv, "GET", "/api/users/"+cu1.User.ID+"/transfers?account_id="+b1, nil, nil)
	body.Transfers = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Transfers) != 3 {
		t.Errorf("expected 3 transfers for account filter, got %d", len(body.Transfers))
	}

	// Trader B's view: only the 2 incoming sends (no fund on B).
	rec = doJSON(t, srv, "GET", "/api/users/"+cu2.User.ID+"/transfers", nil, nil)
	body.Transfers = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Transfers) != 2 {
		t.Errorf("expected 2 transfers in dst view, got %d", len(body.Transfers))
	}
}

// ---------------------------------------------------------------------------
// Trader lookup by phone (demo picker re-entry)

func TestGetUserByPhone_ResolvesExistingTrader(t *testing.T) {
	srv := newServer(t)
	cu, _ := setupTrader(t, srv, "PhoneLookup", "+250****0301", "RWF", 0)

	rec := doJSON(t, srv, "GET", "/api/lookup/user/+250****0301", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("by-phone status=%d body=%s", rec.Code, rec.Body.String())
	}
	var u ledger.User
	if err := json.Unmarshal(rec.Body.Bytes(), &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.ID != cu.User.ID {
		t.Errorf("resolved user id=%s, want %s", u.ID, cu.User.ID)
	}
}

func TestGetUserByPhone_UnknownPhone_404(t *testing.T) {
	srv := newServer(t)
	rec := doJSON(t, srv, "GET", "/api/lookup/user/+250****9999", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("by-phone unknown status=%d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// SSE

func TestSSE_ReceivesTransferEvent(t *testing.T) {
	srv := newServer(t)
	cu1, b1 := setupTrader(t, srv, "SseSrc", "+250****0211", "RWF", 100000)
	_, b2 := setupTrader(t, srv, "SseDst", "+250****0212", "RWF", 0)

	// Open an SSE stream against the live httptest server.
	ts := httptest.NewServer(srv)
	defer ts.Close()

	events := make(chan map[string]any, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go readSSE(t, ctx, ts.URL+"/api/events", events, done)

	// Give the goroutine a moment to subscribe.
	time.Sleep(150 * time.Millisecond)

	// Trigger a transfer -> should emit event: transfer.
	createTransferReq(t, srv, b1, b2, 500, "INV-SSE-1")

	select {
	case ev := <-events:
		if ev["status"] != "SUCCESS" {
			t.Errorf("event status=%v, want SUCCESS", ev["status"])
		}
		if ev["currency"] != "RWF" {
			t.Errorf("event currency=%v, want RWF", ev["currency"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for SSE transfer event")
	}
	close(done)
	_ = cu1
}

// readSSE streams from the events endpoint and forwards parsed frames.
func readSSE(t *testing.T, ctx context.Context, url string, events chan map[string]any, done <-chan struct{}) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for scanner.Scan() {
		select {
		case <-done:
			return
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			continue
		}
		select {
		case events <- ev:
		case <-done:
			return
		}
	}
}

// createTransferReq performs a same-currency transfer via the API.
func createTransferReq(t *testing.T, srv http.Handler, src, dst string, amount float64, invoice string) ledger.Transfer {
	t.Helper()
	rec := doJSON(t, srv, "POST", "/api/transfers", map[string]string{
		"Idempotency-Key": testutil.UniqueID(),
	}, ledger.TransferRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               amount,
		Currency:             "RWF",
		InvoiceNumber:        invoice,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Transfer ledger.Transfer `json:"transfer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Transfer
}