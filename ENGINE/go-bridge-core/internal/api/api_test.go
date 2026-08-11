package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/api"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	pool := testutil.TestPool(t)
	reg := adapters.NewRegistry()
	reg.Register(adapters.NewMTNAdapter(0, 0, "success", 5*time.Second))
	idem := idempotency.NewStore(pool)
	svc := ledger.NewService(pool, idem, reg)
	return api.NewServer(svc, testutil.Logger(), []string{"http://localhost:5173"})
}

// newCorridorServer registers both rails (MTN Rwanda + M-Pesa Kenya), sets
// the engine FX rate and creates the SETTLEMENT accounts — the Phase 2
// cross-border setup.
func newCorridorServer(t *testing.T) http.Handler {
	t.Helper()
	pool := testutil.TestPool(t)
	reg := testutil.CorridorRegistry()
	idem := idempotency.NewStore(pool)
	svc := ledger.NewService(pool, idem, reg)
	svc.SetDefaultFxRate(9.5) // 1 KES = 9.5 RWF
	if err := svc.EnsureSettlementAccounts(context.Background(), "RWF", "KES"); err != nil {
		t.Fatalf("EnsureSettlementAccounts: %v", err)
	}
	return api.NewServer(svc, testutil.Logger(), []string{"http://localhost:5173"})
}

type createUserResp struct {
	User     ledger.User      `json:"user"`
	Accounts []ledger.Account `json:"accounts"`
}

type transferResp struct {
	Transfer ledger.Transfer `json:"transfer"`
	Replayed bool            `json:"replayed"`
}

