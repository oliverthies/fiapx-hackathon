# Deep dive de engenharia — baseline FIAP X

**Diagnóstico técnico do protótipo de processamento de vídeos apresentado aos investidores**

| Campo | Valor |
|-------|-------|
| **Objeto da análise** | `projeto-fiapx/` — `main.go` (437 linhas), `Dockerfile`, `go.mod`, `go.sum` |
| **Natureza do artefato** | Protótipo funcional de demonstração; o próprio `Dockerfile` se declara "exemplo de como NÃO fazer" |
| **Objetivo deste documento** | Avaliar o sistema atual contra práticas de engenharia, arquitetura, segurança e qualidade |
| **O que este documento não é** | Não é comparação com uma solução proposta, nem justificativa de reescrita. A proposta de evolução será registrada separadamente em ADR |
| **Data das medições** | 19/09/2026, 12:34 – 13:36 (UTC−3) |
| **Ambiente** | Windows 10 · Docker Desktop 29.8.0 (WSL2) · Intel i5-8400 6 vCPU · 8,28 GB RAM |
| **Runtime medido** | Imagem `fiapx-baseline` · Go 1.21.13 · Gin 1.9.1 · FFmpeg 6.1.1 (Alpine) |
| **Evidências** | `docs/evidence/` (26 artefatos) · diagramas em `docs/diagrams/` |

---

# Parte I — Enquadramento

## 1. Sumário executivo

O baseline entrega **exatamente aquilo para que foi construído**: recebe um arquivo de vídeo por HTTP, extrai um quadro por segundo com FFmpeg, empacota os PNGs em um `.zip` e devolve o arquivo. Em um cenário de um usuário e um arquivo por vez, o caminho feliz funciona de forma previsível e rápida — 0,91 s para um vídeo de 12 segundos. Como peça de demonstração para uma rodada de investimento, cumpre seu papel.

O diagnóstico técnico, porém, identifica um problema de natureza diferente de "faltam recursos". Sob concorrência, **o sistema não falha de forma segura: ele corrompe e vaza dados**. Em uma rajada de oito uploads simultâneos, um cliente que enviou um vídeo de 12 segundos recebeu um ZIP com 24 imagens, sendo **12 delas quadros do vídeo de outro usuário** (seção 6 e Dimensão 4). Nenhuma das oito requisições recebeu código de erro HTTP; todas responderam `200`.

A causa raiz é uma decisão de projeto isolada e de aparência inofensiva: a identidade do trabalho é um **timestamp com precisão de um segundo**, usado simultaneamente como nome do diretório temporário e do arquivo de saída. Duas requisições no mesmo segundo compartilham área de trabalho, sobrescrevem os quadros uma da outra, e a limpeza (`defer os.RemoveAll`) de uma apaga o material da outra durante a execução do FFmpeg.

As demais constatações se organizam em quatro grupos:

**Confidencialidade.** Não existe autenticação em nenhuma das seis rotas. O endpoint `/api/status` devolve o inventário completo de artefatos de todos os usuários, e tanto os ZIPs quanto **os vídeos originais enviados** são servidos publicamente por rotas estáticas — verificamos o download de um vídeo de terceiros (5,7 MB, `video/mp4`, HTTP 200, sem credencial).

**Durabilidade.** Não há banco de dados nem volume persistente. O estado do sistema é o conteúdo de três diretórios dentro de um container efêmero. Isso não é teórico: durante esta sessão o container foi encerrado e **5 arquivos ZIP totalizando aproximadamente 375 MB desapareceram**, com o inventário voltando a `{"files":null,"total":0}`.

**Recuperabilidade.** Interrompendo o container 12 segundos dentro de um trabalho de 483 quadros, o cliente recebeu `curl: (52) Empty reply from server` — sem código de erro utilizável e sem identificador para reconsultar. Como não existe registro de trabalho, não há o que marcar como falho, retomar ou reprocessar. A recuperação também é lenta: o cold start medido foi de **13,26 s**, porque a imagem não contém binário compilado.

**Capacidade.** Um único upload consome **475,66% de CPU** em uma máquina de 600% (6 vCPU), ou seja, cerca de 79% do host. Não existe folga para processar dois vídeos longos simultaneamente nesta instância, independentemente dos defeitos de concorrência. O volume de saída cresce linearmente a ~0,55 MB por segundo de vídeo, sem cota nem expiração: uma hora de vídeo projeta ~2 GB de ZIP.

Do ponto de vista de **estilo arquitetural** — eixo central deste diagnóstico — o baseline é um **monólito de processo único** cuja estrutura interna **não** corresponde a monólito modular, Clean Architecture, arquitetura em camadas nem hexagonal. A unidade de deploy é monolítica; o interior é um *transaction script* (Fowler) em um arquivo. A classificação completa está na seção 4.1.

Avaliado em 13 dimensões de engenharia (Parte III), o conjunto pontua **0,7 de 5** em média, com quatro dimensões em zero: modelo de domínio, concorrência, persistência e segurança. A distinção relevante para as próximas decisões é que essas quatro não são lacunas de refinamento — são pré-condições para qualquer operação multiusuário.

## 2. Metodologia e limites da análise

A avaliação combinou quatro técnicas, nesta ordem:

**Leitura estática do código.** Mapeamento das 10 funções de `main.go`, das dependências declaradas e do `Dockerfile`, identificando responsabilidades, acoplamentos e pontos de decisão arquitetural.

**Execução e observação do caminho feliz.** Construção da imagem, execução do container e uso da interface web com vídeos reais, registrando os logs de acesso do Gin e as saídas do processo.

**Testes dinâmicos dirigidos.** Experimentos desenhados para exercitar dimensões específicas: concorrência (2 e 8 uploads simultâneos), validação de entrada (arquivo com extensão falsa), controle de acesso (requisições sem credencial, tentativas de travessia de caminho), consumo de recursos (amostragem de `docker stats`), custo por duração (cinco vídeos de 12 s a 480 s) e resiliência (interrupção no meio do processamento).

**Inspeção do artefato de execução.** Análise das camadas da imagem, configuração do container, usuário efetivo, healthcheck, variáveis de ambiente e conteúdo do sistema de arquivos durante e após os testes.

Os vídeos de longa duração foram produzidos com `ffmpeg -stream_loop` a partir de uma das amostras reais, preservando características de codificação e, portanto, o tamanho realista dos PNGs extraídos.

### Limites que devem ser considerados na leitura

Estas restrições delimitam o alcance das conclusões:

- **Amostra de hardware única.** Todas as medições vêm de uma máquina (6 vCPU, 8,28 GB) sob Docker Desktop/WSL2. Os números absolutos variam em outro hardware; as **relações** (linearidade do custo, proporção de CPU, taxa de colisão) são propriedades do desenho e se mantêm.
- **Sem limites de recurso aplicados.** O container rodou sem `--cpus` ou `--memory`. Em Kubernetes com limites definidos, a saturação apareceria como *throttling* ou *OOMKill* em vez de consumo de 79% do host.
- **Concorrência gerada localmente.** A rajada partiu da mesma máquina que hospeda o serviço, o que adiciona contenção de CPU do lado do cliente. Isso afeta a latência medida na rajada, não a ocorrência das colisões.
- **A corrida depende de coincidência temporal.** Requisições que caem em segundos distintos não colidem. As duas requisições iniciais da rajada, separadas pelo custo de inicialização dos processos de teste, concluíram normalmente. O defeito é probabilístico e cresce com a taxa de chegada — o que o torna mais perigoso, não menos: em produção ele se manifesta de forma intermitente e difícil de reproduzir.
- **Travessia de caminho não foi reproduzida.** As tentativas contra `/download` e `/outputs` retornaram 404. Este relatório não afirma existir leitura arbitrária de arquivos.

## 3. Inventário do sistema

### 3.1 Código

