# FIAP X — video processor (produto)

Reescrita do baseline em Go com Clean Architecture: API + worker, Postgres, RabbitMQ, JWT e e-mail em falha.

O protótipo original em `../projeto-fiapx` **não é alterado**.

## Subir

```powershell
cd fiapx-video-processor\infra\compose
docker compose up --build --scale worker=2
```

- UI (browser): http://localhost:8081
- Health: http://localhost:8081/health
- Métricas API: http://localhost:8081/metrics
- Grafana: http://localhost:3000 (admin/admin) — dashboard **FIAP X — escala**
- Prometheus: http://localhost:9091
- Rabbit UI: http://localhost:15672 (guest/guest)
- Mailhog: http://localhost:8025

A página em `web/` é um adapter de apresentação (HTML/JS embutido no binário da API). Ela só chama `/auth` e `/videos` com JWT; domain e application não conhecem HTML.

## Observabilidade (Compose)

Dois workers processam em paralelo sem autoscale. Kind/KEDA fica para uma fase 2.

1. `docker compose up --build --scale worker=2` — Grafana: fila ~0, **2 consumers**, 2 uploads READY.
2. Envie mais vídeos e, noutro terminal: `docker compose up --scale worker=6` — consumers sobem no painel; throughput READY/min acompanha.

Painéis: fila `video.process`, workers (consumers), READY/min, FFmpeg p50/p95, bytes de upload, FAILED/DLQ.

## Fluxo mínimo

No PowerShell, grave o JSON em arquivo (`curl.exe -d '{...}'` quebra as aspas):

```powershell
Set-Content -Encoding ascii $env:TEMP\fiapx.json '{"email":"ana@fiapx.local","password":"secret123"}'
curl.exe -sS -X POST http://localhost:8081/auth/register -H "Content-Type: application/json" --data-binary "@$env:TEMP\fiapx.json"
$login = curl.exe -sS -X POST http://localhost:8081/auth/login -H "Content-Type: application/json" --data-binary "@$env:TEMP\fiapx.json"
$token = ($login | ConvertFrom-Json).token
curl.exe -sS -X POST http://localhost:8081/videos -H "Authorization: Bearer $token" -F "video=@.\clip.mp4"
curl.exe -sS http://localhost:8081/videos -H "Authorization: Bearer $token"
```

`POST /videos` responde **202** com `id` e `QUEUED`. O FFmpeg corre no worker.

OpenAPI: [docs/openapi.yaml](docs/openapi.yaml). Postman: [docs/postman/fiapx.postman_collection.json](docs/postman/fiapx.postman_collection.json).

## Pacotes (Clean Architecture)

| Pacote | Pode importar |
|--------|----------------|
| `internal/domain` | stdlib + uuid |
| `internal/application` | domain |
| `internal/adapter/*` | application, domain, drivers |
| `web/` | estáticos (só o adapter HTTP importa) |
| `cmd/api`, `cmd/worker` | composition root |

## ADR

[docs/adr/0001-arquitetura-produto-fiapx.md](../docs/adr/0001-arquitetura-produto-fiapx.md)
