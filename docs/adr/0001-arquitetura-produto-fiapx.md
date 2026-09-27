# ADR 0001: Arquitetura de produto (Go, Clean Architecture, API + worker)

| Metadado | Valor |
|----------|--------|
| **Status** | Aceita |
| **Data** | 2026-09-19 |
| **Autores** | Oliver Thies / Hackathon FIAP X |
| **Contexto** | POSTECH SOAT Fase 5, processador de vídeos |
| **Relacionados** | [Deep dive do baseline](../RELATORIO_BASELINE_ARQUITETURA.md), evidências em `docs/evidence/` |

---

## Contexto

O baseline (`projeto-fiapx`) entrega o pitch de um vídeo → ZIP e falha como produto. O diagnóstico mediu:

- processamento **dentro** do `POST /upload` (67 s para ~13,6 min de vídeo);
- identidade do trabalho = timestamp de **1 segundo**, com corrida reproduzida (`20260919_161553`): ZIP contaminado com frames de outro upload;
- filesystem efêmero como banco (`--rm`, sem volume): perda total ao matar o container;
- nenhuma autenticação; `/api/status` e `/uploads` públicos;
- *transaction script* em um arquivo. Sem Clean Architecture e sem camadas.

O enunciado pede: vários vídeos ao mesmo tempo, pico sem perda, usuário/senha, listagem de status do usuário, notificação em erro, persistência, testes e CI.

A linguagem do baseline é Go. O curso aplicou Clean Architecture em Java. Clean Architecture é regra de dependência, não framework: em Go ela se expressa com pacotes e interfaces.

---

## Decisão

Adotar um **monólito modular de dois processos** em **Go**, com Clean Architecture internamente:

1. **API** admite trabalho: JWT (usuário/senha), `POST /videos` responde `202` com `jobId`, persiste o job e publica na fila.
2. **Worker(s)** consomem `video.process`, executam FFmpeg (`fps=1`) e ZIP em pastas isoladas por UUID, atualizam o job, notificam em `FAILED`.
3. **PostgreSQL** é a fonte de verdade de usuários e jobs (golang-migrate).
4. **RabbitMQ** é o buffer de pico (ack após conclusão ou falha registrada; DLQ com retry limitado).
5. Binários em **volume**; porta de storage pronta para S3. SMTP via Mailhog na demo.
6. O `projeto-fiapx/` fica intacto. Ele é a evidência do antes.

Estados do agregado `VideoJob`: `UPLOADED → QUEUED → PROCESSING → READY | FAILED`.

Regra de dependência: `internal/domain` e `internal/application` não importam Gin/Chi, AMQP nem `os/exec`. FFmpeg, disco, JWT e e-mail ficam em `internal/adapter/*`. Os `cmd/api` e `cmd/worker` são o composition root.

---

## Consequências

### Positivas

- Responde às três causas estruturais do diagnóstico (HTTP como orquestrador, timestamp compartilhado, disco como banco).
- Dá para subir N workers no Compose e processar mais de um vídeo ao mesmo tempo.
- Mesma disciplina da oficina (CA, fila, estados, JWT, migrate) sem portar entidades de oficina.
- Linguagem do baseline: o vídeo pode contrastar *transaction script* vs pacotes/portas no mesmo ecossistema.

### Negativas / custos

- Dois processos e mais moving parts que o zip original.
- A máquina de demo pode não ter Go. O build fica nas imagens Docker (multi-stage, sem `go run`).
- Sem Jacoco: cobertura via `go test -cover` (equivalente a mostrar no vídeo).

### Riscos mitigados

| Risco | Mitigação |
|-------|-----------|
| Recair no script do baseline | UUID + pastas por job + testes de domínio que proíbem import de adapter |
| Pico perder request | Persistência + publish + 202 antes do FFmpeg |
| IDOR | JWT + `user_id` em toda query |
| Compensação apagar o job alheio | `RemoveAll` só de `temp/{jobId}` |

---

## Alternativas consideradas

| Alternativa | Veredito |
|-------------|----------|
| **A. Go + CA + API/worker + Rabbit + Postgres** | **Escolhida** |
| B. Evoluir `main.go` in-place (mutex, UUID no script) | Rejeitada. Não cria domínio, fila nem auth |
| C. Java/Spring (mesmo desenho) | Equivalente arquitetural; rejeitada para reusar a língua do baseline e contrastar CA vs o zip |
| D. Thread pool no request HTTP | Não absorve pico nem sobrevive a crash |
| E. Serverless (S3 + SQS + Lambda) | Fora do prazo e da stack demonstrável no lab |
| F. EKS completo no caminho crítico | Overkill; Compose primeiro, manifests K8s se sobrar tempo |

---

## Critérios de aceite

- [x] Upload responde `202` em milissegundos; FFmpeg não está no request
- [x] Rajada de 4 uploads (clip 3 s): 4 UUIDs READY, workers 1 e 2 em paralelo
- [x] Kill do worker: job persiste no Postgres; API em `:8081` sobrevive (volume `mediadata`)
- [x] Sem JWT → 401; zip de outro usuário → 403
- [x] `FAILED` dispara e-mail (Mailhog)
- [x] `internal/domain` sem import de HTTP/AMQP/`os/exec`
- [x] Baseline `projeto-fiapx` intacto

---

## Histórico de revisões

| Versão | Data | Alteração |
|--------|------|-----------|
| 0.1 | 2026-09-19 | Proposta |
| 1.0 | 2026-09-19 | Aceita. Produto em `fiapx-video-processor/` sobe no Compose |