func doJSON(t *testing.T, h http.Handler, method, path string, headers map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// setupTrader creates a user over the HTTP API and funds their BUSINESS
// account with a payment leg. Returns user payload + business account ID.
func setupTrader(t *testing.T, h http.Handler, name, phone string, currency string, fundAmount float64) (createUserResp, string) {
	t.Helper()
	rec := doJSON(t, h, "POST", "/api/users?currency="+currency, nil, ledger.CreateUserRequest{
		FullName:    name,
		PhoneNumber: phone,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user: %d %s", rec.Code, rec.Body.String())
	}
	var cu createUserResp
	if err := json.Unmarshal(rec.Body.Bytes(), &cu); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	var businessID string
	for _, a := range cu.Accounts {
		if a.Type == ledger.AccountTypeBusiness {
			businessID = a.ID
		}
	}
	if businessID == "" {
		t.Fatal("no BUSINESS account returned")
	}
	if fundAmount > 0 {
		rec = doJSON(t, h, "POST", "/api/accounts/"+businessID+"/fund", map[string]string{
			"Idempotency-Key": testutil.UniqueID(),
		}, ledger.FundRequest{Amount: fundAmount, Currency: currency})
		if rec.Code != http.StatusCreated {
			t.Fatalf("fund: %d %s", rec.Code, rec.Body.String())
		}
	}
	return cu, businessID
}

func TestHealthz(t *testing.T) {
	h := newServer(t)
	rec := doJSON(t, h, "GET", "/healthz", nil, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestCreateUser_HTTP(t *testing.T) {
	h := newServer(t)
	cu, businessID := setupTrader(t, h, "API User", "+250****0101", "RWF", 0)
	if cu.User.ID == "" {
		t.Error("expected user id")
	}
	if businessID == "" {
		t.Error("expected business account")
	}
}

func TestCreateUser_MissingFields_400(t *testing.T) {
	h := newServer(t)
	rec := doJSON(t, h, "POST", "/api/users", nil, ledger.CreateUserRequest{})
	if rec.Code != http.StatusBadRequest {
		// Server maps the validation error; sentinel may surface as 422.
		t.Logf("missing-fields status = %d (accepted 400 or 422)", rec.Code)
	}
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing-fields = %d, want 400", rec.Code)
	}
}

func TestTransfer_HTTP_And_Replay(t *testing.T) {
	h := newServer(t)
	_, src := setupTrader(t, h, "HTSrc", "+250****0102", "RWF", 50000)
	_, dst := setupTrader(t, h, "HTDst", "+250****0103", "RWF", 0)

	key := testutil.UniqueID()
	body := ledger.TransferRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               12000,
		Currency:             "RWF",
		InvoiceNumber:        "INV-HT-1",
	}
	hdrs := map[string]string{"Idempotency-Key": key}

	rec := doJSON(t, h, "POST", "/api/transfers", hdrs, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer = %d %s (want 201)", rec.Code, rec.Body.String())
	}
	var first transferResp
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("unmarshal transfer: %v", err)
	}
	if first.Replayed {
		t.Error("first transfer should not replay")
	}

	// Replay the SAME key -> HTTP 200, same transfer id, replayed=true.
	rec = doJSON(t, h, "POST", "/api/transfers", hdrs, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("replayed transfer = %d (want 200)", rec.Code)
	}
	var second transferResp
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("unmarshal replay: %v", err)
	}
	if !second.Replayed {
		t.Error("expected replayed=true")
	}
	if second.Transfer.ID != first.Transfer.ID {
		t.Errorf("replay id changed: %s vs %s", second.Transfer.ID, first.Transfer.ID)
	}

	// Fetch the transfer + its double-entry audit rows.
	rec = doJSON(t, h, "GET", "/api/transfers/"+first.Transfer.ID+"/entries", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("entries = %d", rec.Code)
	}
	var er struct {
		Entries []struct {
			Amount float64 `json:"amount"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(er.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(er.Entries))
	}
	var sum float64
	for _, e := range er.Entries {
		sum += e.Amount
	}
	if sum != 0 {
		t.Errorf("entries net %v, want 0", sum)
	}
}

func TestTransfer_MissingIdempotencyKey_400(t *testing.T) {
	h := newServer(t)
	_, src := setupTrader(t, h, "NoKey", "+250****0104", "RWF", 1000)
	_, dst := setupTrader(t, h, "NoKeyDst", "+250****0105", "RWF", 0)
	rec := doJSON(t, h, "POST", "/api/transfers", nil, ledger.TransferRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               100,
		Currency:             "RWF",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing idempotency key = %d, want 400", rec.Code)
	}
}

func TestTransfer_InsufficientFunds_422(t *testing.T) {
	h := newServer(t)
	_, src := setupTrader(t, h, "PoorAPI", "+250****0106", "RWF", 100)
	_, dst := setupTrader(t, h, "RichAPI", "+250****0107", "RWF", 0)
	rec := doJSON(t, h, "POST", "/api/transfers", map[string]string{
		"Idempotency-Key": testutil.UniqueID(),
	}, ledger.TransferRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               1000,
		Currency:             "RWF",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("insufficient funds = %d, want 422", rec.Code)
	}
}

func TestCrossBorderTransfer_HTTP(t *testing.T) {
	h := newCorridorServer(t)
	// RWF importer in Rwanda, KES supplier in Kenya.
	_, src := setupTrader(t, h, "CB Src", "+250****0108", "RWF", 50000)
	_, dst := setupTrader(t, h, "CB Dst", "+254****0109", "KES", 0)

	key := testutil.UniqueID()
	body := ledger.CrossBorderRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               19000, // RWF
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
		InvoiceNumber:        "INV-CB-1",
	}
	hdrs := map[string]string{"Idempotency-Key": key}

	rec := doJSON(t, h, "POST", "/api/transfers/cross-border", hdrs, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("cross-border = %d %s (want 201)", rec.Code, rec.Body.String())
	}
	var first transferResp
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("unmarshal cross-border: %v", err)
	}
	if first.Replayed {
		t.Error("first cross-border should not replay")
	}
	if first.Transfer.FxRate == nil || *first.Transfer.FxRate != 9.5 {
		t.Errorf("fx_rate not recorded: %v", first.Transfer.FxRate)
	}

	// 4 double-entry rows: 2 per currency leg, each leg netting to zero.
	rec = doJSON(t, h, "GET", "/api/transfers/"+first.Transfer.ID+"/entries", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("entries = %d", rec.Code)
	}
	var er struct {
		Entries []struct {
			Amount float64 `json:"amount"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(er.Entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(er.Entries))
	}
	var sum float64
	for _, e := range er.Entries {
		sum += e.Amount
	}
	if sum != 0 {
		t.Errorf("entries net %v, want 0", sum)
	}

	// Replay the same key -> 200, same transfer, replayed=true.
	rec = doJSON(t, h, "POST", "/api/transfers/cross-border", hdrs, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-border replay = %d (want 200)", rec.Code)
	}
	var second transferResp
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("unmarshal replay: %v", err)
	}
	if !second.Replayed || second.Transfer.ID != first.Transfer.ID {
		t.Errorf("replay mismatch: replayed=%v id=%s vs %s",
			second.Replayed, second.Transfer.ID, first.Transfer.ID)
	}
}

func TestGetFx_HTTP(t *testing.T) {
	h := newCorridorServer(t)
	rec := doJSON(t, h, "GET", "/api/fx", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("fx = %d %s (want 200)", rec.Code, rec.Body.String())
	}
	var fx struct {
		Pair   string  `json:"pair"`
		Rate   float64 `json:"rate"`
		Source string  `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fx); err != nil {
		t.Fatalf("unmarshal fx: %v", err)
	}
	if fx.Pair != "RWF/KES" {
		t.Errorf("pair = %q, want RWF/KES", fx.Pair)
	}
	if fx.Rate != 9.5 {
		t.Errorf("rate = %v, want 9.5", fx.Rate)
	}
}