# Roteiro do vídeo: FIAP X

Alvo: até 10 minutos. Primeiro o problema do baseline, depois o produto rodando.

## 1. Problema (1 min)

Abrir `docs/RELATORIO_BASELINE_ARQUITETURA.html` e falar três coisas:

1. O FFmpeg rodava **dentro** do `POST /upload`.
2. O job era um **timestamp de 1 segundo**. Dois uploads no mesmo segundo misturavam os quadros no ZIP.
3. O disco do container era o banco. `docker run --rm` apagava tudo.

## 2. ADR (1 min)

Abrir `docs/adr/0001-arquitetura-produto-fiapx.md`.

A decisão foi Go, Clean Architecture, API + worker, Postgres e RabbitMQ. O `projeto-fiapx/` ficou como estava, para comparar o antes e o depois.

## 3. Pacotes (1 min)

Mostrar a árvore:

- `internal/domain`: `VideoJob`, estados, UUID
- `internal/application`: portas, sem Chi, AMQP ou `os/exec`
- `internal/adapter/*`: postgres, rabbit, ffmpeg, gstreamer, smtp, http
- `cmd/api` e `cmd/worker`: onde os drivers são ligados

Rodar `go test -cover ./internal/domain/ ./internal/application/`.

## 4. Demo (3 min)

A stack já deve estar no ar, com dois workers:

```text
docker compose -f fiapx-video-processor/infra/compose/docker-compose.yml up --scale worker=2
```

Abrir **http://localhost:8081**. A tela é o Studio, em `web/`, servida pela API. O domain não conhece HTML.

1. Criar conta ou entrar. O JWT fica no sessionStorage.
2. Enviar um vídeo curto. A tela mostra **202 QUEUED** na hora, sem esperar o FFmpeg.
3. A lista atualiza sozinha: PROCESSING e depois READY. A thumbnail é o primeiro frame.
4. Baixar o ZIP. O download vai com Bearer. Não tem pasta pública `/uploads`.
5. Grafana em **http://localhost:3000** (admin/admin): fila, 2 consumers, READY/min.
6. Se sobrar tempo, `docker compose up --scale worker=6`. Os consumers sobem no painel.

## 5. Provas do enunciado (3 min)

- Dois workers: logs do `compose-worker-1` e do `compose-worker-2` em jobs diferentes.
- Rajada: 4 uploads, 4 UUIDs, cada um na própria pasta.
- IDOR: o usuário B tenta ler o job da Ana e recebe **403**.
- Falha: um mp4 quebrado fica FAILED e o e-mail aparece no Mailhog (`http://localhost:8025`), com Correlation-ID.
- Se quiser o antes e o depois lado a lado, o baseline continua em `:8080`.

## 6. Fecho (30 s)

Mostrar o C4 em `docs/diagrams/c4-container-produto.puml`. A API só admite o vídeo. A fila segura o pico. O worker processa. O Postgres guarda o estado.
