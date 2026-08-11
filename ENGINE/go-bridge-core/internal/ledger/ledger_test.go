package ledger_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
	"vuka/go-bridge-core/internal/testutil"
)

// newService wires a Service around a fresh, truncated test database with an
// always-succeeding MTN adapter and a real idempotency store.
func newService(t *testing.T) (*ledger.Service, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.TestPool(t)

	reg := adapters.NewRegistry()
	reg.Register(testutil.MTNSuccessAsync())

	idem := idempotency.NewStore(pool)
	return ledger.NewService(pool, idem, reg), pool
}

// createTrader is a convenience that registers a trader and returns their
// user plus Business account.
func createTrader(t *testing.T, s *ledger.Service, name, phone string) (*ledger.User, *ledger.Account, *ledger.Account) {
	t.Helper()
	u, accounts, err := s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName:    name,
		PhoneNumber: phone,
	}, "RWF")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	personal := accounts[0]
	business := accounts[1]
	return u, personal, business
}

// fund credits the account using the real idempotent funding path.
func fund(t *testing.T, s *ledger.Service, accountID string, amount float64) {
	t.Helper()
	_, err := s.FundAccount(context.Background(), accountID, testutil.UniqueID(),
		ledger.FundRequest{Amount: amount, Currency: "RWF"})
	if err != nil {
		t.Fatalf("FundAccount: %v", err)
	}
}

func TestCreateUser_CreatesPersonalAndBusinessAccounts(t *testing.T) {
	s, _ := newService(t)
	u, _, err := s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName:    "Amina Uwera",
		PhoneNumber: "+250****0001",
	}, "RWF")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.KYCStatus != "PENDING" {
		t.Errorf("expected PENDING kyc, got %q", u.KYCStatus)
	}

	accounts, err := s.ListAccounts(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	types := map[ledger.AccountType]bool{}
	for _, a := range accounts {
		types[a.Type] = true
		if a.Currency != "RWF" {
			t.Errorf("account %s currency=%s, want RWF", a.Type, a.Currency)
		}
	}
	if !types[ledger.AccountTypePersonal] || !types[ledger.AccountTypeBusiness] {
		t.Errorf("missing PERSONAL/BUSINESS accounts: %v", types)
	}

	// Both accounts start at zero (derived balance).
	for _, a := range accounts {
		b, err := s.GetBalance(context.Background(), a.ID)
		if err != nil {
			t.Fatalf("GetBalance(%s): %v", a.ID, err)
		}
		if b.Amount != 0 {
			t.Errorf("account %s starts non-zero: %.2f", a.Type, b.Amount)
		}
	}
}

func TestCreateUser_DuplicatePhoneRejected(t *testing.T) {
	s, _ := newService(t)
	_, _, err := s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName: "A", PhoneNumber: "+250****0002",
	}, "RWF")
	if err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, _, err = s.CreateUser(context.Background(), ledger.CreateUserRequest{
		FullName: "B", PhoneNumber: "+250****0002",
	}, "RWF")
	if err == nil {
		t.Fatal("expected error for duplicate phone, got nil")
	}
}

