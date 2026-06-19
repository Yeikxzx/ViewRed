$ErrorActionPreference = "Stop"

$Version = "1.18.0"
$Url = "https://github.com/microsoft/onnxruntime/releases/download/v$Version/onnxruntime-win-x64-$Version.zip"
$ZipPath = "$env:TEMP\onnxruntime.zip"
$ExtractPath = "$env:TEMP\onnxruntime"

Write-Host "Downloading ONNX Runtime v$Version..."
Invoke-WebRequest -Uri $Url -OutFile $ZipPath

Write-Host "Extracting to $ExtractPath..."
If (Test-Path $ExtractPath) { Remove-Item -Recurse -Force $ExtractPath }
Expand-Archive -Path $ZipPath -DestinationPath $ExtractPath -Force

$DllSrc = "$ExtractPath\onnxruntime-win-x64-$Version\lib\onnxruntime.dll"
$DllDst = "$PSScriptRoot\..\onnxruntime.dll"

Write-Host "Copying onnxruntime.dll to repository root..."
Copy-Item -Path $DllSrc -Destination $DllDst -Force

Write-Host "Cleaning up..."
Remove-Item $ZipPath -Force
Remove-Item -Recurse -Force $ExtractPath

Write-Host "Done! onnxruntime.dll is now in the root of the project."
