<# Verificacao rapida de integridade do HTML gerado. #>
$ErrorActionPreference = 'Stop'
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$h = Get-Content (Join-Path $dir 'RELATORIO_BASELINE_ARQUITETURA.html') -Raw -Encoding UTF8

$checks = [ordered]@{
    'figure.diagram'                = [regex]::Matches($h, 'class="diagram"').Count
    'svg raiz fluido'               = [regex]::Matches($h, '<svg width="100%"').Count
    'viewBox preservado'            = [regex]::Matches($h, 'viewBox=').Count
    'rects com width (nao apagado)' = [regex]::Matches($h, '<rect [^>]*width="\d').Count
    'texts do svg'                  = [regex]::Matches($h, '<text ').Count
    'tabelas'                       = [regex]::Matches($h, '<table>').Count
    'blocos de codigo'              = [regex]::Matches($h, 'class="code-block"').Count
    'referencias de arquivo:linha'  = [regex]::Matches($h, 'class="code-ref"').Count
    'links do sumario'              = [regex]::Matches($h, 'class="toc-').Count
    'marcador NEXT residual'        = [regex]::Matches($h, 'NEXT--').Count
    'pipes de tabela vazados'       = [regex]::Matches($h, '<p>\|').Count
    'asteriscos nao convertidos'    = [regex]::Matches($h, '\*\*').Count
    'markdown de imagem vazado'     = [regex]::Matches($h, '!\[').Count
    'diagramas ausentes'            = [regex]::Matches($h, 'class="missing"').Count
}
$checks.GetEnumerator() | ForEach-Object {
    '{0,-32} {1}' -f $_.Key, $_.Value
}