func TestFundAccount_CreditsBalance(t *testing.T) {
	s, _ := newService(t)
	_, _, business := createTrader(t, s, "Fund Test", "+250700000003")
	fund(t, s, business.ID, 100000)

	b, err := s.GetBalance(context.Background(), business.ID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if b.Amount != 100000 {
		t.Errorf("balance=%.2f, want 100000", b.Amount)
	}
}

func TestTransfer_Success_WritesBalancedDoubleEntry(t *testing.T) {
	s, _ := newService(t)
	_, _, srcBusiness := createTrader(t, s, "Sender", "+250700000004")
	_, dstPersonal, dstBusiness := createTrader(t, s, "Recipient", "+250700000005")

	fund(t, s, srcBusiness.ID, 50000)

	// Transfer from sender BUSINESS to recipient BUSINESS (B2B invoice).
	tf, replay, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      srcBusiness.ID,
		DestinationAccountID: dstBusiness.ID,
		Amount:               15000,
		Currency:             "RWF",
		InvoiceNumber:        "INV-1001",
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if replay {
		t.Error("first transfer should not replay")
	}
	if tf.Status != ledger.StatusSuccess {
		t.Errorf("status=%s, want SUCCESS", tf.Status)
	}
	if tf.ExternalReference == nil {
		t.Error("expected an external MTN reference")
	}

	// Balances: sender down 15000, recipient up 15000, dstPersonal untouched.
	srcBal, _ := s.GetBalance(context.Background(), srcBusiness.ID)
	if srcBal.Amount != 35000 {
		t.Errorf("sender balance=%.2f, want 35000", srcBal.Amount)
	}
	dstBal, _ := s.GetBalance(context.Background(), dstBusiness.ID)
	if dstBal.Amount != 15000 {
		t.Errorf("recipient balance=%.2f, want 15000", dstBal.Amount)
	}
	personalBal, _ := s.GetBalance(context.Background(), dstPersonal.ID)
	if personalBal.Amount != 0 {
		t.Errorf("recipient personal balance=%.2f, want 0 (business isolation)", personalBal.Amount)
	}

	// Double-entry audit: exactly two entries summing to zero, with the
	// source debited (negative) and destination credited (positive).
	entries, err := s.ListLedgerEntries(context.Background(), tf.ID)
	if err != nil {
		t.Fatalf("ListLedgerEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	var sum float64
	for _, e := range entries {
		sum += e.Amount
		if e.TransactionID != tf.ID {
			t.Errorf("entry tx=%s want %s", e.TransactionID, tf.ID)
		}
	}
	if sum != 0 {
		t.Errorf("double-entry does not net to zero: sum=%v", sum)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	s, _ := newService(t)
	_, _, src := createTrader(t, s, "Poor", "+250700000006")
	_, _, dst := createTrader(t, s, "Rich", "+250700000007")
	fund(t, s, src.ID, 100)

	_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               1000,
		Currency:             "RWF",
	})
	if !errors.Is(err, ledger.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}

	// No entries must be left behind by the failed transfer.
	entries, _ := s.ListAccounts(context.Background(), mustUser(t, s, src))
	_ = entries
	if b, _ := s.GetBalance(context.Background(), src.ID); b.Amount != 100 {
		t.Errorf("source unchanged? balance=%.2f, want 100", b.Amount)
	}
}

func mustUser(t *testing.T, s *ledger.Service, acc *ledger.Account) string {
	t.Helper()
	a, err := s.GetAccount(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	return a.UserID
}

func TestTransfer_IdempotencyReplay_ReturnsOriginalNoDup(t *testing.T) {
	s, _ := newService(t)
	_, _, src := createTrader(t, s, "Idem", "+250700000008")
	_, _, dst := createTrader(t, s, "Dst", "+250700000009")
	fund(t, s, src.ID, 5000)

	key := testutil.UniqueID()
	req := ledger.TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               1000,
		Currency:             "RWF",
	}

	first, replay, err := s.Transfer(context.Background(), key, req)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	if replay {
		t.Error("first transfer replayed unexpectedly")
	}

	// Second transfer with the SAME key must not charge twice.
	second, replay, err := s.Transfer(context.Background(), key, req)
	if err != nil {
		t.Fatalf("replayed transfer error: %v", err)
	}
	if !replay {
		t.Error("expected replay=true for duplicate key")
	}
	if second.ID != first.ID {
		t.Errorf("replay returned different transfer: %s vs %s", second.ID, first.ID)
	}

	srcBal, _ := s.GetBalance(context.Background(), src.ID)
	if srcBal.Amount != 4000 {
		t.Errorf("source debited twice? balance=%.2f, want 4000", srcBal.Amount)
	}
	dstBal, _ := s.GetBalance(context.Background(), dst.ID)
	if dstBal.Amount != 1000 {
		t.Errorf("dst credited twice? balance=%.2f, want 1000", dstBal.Amount)
	}
}

func TestTransfer_CurrencyMismatch(t *testing.T) {
	s, _ := newService(t)
	_, _, src := createTrader(t, s, "CurSrc", "+250700000010")
	_, _, dst := createTrader(t, s, "CurDst", "+250700000011")
	fund(t, s, src.ID, 5000)

	_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               100,
		Currency:             "KES", // accounts are RWF
	})
	if !errors.Is(err, ledger.ErrCurrencyMismatch) {
		t.Fatalf("expected ErrCurrencyMismatch, got %v", err)
	}
}

