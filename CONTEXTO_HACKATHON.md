# Contexto — Hackathon FIAP X (Fase 5 / SOAT)

| Campo | Valor |
|-------|--------|
| **Participante** | Oliver Thies |
| **Turma** | 12SOAT / Postech SOAT |
| **Data do contexto** | 19/09/2026 |
| **Enunciado** | `Challenge/POSTECH - SOAT - Fase 5 - Hacka.pdf` |
| **Domínio anterior** | Sistema de Gestão de Oficina Mecânica (Fases 1–4) |
| **Domínio deste hackathon** | Processamento de vídeos — **FIAP X** (sistema totalmente diferente) |

---

## 1. O que é este hackathon

A empresa **FIAP X** apresentou aos investidores um protótipo que:

1. Recebe um vídeo
2. Extrai frames / imagens
3. Devolve um arquivo **`.zip`**

Os investidores querem uma versão **produto**: o usuário envia o vídeo e faz download do zip, com arquitetura séria — não o “script de demo” da rodada de investimento.

### Projeto base

Há um **projeto base** (download no PDF) usado na apresentação. Ele **não** aplica as boas práticas do curso. O desafio é **rearquitetar / evoluir** esse domínio de vídeo aplicando o que foi aprendido nos Tech Challenges anteriores.

> **Importante:** o domínio muda (oficina → vídeos). O que se reaproveita são **técnicas, padrões e disciplina de entrega**, não o código da oficina.

---

## 2. Requisitos do enunciado (resumo)

### Funcionais essenciais

| # | Requisito | Interpretação prática |
|---|-----------|------------------------|
| 1 | Processar **mais de um vídeo ao mesmo tempo** | Concorrência / workers / filas / escala horizontal |
| 2 | Em **picos**, não perder requisição | Buffer assíncrono (mensageria), persistência de job, retry |
| 3 | Protegido por **usuário e senha** | Auth (JWT / sessão / API Gateway + credenciais) |
| 4 | **Listagem de status** dos vídeos do usuário | API de jobs/vídeos com estados (PENDING, PROCESSING, DONE, FAILED…) |
| 5 | Em **erro**, notificar o usuário (e-mail ou outro) | Evento de falha → notificação (como na oficina com mudança de status) |

### Técnicos

- Persistência de dados
- Arquitetura **escalável**
- Versionamento no **GitHub**
- **Testes** de qualidade
- **CI/CD**
- Stack sugerida: Docker + K8s (ou Compose); RabbitMQ/Kafka; PostgreSQL + Redis; Prometheus/Grafana/ELK (ou equivalente); GitHub Actions

### Entregáveis

1. Documentação de arquitetura
2. Script de criação de banco / recursos
3. Link(s) GitHub
4. Vídeo ≤ **10 min**: documentação + arquitetura + projeto funcionando

---

## 3. O que NÃO é este projeto

- **Não** é evolução da oficina mecânica (ms-os / billing / execution).
- **Não** precisa reutilizar entidades Cliente, OS, Mercado Pago, etc.
- **Não** copiar namespaces EKS `oficina-*` — criar identidade FIAP X / video-processor.
- O código da oficina serve como **referência de como fizemos**, não como base a forkear cegamente.

---

## 4. Técnicas dos challenges anteriores a aplicar aqui

Mapa **oficina (aprendido) → FIAP X (aplicar)**.

### 4.1 Arquitetura e desenho

| Técnica (oficina) | Onde usamos | Como aplicar no FIAP X |
|-------------------|-------------|-------------------------|
| Diagramas C4 / sequência / PlantUML | Fases 3–4 | Documentar API, workers, storage, fila |
| Bounded contexts / data ownership | Fase 4 (OS / Billing / Execution) | Separar: **Auth+API**, **Ingestão/Jobs**, **Processamento**, **Notificação** (ajustar ao desenho do grupo) |
| Clean Architecture (domain / application / adapters) | API monólito + MS | Manter domínio de “VideoJob” limpo; FFmpeg/storage/mail na borda |
| RFC / ADR | Fase 3 | Registrar decisões (fila vs sync, S3 vs disco, K8s vs Compose) |

### 4.2 Microsserviços e comunicação

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| MS com repo/banco/deploy próprios | 3 MS Fase 4 | Ex.: API de upload/status + worker de processamento (+ opcional notificação) |
| REST entre serviços | HttpBillingGateway / HttpStockGateway | API consulta status; worker não precisa expor UI |
| **Mensageria** (RabbitMQ) | Saga commands/events | Fila de “processar vídeo”; ack só após zip pronto ou falha registrada |
| **Saga / orquestração** (conceito) | Saga OS com compensações | Pipeline de job: UPLOADED → QUEUED → PROCESSING → ZIP_READY / FAILED; compensar (limpar arquivos temporários) em falha |
| Idempotência / estado de processo | `saga_process` | Tabela `video_jobs` com step atual e correlation id |

### 4.3 Persistência

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| PostgreSQL + Flyway | RDS / schemas MS | Usuários, jobs, status, metadados do vídeo |
| NoSQL quando faz sentido | DynamoDB no billing | Opcional: metadados de frames ou cache de sessão; **Redis** (sugerido) para cache / rate limit / lock |
| Ownership sem leitura cruzada de bancos | Fase 4 | Worker e API compartilham contrato (API+DB ou eventos), sem gambiarra entre stores |

### 4.4 Auth e segurança

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| Auth + JWT | Lambda CPF + API Spring | **Usuário e senha** (requisito); JWT após login; escopo: só listar/baixar **vídeos do próprio usuário** |
| API Gateway (conceito) | Fase 3 | Opcional; Compose/K8s com serviço de auth também vale |

