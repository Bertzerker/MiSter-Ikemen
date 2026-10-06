$ErrorActionPreference = 'Stop'
$fpga = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../fpga'))
$compiler = 'ghcr.io/raetro/quartus@sha256:55442b161163dc7723e3d4917ce046fcb682478365fe9320756c9a4ae7d7d4ce'
# Mount only the FPGA workspace; no device or privileged container access.
docker run --rm --name ikemen-quartus --mount "type=bind,source=$fpga,target=/build" --workdir /build $compiler quartus_sh --flow compile IKEMEN
if ($LASTEXITCODE -ne 0) { throw "Quartus build failed ($LASTEXITCODE)" }
$timing = Join-Path $fpga 'output_files/IKEMEN.sta.summary'
if (!(Test-Path -LiteralPath $timing)) { throw 'Quartus produced no timing summary.' }
if (Select-String -LiteralPath $timing -Pattern '^Slack\s*:\s*-' -Quiet) {
    throw 'FPGA timing requirements failed. Do not deploy this RBF; inspect IKEMEN.sta.summary and detailed timing paths.'
}
Write-Output 'FPGA compile and reported timing checks passed. Review warnings before board testing.'