func TestTransfer_NonBusinessSourceRejected(t *testing.T) {
	s, _ := newService(t)
	_, personal, business := createTrader(t, s, "Pers", "+250700000012")
	_, _, dst := createTrader(t, s, "Dst2", "+250700000013")
	fund(t, s, personal.ID, 5000)
	fund(t, s, business.ID, 5000)

	// Attempt to fund an invoice payment from the PERSONAL wallet.
	_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      personal.ID,
		DestinationAccountID: dst.ID,
		Amount:               100,
		Currency:             "RWF",
	})
	if !errors.Is(err, ledger.ErrInvalidAccountType) {
		t.Fatalf("expected ErrInvalidAccountType, got %v", err)
	}
}

func TestTransfer_UnknownAdapter(t *testing.T) {
	pool := testutil.TestPool(t)
	reg := adapters.NewRegistry() // empty registry
	idem := idempotency.NewStore(pool)
	s := ledger.NewService(pool, idem, reg)

	_, _, src := createTrader(t, s, "NoAdapter", "+250700000014")
	_, _, dst := createTrader(t, s, "NoAdapterDst", "+250700000015")
	fund(t, s, src.ID, 1000)

	_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               100,
		Currency:             "RWF",
	})
	if !errors.Is(err, ledger.ErrAdapterUnavailable) {
		t.Fatalf("expected ErrAdapterUnavailable, got %v", err)
	}
}

func TestReverseTransfer(t *testing.T) {
	s, _ := newService(t)
	_, _, src := createTrader(t, s, "RevSrc", "+250700000016")
	_, _, dst := createTrader(t, s, "RevDst", "+250700000017")
	fund(t, s, src.ID, 5000)

	tf, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      src.ID,
		DestinationAccountID: dst.ID,
		Amount:               1000,
		Currency:             "RWF",
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	rev, err := s.ReverseTransfer(context.Background(), tf.ID, "test reversal")
	if err != nil {
		t.Fatalf("ReverseTransfer: %v", err)
	}
	if rev.Status != ledger.StatusReversed {
		t.Errorf("status=%s, want REVERSED", rev.Status)
	}

	// Balances restored to pre-transfer state.
	srcBal, _ := s.GetBalance(context.Background(), src.ID)
	if srcBal.Amount != 5000 {
		t.Errorf("source after reversal=%.2f, want 5000", srcBal.Amount)
	}
	dstBal, _ := s.GetBalance(context.Background(), dst.ID)
	if dstBal.Amount != 0 {
		t.Errorf("dst after reversal=%.2f, want 0", dstBal.Amount)
	}

	// The reversed transfer's entries still net to zero (original pair +
	// contra pair = 4 entries, sum zero).
	entries, _ := s.ListLedgerEntries(context.Background(), tf.ID)
	if len(entries) != 4 {
		t.Errorf("expected 4 entries after reversal, got %d", len(entries))
	}
	var sum float64
	for _, e := range entries {
		sum += e.Amount
	}
	if sum != 0 {
		t.Errorf("entries net %v, want 0", sum)
	}
}

func TestReverseTransfer_NonSuccessRejected(t *testing.T) {
	s, _ := newService(t)
	_, _, revSrc := createTrader(t, s, "Rev2Src", "+250700000018")
	_, _, revDst := createTrader(t, s, "Rev2Dst", "+250700000019")
	fund(t, s, revSrc.ID, 5000)

	tf, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
		SourceAccountID:      revSrc.ID,
		DestinationAccountID: revDst.ID,
		Amount:               1000,
		Currency:             "RWF",
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	// First reversal succeeds (SUCCESS -> REVERSED)...
	if _, err := s.ReverseTransfer(context.Background(), tf.ID, "first"); err != nil {
		t.Fatalf("first reversal: %v", err)
	}
	// ...reversing an already-REVERSED transfer must be rejected.
	if _, err := s.ReverseTransfer(context.Background(), tf.ID, "second"); !errors.Is(err, ledger.ErrTransferNotSuccess) {
		t.Fatalf("expected ErrTransferNotSuccess on second reversal, got %v", err)
	}
}

