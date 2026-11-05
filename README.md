# GovFac Report

Backend aplikasi pelaporan kerusakan fasilitas umum Kota Surabaya. REST API JSON dengan Go native
(`net/http` + `database/sql`) dan PostgreSQL.

## Menjalankan

Butuh Go 1.26+ dan PostgreSQL.

```bash
cp .env.example .env      # sesuaikan DATABASE_URL
docker compose up -d      # opsional, kalau belum punya PostgreSQL lokal
go run ./cmd/server       # migrasi jalan otomatis saat start
```

```bash
go test ./...
```

## Struktur

```
cmd/server/          entry point + wiring semua fitur (router.go)
internal/<fitur>/    satu folder per fitur
  handler.go         HTTP: decode request, tulis response
  service.go         aturan bisnis
  repository.go      query database
  model.go           tipe domain + validasi
internal/platform/   kode bersama (config, database, middleware, httpx, identity)
migrations/          NNN_nama.sql, jalan berurutan
```

Alurnya selalu `handler → service → repository`. Fitur tidak saling import; kalau butuh user yang
sedang login, ambil dari `platform/identity`.

## API

| Method | Path | Auth | Keterangan |
|---|---|---|---|
| POST | `/api/v1/auth/register` | – | `{name, email, password}` → `201 {user}` |
| POST | `/api/v1/auth/login` | – | `{email, password}` → `200 {token, token_type, expires_at, user}` |
| POST | `/api/v1/auth/logout` | Bearer | `204` |
| GET | `/api/v1/auth/me` | Bearer | `200 {user}` |

Error selalu berbentuk `{"error": "...", "fields": {...}}`. `fields` hanya ada untuk error validasi (422).

## Git

Satu item sprint = satu branch + satu PR, dengan nama `feature/SCRUM-xx-nama-singkat`.
