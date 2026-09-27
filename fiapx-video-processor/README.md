# FIAP X: video processor (produto)

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
- Grafana: http://localhost:3000 (admin/admin), dashboard **FIAP X escala**
- Prometheus: http://localhost:9091
- Rabbit UI: http://localhost:15672 (guest/guest)
- Mailhog: http://localhost:8025

A página em `web/` é um adapter de apresentação (HTML/JS embutido no binário da API). Ela só chama `/auth` e `/videos` com JWT; domain e application não conhecem HTML. O input aceita **vários arquivos**; a tela separa **Em processamento** e **Processados**, com tempo de fila, processamento e tamanho do ZIP por job.

## Observabilidade (Compose)

Dois workers processam em paralelo sem autoscale. Kind/KEDA fica para uma fase 2.

1. `docker compose up --build --scale worker=2`. No Grafana: fila perto de 0, **2 consumers**, 2 uploads READY.
2. Envie mais vídeos e, noutro terminal: `docker compose up --scale worker=6`. Os consumers sobem no painel e o READY/min acompanha.

Painéis: fila `video.process`, workers (consumers), READY/min, extração p50/p95, bytes de upload, FAILED/DLQ.

## Drivers (portas)

API e worker só conhecem as interfaces `Storage` e `VideoProcessor`. O driver concreto entra no composition root (`cmd/*` → `internal/adapter/wiring`).

### Processador (escolha no upload)

| Valor | Adapter | Uso |
|--------|---------|-----|
| `ffmpeg` (padrão) | `internal/adapter/ffmpeg` | Extração 1 fps. Binário `PROCESSOR_BIN`. |
| `gstreamer` | `internal/adapter/gstreamer` | Mesma extração via `gst-launch-1.0` (LGPL). Binário `GSTREAMER_BIN`. |

O motor é escolhido **no upload** (campo `processor` / rádio no Studio). O worker registra os dois e roteia pelo valor do job. `PROCESSOR_DRIVER` só é fallback de jobs antigos.

GStreamer é a alternativa open source de produção para esse fluxo. SaaS (MediaConvert, Mux, Bitmovin) podem virar adapters novos, mas não entram no seletor. Ambos os motores abrem o original via `Storage`, trabalham num scratch (`PROCESSOR_WORKDIR`) e gravam ZIP/thumb com `Put`.

### Storage (`STORAGE_DRIVER`)

| Valor | Adapter | Variáveis |
|--------|---------|-----------|
| `fs` (padrão) | `internal/adapter/fs` | `STORAGE_ROOT` (volume `/data`) |
| `s3` | `internal/adapter/s3store` | `S3_ENDPOINT`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`, `S3_USE_SSL` |

`s3` fala a API S3 (AWS, MinIO, Cloudflare R2, GCS modo S3). Trocar de disco para objeto **não** exige mudar domain/application: as chaves continuam `uploads/`, `outputs/`, `thumbs/`. API e worker precisam do **mesmo** driver e bucket.

Antes desta porta, o worker montava caminho local (`STORAGE_ROOT + original_path`) e o HTTP dependia de `*fs.Storage`. Isso impedia S3 sem reescrever o FFmpeg. Agora o volume compartilhado só é obrigatório no driver `fs`.

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

`POST /videos` responde **202** com `id` e `QUEUED`. O processamento de frames corre no worker (`PROCESSOR_DRIVER`).

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
