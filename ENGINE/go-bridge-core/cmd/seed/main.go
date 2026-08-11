// Package main — demo trader seeder.
//
// `go run ./cmd/seed` (or `make demo-seed`) creates the demo traders the
// dashboard's TraderSelect page expects. It is idempotent: a trader whose
// phone already exists is looked up and skipped, so re-running is safe.
//
// The three demo traders mirror the DEMO_TRADERS list in
// FRONTEND/web-dashboard/src/pages/TraderSelect.tsx. A KES trader is included
// so the dashboard can demonstrate the cross-border (RWF -> KES) story.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"vuka/go-bridge-core/internal/adapters"
	"vuka/go-bridge-core/internal/config"
	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/ledger"
)

type demoTrader struct {
	name        string
	phone       string
	currency    string
	businessReg string
}

var demoTraders = []demoTrader{
	{name: "Amina Uwera", phone: "+250700000991", currency: "RWF", businessReg: "RWC-2026-0441"},
	{name: "Kethan Gasana", phone: "+254700000882", currency: "KES", businessReg: "RWC-2026-0772"},
	{name: "Jean-Paul Niyonzima", phone: "+250700000773", currency: "RWF", businessReg: "RWC-2026-0108"},
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.DatabaseURL == "" {
		log.Error("VUKA_DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	reg := adapters.NewRegistry()
	reg.Register(adapters.NewMTNAdapter(0, 0, "success", 5*time.Second))
	reg.Register(adapters.NewMpesaAdapter(0, 0, "success", 5*time.Second))
	svc := ledger.NewService(pool, idempotency.NewStore(pool), reg)

	for _, d := range demoTraders {
		// Idempotent: if the phone already exists, just report it.
		if existing, err := svc.GetUserByPhone(ctx, d.phone); err == nil {
			log.Info("demo trader already exists", "name", d.name, "id", existing.ID)
			continue
		}
		user, accounts, err := svc.CreateUser(ctx, ledger.CreateUserRequest{
			FullName:          d.name,
			PhoneNumber:       d.phone,
			BusinessRegNumber: d.businessReg,
		}, d.currency)
		if err != nil {
			log.Error("create", "name", d.name, "err", err)
			os.Exit(1)
		}
		log.Info("seeded demo trader", "name", d.name, "id", user.ID, "accounts", len(accounts))
	}

	if err := seedInvoices(ctx, svc, log); err != nil {
		log.Error("seed invoices", "err", err)
		os.Exit(1)
	}

	log.Info("demo seeding complete")
}

// seedInvoices creates a real invoice document for every invoice number that
// the ledger's transfer register references, so the dashboard's Invoices page
// shows the documents behind the paid transfers instead of an empty list.
// Idempotent: an invoice number that already exists is skipped.
func seedInvoices(ctx context.Context, svc *ledger.Service, log *slog.Logger) error {
	// Resolve the demo traders' ids so invoices link real users.
	traders := make(map[string]string) // phone -> id
	for _, d := range demoTraders {
		u, err := svc.GetUserByPhone(ctx, d.phone)
		if err != nil {
			log.Warn("skip invoice seeding: trader missing", "name", d.name, "err", err)
			continue
		}
		traders[d.phone] = u.ID
	}
	// The seeded history is Amina (importer) paying her suppliers. The seller
	// behind an invoice reference depends on the corridor: RWF refs settle to
	// Jean-Paul (Musanze exporter), KES refs settle to Kethan (Nairobi
	// supplier) via the cross-border rail. Resolve by name to avoid phone
	// literals here.
	buyerID, buyerOK := resolveByName(traders, demoTraders, "Amina Uwera")
	if !buyerOK {
		return fmt.Errorf("invoice seeding needs the buyer trader")
	}

	// Distinct invoice numbers referenced by real transfers, with the settled
	// amount and status derived from the ledger register itself.
	refs, err := svc.ListInvoiceReferences(ctx)
	if err != nil {
		return fmt.Errorf("list invoice references: %w", err)
	}

	issued := 0
	for _, r := range refs {
		if _, err := svc.GetInvoiceByNumber(ctx, r.Number); err == nil {
			log.Info("invoice already exists", "number", r.Number)
			continue
		}
		// The seller behind an invoice reference depends on the corridor:
		// domestic RWF refs settle to Jean-Paul (Musanze exporter), refs that
		// crossed the corridor (fx_rate set) settled to Kethan (Nairobi
		// supplier) and are billed in KES. Resolve by name to avoid repeating
		// phone literals here.
		sellerName := "Jean-Paul Niyonzima" // domestic RWF
		invCurrency := r.Currency
		if r.CrossBorder {
			sellerName = "Kethan Gasana"
			invCurrency = "KES"
		}
		sellerID, ok := resolveByName(traders, demoTraders, sellerName)
		if !ok {
			log.Warn("skip invoice: seller not seeded", "name", sellerName, "number", r.Number)
			continue
		}
		// The settled amount follows the invoice currency: for cross-border
		// refs the register amount is the RWF source leg, so the KES invoice
		// amount is the converted destination leg (round to 2dp).
		invAmount := r.Amount
		if r.CrossBorder && r.FxRate > 0 {
			invAmount = math.Round(r.Amount/r.FxRate*100) / 100
		}
		date := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
		due := time.Now().AddDate(0, 0, 14).Format("2006-01-02")
		inv, err := svc.CreateInvoice(ctx, sellerID, ledger.CreateInvoiceRequest{
			Number:             r.Number,
			CounterpartyUserID: buyerID,
			Currency:           invCurrency,
			IssueDate:          date,
			DueDate:            due,
			VATRate:            18,
			Terms:              "Net 14 — settled via VUKA cross-border rail.",
			Notes:              "Seeded from the engine transfer register.",
			Items: []ledger.InvoiceItemRequest{
				{Description: "Cross-border goods invoice " + r.Number, Quantity: 1, UnitPrice: invAmount / 1.18},
			},
		})
		if err != nil {
			return fmt.Errorf("create invoice %s: %w", r.Number, err)
		}
		issued++
		log.Info("seeded invoice", "number", inv.Number, "status", inv.Status)
	}
	log.Info("invoice seeding complete", "referenced", len(refs), "created", issued)
	return nil
}

// resolveByName maps a demo trader's display name to the user id captured
// from the engine, using the phone in the source list (never a typed literal).
func resolveByName(ids map[string]string, traders []demoTrader, name string) (string, bool) {
	for _, d := range traders {
		if d.name == name {
			id, ok := ids[d.phone]
			return id, ok
		}
	}
	return "", false
}
