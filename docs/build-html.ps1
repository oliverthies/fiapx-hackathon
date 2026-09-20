<#
    Converte o relatorio Markdown em uma pagina HTML autocontida e imprimivel.

    Os SVGs referenciados por ![alt](diagrams/svg/*.svg) sao embutidos como XML
    inline, o que mantem o texto dos diagramas selecionavel e nitido em qualquer
    zoom ou impressao. Nao depende de Node, Pandoc ou acesso a rede.

    Uso:  powershell -ExecutionPolicy Bypass -File docs\build-html.ps1
#>

[CmdletBinding()]
param(
    [string]$Source,
    [string]$Output,
    [string]$Title = 'Deep dive de engenharia - baseline FIAP X'
)

$ErrorActionPreference = 'Stop'

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $Source) { $Source = Join-Path $scriptDir 'RELATORIO_BASELINE_ARQUITETURA.md' }
if (-not $Output) { $Output = Join-Path $scriptDir 'RELATORIO_BASELINE_ARQUITETURA.html' }

if (-not (Test-Path $Source)) { throw "Fonte nao encontrada: $Source" }
$docRoot = Split-Path -Parent (Resolve-Path $Source)

# Os diagramas sao mantidos em diagrams/src/*.svgsrc e materializados como .svg
# aqui. A fonte separada existe porque gravar diretamente em .svg nesta maquina
# corrompe caracteres acentuados; a conversao abaixo garante UTF-8 valido.
$srcDir = Join-Path $docRoot 'diagrams\src'
$svgDir = Join-Path $docRoot 'diagrams\svg'
if (Test-Path $srcDir) {
    if (-not (Test-Path $svgDir)) { New-Item -ItemType Directory -Path $svgDir | Out-Null }
    $utf8 = New-Object Text.UTF8Encoding $false
    $converted = 0
    foreach ($s in Get-ChildItem $srcDir -Filter '*.svgsrc' -File) {
        $content = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($s.FullName))
        if ($content -match "\uFFFD") { throw "Fonte de diagrama corrompida (U+FFFD): $($s.Name)" }
        [IO.File]::WriteAllText((Join-Path $svgDir ($s.BaseName + '.svg')), $content, $utf8)
        $converted++
    }
    Write-Host "Diagramas convertidos de .svgsrc para .svg: $converted"
}

function Convert-Inline {
    param([string]$Text)

    $t = $Text -replace '&', '&amp;' -replace '<', '&lt;' -replace '>', '&gt;'
    # codigo inline primeiro, para que seu conteudo nao sofra outras substituicoes
    $t = [regex]::Replace($t, '`([^`]+)`', { param($m) '<code>' + $m.Groups[1].Value + '</code>' })
    $t = [regex]::Replace($t, '\*\*([^*]+)\*\*', '<strong>$1</strong>')
    $t = [regex]::Replace($t, '(?<![\*\w])\*([^*]+)\*(?!\*)', '<em>$1</em>')
    $t = [regex]::Replace($t, '\[([^\]]+)\]\(([^)]+)\)', '<a href="$2">$1</a>')
    return $t
}

function New-Slug {
    param([string]$Text)
    $s = $Text.ToLowerInvariant()
    $s = $s -replace '<[^>]+>', ''
    $s = $s.Normalize([Text.NormalizationForm]::FormD)
    $s = ($s.ToCharArray() | Where-Object {
        [Globalization.CharUnicodeInfo]::GetUnicodeCategory($_) -ne [Globalization.UnicodeCategory]::NonSpacingMark
    }) -join ''
    $s = $s -replace '[^a-z0-9]+', '-'
    return $s.Trim('-')
}

$lines = Get-Content -LiteralPath $Source -Encoding UTF8
$html  = [Text.StringBuilder]::new()
$toc   = [Collections.Generic.List[object]]::new()

$i = 0
$inCode = $false
$codeBuf = [Collections.Generic.List[string]]::new()
$codeMeta = ''

while ($i -lt $lines.Count) {
    $line = $lines[$i]

    # blocos de codigo
    if ($line -match '^\s*```(.*)$') {
        if (-not $inCode) {
            $inCode = $true
            $codeMeta = $Matches[1].Trim()
            $codeBuf.Clear()
        } else {
            $inCode = $false
            $body = ($codeBuf -join "`n")
            $body = $body -replace '&', '&amp;' -replace '<', '&lt;' -replace '>', '&gt;'
            $caption = ''
            if ($codeMeta -match '^(\d+):(\d+):(.+)$') {
                $caption = '<div class="code-ref">' + $Matches[3] + ' &middot; linhas ' + $Matches[1] + '-' + $Matches[2] + '</div>'
            }
            [void]$html.AppendLine('<div class="code-block">' + $caption + '<pre><code>' + $body + '</code></pre></div>')
        }
        $i++; continue
    }
    if ($inCode) { $codeBuf.Add($line); $i++; continue }

    # imagem isolada -> SVG inline
    if ($line -match '^!\[([^\]]*)\]\(([^)]+)\)\s*$') {
        $alt = $Matches[1]; $rel = $Matches[2]
        $path = Join-Path $docRoot $rel
        if (Test-Path $path) {
            $svg = Get-Content -LiteralPath $path -Raw -Encoding UTF8
            $svg = $svg -replace '<\?xml[^>]*\?>', ''
            # ajusta somente o elemento raiz: o viewBox cuida da escala, portanto
            # width/height fixos sao trocados por largura fluida. Atributos de
            # <rect>, <text> etc. precisam ser preservados intactos.
            $svg = [regex]::Replace($svg, '<svg\b[^>]*>', {
                param($m)
                $tag = $m.Value
                $tag = [regex]::Replace($tag, '\s(width|height)="[^"]*"', '')
                return ($tag -replace '<svg', '<svg width="100%"')
            }, 'IgnoreCase')
            [void]$html.AppendLine('<figure class="diagram">' + $svg.Trim() + '<figcaption>' + (Convert-Inline $alt) + '</figcaption></figure>')
        } else {
            [void]$html.AppendLine('<p class="missing">Diagrama ausente: ' + $rel + '</p>')
        }
        $i++; continue
    }

    # tabelas
    if ($line -match '^\s*\|' -and ($i + 1) -lt $lines.Count -and $lines[$i+1] -match '^\s*\|[\s:\-\|]+\|\s*$') {
        $headerCells = ($line.Trim().Trim('|') -split '(?<!\\)\|') | ForEach-Object { $_.Trim() }
        $aligns = ($lines[$i+1].Trim().Trim('|') -split '(?<!\\)\|') | ForEach-Object {
            $c = $_.Trim()
            if ($c.StartsWith(':') -and $c.EndsWith(':')) { 'center' }
            elseif ($c.EndsWith(':')) { 'right' }
            else { 'left' }
        }
        [void]$html.AppendLine('<div class="table-wrap"><table><thead><tr>')
        for ($c = 0; $c -lt $headerCells.Count; $c++) {
            $a = if ($c -lt $aligns.Count) { $aligns[$c] } else { 'left' }
            [void]$html.AppendLine('<th style="text-align:' + $a + '">' + (Convert-Inline $headerCells[$c]) + '</th>')
        }
        [void]$html.AppendLine('</tr></thead><tbody>')
        $i += 2
        while ($i -lt $lines.Count -and $lines[$i] -match '^\s*\|') {
            $cells = ($lines[$i].Trim().Trim('|') -split '(?<!\\)\|') | ForEach-Object { $_.Trim() -replace '\\\|', '|' }
            [void]$html.AppendLine('<tr>')
            for ($c = 0; $c -lt $cells.Count; $c++) {
                $a = if ($c -lt $aligns.Count) { $aligns[$c] } else { 'left' }
                [void]$html.AppendLine('<td style="text-align:' + $a + '">' + (Convert-Inline $cells[$c]) + '</td>')
            }
            [void]$html.AppendLine('</tr>')
            $i++
        }
        [void]$html.AppendLine('</tbody></table></div>')
        continue
    }

    # titulos
    if ($line -match '^(#{1,4})\s+(.*)$') {
        $level = $Matches[1].Length
        $text  = Convert-Inline $Matches[2]
        $slug  = New-Slug $Matches[2]
        if ($level -le 2) { $toc.Add([pscustomobject]@{ Level = $level; Text = $text; Slug = $slug }) }
        # o titulo do documento tambem e h1, mas nao deve receber o estilo de divisor de parte
        $cls = if ($level -eq 1 -and $Matches[2] -match '^Parte\b') { ' class="part"' } else { '' }
        [void]$html.AppendLine("<h$level id=`"$slug`"$cls>$text</h$level>")
        $i++; continue
    }

    # regua
    if ($line -match '^\s*---\s*$') { [void]$html.AppendLine('<hr/>'); $i++; continue }

    # listas
    if ($line -match '^\s*[-*]\s+(.*)$') {
        [void]$html.AppendLine('<ul>')
        while ($i -lt $lines.Count -and $lines[$i] -match '^\s*[-*]\s+(.*)$') {
            [void]$html.AppendLine('<li>' + (Convert-Inline $Matches[1]) + '</li>')
            $i++
        }
        [void]$html.AppendLine('</ul>')
        continue
    }
    if ($line -match '^\s*\d+\.\s+(.*)$') {
        [void]$html.AppendLine('<ol>')
        while ($i -lt $lines.Count -and $lines[$i] -match '^\s*\d+\.\s+(.*)$') {
            [void]$html.AppendLine('<li>' + (Convert-Inline $Matches[1]) + '</li>')
            $i++
        }
        [void]$html.AppendLine('</ol>')
        continue
    }

    # paragrafo
    if ($line.Trim() -ne '') {
        $buf = [Collections.Generic.List[string]]::new()
        while ($i -lt $lines.Count -and $lines[$i].Trim() -ne '' -and
               $lines[$i] -notmatch '^(#{1,4}\s|\s*```|\s*\||\s*[-*]\s|\s*\d+\.\s|!\[)' -and
               $lines[$i] -notmatch '^\s*---\s*$') {
            $buf.Add($lines[$i].Trim()); $i++
        }
        if ($buf.Count -gt 0) {
            [void]$html.AppendLine('<p>' + (Convert-Inline ($buf -join ' ')) + '</p>')
            continue
        }
    }
    $i++
}

$tocHtml = [Text.StringBuilder]::new()
foreach ($t in $toc) {
    $c = if ($t.Level -eq 1) { 'toc-part' } else { 'toc-sec' }
    [void]$tocHtml.AppendLine('<a class="' + $c + '" href="#' + $t.Slug + '">' + $t.Text + '</a>')
}

$css = @'
:root{--ink:#1a1a1a;--muted:#5f6b76;--line:#e3e6ea;--accent:#1f4e79;--crit:#b02a37;--warn:#9a6700;--good:#1f7a3d;--bg:#ffffff;--soft:#f6f8fa}
*{box-sizing:border-box}
html{scroll-behavior:smooth}
body{margin:0;background:#eef1f4;color:var(--ink);font:16px/1.65 "Segoe UI",Calibri,system-ui,sans-serif;-webkit-font-smoothing:antialiased}
.shell{display:grid;grid-template-columns:284px minmax(0,1fr);gap:0;max-width:1440px;margin:0 auto;background:var(--bg);box-shadow:0 0 34px rgba(0,0,0,.09);min-height:100vh}
nav{position:sticky;top:0;align-self:start;height:100vh;overflow-y:auto;padding:26px 18px 40px;background:#12263a;color:#cfd9e3}
nav .brand{font-size:12px;letter-spacing:.11em;text-transform:uppercase;color:#7f9cb8;margin-bottom:4px}
nav .brandsub{font-size:15px;font-weight:600;color:#fff;line-height:1.35;margin-bottom:22px}
nav a{display:block;text-decoration:none;color:#cfd9e3;padding:5px 9px;border-radius:4px;font-size:13.2px;line-height:1.4}
nav a:hover{background:rgba(255,255,255,.1);color:#fff}
nav a.toc-part{margin-top:15px;font-weight:700;color:#fff;font-size:12.4px;letter-spacing:.05em;text-transform:uppercase;border-left:3px solid #4a90c2;padding-left:9px;border-radius:0}
nav a.toc-sec{padding-left:16px;color:#aebccb}
main{padding:44px 58px 90px;max-width:1060px;min-width:0}
h1.part{font-size:15px;letter-spacing:.14em;text-transform:uppercase;color:var(--accent);border-bottom:2.5px solid var(--accent);padding-bottom:9px;margin:58px 0 26px}
h1.part:first-of-type{margin-top:0}
h1:not(.part){font-size:31px;line-height:1.24;margin:0 0 6px;letter-spacing:-.4px}
h2{font-size:22.5px;margin:44px 0 12px;padding-bottom:7px;border-bottom:1px solid var(--line);line-height:1.3}
h3{font-size:17.5px;margin:30px 0 9px;color:#26384a}
h4{font-size:15px;margin:22px 0 7px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em}
p{margin:0 0 13px}
strong{font-weight:650}
a{color:var(--accent)}
hr{border:0;border-top:1px solid var(--line);margin:40px 0}
ul,ol{margin:0 0 15px;padding-left:23px}
li{margin-bottom:6px}
code{font-family:Consolas,"Cascadia Mono",monospace;font-size:.875em;background:var(--soft);border:1px solid #e1e6eb;border-radius:3px;padding:1px 5px;color:#7b2d6b}
.code-block{margin:0 0 20px;border:1px solid var(--line);border-radius:6px;overflow:hidden;background:#1e2733}
.code-ref{font:600 11.5px/1 Consolas,monospace;color:#9fb6cd;background:#16202b;padding:8px 13px;border-bottom:1px solid #2a3947;letter-spacing:.03em}
.code-block pre{margin:0;padding:15px 17px;overflow-x:auto}
.code-block code{background:none;border:0;padding:0;color:#dbe6f0;font-size:12.9px;line-height:1.6}
.table-wrap{overflow-x:auto;margin:0 0 22px;border:1px solid var(--line);border-radius:6px}
table{border-collapse:collapse;width:100%;font-size:13.7px}
th{background:#f0f3f6;font-weight:650;text-align:left;padding:10px 13px;border-bottom:2px solid #d6dce3;white-space:nowrap;color:#26384a}
td{padding:9px 13px;border-bottom:1px solid #eef1f4;vertical-align:top}
tbody tr:last-child td{border-bottom:0}
tbody tr:nth-child(even){background:#fbfcfd}
figure.diagram{margin:26px 0 30px;padding:16px 16px 10px;border:1px solid var(--line);border-radius:7px;background:#fff;box-shadow:0 1px 4px rgba(0,0,0,.05)}
figure.diagram svg{display:block;height:auto;max-width:100%}
figcaption{margin-top:11px;padding-top:9px;border-top:1px solid var(--line);font-size:12.4px;color:var(--muted);text-align:center;font-style:italic}
.missing{color:var(--crit);font-style:italic}
td:first-child code{white-space:nowrap}
@media print{
  body{background:#fff;font-size:10.6pt}
  .shell{display:block;max-width:none;box-shadow:none}
  nav{display:none}
  main{padding:0;max-width:none}
  h1.part{page-break-before:always;margin-top:0}
  h1.part:first-of-type{page-break-before:avoid}
  h2,h3{page-break-after:avoid}
  figure.diagram,.table-wrap,.code-block{page-break-inside:avoid}
  tr{page-break-inside:avoid}
  a{color:var(--ink);text-decoration:none}
  @page{margin:14mm 13mm}
}
@media(max-width:1020px){
  .shell{grid-template-columns:1fr}
  nav{position:static;height:auto}
  main{padding:28px 20px 60px}
}
'@

$final = @"
<!DOCTYPE html>
<html lang="pt-BR">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>$Title</title>
<style>
$css
</style>
</head>
<body>
<div class="shell">
<nav>
<div class="brand">FIAP X &middot; Hackathon</div>
<div class="brandsub">Deep dive de engenharia<br/>baseline as-is</div>
$($tocHtml.ToString())
</nav>
<main>
$($html.ToString())
</main>
</div>
</body>
</html>
"@

# UTF-8 sem BOM
[IO.File]::WriteAllText($Output, $final, (New-Object Text.UTF8Encoding $false))

$kb = [math]::Round((Get-Item $Output).Length / 1KB, 1)
Write-Host "HTML gerado: $Output ($kb KB)"
Write-Host "Secoes no sumario: $($toc.Count)"
