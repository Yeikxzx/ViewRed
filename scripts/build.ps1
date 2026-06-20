$ErrorActionPreference = "Stop"

$OrtVersion = "1.20.1"
$ZipUrl = "https://github.com/microsoft/onnxruntime/releases/download/v$OrtVersion/onnxruntime-win-x64-$OrtVersion.zip"
$ZipFile = "onnxruntime.zip"
$ExtractDir = "onnxruntime-ext"

Write-Host "Construyendo ViewRed con CGO_ENABLED=1..."
$env:CGO_ENABLED = "1"
go build -o viewred.exe ./cmd/viewred

if (-not (Test-Path "onnxruntime.dll")) {
    Write-Host "Descargando ONNX Runtime v$OrtVersion..."
    Invoke-WebRequest -Uri $ZipUrl -OutFile $ZipFile
    
    Write-Host "Extrayendo onnxruntime.dll..."
    Expand-Archive -Path $ZipFile -DestinationPath $ExtractDir -Force
    Copy-Item -Path "$ExtractDir\onnxruntime-win-x64-$OrtVersion\lib\onnxruntime.dll" -Destination ".\onnxruntime.dll"
    
    # Limpieza
    Remove-Item $ZipFile
    Remove-Item -Recurse -Force $ExtractDir
}

Write-Host "Build completado exitosamente. Ejecuta .\viewred.exe para iniciar."