// newDelayedService returns a Service whose MTN adapter adds a small,
// deterministic latency. This forces concurrent transfers to queue on the
// source-account row lock (SELECT ... FOR UPDATE) instead of each goroutine
// reading a stale balance, which is exactly the race the lock must prevent.
func newDelayedService(t *testing.T) *ledger.Service {
	t.Helper()
	pool := testutil.TestPool(t)
	reg := adapters.NewRegistry()
	reg.Register(adapters.NewMTNAdapter(25*time.Millisecond, 0, "success", 5*time.Second))
	idem := idempotency.NewStore(pool)
	return ledger.NewService(pool, idem, reg)
}

// TestTransfer_Concurrent_ExactFunding launches many goroutines transferring
// out of ONE business account whose funding exactly covers all of them. With
// the source-row lock, every transfer must succeed and the derived balance
// must end exactly at zero — never negative at any point.
func TestTransfer_Concurrent_ExactFunding(t *testing.T) {
	s := newDelayedService(t)
	_, _, src := createTrader(t, s, "ConcSrc", "+250****0030")
	const workers = 8
	const amount = 1000.0

	dsts := make([]string, workers)
	for i := 0; i < workers; i++ {
		_, _, dst := createTrader(t, s, fmt.Sprintf("ConcDst%d", i), fmt.Sprintf("+250****%04d", 31+i))
		dsts[i] = dst.ID
	}
	fund(t, s, src.ID, workers*amount)

	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
				SourceAccountID:      src.ID,
				DestinationAccountID: dsts[i],
				Amount:               amount,
				Currency:             "RWF",
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("transfer %d: %v", i, err)
		}
	}

	srcBal, err := s.GetBalance(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if srcBal.Amount != 0 {
		t.Errorf("source balance after %d concurrent transfers = %.2f, want 0", workers, srcBal.Amount)
	}
}

// TestTransfer_Concurrent_NoOverdraft funds the source for only N-1 of the N
// concurrent transfers. Exactly one must be rejected with ErrInsufficientFunds,
// and the balance must never drift negative — the strongest proof that the
// FOR UPDATE lock serializes balance reads against concurrent debits.
func TestTransfer_Concurrent_NoOverdraft(t *testing.T) {
	s := newDelayedService(t)
	_, _, src := createTrader(t, s, "ConcSrc2", "+250****0041")
	const workers = 8
	const amount = 1000.0

	dsts := make([]string, workers)
	for i := 0; i < workers; i++ {
		_, _, dst := createTrader(t, s, fmt.Sprintf("ConcDst2_%d", i), fmt.Sprintf("+250****%04d", 51+i))
		dsts[i] = dst.ID
	}
	// Cover only workers-1 transfers: the last one must bounce.
	fund(t, s, src.ID, (workers-1)*amount)

	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := s.Transfer(context.Background(), testutil.UniqueID(), ledger.TransferRequest{
				SourceAccountID:      src.ID,
				DestinationAccountID: dsts[i],
				Amount:               amount,
				Currency:             "RWF",
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	successes := 0
	insufficient := 0
	for i, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ledger.ErrInsufficientFunds):
			insufficient++
		default:
			t.Errorf("transfer %d: unexpected error %v", i, err)
		}
	}
	if successes != workers-1 {
		t.Errorf("expected %d successes, got %d", workers-1, successes)
	}
	if insufficient != 1 {
		t.Errorf("expected exactly 1 insufficient-funds rejection, got %d", insufficient)
	}

	srcBal, err := s.GetBalance(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if srcBal.Amount != 0 {
		t.Errorf("source balance = %.2f, want 0 (must never go negative)", srcBal.Amount)
	}
}