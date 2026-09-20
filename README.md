# FIAP X — hackathon POSTECH SOAT Fase 5

Produto de processamento de vídeo (upload → frames → ZIP) em Go + Clean Architecture.

- Baseline (não evoluir): [`projeto-fiapx/`](projeto-fiapx/)
- Produto: [`fiapx-video-processor/`](fiapx-video-processor/)
- Diagnóstico as-is: [`docs/RELATORIO_BASELINE_ARQUITETURA.md`](docs/RELATORIO_BASELINE_ARQUITETURA.md)
- ADR: [`docs/adr/0001-arquitetura-produto-fiapx.md`](docs/adr/0001-arquitetura-produto-fiapx.md)

```powershell
docker compose -f fiapx-video-processor/infra/compose/docker-compose.yml up --build --scale worker=2
```

UI: http://localhost:8081 · Grafana: http://localhost:3000 (admin/admin)