| Métrica | Valor |
|---------|-------|
| Arquivos de código | 1 (`main.go`) |
| Linhas totais | 437 |
| Funções | 10, todas no pacote `main` |
| Interfaces declaradas | 0 |
| Linhas de HTML/CSS/JS embutidas em string Go | 144 (33% do arquivo, a partir da linha 293) |
| Arquivos de teste | 0 |
| Dependência direta | `github.com/gin-gonic/gin v1.9.1` |
| Módulos no grafo (`go.sum`) | 36 |
| Configuração externalizada | Nenhuma variável de ambiente lida pelo código |

Distribuição das funções:

| Linha | Função | Responsabilidades acumuladas |
|-------|--------|------------------------------|
| 30 | `main()` | rotas, CORS inline, arranque |
| 68 | `createDirs()` | criação dos diretórios de trabalho |
| 75 | `handleVideoUpload()` | parsing HTTP, gravação em disco, orquestração, resposta |
| 126 | `processVideo()` | nome do trabalho, invocação do FFmpeg, coleta de quadros, empacotamento, log |
| 187 | `createZipFile()` | criação do arquivo ZIP |
| 207 | `addFileToZip()` | escrita de uma entrada |
| 236 | `handleDownload()` | entrega do artefato |
| 253 | `handleStatus()` | varredura do diretório de saída |
| 281 | `isValidVideoFile()` | verificação de extensão |
| 293 | `getHTMLForm()` | interface web completa |

### 3.2 Superfície HTTP

Seis rotas, nenhuma protegida:

| Método | Rota | Comportamento | Autenticação |
|--------|------|---------------|--------------|
| GET | `/` | HTML embutido no binário | Não |
| POST | `/upload` | Upload + FFmpeg + ZIP no mesmo request | Não |
| GET | `/download/:filename` | Envia ZIP de `outputs/` | Não |
| GET | `/api/status` | `Glob("outputs/*.zip")` — inventário global | Não |
| GET | `/uploads/*filepath` | Serve estaticamente os **vídeos enviados** | Não |
| GET | `/outputs/*filepath` | Serve estaticamente os ZIPs | Não |

Cabeçalhos de resposta observados em `GET /`:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: POST, GET, OPTIONS
Access-Control-Allow-Headers: Content-Type
Content-Type: text/html
```

Não há `Authorization`, cookie de sessão, `Content-Security-Policy`, `X-Content-Type-Options` nem HTTPS.

### 3.3 Artefato de execução

| Propriedade | Valor observado |
|-------------|-----------------|
| Tamanho da imagem | **852 MB** |
| Base | `golang:1.21-alpine` (SDK completo, não runtime) |
| Camadas relevantes | 256 MB (toolchain Go) · 237 MB (`go mod tidy`) · 129 MB (FFmpeg) · 8,46 MB (Alpine) |
| Comando de entrada | `CMD ["go","run","main.go"]` — compila a cada inicialização |
| Usuário efetivo | vazio → **root** |
| Healthcheck | `null` |
| Multi-stage build | Não |
| Volumes | `Mounts=[]` |
| Política de reinício | `no` |
| Conteúdo acidental | diretório `__MACOSX` com 7 arquivos `._*` empacotados na imagem |
| Cold start até HTTP 200 | **13 260 ms** |
| Consumo em repouso | 202,3 MB / 19 PIDs |

O `go mod tidy` executado em tempo de build (em vez de `go mod download` com cache de camada) e a ausência de binário compilado explicam simultaneamente o tamanho da imagem e a lentidão de inicialização.

---

# Parte II — Arquitetura as-is

## 4. Visão de container

Todo o sistema é um processo dentro de um container. Não existe segundo serviço, banco, broker, cache, armazenamento de objetos ou componente de notificação.

![Diagrama de container do baseline FIAP X](diagrams/svg/c4-container.svg)

Três características desse desenho determinam quase todas as constatações posteriores:

**O FFmpeg roda dentro do request HTTP.** A conexão TCP do cliente é, na prática, o mecanismo de orquestração. Quem espera é o navegador. Não há resposta `202 Accepted`, identificador de trabalho ou consulta posterior.

**O sistema de arquivos é o banco de dados.** Estado, resultado e histórico são o conteúdo de `uploads/`, `temp/` e `outputs/`. Como não há volume, o ciclo de vida do dado está atado ao ciclo de vida do container.

**Interface e processamento compartilham o mesmo deploy.** As 144 linhas de HTML/CSS/JS moram na mesma binary que executa o FFmpeg. Não é possível escalar, publicar ou proteger uma parte sem a outra.

## 4.1 Classificação do estilo arquitetural

Esta seção responde à pergunta que o restante do diagnóstico pressupõe: **qual padrão o baseline implementa, e contra quais padrões do curso ele deve ser julgado?** Sem essa classificação, é fácil tratar o sistema como “um monólito incompleto” ou como “falta de microsserviços”. Nenhuma das duas leituras é precisa.

### Unidade de deploy: monólito — e isso, sozinho, não é um defeito

Pelo critério de **deploy e processo**, o baseline é um monólito clássico:

- um repositório, um módulo Go (`video-processor`), um `main`;
- um container, um PID, uma porta;
- UI, API e processamento no mesmo artefato.

Monólito **não é, por si, um anti-padrão**. A oficina da Fase 3 também era um monólito (`oficina-api`) e ainda assim aplicava Clean Architecture, Flyway, eventos de domínio e fronteiras internas. O curso distingue com clareza:

| Conceito | O que significa | O baseline |
|----------|-----------------|------------|
| **Monólito de deploy** | Um artefato, um processo | **Sim** — e isso é aceitável para o tamanho do problema |
| **Monólito modular** | Mesmo deploy, módulos com contratos e ownership | **Não** — não há módulos |
| **Microsserviços** | Serviços com ciclo de vida, banco e deploy próprios | **Não** — e a ausência, neste tamanho, não é o problema principal |

Criticar o baseline por “não ser microsserviço” seria o erro inverso: fragmentar 437 linhas em três repositórios sem fronteiras internas só espalharia o *transaction script*. O julgamento correto é sobre **estrutura interna**, não sobre o número de processos.

### Estrutura interna: *transaction script*, não modelo de domínio

Martin Fowler descreve dois padrões de organização da lógica:

- **Transaction Script** — um procedimento por operação, que lê entrada, executa passos e devolve resultado. É o que `handleVideoUpload` + `processVideo` fazem: um roteiro linear (receber → gravar → ffmpeg → zip → responder).
- **Domain Model** — entidades com identidade, invariantes e transições; a aplicação orquestra, o domínio decide o que é válido.

O baseline é **Transaction Script puro**. Não há agregado `VideoJob`, não há invariante (“um trabalho pertence a um usuário”), não há transição de estado. `VideoRequest` e `ProcessingResult` são DTOs de transporte; o primeiro sequer é usado. Isso explica por que a corrida de timestamp não viola “regra de domínio”: **não existe regra de domínio para violar**. O procedimento simplesmente compartilha um diretório.

Para um protótipo de um clique, Transaction Script é o padrão **correto e honesto**. O diagnóstico é que o mesmo padrão foi levado a um contexto (concorrência, multi-tenant implícito, persistência de resultado) no qual ele deixa de ser suficiente.

### Clean Architecture: não atende

A regra de dependência de Uncle Bob exige que o domínio não conheça framework, I/O nem UI; casos de uso dependem de **portas**; adaptadores implementam as portas na borda.

Mapeamento das camadas sobre o código real:

| Camada (Clean / oficina) | O que deveria existir | O que existe no `main.go` |
|--------------------------|----------------------|---------------------------|
| **Domínio** | `VideoJob`, estados, invariantes | Ausente |
| **Aplicação** | caso de uso “processar vídeo”, portas | Ausente — o handler *é* o caso de uso |
| **Adaptador de entrada** | HTTP / HTML | `handleVideoUpload`, `getHTMLForm` **no mesmo arquivo** |
| **Adaptador de saída** | FFmpeg, disco, e-mail | `exec.Command` e `os.*` **dentro** de `processVideo` |

A dependência aponta **para fora**: a função que deveria ser o caso de uso importa `os/exec` e escreve `fmt.Printf`. Não há `type VideoProcessor interface`. Sem porta, não há como substituir FFmpeg, gravar em S3 ou publicar na fila sem editar o núcleo.

Conclusão formal: **o baseline não implementa Clean Architecture**, nem uma versão reduzida dela. Implementa um único adaptador que contém tudo.

### Arquitetura em camadas e hexagonal: camadas colapsadas

Uma arquitetura em camadas clássica (apresentação → aplicação → domínio → infraestrutura) exigiria pelo menos pacotes ou diretórios. Hexagonal (ports & adapters) exigiria o domínio no centro e adaptadores plugáveis.

No baseline as quatro camadas **colapsam em uma**:

```
apresentação (HTML string)
        │