### 4.5 Qualidade e entrega

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| Testes + cobertura (JaCoCo ≥ 80%) | 3 MS | Unit + integração no fluxo de job / fila |
| BDD (Cucumber) | ms-os saga | Opcional: feature “upload → process → download” |
| SonarCloud / quality gate | CI | Manter no pipeline |
| CI/CD → build/test → imagem (ECR) | GitHub Actions | Build + push imagem; deploy Compose ou K8s |
| Postman / Swagger | Fases 3–4 | Collection: login, upload, list status, download zip |
| Branch protection / PR | Repos oficina | Mesmo padrão nos repos do hackathon |

### 4.6 Observabilidade

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| APM + tracing distribuído | New Relic | New Relic **ou** Prometheus + Grafana (enunciado aceita preferência) |
| Correlation ID | Header + MDC | Propagar `jobId` / `correlationId` API → fila → worker → logs |
| Custom events / métricas de negócio | Status OS, duração por fase | Tempo de processamento, filas, taxa de falha, vídeos/dia |
| Alertas | Falha de OS / integração | Falha de job, fila crescendo, worker down |
| Notificação em erro | E-mail em mudança de status | E-mail (ou outro) quando job `FAILED` |

### 4.7 Infraestrutura

| Técnica | Oficina | FIAP X |
|---------|---------|--------|
| Terraform / IaC | VPC, EKS, RDS | Reutilizar conhecimento; escala do hackathon pode ser **Compose local + K8s** se o tempo for curto |
| Kubernetes namespaces / Deployments | EKS oficina | Deploy API + worker + Rabbit + Postgres (+ Redis) |
| Secrets | NR, Mercado Pago | Credenciais DB, SMTP, storage |
| Port-forward / demo | Postman nos MS | Roteiro curto ≤ 10 min |

---

## 5. Esboço de arquitetura alvo (ponto de partida — não fechado)

```
[Cliente]
   │  HTTPS
   ▼
[API Gateway / API Auth] ── JWT (usuário/senha)
   │
   ├─ POST /videos (upload)  → grava metadados + arquivo (S3/disco)
   │                          → publica mensagem na fila
   ├─ GET  /videos            → lista status do usuário
   └─ GET  /videos/{id}/zip   → download quando READY
   │
   ▼
[RabbitMQ / Kafka]  ← buffer de pico (não perde requisição)
   │
   ▼
[Worker(s) escaláveis]  → FFmpeg / extração de frames → ZIP
   │                         atualiza status no Postgres
   │                         em erro → notificação
   ▼
[Object storage / volume] + [PostgreSQL] + [Redis opcional]
```

**Por que isso espelha a oficina:** upload assíncrono ≈ abrir OS e orquestrar passos; fila ≈ Rabbit da Saga; workers ≈ execution; status listável ≈ `GET .../saga`; notificação ≈ listener de falha/status.

---

## 6. Estados sugeridos do job de vídeo

```
UPLOADED → QUEUED → PROCESSING → READY
                         └──────→ FAILED  (notifica usuário)
```

Compensações possíveis: apagar frames temporários, marcar job FAILED, republicar na DLQ com limite de retry.

---

## 7. Repositórios de referência (oficina — só consulta)

| Repo | Uso como referência |
|------|---------------------|
| https://github.com/oliverthies/ms-os-oficina | Orquestração, Saga, Rabbit, Clean Arch, Cucumber |
| https://github.com/oliverthies/ms-billing-oficina | MS independente, Dynamo/NoSQL, integração externa |
| https://github.com/oliverthies/ms-execution-oficina | Worker de domínio operacional |
| https://github.com/oliverthies/infra-geral-oficina | EKS, CI, deploy |
| https://github.com/oliverthies/infra-database | RDS / Postgres |
| https://github.com/oliverthies/lambda-auth-oficina | Auth serverless / JWT |
| https://github.com/oliverthies/oficina-api | Monólito Fase 3, New Relic, Flyway |

Workspace local da oficina: `../oficina-project` (irmão desta pasta `hackathon`).

---

## 8. Como usar este arquivo (agentes / Cursor)

Ao trabalhar neste hackathon:

1. **Ler este arquivo primeiro** — domínio = FIAP X / vídeos, não oficina.
2. Aplicar o **mapa da seção 4**; não portar código de OS/Cliente/Mercado Pago.
3. Preferir decisões documentáveis (ADR curto) e entregáveis do PDF.
4. Priorizar demo que mostre: auth → upload concorrente → fila sob pico → status → zip → falha com notificação → CI + testes.
5. Manter artefatos nesta pasta `hackathon/` (docs, scripts, notas) separados do monorepo da oficina.

### Checklist mental antes de cada entrega

- [ ] Mais de um vídeo em paralelo
- [ ] Pico não perde request (fila + persistência)
- [ ] Login usuário/senha
- [ ] Listagem de status por usuário
- [ ] Notificação em erro
- [ ] Persistência + escala
- [ ] GitHub + testes + CI/CD
- [ ] Doc de arquitetura + script de DB
- [ ] Vídeo ≤ 10 min

---

## 9. Próximos passos sugeridos

1. Baixar e inspecionar o **projeto base** do PDF (baseline do investidor).
2. Fechar desenho C4 (API / worker / fila / DB / storage / auth).
3. Criar repo(s) GitHub do hackathon (nome neutro, ex. `fiapx-video-processor`).
4. Implementar happy path mínimo + falha notificada.
5. Empacotar Compose/K8s, Postman, doc e roteiro do vídeo.

---

## 10. Histórico deste contexto

| Data | Nota |
|------|------|
| 19/09/2026 | Contexto inicial gerado a partir do PDF Fase 5 + técnicas das Fases 3–4 da oficina |
|
