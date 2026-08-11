package ledger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

// newCorridorService returns a Service with the corridor registry (MTN + M-Pesa),
// a default FX rate, and RWF/KES SETTLEMENT accounts seeded.
func newCorridorService(t *testing.T) *ledger.Service {
	t.Helper()
	pool := testutil.TestPool(t)
	idem := idempotency.NewStore(pool)
	svc := ledger.NewService(pool, idem, testutil.CorridorRegistry())
	svc.SetDefaultFxRate(9.5) // 1 KES = 9.5 RWF
	if err := svc.EnsureSettlementAccounts(context.Background(), "RWF", "KES"); err != nil {
		t.Fatalf("EnsureSettlementAccounts: %v", err)
	}
	return svc
}

// createCorridorTrader creates a trader whose BUSINESS wallet is in the given
// currency. Returns the business account.
func createCorridorTrader(t *testing.T, s *ledger.Service, name, phone, currency string) *ledger.Account {
	t.Helper()
	_, accounts, err := s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName: name, PhoneNumber: phone,
	}, currency)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return accounts[1] // BUSINESS
}

// fundCorridor funds a business account via the idempotent funding path.
func fundCorridor(t *testing.T, s *ledger.Service, accountID string, amount float64) {
	t.Helper()
	if _, err := s.FundAccount(context.Background(), accountID, testutil.UniqueID(),
		ledger.FundRequest{Amount: amount, Currency: mustCurrency(s, accountID)}); err != nil {
		t.Fatalf("FundAccount: %v", err)
	}
}

// mustCurrency reads an account's currency (test helper).
func mustCurrency(s *ledger.Service, accountID string) string {
	acc, err := s.GetAccount(context.Background(), accountID)
	if err != nil {
		return "RWF"
	}
	return acc.Currency
}

// collectBalances returns the derived balances for a list of account ids.
func collectBalances(t *testing.T, s *ledger.Service, ids ...string) map[string]float64 {
	t.Helper()
	out := make(map[string]float64, len(ids))
	for _, id := range ids {
		b, err := s.GetBalance(context.Background(), id)
		if err != nil {
			t.Fatalf("GetBalance(%s): %v", id, err)
		}
		out[id] = b.Amount
	}
	return out
}

func TestCrossBorderTransfer_LegsBalanceAndFxConvert(t *testing.T) {
	s := newCorridorService(t)
	rwf := createCorridorTrader(t, s, "RW Importer", "+250****0061", "RWF")
	kes := createCorridorTrader(t, s, "KE Supplier", "+254****0062", "KES")
	fundCorridor(t, s, rwf.ID, 190000) // RWF

	tf, replayed, err := s.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               95000, // RWF
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5, // 1 KES = 9.5 RWF
		InvoiceNumber:        "INV-CB-001",
	})
	if err != nil {
		t.Fatalf("CrossBorderTransfer: %v", err)
	}
	if replayed {
		t.Error("expected a fresh transfer, not a replay")
	}
	if tf.Status != ledger.StatusSuccess {
		t.Errorf("status=%s, want SUCCESS", tf.Status)
	}
	if tf.FxRate == nil || *tf.FxRate != 9.5 {
		t.Errorf("fx_rate not recorded: %v", tf.FxRate)
	}
	if tf.ExternalReference == nil || *tf.ExternalReference == "" {
		t.Error("expected an external reference from the M-Pesa rail")
	}

	// KES received = 95000 / 9.5 = 10000.
	kesBal, _ := s.GetBalance(context.Background(), kes.ID)
	if kesBal.Amount != 10000 {
		t.Errorf("destination KES balance=%.2f, want 10000", kesBal.Amount)
	}
	// Source RWF reduced by 95000.
	rwfBal, _ := s.GetBalance(context.Background(), rwf.ID)
	if rwfBal.Amount != 95000 { // 190000 - 95000
		t.Errorf("source RWF balance=%.2f, want 95000", rwfBal.Amount)
	}

	// Four entries: two legs, total sum zero.
	entries, err := s.ListLedgerEntries(context.Background(), tf.ID)
	if err != nil {
		t.Fatalf("ListLedgerEntries: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries (2 FX legs), got %d", len(entries))
	}
	var sum float64
	var kesEntries int
	for _, e := range entries {
		sum += e.Amount
		if e.Amount == 10000 || e.Amount == -10000 {
			kesEntries++
		}
	}
	if sum != 0 {
		t.Errorf("entries net %v, want 0", sum)
	}
	if kesEntries != 2 {
		t.Errorf("expected 2 KES-denominated entries, got %d", kesEntries)
	}
}

