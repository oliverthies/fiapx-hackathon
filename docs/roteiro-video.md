# Roteiro do vídeo — FIAP X produto

Duração alvo: 8–12 min. Contraste explícito com o baseline.

## 1. Problema (1 min)

Abrir `docs/RELATORIO_BASELINE_ARQUITETURA.html`. Três frases:

1. O FFmpeg estava **dentro** do `POST /upload`.
2. O job era um **timestamp de 1 segundo** — ZIP contaminado (`docs/evidence/PROVA-zip-contaminado-24frames.zip`).
3. Disco do container era o banco: `docker run --rm` apagava tudo.

## 2. ADR (1 min)

`docs/adr/0001-arquitetura-produto-fiapx.md`: Go + Clean Architecture, API + worker, Postgres, RabbitMQ. O `projeto-fiapx/` **não** foi evoluído.

## 3. Pacotes (1 min)

Mostrar a árvore:

- `internal/domain` — `VideoJob`, estados, UUID
- `internal/application` — portas, sem Chi/AMQP/`os/exec`
- `internal/adapter/*` — postgres, rabbit, ffmpeg, smtp, http
- `cmd/api` e `cmd/worker` — composition root

Rodar `go test -cover ./internal/domain/ ./internal/application/`.

## 4. Demo feliz (3 min)

```text
docker compose -f fiapx-video-processor/infra/compose/docker-compose.yml up --scale worker=2
```

Abrir **http://localhost:8081** (UI adapter, não o HTML embutido no `main.go` do baseline):

1. Criar conta / entrar — JWT no sessionStorage
2. Enviar vídeo — a tela mostra **202 QUEUED** na hora (sem spinner de FFmpeg)
3. A lista faz poll: PROCESSING → READY
4. **Baixar ZIP** (fetch com Bearer; não é pasta pública `/uploads`)
5. Grafana **http://localhost:3000** (admin/admin): fila, 2 consumers, READY/min
6. Scale manual: `docker compose up --scale worker=6` — consumers sobem (KEDA é fase 2)

A UI **não** entra no domain: `web/` é estático servido pelo Chi.

## 5. Provas do enunciado (3 min)

- **Dois workers:** logs `compose-worker-1` e `compose-worker-2` processando jobs diferentes.
- **Rajada:** 4 uploads; 4 UUIDs; nenhum diretório compartilhado.
- **IDOR:** usuário B lê job da Ana → **403**.
- **Falha:** `broken.mp4` → FAILED + Mailhog (`http://localhost:8025`) com Correlation-ID.
- Baseline ainda em `:8080` se quiser split-screen “antes/depois”.

## 6. Fecho (30 s)

C4 to-be em `docs/diagrams/c4-container-produto.puml`. Próximo comparativo: mesmo roteiro de medições do relatório, agora no produto.
