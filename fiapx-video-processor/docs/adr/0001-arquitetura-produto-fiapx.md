# ADR 0001 — Arquitetura de produto: Go, Clean Architecture, API + worker

| Metadado | Valor |
|----------|--------|
| **Status** | Aceita |
| **Data** | 2026-09-19 |
| **Autores** | Oliver Thies / Hackathon FIAP X |
| **Contexto** | POSTECH SOAT Fase 5 — processador de vídeos |
| **Relacionados** | Fonte no workspace: `docs/adr/0001-arquitetura-produto-fiapx.md` |

Cópia local do ADR aceito. O baseline em `projeto-fiapx/` permanece intacto.

O produto é um monólito modular de dois processos (API + worker) com Postgres, RabbitMQ, JWT e Mailhog. Estados: `UPLOADED → QUEUED → PROCESSING → READY | FAILED`.
