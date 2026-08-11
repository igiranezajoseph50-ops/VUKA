# VUKA — Deploy to Render (working-platform link for the Stage 2 form)

**Goal:** a permanent public URL for field 8 (working platform) that survives
this laptop being closed. The temporary `trycloudflare.com` tunnel is NOT the
submission link — it dies with this machine.

**Verified already (2026-08-11):**
- `ENGINE/deploy/Dockerfile` — multi-stage: Node builds the SPA → Go builds a
  static engine binary → alpine runtime serves SPA + `/deck` + `/api` on ONE port.
- All three stages proven locally: `npm run build` (dist/), `go build` (binary),
  `VUKA_WEB_DIR=dist` single-port serving live on :8080 + via tunnel.
- `ENGINE/deploy/render.yaml` — Render Blueprint: Web Service (Docker, free) +
  Postgres (free, 90-day expiry).

---

## Options — pick ONE

### Option A — Render Blueprint (recommended, one click)
1. Push this project to a **public GitHub/GitLab repo** (no git repo exists
   yet — `git init` + first commit required; secrets are NOT in the repo).
2. On Render: **New → Blueprint** → select the repo.
3. Render provisions `vuka` web service + `vuka-db` postgres automatically.
4. After creation, copy the **Service URL** (e.g. `https://vuka-xxxx.onrender.com`)
   and put it into `VUKA_CORS_ORIGINS` (Settings → Environment) — replace the
   placeholder `https://vuka.onrender.com`.
5. Seed the demo data (see below) and verify: `{SERVICE_URL}/`, `{SERVICE_URL}/deck`,
   `{SERVICE_URL}/api/fx` should all return 200. The ledger starts EMPTY on a
   fresh DB — run the seed.

### Option B — Render Blueprint with existing DB
If you already have a Postgres (Render, Neon, Supabase — Neon is recommended:
perpetual free tier, no expiry):
1. Create the repo + Blueprint as in Option A.
2. Delete the `databases:` block from `render.yaml`, remove the
   `fromDatabase` line, and set `VUKA_DATABASE_URL` directly to your external
   DSN (must be a `postgres://` URL, `sslmode=disable` works).

---

## Seeding the demo data on a fresh Render DB

The engine creates settlement accounts on boot, but demo users/transactions
come from the seed:

```
# from ENGINE/go-bridge-core (local machine, pointing at the Render DB URL)
export VUKA_DATABASE_URL="postgres://vuka:***@<render-host>/vuka?sslmode=disable"
go run ./cmd/seed
```

Then reset to the pristine 10-transaction register:
```
bash scripts/build_demo_register.sh   # 10 txns Aug 1–9, Aug 10 empty, FX 9.5
```

⚠️ Use the exact demo phones from the seed — the frontend matches them
byte-for-byte. Never retype from memory.

---

## Post-deploy checklist (before pasting into the form)

- [ ] `{SERVICE_URL}/` → 200, SPA loads, `● LIVE` badge appears (FX from engine)
- [ ] `{SERVICE_URL}/deck` → 200, pitch deck page
- [ ] `{SERVICE_URL}/api/fx` → 200, `{"rate":9.5}`
- [ ] Open the platform in an **incognito window** (no login wall)
- [ ] Paste `{SERVICE_URL}/` into form field 8
- [ ] Keep the local tunnel as a backup until Render is confirmed

## Render free-tier caveats (checked 2026-08-11)

- Web service spins down after ~15 min idle → first click takes ~1 min to wake.
  Acceptable for a demo; set a "Keep alive" cron ping if you want it always-hot.
- Free Postgres **expires after ~90 days** — fine for the evaluation window,
  or use Neon's perpetual free tier instead.
- 750 free instance-hours/month — plenty for a demo.