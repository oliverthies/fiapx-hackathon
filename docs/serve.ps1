<#
    Servidor estatico minimo para pre-visualizar o relatorio HTML no navegador.
    Necessario porque o navegador integrado nao abre URLs file://.

    Uso:  powershell -ExecutionPolicy Bypass -File docs\serve.ps1 [-Port 8090]
    Encerre com Ctrl+C.
#>
param([int]$Port = 8090)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

$types = @{
    '.html' = 'text/html; charset=utf-8'
    '.svg'  = 'image/svg+xml; charset=utf-8'
    '.md'   = 'text/plain; charset=utf-8'
    '.css'  = 'text/css; charset=utf-8'
    '.png'  = 'image/png'
    '.json' = 'application/json; charset=utf-8'
    '.txt'  = 'text/plain; charset=utf-8'
    '.csv'  = 'text/csv; charset=utf-8'
    '.zip'  = 'application/zip'
}

$listener = New-Object Net.HttpListener
$listener.Prefixes.Add("http://localhost:$Port/")
$listener.Start()
Write-Host "Servindo $root em http://localhost:$Port/"
Write-Host "Relatorio: http://localhost:$Port/RELATORIO_BASELINE_ARQUITETURA.html"

try {
    while ($listener.IsListening) {
        $ctx = $listener.GetContext()
        $rel = [Uri]::UnescapeDataString($ctx.Request.Url.AbsolutePath.TrimStart('/'))
        if ([string]::IsNullOrWhiteSpace($rel)) { $rel = 'RELATORIO_BASELINE_ARQUITETURA.html' }
        $path = Join-Path $root $rel

        # impede sair do diretorio servido
        $full = [IO.Path]::GetFullPath($path)
        if (-not $full.StartsWith([IO.Path]::GetFullPath($root), [StringComparison]::OrdinalIgnoreCase)) {
            $ctx.Response.StatusCode = 403; $ctx.Response.Close(); continue
        }

        if (Test-Path -LiteralPath $full -PathType Leaf) {
            $ext = [IO.Path]::GetExtension($full).ToLowerInvariant()
            $ctx.Response.ContentType = if ($types.ContainsKey($ext)) { $types[$ext] } else { 'application/octet-stream' }
            # o relatorio e regerado com frequencia durante a revisao
            $ctx.Response.Headers.Add('Cache-Control', 'no-store, must-revalidate')
            $bytes = [IO.File]::ReadAllBytes($full)
            $ctx.Response.ContentLength64 = $bytes.Length
            $ctx.Response.OutputStream.Write($bytes, 0, $bytes.Length)
        } else {
            $ctx.Response.StatusCode = 404
            $msg = [Text.Encoding]::UTF8.GetBytes("404 - $rel")
            $ctx.Response.OutputStream.Write($msg, 0, $msg.Length)
        }
        $ctx.Response.Close()
    }
} finally {
    $listener.Stop(); $listener.Close()
}
