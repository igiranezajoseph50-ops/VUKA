#!/usr/bin/env bash
# Rebuild the VUKA demo register through the live engine API.
# All transfers use the engine default FX (9.5) — no hardcoded rates.
# Every call carries a fresh UUIDv4 Idempotency-Key.
set -euo pipefail

API="http://localhost:8080"
AMINA_BUSINESS="d18a025a-9a4d-48a9-aaf6-481a86925817"    # Amina Uwera BUSINESS RWF
JEANPAUL_BUSINESS="9fe6b316-be90-4b66-8436-21fb5033642f" # Jean-Paul Niyonzima BUSINESS RWF
KETHAN_BUSINESS="e3efcd15-b8ff-4b3d-a62d-fae1c887c5bd"   # Kethan Gasana BUSINESS KES

key() { cat /proc/sys/kernel/random/uuid; }

call() { # method, path, key, body
  local method="$1" path="$2" k="$3" body="$4"
  curl -sS -X "$method" "$API$path" \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $k" \
    -d "$body"
}

echo "== 1. Fund Amina's business wallet (opening cash-in) =="
call POST /api/accounts/$AMINA_BUSINESS/fund "$(key)" \
  '{"amount":5000000,"currency":"RWF","reference":"MTN-MM-CASHIN-77410922"}' \
  | python3 -c 'import json,sys; t=json.load(sys.stdin)["transfer"]; print(t["id"],t["status"],t["amount"],t["currency"])'

echo "== 2. Domestic RWF payments Amina -> Jean-Paul (Musanze exporter) =="
dom() { # amount, invoice
  call POST /api/transfers "$(key)" \
    "{\"source_account_id\":\"$AMINA_BUSINESS\",\"destination_account_id\":\"$JEANPAUL_BUSINESS\",\"amount\":$1,\"currency\":\"RWF\",\"invoice_number\":\"$2\"}" \
    | python3 -c 'import json,sys; t=json.load(sys.stdin)["transfer"]; print(t["invoice_number"],t["status"],t["amount"],t["currency"])'
}
dom 250000 INV-2026-00890
dom 112500 INV-2026-00892
dom 420000 INV-2026-00893
dom 78500  INV-2026-00895
dom 365000 INV-2026-00896

echo "== 3. Cross-border RWF -> KES payments Amina -> Kethan (Nairobi supplier) @ engine FX =="
cb() { # source amount, invoice (engine computes KES leg via default fx 9.5)
  call POST /api/transfers/cross-border "$(key)" \
    "{\"source_account_id\":\"$AMINA_BUSINESS\",\"destination_account_id\":\"$KETHAN_BUSINESS\",\"amount\":$1,\"currency_from\":\"RWF\",\"currency_to\":\"KES\",\"invoice_number\":\"$2\"}" \
    | python3 -c 'import json,sys; t=json.load(sys.stdin)["transfer"]; print(t["invoice_number"],t["status"],t["amount"],t["currency"],"fx",t.get("fx_rate"))'
}
cb 101500 INV-2026-00891   # hero transfer: 101,500 RWF -> 10,684 KES @ 9.50
cb 475000 INV-2026-00894   # 475,000 RWF -> 50,000 KES
cb 90250  INV-2026-00897   # 90,250 RWF  -> 9,500 KES
cb 190000 INV-2026-00898   # 190,000 RWF -> 20,000 KES

echo "== 4. Verify balances (must balance to zero) =="
psql -h 127.0.0.1 -U vuka -d vuka -t -c "
SELECT a.type||' '||a.currency||': '||COALESCE(SUM(le.amount),0)
FROM accounts a LEFT JOIN ledger_entries le ON le.account_id=a.id
GROUP BY a.id,a.type,a.currency ORDER BY a.type,a.currency;" | cat
echo DONE