func TestCrossBorderTransfer_Replay_ReturnsOriginal_NoDup(t *testing.T) {
	s := newCorridorService(t)
	rwf := createCorridorTrader(t, s, "RW2", "+250****0063", "RWF")
	kes := createCorridorTrader(t, s, "KE2", "+254****0064", "KES")
	fundCorridor(t, s, rwf.ID, 100000)

	key := testutil.UniqueID()
	req := ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               47500,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
	}

	first, replay1, err := s.CrossBorderTransfer(context.Background(), key, req)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	if replay1 {
		t.Error("first call must not be a replay")
	}

	second, replay2, err := s.CrossBorderTransfer(context.Background(), key, req)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("replay returned different transfer: %s vs %s", second.ID, first.ID)
	}
	if !replay2 {
		t.Error("second call must be a replay")
	}

	// No double charge: source debited exactly once (47500).
	balances := collectBalances(t, s, rwf.ID, kes.ID)
	if balances[kes.ID] != 5000 { // 47500 / 9.5
		t.Errorf("KES balance=%.2f, want 5000 (single credit)", balances[kes.ID])
	}

	// Replaying the SAME idempotency key inside one transaction (not here) is
	// additionally protected by the UNIQUE constraint; 4 entries only.
	entries, _ := s.ListLedgerEntries(context.Background(), first.ID)
	if len(entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(entries))
	}
}

func TestCrossBorderTransfer_InsufficientFunds(t *testing.T) {
	s := newCorridorService(t)
	rwf := createCorridorTrader(t, s, "RW3", "+250****0065", "RWF")
	kes := createCorridorTrader(t, s, "KE3", "+254****0066", "KES")
	fundCorridor(t, s, rwf.ID, 100)

	_, _, err := s.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               1000,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
	})
	if !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestCrossBorderTransfer_RejectsWrongSourceType(t *testing.T) {
	s := newCorridorService(t)
	_, accounts, _ := s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName: "Pers", PhoneNumber: "+250****0067",
	}, "RWF")
	personal := accounts[0] // PERSONAL
	kes := createCorridorTrader(t, s, "KE4", "+254****0068", "KES")
	fundCorridor(t, s, personal.ID, 5000)

	_, _, err := s.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      personal.ID,
		DestinationAccountID: kes.ID,
		Amount:               1000,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
	})
	if !errors.Is(err, ledger.ErrInvalidAccountType) {
		t.Fatalf("expected ErrInvalidAccountType, got %v", err)
	}
}

func TestCrossBorderTransfer_MissingFxRate_Defaults(t *testing.T) {
	s := newCorridorService(t)
	rwf := createCorridorTrader(t, s, "RW4", "+250****0069", "RWF")
	kes := createCorridorTrader(t, s, "KE5", "+254****0070", "KES")
	fundCorridor(t, s, rwf.ID, 95000)

	// No explicit fx_rate -> engine default (9.5).
	tf, _, err := s.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               47500,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
	})
	if err != nil {
		t.Fatalf("CrossBorderTransfer (default rate): %v", err)
	}
	if tf.FxRate == nil || *tf.FxRate != 9.5 {
		t.Errorf("expected default fx_rate 9.5, got %v", tf.FxRate)
	}
}

func TestCrossBorderTransfer_Reversal_RestoresBalances(t *testing.T) {
	s := newCorridorService(t)
	rwf := createCorridorTrader(t, s, "RW5", "+250****0071", "RWF")
	kes := createCorridorTrader(t, s, "KE6", "+254****0072", "KES")
	fundCorridor(t, s, rwf.ID, 95000)

	tf, _, err := s.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               47500,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
	})
	if err != nil {
		t.Fatalf("CrossBorderTransfer: %v", err)
	}

	// Reversal restores both balances and adds 4 contra entries (8 total).
	if _, err := s.ReverseTransfer(context.Background(), tf.ID, "op cancelled"); err != nil {
		t.Fatalf("ReverseTransfer: %v", err)
	}
	balances := collectBalances(t, s, rwf.ID, kes.ID)
	if balances[rwf.ID] != 95000 {
		t.Errorf("RWF after reversal=%.2f, want 95000", balances[rwf.ID])
	}
	if balances[kes.ID] != 0 {
		t.Errorf("KES after reversal=%.2f, want 0", balances[kes.ID])
	}
	entries, _ := s.ListLedgerEntries(context.Background(), tf.ID)
	if len(entries) != 8 { // 4 original + 4 contra
		t.Errorf("expected 8 entries after cross-border reversal, got %d", len(entries))
	}
}

func TestCrossBorderTransfer_RequiresSettlementAccounts(t *testing.T) {
	// A registry WITHOUT M-Pesa and WITHOUT seeded settlement accounts must
	// fail cleanly (no panics, no silent corruption).
	pool := testutil.TestPool(t)
	reg := adapters.NewRegistry()
	reg.Register(adapters.NewMTNAdapter(0, 0, "success", 5*time.Second)) // no mpesa-ke
	idem := idempotency.NewStore(pool)
	svc := ledger.NewService(pool, idem, reg)
	svc.SetDefaultFxRate(9.5)

	rwf := createCorridorTrader(t, svc, "RW6", "+250****0073", "RWF")
	kes := createCorridorTrader(t, svc, "KE7", "+254****0074", "KES")
	fundCorridor(t, svc, rwf.ID, 95000)

	_, _, err := svc.CrossBorderTransfer(context.Background(), testutil.UniqueID(), ledger.CrossBorderRequest{
		SourceAccountID:      rwf.ID,
		DestinationAccountID: kes.ID,
		Amount:               47500,
		CurrencyFrom:         "RWF",
		CurrencyTo:           "KES",
		FxRate:               9.5,
	})
	if err == nil {
		t.Fatal("expected error when settlement accounts / mpesa rail missing")
	}
}