aplicação (orquestração no handler)
        │          ← tudo no pacote main
domínio (inexistente)
        │
infra (ffmpeg, filesystem, log)
```

Não há pacote `domain`, `application` ou `adapter`. Não há injeção de dependência. O “centro” do sistema é `processVideo()`, que é precisamente a função mais acoplada à infraestrutura.

![Camadas esperadas versus camadas reais do baseline](diagrams/svg/architecture-layers.svg)

### O que o baseline *não* é — para não forçar o rótulo errado

| Padrão | Atende? | Por quê |
|--------|---------|---------|
| Monólito de deploy | **Sim** | Um processo, um artefato |
| Monólito modular | Não | Sem módulos, sem contratos internos |
| Clean Architecture | Não | Sem domínio, sem portas, dependência invertida |
| Camadas (n-tier) | Não | Camadas colapsadas no `package main` |
| Hexagonal / ports & adapters | Não | Zero interfaces |
| MVC / separação UI | Não | HTML/CSS/JS embutidos na API |
| Microsserviços | Não | Um bounded context implícito, um deploy |
| Orientado a eventos / mensageria | Não | HTTP síncrono é o único estilo |
| CQRS | Não | Mesmo caminho lê e escreve; “status” é `Glob` no disco |
| Saga / orquestração de processo | Não | Não há processo de longa duração modelado |

### Crítica arquitetural (o que importa para o trabalho)

Três críticas independentes, em ordem de gravidade estrutural:

1. **Ausência de fronteira de domínio.** Sem entidade e sem estados, o sistema não tem onde aplicar regra, autorização, idempotência ou observabilidade de negócio. Isso não se resolve com mais endpoints.
2. **Acoplamento do caso de uso à infraestrutura.** FFmpeg, disco e HTTP estão na mesma função. Qualquer mudança de estilo (fila, worker, object storage) é reescrita do núcleo, não troca de adaptador.
3. **Deploy monolítico sem modularidade interna.** O monólito seria defensável se houvesse módulos (auth, job, processing, delivery). Sem eles, o único eixo de escala é “mais CPU no mesmo PID” — e as medições mostram que um upload já consome ~79% do host.

O que **não** se critica aqui: a decisão de um único deployável neste estágio. Um monólito modular com Clean Architecture internamente atenderia o curso e o problema sem exigir microsserviços no dia um.

## 5. Visão de componentes

![Componentes internos de main.go](diagrams/svg/components.svg)

O arquivo é legível e a nomenclatura é clara — um leitor entende o fluxo em poucos minutos. O problema não é organização superficial, é **ausência de fronteiras**.

Duas funções concentram responsabilidades heterogêneas. `handleVideoUpload()` faz parsing de multipart, gravação em disco, orquestração do processamento e formatação da resposta. `processVideo()` define a identidade do trabalho, invoca um processo externo, varre o sistema de arquivos, compacta o resultado e escreve log. São as duas funções onde todos os defeitos observados se originam.

Três funções são coesas e testáveis isoladamente: `isValidVideoFile()`, `createZipFile()` e `addFileToZip()` recebem entrada, produzem saída ou erro, e não dependem de estado global. Elas demonstram que o problema é estrutural, não de habilidade — o autor sabe escrever função pura; apenas não havia razão, em um protótipo, para aplicar isso ao caminho principal.

Não existe nenhuma interface (`type X interface`). O FFmpeg está acoplado por `exec.Command` literal, os caminhos `"uploads"`, `"temp"`, `"outputs"` são strings embutidas, a taxa `fps=1` é fixa e a porta `8080` é constante.

## 6. O comportamento sob concorrência

Esta é a constatação central do diagnóstico e merece a sequência completa.

![Sequência da corrida com seis uploads no mesmo segundo](diagrams/svg/sequence-race.svg)

O mecanismo, linha por linha do código:

```126:134:projeto-fiapx/main.go
func processVideo(videoPath, timestamp string) ProcessingResult {
	fmt.Printf("Iniciando processamento: %s\n", videoPath)

	tempDir := filepath.Join("temp", timestamp)
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	framePattern := filepath.Join(tempDir, "frame_%04d.png")
```

O `timestamp` chega de `handleVideoUpload()` como `time.Now().Format("20060102_150405")` — resolução de um segundo. Ele é usado para três finalidades que exigiriam exclusividade: nome do diretório temporário, padrão de nome dos quadros (`frame_%04d.png`, reiniciando em 1 para cada vídeo) e nome do ZIP de saída.

O resultado medido com oito uploads simultâneos:

| Requisição | Vídeo | HTTP | `success` | Latência | Desfecho |
|------------|-------|------|-----------|----------|----------|
| REQ0 | short 12 s | 200 | `true` | 1,28 s | correto (segundo isolado) |
| REQ1 | short 12 s | 200 | `true` | 0,86 s | correto (segundo isolado) |
| REQ2 | short 12 s | **200** | `false` | 4,12 s | `open temp/20260919_161553/frame_0019.png: no such file or directory` |
| REQ3 | short 12 s | **200** | `false` | 4,10 s | `frame_0011.png: no such file or directory` |
| REQ4 | short 12 s | **200** | `false` | 4,03 s | `frame_0021.png: no such file or directory` |
| REQ5 | short 12 s | **200** | `true` | 4,07 s | **ZIP com 24 quadros — 12 de outro vídeo** |
| REQ6 | long 40 s | **200** | `false` | 4,25 s | `ffmpeg exit status 251` · `Could not open file … frame_0030.png` |
| REQ7 | long 40 s | **200** | `false` | 4,25 s | `ffmpeg exit status 251` · `Conversion failed!` |

Oito uploads produziram três ZIPs. Cinco requisições (62,5%) perderam o trabalho integralmente. Todas responderam `HTTP 200`.

### A prova do vazamento

O arquivo `frames_20260919_161553.zip` foi baixado e inspecionado entrada por entrada:

```
frame_0001.png  377656 bytes   ┐
frame_0002.png  365815 bytes   │ idênticos byte a byte aos quadros
...                            │ do vídeo de 12 s (confirmado contra
frame_0012.png  657074 bytes   ┘ os ZIPs de REQ0 e REQ1)
frame_0013.png  511897 bytes   ┐
frame_0014.png  513949 bytes   │ não podem pertencer a um vídeo de
...                            │ 12 s com fps=1 — vêm do vídeo de
frame_0024.png  647808 bytes   ┘ 40 s enviado por outra requisição
```

Um vídeo de 12 segundos a um quadro por segundo produz no máximo 12 imagens. As entradas 13 a 24 só podem ter origem em outro upload. O cliente recebeu `success: true` e um arquivo aparentemente válido: **não existe sinal algum, na resposta ou no artefato, que permita perceber a contaminação**.

O arquivo está preservado em `docs/evidence/PROVA-zip-contaminado-24frames.zip`.

### Por que o FFmpeg falhou

As requisições 6 e 7 não falharam por vídeo inválido. O `stderr` mostra o processamento avançando normalmente e sendo interrompido:

```
frame=   28 fps=7.6 q=-0.0 size=N/A time=00:00:27.00 bitrate=N/A speed=7.35x
[image2 @ 0x…] Could not open file : temp/20260919_161553/frame_0030.png
[vost#0:0/png @ 0x…] Error submitting a packet to the muxer: I/O error
Conversion failed!
```

O diretório foi removido **enquanto o FFmpeg escrevia nele** — o `defer os.RemoveAll(tempDir)` de uma das requisições que terminou primeiro. A compensação de um trabalho destrói o trabalho em andamento de outro.

### Degradação de latência

A mesma operação custa 0,86–1,28 s isolada e 4,03–4,25 s dentro da rajada: aproximadamente **4x** de degradação por contenção de CPU. Não há fila, limite de concorrência ou *backpressure* — a admissão é ilimitada e a degradação é repassada integralmente a todos os clientes.

## 7. Ciclo de vida do dado

![Ciclo de vida do dado e comportamento na falha](diagrams/svg/dataflow-lifecycle.svg)

A assimetria entre sucesso e falha é relevante. No sucesso, o vídeo original é apagado e o diretório temporário é limpo. Na falha, o `os.Remove(videoPath)` não é alcançado e **o vídeo permanece indefinidamente** — confirmamos dois órfãos (5,5 MB e 68 MB) após os testes. Nenhum dos caminhos aplica expiração ao ZIP resultante: o diretório de saída acumulou 491,8 MB em cerca de uma hora de uso exploratório.

---

# Parte III — Análise por dimensão

Cada dimensão segue a mesma estrutura: a prática de referência adotada nas fases anteriores do curso, o que foi observado, o que o baseline acerta, onde ele é frágil, o risco resultante e uma estimativa do esforço de correção.

## Dimensão 1 — Estilo arquitetural e separação de responsabilidades

A classificação formal (monólito de deploy × *transaction script* interno × não-Clean / não-hexagonal) está na seção 4.1. Aqui o julgamento é de maturidade.

**Referência.** Clean Architecture e monólito modular das Fases 3–4: domínio sem dependências de framework, casos de uso na aplicação, adaptadores na borda, regra de dependência apontando para dentro. Microsserviços só depois que os bounded contexts já existem no monólito.

**Observado.** Unidade de deploy monolítica (adequada). Estrutura interna: *transaction script* em um arquivo, camadas colapsadas, zero portas. `processVideo()` depende de `os/exec` e de `fmt`. A apresentação (144 linhas de HTML) vive no mesmo pacote que o FFmpeg.

**Pontos positivos.** O monólito de um processo é o tamanho certo para o problema atual — não se critica “falta de microsserviço”. O código é linear e legível; três funções já são coesas. Go + Gin são escolha coerente com um único binário.

**Fragilidades.** Sem módulo interno, sem interface, sem domínio. Trocar FFmpeg, disco ou o estilo síncrono exige editar o núcleo. Não é possível testar o caso de uso sem o sistema de arquivos e o binário `ffmpeg`. SOLID: responsabilidade única quebrada em `handleVideoUpload` e `processVideo`; inversão de dependência inexistente.

**Risco.** Toda evolução arquitetural (fila, worker, auth, persistência) concentra-se no mesmo par de funções onde já estão os defeitos de concorrência. O custo de mudança é estrutural.

**Esforço de correção.** Alto — reestruturação para monólito modular (ou dois processos API/worker) com portas, não um ajuste pontual.

**Maturidade: 1/5** — o ponto vem do deploy monolítico consciente e da legibilidade; não vem de aderência a Clean Architecture ou a camadas.

## Dimensão 2 — Modelo de domínio e estados

**Referência.** Agregado com máquina de estados explícita e transições validadas, como `ServiceOrder` e `ServiceOrderStatus` na oficina, com histórico e momento de entrada em cada estado.

**Observado.** Não existe entidade que represente o trabalho de processamento. Existem `VideoRequest` e `ProcessingResult`, ambos DTOs de transporte — e `VideoRequest` não é usado em nenhum handler. O "estado" é implícito: se há ZIP no diretório, terminou; se não há, não se sabe distinguir entre nunca enviado, em processamento e falhado.

**Pontos positivos.** `ProcessingResult` é coerente e bem tipado, transportando `success`, `message`, `zip_path`, `frame_count` e `images`. É uma base razoável para um futuro DTO de resposta.

**Fragilidades.** Sem a entidade, não há como responder "qual o estado do meu vídeo", atribuir dono, registrar tentativa, medir duração por etapa ou decidir o que reprocessar. A ausência de `UPLOADED → QUEUED → PROCESSING → READY/FAILED` não é uma lacuna de recurso: é a razão pela qual as outras dimensões não têm onde se ancorar.

**Risco.** Nenhuma observabilidade de negócio, nenhuma recuperação, nenhuma auditoria são possíveis.

**Esforço de correção.** Médio — modelar a entidade e suas transições é direto; o custo está em propagá-la por persistência e API.

**Maturidade: 0/5**

## Dimensão 3 — Integração e comunicação

**Referência.** Comunicação assíncrona por mensageria para trabalho de longa duração (RabbitMQ na Saga da Fase 4), com confirmação após conclusão, e REST para consulta de estado.

**Observado.** Um único estilo: HTTP síncrono. A admissão do trabalho e sua execução são a mesma operação. A resposta só chega depois que o FFmpeg e a compactação terminam — medimos 67 s para um vídeo de ~13,6 minutos.

**Pontos positivos.** O contrato de upload é simples e universal (`multipart/form-data`), sem SDK ou protocolo proprietário. A API responde JSON consistente. Para um arquivo pequeno e um cliente, a experiência é imediata.

**Fragilidades.** Não há *buffer* entre chegada e execução, portanto não há como absorver picos. Não há limite de concorrência, então a saturação é repassada ao cliente como latência. Não há idempotência: reenviar o mesmo arquivo cria trabalho duplicado com nome novo. Não há confirmação de processamento dissociada da conexão TCP — se a conexão cai, o resultado do trabalho é indeterminado para o cliente.

**Risco.** Acima de ~30 s de processamento, proxies e navegadores começam a encerrar a conexão antes da resposta. Nossa curva mostra que isso ocorre em vídeos acima de ~8 minutos.

**Esforço de correção.** Médio-alto — introduzir broker, worker e protocolo de aceite assíncrono.

**Maturidade: 1/5**

## Dimensão 4 — Concorrência e consistência

**Referência.** Identificadores únicos (UUID), isolamento de área de trabalho por execução, idempotência registrada, e — na oficina — controle explícito de processo com `saga_process`.

**Observado.** Documentado em detalhe na seção 6. A identidade do trabalho tem resolução de um segundo e é usada como nome de diretório e de arquivo. O Gin processa requisições em *goroutines* paralelas, mas o recurso compartilhado (o diretório) não tem qualquer proteção: nem mutex, nem lock de arquivo, nem sufixo aleatório.

**Pontos positivos.** O paralelismo do runtime funciona: as oito requisições foram atendidas concorrentemente em 18,6 s de tempo total, e o FFmpeg explora múltiplos núcleos. A infraestrutura de concorrência existe — falta apenas o isolamento de recursos.

**Fragilidades.** Três modos de falha distintos foram reproduzidos: quadros de um vídeo entregues no ZIP de outro cliente (perda de confidencialidade e integridade); remoção do diretório durante a escrita do FFmpeg (falha do trabalho alheio); sobrescrita do arquivo de upload quando nome e segundo coincidem. O nome do ZIP também colide, de forma que o último a escrever vence silenciosamente.

**Risco.** **Crítico.** Este é o único achado que produz corrupção silenciosa e cruzamento de dados entre usuários. Em produção, apareceria como reclamação esporádica e praticamente irreprodutível — a pior categoria de defeito.

**Esforço de correção.** Baixo para mitigar (UUID por trabalho isola o diretório e o arquivo), alto para resolver adequadamente (fila com concorrência controlada e propriedade exclusiva de recursos).

**Maturidade: 0/5**

## Dimensão 5 — Persistência e durabilidade

**Referência.** PostgreSQL com migrações versionadas (Flyway), propriedade de dados por serviço, e armazenamento de objetos para binários.

**Observado.** Nenhum mecanismo de persistência além do sistema de arquivos do container, sem volume (`Mounts=[]`) e com `--rm` no comando documentado. O "esquema de dados" é a convenção de nomes dos três diretórios.

**Pontos positivos.** A escolha de manter os binários fora da memória (streaming para disco e depois para o ZIP) evita estourar RAM em arquivos grandes: o consumo máximo observado foi 607 MB processando 483 quadros. É uma decisão correta que se mantém em qualquer arquitetura futura.

**Fragilidades.** Perda total na destruição do container — observada, não hipotética: o inventário voltou a `{"files":null,"total":0}` e cerca de 375 MB de ZIPs desapareceram quando o processo foi encerrado. Sem metadados, não é possível saber quem enviou o quê, quando, com que resultado. Sem transação, `outputs/` pode conter um ZIP cuja origem falhou parcialmente (foi o caso do arquivo contaminado).

**Risco.** Qualquer reinício, atualização, evicção de pod ou reciclagem de nó apaga os resultados dos clientes. Não há backup possível porque não há fonte de verdade.

**Esforço de correção.** Médio — banco para metadados, volume ou armazenamento de objetos para binários.

**Maturidade: 0/5**

## Dimensão 6 — Segurança

**Referência.** Autenticação por usuário e senha com JWT, autorização por escopo de proprietário, validação de conteúdo, segredos externalizados, contêineres sem privilégio.

**Observado.** Consolidado na tabela abaixo. Cada item foi verificado empiricamente, sem credencial alguma.

| ID | Achado | Severidade | Verificação |
|----|--------|-----------|-------------|
| S1 | Nenhuma autenticação nas seis rotas | Crítica | Todas as requisições deste relatório foram anônimas |
| S2 | `/api/status` expõe o inventário de todos os usuários | Crítica | HTTP 200, lista completa com nomes e tamanhos |
| S3 | ZIPs de terceiros baixáveis sem credencial | Crítica | `GET /download/{zip}` → 200, 6 055 887 bytes |
| S4 | **Vídeos originais de terceiros baixáveis** | Crítica | `GET /uploads/{arquivo}` → 200, 5 727 772 bytes, `video/mp4` |
| S5 | Vazamento de conteúdo entre requisições pela corrida | Crítica | 12 quadros alheios em ZIP entregue (seção 6) |
| S6 | `Access-Control-Allow-Origin: *` | Alta | Cabeçalho observado em `GET /` |
| S7 | Validação apenas por extensão do nome | Alta | `.txt` rejeitado; `fake.mp4` com 11 bytes de texto aceito e entregue ao FFmpeg |
| S8 | Sem limite de tamanho, duração ou cota | Alta | ZIP de 367,7 MB aceito; entrada de 68 MB processada |
| S9 | FFmpeg sem timeout nem cancelamento | Alta | Requisição de 67 s observada; `context` não é usado |
| S10 | `stderr` do FFmpeg devolvido ao cliente | Média | 3 888 bytes de saída interna, incluindo caminhos e configuração de build |
| S11 | Container roda como **root**, sem healthcheck | Média | `User=''`, `Healthcheck=null` |
| S12 | Gin em modo debug e confiando em todos os proxies | Média | Avisos no log de arranque |
| S13 | Nome do arquivo do cliente concatenado no caminho | Média | `timestamp + header.Filename` sem sanitização |
| S14 | `go run` como processo principal (toolchain em produção) | Média | `CMD ["go","run","main.go"]` |

Mapeando ao OWASP API Security Top 10, os achados concentram-se em **API1 (autorização de objeto quebrada)**, **API2 (autenticação quebrada)**, **API3 (exposição excessiva de dados)** e **API4 (ausência de limitação de recursos)**.

**Pontos positivos.** Há uma verificação de extensão explícita, que rejeita `.txt` corretamente com HTTP 400 em 12 ms. As tentativas de travessia de caminho (`/download/../etc/passwd`, `/outputs/../main.go` e variantes codificadas) retornaram 404 — o roteamento do Gin não permitiu escapar dos diretórios servidos. O código não constrói SQL nem executa shell com entrada do usuário: o `exec.Command` passa argumentos em vetor, sem interpretação por shell, o que evita injeção de comando.

**Fragilidades.** A ausência de identidade é o eixo: sem usuário não há dono, e sem dono todo controle de acesso é impossível por construção. O achado S4 merece ênfase — não é apenas o resultado derivado que vaza, é **o material original enviado pelo cliente**, cujo nome é previsível (`{timestamp}_{nome original}`) e cujos timestamps são revelados por `/api/status`.

**Risco.** Qualquer pessoa com acesso de rede ao serviço lê e baixa todo o conteúdo de todos os usuários.

**Esforço de correção.** Médio — autenticação, escopo por proprietário, validação de conteúdo real, limites e endurecimento do container são trabalho conhecido e bem delimitado.

**Maturidade: 0/5**

## Dimensão 7 — Confiabilidade e resiliência

**Referência.** Timeouts, política de retentativa, fila de mensagens mortas, compensação transacional e recuperação após reinício.

**Observado.** Erros são capturados e devolvidos ao cliente, mas não existe timeout, retentativa, circuit breaker ou recuperação. O teste de interrupção (container encerrado 12 s dentro de um trabalho de 483 quadros) produziu: `curl: (52) Empty reply from server`, nenhum registro de trabalho, perda do ZIP de 65,7 MB já concluído de um trabalho anterior, e desaparecimento do container.

**Pontos positivos.** O tratamento de erro no código é consistente: cada operação de I/O tem seu `if err != nil` e produz mensagem específica. A limpeza por `defer os.RemoveAll(tempDir)` demonstra intenção correta de compensação — o problema é o escopo do recurso, não a ideia. O `os.Remove(videoPath)` condicionado ao sucesso evita descartar a entrada de um trabalho que falhou.

**Fragilidades.** A compensação de um trabalho interfere em outro (seção 6). Na falha, a entrada não é liberada, acumulando lixo. Não há estado para retomar: um trabalho interrompido simplesmente deixa de existir. O cold start de 13,26 s prolonga qualquer indisponibilidade.

**Risco.** Toda falha é definitiva e silenciosa para o operador; toda reinicialização perde resultados entregues.

**Esforço de correção.** Médio-alto — depende de persistência e fila para ter onde registrar e reprocessar.

**Maturidade: 1/5**

## Dimensão 8 — Performance e escalabilidade

**Referência.** Escala horizontal por réplicas sem estado, com armazenamento compartilhado e trabalho distribuído por fila.

**Observado.** Duas medições independentes: custo por duração de vídeo e consumo de recursos ao longo do tempo.

![Curva de custo por duração do vídeo](diagrams/svg/cost-curve.svg)

| Duração | Entrada | Quadros | ZIP | Tempo de resposta |
|---------|---------|---------|-----|-------------------|
| 12 s | 3,43 MB | 12 | 6,06 MB | 0,91 s |
| 40 s | 5,46 MB | 40 | 21,59 MB | 2,25 s |
| 120 s | 16,38 MB | 121 | 65,72 MB | 6,17 s |
| 240 s | 32,77 MB | 241 | 131,91 MB | 12,92 s |
| 480 s | 65,53 MB | 483 | 265,34 MB | 29,60 s |
| ~817 s | — | 817 | 367,70 MB | 67,00 s |

O comportamento é estritamente linear: **um quadro por segundo de vídeo**, **~0,55 MB de ZIP por segundo de vídeo** e **~0,062 s de processamento por quadro** (cerca de 6% da duração do material). Extrapolando: uma hora de vídeo produz ~3 600 quadros, ~2 GB de ZIP e ~3,7 minutos de processamento.

![CPU do container durante cinco processamentos](diagrams/svg/cpu-timeline.svg)

| Métrica | Repouso | Durante processamento |
|---------|---------|-----------------------|
| CPU | 0,00% | **475,66%** de 600% (~79% do host) |
| Memória | 202,3 MB | 607,2 MB |
| PIDs | 19 | 53 |

A curva revela duas fases distintas: o FFmpeg satura múltiplos núcleos (até 29 threads), e a compactação subsequente fica presa em ~100% — um único núcleo, porque `archive/zip` é sequencial.

**Pontos positivos.** O desempenho de um trabalho isolado é bom e previsível. O FFmpeg é usado de forma eficiente, explorando paralelismo interno sem configuração adicional. A linearidade torna a capacidade fácil de projetar. O uso de memória é modesto e não cresce com o tamanho do arquivo de forma perigosa.

**Fragilidades.** Não existe folga de CPU: um trabalho já consome 79% da máquina, de modo que "processar vários vídeos ao mesmo tempo" nesta instância significa apenas redistribuir o mesmo recurso, com a degradação de 4x que medimos. A escala horizontal é inviável por três razões acumuladas — o disco é local, os identificadores não são globalmente únicos e não há coordenação de trabalho. A compactação single-thread desperdiça capacidade justamente quando o FFmpeg liberou os núcleos. Aplicar Deflate sobre PNG (já comprimido) gasta CPU para ganho marginal.

**Risco.** A capacidade máxima é de aproximadamente um vídeo longo por instância; picos degradam todos os clientes simultaneamente; o volume de saída cresce sem limite (491,8 MB em uma hora de uso exploratório).

**Esforço de correção.** Médio — extrair o processamento para workers escaláveis, com armazenamento compartilhado e concorrência controlada.

**Maturidade: 1/5**

## Dimensão 9 — Observabilidade

**Referência.** Métricas técnicas e de negócio, logs estruturados com identificador de correlação, rastreamento distribuído, alertas e verificação de saúde (Fase 3, com New Relic).

**Observado.** O log de acesso do Gin (método, status, latência, IP) e três `fmt.Printf` no fluxo de processamento. Nada além disso.

**Pontos positivos.** O log de acesso do Gin é, por si só, útil: foi a partir dele que identificamos a requisição de 67 s. As mensagens de progresso informam quantidade de quadros e criação do ZIP, o que ajuda no diagnóstico manual. A latência registrada por requisição é uma métrica técnica legítima e já disponível.

**Fragilidades.** Não existe identificador de correlação — com requisições concorrentes compartilhando o mesmo timestamp, é impossível reconstruir qual linha de log pertence a qual cliente (as mensagens da rajada são indistinguíveis entre si). Não há métrica de negócio: vídeos processados, taxa de falha, duração por etapa, tamanho médio. Não há endpoint de saúde, o que impede *probes* de liveness/readiness. Não há alerta possível: a falha de um trabalho não produz nenhum sinal fora do corpo da resposta HTTP.

**Risco.** Falhas e corrupção passam despercebidas — exatamente o que ocorreu com o ZIP contaminado, que do ponto de vista do servidor foi um sucesso.

**Esforço de correção.** Baixo-médio — log estruturado com correlação, métricas e health são incrementais.

**Maturidade: 1/5**

## Dimensão 10 — Testabilidade e qualidade

**Referência.** Testes unitários e de integração com cobertura mínima aferida (JaCoCo ≥ 80% na oficina), BDD para fluxos de negócio, análise estática com *quality gate*.

**Observado.** Nenhum arquivo `*_test.go`. Nenhuma configuração de linter, análise estática ou cobertura. Nenhuma coleção de testes de API.

**Pontos positivos.** Três funções são testáveis como estão, sem refatoração: `isValidVideoFile()` é uma função pura, e `createZipFile()`/`addFileToZip()` operam sobre caminhos passados como parâmetro, o que permite usar diretórios temporários. Há, portanto, um ponto de partida imediato para os primeiros testes.

**Fragilidades.** O problema não é a ausência de testes, é a **resistência ao teste**. `processVideo()` invoca `exec.Command("ffmpeg")` diretamente: testá-la exige FFmpeg instalado, um vídeo real e tolerância a variações de codificação. `handleVideoUpload()` mistura HTTP e disco, exigindo servidor e sistema de arquivos reais. Sem interface para o processador ou para o armazenamento, não há ponto de injeção — nenhum dublê é possível.

O impacto é concreto e mensurável neste relatório: **o defeito de concorrência da seção 6 é trivialmente detectável por um teste** que dispare duas execuções no mesmo segundo e verifique a contagem de quadros. Ele sobreviveu porque não existe nenhum teste e porque o desenho não permite escrevê-lo com facilidade.

**Risco.** Nenhuma regressão é detectável antes de produção; a corretude depende de inspeção manual.

**Esforço de correção.** Alto — requer as fronteiras da Dimensão 1 para ser feito adequadamente.

**Maturidade: 0/5**

## Dimensão 11 — Engenharia de entrega

**Referência.** Build multi-stage com imagem de runtime enxuta, configuração por ambiente (12-factor), pipeline de integração contínua com testes e publicação de imagem versionada.

**Observado.** Um `Dockerfile` de estágio único, autodeclarado como contraexemplo:

```1:2:projeto-fiapx/Dockerfile
# DOCKERFILE SIMPLES (sem boas práticas - propositalmente!)
# Este é um exemplo de como NÃO fazer um Dockerfile
```

| Aspecto | Situação | Consequência medida |
|---------|----------|---------------------|
| Base | `golang:1.21-alpine` (SDK) | 256 MB só de toolchain na imagem final |
| Estágios | 1 | 852 MB totais |
| Dependências | `go mod tidy` no build | camada de 237 MB; sem cache eficiente |
| Processo principal | `go run main.go` | compila a cada boot → cold start de 13,26 s |
| Usuário | root | S11 |
| Healthcheck | ausente | impede probes de orquestrador |
| `.dockerignore` | ausente | `__MACOSX` e artefatos locais entram na imagem |
| Configuração | nenhuma variável de ambiente | porta, fps e caminhos exigem recompilar |
| CI/CD | inexistente | build e execução manuais |

**Pontos positivos.** O build é **reprodutível e autocontido**: um `docker build` seguido de `docker run` sobe o sistema com FFmpeg na versão correta, sem instalar nada no host. Isso tem valor real — foi o que permitiu conduzir toda esta análise sem Go nem FFmpeg instalados na máquina. A imagem é fixada em versão de base (`golang:1.21-alpine`) e o `go.sum` garante integridade das 36 dependências do grafo.

**Fragilidades.** A imagem é ~50x maior do que um binário Go estático com FFmpeg exigiria, o que encarece registro, transferência e tempo de *pull* em escala. O `go run` mantém o compilador no ambiente de execução — ampliando a superfície de ataque e tornando o arranque lento, o que é especialmente ruim para autoescala e para recuperação de falha. A ausência de configuração externalizada viola diretamente o princípio de configuração do 12-factor: não há como apontar para outro armazenamento ou mudar a taxa de quadros sem alterar código.

**Risco.** Implantação lenta, recuperação lenta, custo de infraestrutura desnecessário e impossibilidade de promover o mesmo artefato entre ambientes com configurações distintas.

**Esforço de correção.** Baixo — multi-stage com `distroless`/`alpine`, binário compilado, usuário não-root, healthcheck e leitura de variáveis de ambiente são mudanças pontuais de alto retorno.

**Maturidade: 1/5**

## Dimensão 12 — Contrato de API

**Referência.** Semântica HTTP correta, contrato documentado (OpenAPI/Swagger), versionamento e coleção de testes (Postman) — todos presentes nas fases anteriores.

**Observado.** Quatro rotas funcionais mais duas estáticas, respostas em JSON com estrutura estável.

**Pontos positivos.** O formato de resposta é consistente e autoexplicativo: `success`, `message` e, no sucesso, `zip_path`, `frame_count` e `images`. O erro de validação de entrada usa corretamente **HTTP 400**, e o arquivo inexistente em `/download/:filename` usa corretamente **HTTP 404**. Os cabeçalhos de download são apropriados (`Content-Disposition: attachment`, `Content-Type: application/zip`). O `Content-Type` do vídeo servido estaticamente também é correto (`video/mp4`).

**Fragilidades.** A falha de processamento retorna **HTTP 200 com `success: false`** — comprovado em todos os cenários de erro da rajada e no `fake.mp4`. Um cliente que verifique apenas o código de status conclui que o trabalho deu certo. Esse é o mecanismo pelo qual a corrida da seção 6 permanece invisível. O `stderr` completo do FFmpeg no campo `message` (3 888 bytes) mistura diagnóstico interno com contrato público. Não existe OpenAPI, versionamento (`/v1`) nem paginação em `/api/status`, que devolve a lista inteira. O DTO `VideoRequest` está declarado mas não é usado por nenhum handler — código morto no contrato.

**Risco.** Integrações construídas sobre esta API tratarão falhas como sucesso.

**Esforço de correção.** Baixo — mapear erros para 4xx/5xx, sanitizar mensagens e publicar contrato.

**Maturidade: 2/5**

## Dimensão 13 — Manutenibilidade e evolutividade

**Referência.** Módulos com fronteiras claras, front-end desacoplado, dívida técnica registrada e decisões documentadas em ADR/RFC.

**Observado.** 437 linhas legíveis, mas com 33% dedicadas a HTML/CSS/JavaScript dentro de uma string Go. Nenhum README, documentação de arquitetura, ADR ou registro de decisão acompanha o projeto.

**Pontos positivos.** O tamanho reduzido é uma vantagem genuína: o sistema inteiro é auditável em uma sessão, sem ferramenta de navegação de código — foi o que permitiu produzir este diagnóstico com precisão. Os nomes de função descrevem intenção. O comentário autodepreciativo no `Dockerfile` demonstra que as limitações são conscientes e intencionais para o contexto de demonstração, o que é preferível a dívida oculta.

**Fragilidades.** Alterar o CSS de um botão exige recompilar e reimplantar o serviço que executa o FFmpeg. O front-end não tem build, lint, minificação ou versionamento independente. Os *hotspots* estão concentrados: `processVideo()` e `handleVideoUpload()` acumulam a maior parte da complexidade e a totalidade dos defeitos observados, e qualquer evolução passa por elas. A ausência de configuração externa significa que cada ambiente exige um binário diferente.

**Risco.** O custo de cada mudança futura é desproporcional ao seu tamanho, e a falta de fronteiras faz com que correções em uma dimensão exijam tocar código de outra.

**Esforço de correção.** Médio — separar a interface e externalizar configuração são passos independentes e de baixo risco.

**Maturidade: 1/5**

---

# Parte IV — Consolidação

## 14. Pontos fortes do baseline

Esta seção não é cortesia: são decisões e propriedades que devem ser **preservadas** em qualquer evolução, e que reduzem o esforço futuro.

**A escolha tecnológica é adequada ao problema.** Go oferece concorrência nativa e binário único; Gin é leve e suficiente para a superfície necessária; FFmpeg é a ferramenta correta para extração de quadros e foi invocado com os parâmetros apropriados (`-vf fps=1`, `-y`). Nada aqui precisa ser trocado por razões técnicas.

**O empacotamento é reprodutível.** `docker build` seguido de `docker run` entrega o sistema funcionando, com a versão exata do FFmpeg, sem instalar dependências no host. Toda esta análise foi conduzida sem Go nem FFmpeg na máquina — o valor prático disso é considerável.

**O processamento evita carregar os dados em memória.** Vídeo e quadros transitam por disco; o pico de memória em 483 quadros foi de 607 MB. É a decisão correta e permanece válida em qualquer arquitetura futura.

**A invocação de processo externo é segura contra injeção.** `exec.Command` recebe argumentos em vetor, sem interpretação por shell. Não há concatenação de entrada do usuário em linha de comando.

**O tratamento de erro é disciplinado no nível da função.** Cada operação de I/O verifica seu erro e produz mensagem específica. A intenção de compensação existe (`defer os.RemoveAll`, `os.Remove` condicionado ao sucesso) — o que falta é escopo de recurso, não cuidado.

**Existem funções prontas para reuso e teste.** `isValidVideoFile()`, `createZipFile()` e `addFileToZip()` são coesas e podem ser aproveitadas praticamente como estão.

**Há proteções pontuais funcionando.** A validação de extensão rejeita corretamente arquivos não suportados; as tentativas de travessia de caminho falharam; os códigos 400 e 404 são usados adequadamente nos casos em que aparecem.

**As limitações são intencionais e declaradas.** O `Dockerfile` avisa explicitamente que não segue boas práticas. Um protótipo honesto sobre seu escopo é um ponto de partida melhor do que um sistema que aparenta maturidade que não tem.

**O tamanho é uma vantagem.** 437 linhas auditáveis integralmente significam que a evolução pode ser feita com entendimento completo do comportamento atual — situação rara.

## 15. Matriz de risco

Severidade considera impacto sobre confidencialidade, integridade e disponibilidade. Probabilidade considera a chance de manifestação em uso real com múltiplos usuários.

| # | Risco | Dim. | Severidade | Probabilidade | Evidência |
|---|-------|------|-----------|---------------|-----------|
| R1 | Quadros de um cliente entregues no ZIP de outro | 4 | **Crítica** | Alta sob concorrência | `18-zip-contaminacao.txt`, `PROVA-zip-contaminado-24frames.zip` |
| R2 | Qualquer pessoa lista e baixa artefatos de todos | 6 | **Crítica** | Certa | `01-status.json`, `03-download.txt`, `04-static-outputs.txt` |
| R3 | Vídeo original de terceiros baixável sem credencial | 6 | **Crítica** | Certa | `26-uploads-expostos.txt` (HTTP 200, 5,7 MB) |
| R4 | Perda total de resultados ao reiniciar o container | 5 | **Crítica** | Certa | `16-status-inicial-pos-perda.json`, `21-teste-interrupcao.txt` |
| R5 | Falha de processamento reportada como HTTP 200 | 12 | Alta | Certa | `14-burst-8.txt`, `07-fake-mp4-curl.txt` |
| R6 | Perda de trabalho sob rajada (62,5% medido) | 4 | Alta | Alta | `14-burst-8.txt` |
| R7 | Trabalho interrompido é irrecuperável | 7 | Alta | Média | `21-teste-interrupcao.txt` |
| R8 | Esgotamento de disco por acúmulo sem cota | 5, 8 | Alta | Alta | `23-disco-final.txt` (491,8 MB/hora de uso) |
| R9 | Saturação de CPU por requisição única | 8 | Alta | Alta | `19-stats-curva.csv` (475,66%) |
| R10 | Timeout de cliente em vídeos longos | 3 | Alta | Alta | curva: 67 s para ~13,6 min |
| R11 | Conteúdo malicioso entregue ao FFmpeg | 6 | Média | Média | `07-fake-mp4-curl.txt` |
| R12 | Vazamento de diagnóstico interno ao cliente | 6, 12 | Média | Certa | 3 888 bytes de `stderr` na resposta |
| R13 | Container root sem healthcheck | 6, 11 | Média | Certa | `10-image-config.txt` |
| R14 | Regressão indetectável por ausência de testes | 10 | Média | Certa | nenhum `*_test.go` |
| R15 | Recuperação lenta (cold start 13,26 s) | 11 | Baixa | Certa | `22-cold-start.txt` |

Quatro riscos críticos, dos quais três são certos (não dependem de condição) e um é altamente provável sob uso concorrente.

## 16. Maturidade consolidada

![Maturidade por dimensão de engenharia](diagrams/svg/maturity-heatmap.svg)

| Dimensão | Nota | Leitura |
|----------|------|---------|
| D1 Estilo arquitetural | 1 | monólito de deploy; interior = transaction script, não Clean Architecture |
| D2 Modelo de domínio | 0 | a entidade "trabalho" não existe |
| D3 Integração | 1 | apenas HTTP síncrono |
| D4 Concorrência | 0 | corrompe e vaza dados sob carga |
| D5 Persistência | 0 | filesystem efêmero |
| D6 Segurança | 0 | sem identidade, artefatos públicos |
| D7 Confiabilidade | 1 | erros tratados, nada recuperável |
| D8 Performance | 1 | bom isolado, sem folga nem escala |
| D9 Observabilidade | 1 | log de acesso apenas |
| D10 Testabilidade | 0 | zero testes e desenho resistente a teste |
| D11 Entrega | 1 | reprodutível, porém 852 MB e `go run` |
| D12 Contrato de API | 2 | JSON consistente, semântica incorreta |
| D13 Manutenibilidade | 1 | pequeno, mas front acoplado ao back |

**Média: 0,7 / 5.**

A leitura relevante não é a média, mas a distribuição. As dimensões em zero — domínio, concorrência, persistência e segurança — se sustentam mutuamente: sem entidade de trabalho não há onde registrar estado; sem persistência não há dono; sem dono não há autorização; sem identificador único não há isolamento. Atacar qualquer uma isoladamente produz ganho limitado.

Dimensões com nota 1 ou 2 (entrega, contrato, observabilidade) admitem melhoria incremental de alto retorno e baixo risco, independentemente de decisões arquiteturais maiores.

## 17. Síntese do diagnóstico

Arquiteturalmente, o baseline é um **monólito de deploy** (aceitável) cuja estrutura interna é **Transaction Script**, não Clean Architecture, não camadas e não hexagonal. Não se critica a ausência de microsserviços; critica-se a ausência de fronteiras *dentro* do único processo.

É um **protótipo de demonstração competente para o propósito com que foi feito** e inadequado para operação multiusuário — não por falta de recursos, mas porque três decisões de projeto interagem de forma destrutiva:

1. **Processamento dentro do request HTTP** transforma a conexão do cliente no orquestrador, atando a duração da operação ao timeout da rede e impedindo absorção de picos.
2. **Timestamp de um segundo como identidade** faz requisições concorrentes compartilharem área de trabalho e nome de saída, produzindo corrupção e vazamento silenciosos.
3. **Filesystem efêmero como banco de dados** elimina dono, estado, histórico e recuperação.

Nenhuma das três é um defeito de implementação a ser corrigido pontualmente: são escolhas estruturais, adequadas a um script de demonstração e incompatíveis com um produto. É precisamente por isso que o diagnóstico precede a proposta — a decisão de arquitetura deve responder a estes três pontos com evidência, e não a uma preferência tecnológica.

O que **não** precisa mudar: a linguagem, o framework HTTP, a ferramenta de extração, a estratégia de não carregar binários em memória, a reprodutibilidade do empacotamento e as funções já coesas.

## 18. Apêndices

### A. Índice de evidências

Todos os arquivos em `docs/evidence/`.

| Arquivo | Conteúdo |
|---------|----------|
| `01-status.json`, `01-status.json.txt` | Inventário global retornado sem autenticação |
| `02-home-headers.txt` | Cabeçalhos de resposta, incluindo CORS `*` |
| `03-download.txt` | Download de ZIP sem credencial (6 055 887 bytes) |
| `04-static-outputs.txt` | Mesmo arquivo obtido pela rota estática |
| `05-path-traversal.txt` | Cinco tentativas de travessia — todas 404 |
| `06-invalid-txt-curl.txt` | `.txt` rejeitado com HTTP 400 em 12 ms |
| `07-fake-mp4-curl.txt` | Extensão falsa aceita; HTTP 200 com `stderr` completo |
| `08-concurrent-*.txt` | Primeiro teste de concorrência (2 uploads) |
| `09-status-after-concurrent.json` | Inventário após a colisão inicial |
| `10-image-size.txt`, `10-image-history.txt`, `10-image-config.txt` | 852 MB, camadas, root, sem healthcheck |
| `11-dependencies.txt` | 1 dependência direta, 36 módulos no grafo |
| `12-code-structure.txt` | 437 linhas, 10 funções, 144 de HTML |
| `13-docker-stats.txt` | Consumo em repouso e após rajada |
| `14-burst-8.txt` | Rajada de 8 uploads com latência e desfecho de cada um |
| `15-status-after-burst.json` | 3 ZIPs para 8 uploads |
| `16-status-inicial-pos-perda.json`, `16-outputs-vazio.txt` | Estado zerado após perda do container |
| `17-sobras-pos-burst.txt` | Órfãos em `uploads/`, `temp/` limpo, uso de disco |
| `18-zip-contaminacao.txt` | Listagem das 24 entradas com tamanhos |
| `PROVA-zip-contaminado-24frames.zip` | O artefato contaminado (12,3 MB) |
| `19-curva-custo.txt`, `19-stats-curva.csv` | Curva de custo e amostras de CPU/memória |
| `20-status-curva.json` | Tamanhos dos ZIPs da curva |
| `21-teste-interrupcao.txt` | Interrupção no meio do job |
| `22-cold-start.txt`, `22-cold-start-detalhe.txt` | 13 260 ms até HTTP 200 |
| `23-disco-final.txt` | 491,8 MB acumulados |
| `24-macosx-na-imagem.txt` | `__MACOSX` empacotado |
| `25-directory-listing.txt` | Índice de diretório retorna 404 |
| `26-uploads-expostos.txt` | Vídeo original de terceiro baixável |
| `ui-home.png`, `page-*.png` | Interface e lista de arquivos processados |

### B. Diagramas

Fontes e renderizações em `docs/diagrams/`.

| Arquivo | Conteúdo |
|---------|----------|
| `svg/c4-container.svg` | Visão de container com componentes ausentes |
| `svg/architecture-layers.svg` | Camadas Clean Architecture versus camadas reais |
| `svg/components.svg` | Componentes internos de `main.go` com hotspots |
| `svg/sequence-race.svg` | Sequência da corrida e do vazamento |
| `svg/dataflow-lifecycle.svg` | Ciclo de vida do dado, compensação e interrupção |
| `svg/cost-curve.svg` | Custo por duração do vídeo |
| `svg/cpu-timeline.svg` | CPU e memória durante os processamentos |
| `svg/maturity-heatmap.svg` | Maturidade nas 13 dimensões |
| `c4-container-baseline.puml` | Fonte PlantUML da visão de container |
| `sequence-race-baseline.puml` | Fonte PlantUML da sequência |
| `components-baseline.mmd` | Fonte Mermaid dos componentes |

### C. Reprodução das medições

Os experimentos podem ser repetidos com o container em execução:

```powershell
docker build -t fiapx-baseline .
docker run --rm -p 8080:8080 fiapx-baseline
```

- **Corrida de concorrência**: disparar N uploads simultâneos do mesmo arquivo e comparar `frame_count` da resposta com a duração real do vídeo.
- **Exposição de artefatos**: `curl http://localhost:8080/api/status` sem cabeçalho de autorização, seguido de `curl` nos caminhos `/download/` e `/uploads/`.
- **Consumo de recursos**: `docker stats <container> --no-stream` em laço durante um upload longo.
- **Curva de custo**: gerar entradas com `ffmpeg -stream_loop N -i amostra.mp4 -c copy saida.mp4` e medir `time_total` do `curl`.
- **Interrupção**: `docker kill` durante o processamento; observar a resposta do cliente e o estado posterior.

### D. Glossário

| Termo | Significado neste documento |
|-------|------------------------------|
| **Baseline** | O projeto base fornecido, objeto desta análise |
| **Corrida (race condition)** | Defeito em que o resultado depende do intercalamento temporal de operações concorrentes |
| **Backpressure** | Mecanismo que limita a admissão de trabalho conforme a capacidade disponível |
| **Compensação** | Ação que desfaz efeitos de uma operação que falhou |
| **Cold start** | Tempo entre iniciar o processo e ele estar apto a atender requisições |
| **IDOR** | Acesso a objeto de outro usuário por manipulação de referência direta |
| **Hotspot** | Trecho de código que concentra complexidade e defeitos |
| **Efêmero** | Armazenamento cujo conteúdo desaparece com o fim do container |

---

## Histórico de revisões

| Versão | Data | Alteração |
|--------|------|-----------|
| 0.1 | 19/09/2026 | Levantamento inicial com uploads exploratórios e testes de segurança |
| **1.0** | 19/09/2026 | Reescrito como deep dive de engenharia: 13 dimensões, 7 diagramas, rajada de 8 uploads, curva de custo, amostragem de recursos, teste de interrupção, cold start e comprovação do vazamento entre requisições |



