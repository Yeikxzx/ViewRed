# ViewRed — Guía de instalación post-formateo

> **Propósito**: Todo lo necesario para reinstalar el entorno de desarrollo
> de ViewRed desde cero después de formatear el equipo.
>
> **Sistema**: Windows 11

---

## Índice

1. [Herramientas base](#1-herramientas-base)
2. [Go + herramientas Go](#2-go--herramientas-go)
3. [Node.js + frontend](#3-nodejs--frontend)
4. [Git + clonar repo](#4-git--clonar-repo)
5. [ONNX Runtime](#5-onnx-runtime)
6. [Ollama + LLM](#6-ollama--llm)
7. [Docker + PostgreSQL + TimescaleDB](#7-docker--postgresql--timescaledb)
8. [Npcap (Windows)](#8-npcap-windows)
9. [Verificación final](#9-verificación-final)
10. [Resumen: orden de instalación](#10-resumen-orden-de-instalación)

---

## 1. Herramientas base

### Windows Package Managers

```powershell
# Ya viene con Windows 11
winget --version

# Opcionales (no obligatorios)
Set-ExecutionPolicy Bypass -Scope Process -Force
[System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
iex ((New-Object System.Net.WebClient).DownloadString('https://community.chocolatey.org/install.ps1'))

# Scoop (recomendado para dev tools)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

### Terminal (opcional pero recomendado)

```powershell
# Windows Terminal
winget install Microsoft.WindowsTerminal

# Oh My Posh (prompt bonito)
winget install JanDeDobbeleer.OhMyPosh
```

---

## 2. Go + herramientas Go

### Instalar Go

```powershell
winget install GoLang.Go.1.26
# O desde web: https://go.dev/dl/
# Versión: 1.26.3 (o la disponible)
```

### Verificar

```powershell
go version
# Debe mostrar: go version go1.26.3 windows/amd64
```

### Configurar GOPATH

```powershell
# Verificar que GOPATH existe
go env GOPATH
# Normalmente: C:\Users\<tu-user>\go
```

### Dependencias Go (se instalan al buildear)

```powershell
# En el directorio del proyecto, go build las descarga automático
cd C:\Users\<tu-user>\Desktop\Viewred
go mod download
```

Dependencias principales que se instalarán:

| Dependencia | Propósito |
|---|---|
| `github.com/gin-gonic/gin` | HTTP framework |
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `github.com/google/gopacket` | Captura de paquetes |
| `github.com/yalue/onnxruntime_go` | ONNX Runtime bindings |
| `github.com/golang-migrate/migrate/v4` | Migraciones DB |
| `github.com/google/uuid` | UUID generation |
| `github.com/DATA-DOG/go-sqlmock` | Mock DB para tests |

---

## 3. Node.js + frontend

### Instalar Node.js

```powershell
# Opción 1: winget
winget install OpenJS.NodeJS.LTS
# Versión: 24.x LTS (proyecto usa Node 24.16.0)

# Opción 2: nvm-windows (para cambiar versiones)
winget install CoreyButler.NVMforWindows
nvm install 24.16.0
nvm use 24.16.0
```

### Verificar

```powershell
node --version   # v24.16.0
npm --version    # 11.13.0

# Instalar pnpm globalmente (package manager del proyecto)
npm install -g pnpm@9.12.0
pnpm --version   # 9.12.0
```

### Instalar pnpm (package manager)

```powershell
# El proyecto usa pnpm en vez de npm
npm install -g pnpm
# pnpm 9.12+ (proyecto usa pnpm 9.12.0)
```

### Instalar dependencias del frontend

```powershell
cd web
pnpm install
```

Esto instala:

| Dependencia | Propósito |
|---|---|
| `react`, `react-dom` | UI framework |
| `react-router-dom` | Routing SPA |
| `zustand` | Estado global |
| `recharts` | Charts (dashboard) |
| `sonner` | Toast notifications |
| `tailwindcss`, `@tailwindcss/vite` | CSS utility framework |
| `vitest`, `@testing-library/react` | Testing |
| `typescript` | Type checking |

---

## 4. Git + clonar repo

### Instalar Git

```powershell
winget install Git.Git
```

### Configurar

```powershell
git config --global user.name "Tu Nombre"
git config --global user.email "tu@email.com"
git config --global core.autocrlf true     # Windows: CRLF → LF en commits
```

### Clonar el repositorio

```powershell
# Si el repo está en GitHub
cd C:\Users\<tu-user>\Desktop
git clone <url-del-repo>
cd Viewred

# Si tenés el repo en backup local
# Copiar la carpeta y luego:
cd C:\Users\<tu-user>\Desktop\Viewred
git checkout feat/plan4-ml-and-alerts
```

### Verificar estado

```powershell
git log --oneline -5
# Debe mostrar los últimos commits del branch activo
```

---

## 5. ONNX Runtime

### Descargar ONNX Runtime DLL

```powershell
# Versión: 1.16.x (compatible con github.com/yalue/onnxruntime_go v1.16.0)
# CPU-only (no necesitamos CUDA para ViewRed)

# Opción A: Descarga directa
# 1. Ir a: https://github.com/microsoft/onnxruntime/releases
# 2. Buscar "onnxruntime-win-x64-1.16.x.zip"
# 3. Extraer y copiar onnxruntime.dll a C:\Users\<tu-user>\Desktop\Viewred\

# Opción B: Con scripts (si existe)
.\scripts\install-onnx-win.ps1
```

### Verificar

El archivo `onnxruntime.dll` debe estar en la raíz del proyecto.
Está en `.gitignore` — no se commitea.

### Build tag CGO

El ML engine requiere CGO. Si no tenés GCC, instalá uno:

```powershell
# MinGW-w64 (para CGO en Windows)
winget install MSYS2.MSYS2
# Después de instalar MSYS2, abrí MSYS2 y:
pacman -S mingw-w64-x86_64-gcc

# Agregar a PATH:
# C:\msys64\mingw64\bin
```

**Verificar CGO**:

```powershell
go env CGO_ENABLED
# Debe mostrar: 1
```

Si no querés instalar CGO, el proyecto compila igual con el stub
(`engine_stub.go`), pero el ML engine arranca en modo disabled.

---

## 6. Ollama + LLM

### Instalar Ollama

```powershell
# Descargar de: https://ollama.com/download/windows
# O con winget:
winget install Ollama.Ollama
```

### Descargar modelo Qwen

```powershell
# Después de instalar, en una terminal nueva:
ollama pull qwen2.5:3b
```

### Verificar

```powershell
ollama list
# Debe mostrar: qwen2.5:3b

# Test rápido:
ollama run qwen2.5:3b "Hola, decime que funcionás"
```

### Configuración

Ollama corre en `http://localhost:11434` por defecto. ViewRed se conecta
automáticamente a esa dirección.

### (Opcional) Modelos alternativos

```powershell
# Si querés un modelo más chico (más rápido, menos preciso):
ollama pull qwen2.5:1.5b

# Si querés un modelo más grande (más preciso, más lento):
ollama pull qwen2.5:7b
```

---

## 7. Docker + PostgreSQL + TimescaleDB

> **RECOMENDADO**: Usar Docker para la base de datos. Es mucho más
> sencillo que instalar PostgreSQL y TimescaleDB nativos en Windows.
> El proyecto ya tiene un `docker-compose.dev.yml` listo.

### Instalar Docker Desktop

```powershell
# Opción 1: winget
winget install Docker.DockerDesktop

# Opción 2: Descargar de https://www.docker.com/products/docker-desktop/
```

Después de instalar, **reiniciar la máquina** y abrir Docker Desktop
para que termine la configuración inicial.

### Verificar Docker

```powershell
docker --version
docker compose version
# Ambos deben mostrar versión sin errores
```

### Levantar la base de datos

```powershell
# Desde la raíz del proyecto
cd C:\Users\<tu-user>\Desktop\Viewred
docker compose -f deploy/docker-compose.dev.yml up -d

# Verificar que está corriendo
docker ps
# Debe mostrar: viewred-postgres, puerto 5432, status Up
```

### Verificar conexión

```powershell
# Usando psql dentro del container
docker exec -it viewred-postgres psql -U viewred -d viewred -c "SELECT 1 AS ok;"

# Debe devolver:
#  ok
# ---- 
#   1
```

### Connection string (dev)

```
postgres://viewred:viewred_dev_only@localhost:5432/viewred?sslmode=disable
```

### Ejecutar migraciones

```powershell
# Las migraciones se ejecutan automáticamente al iniciar el binario.
cd cmd/viewred
go run .
# Si la DB está disponible, corre migraciones solo
```

### Detener la base de datos

```powershell
docker compose -f deploy/docker-compose.dev.yml down
# Para eliminar también los datos:
docker compose -f deploy/docker-compose.dev.yml down -v
```

### Alternativa: PostgreSQL standalone (sin Docker)

Si no querés Docker, instalá PostgreSQL directamente:

```powershell
winget install PostgreSQL.PostgreSQL.16
```

Y creá la base manualmente:

```powershell
psql -U postgres
CREATE USER viewred WITH PASSWORD 'viewred_dev_only';
CREATE DATABASE viewred OWNER viewred;
\q
```

> **Nota**: Sin Docker no vas a tener TimescaleDB (a menos que lo
> instales aparte). Para desarrollo básico PostgreSQL solo alcanza —
> las features de TimescaleDB no son críticas.

---

## 8. Npcap (Windows)

> Necesario para capturar paquetes en Windows. Sin Npcap, el modo
> captura no funciona pero el resto del proyecto sí.

```powershell
# Descargar de: https://npcap.com/#download
# Instalar con opción "WinPcap API-compatible Mode" marcada
```

### Verificar

```powershell
# Después de instalar, reiniciar. Luego:
Get-NetAdapter
# Debe mostrar las interfaces de red disponibles
```

Si no instalás Npcap, el `capture.Start()` va a fallar con error
"pcap: no device found" o similar, pero el binario sigue funcionando
(solo sin captura en vivo — el dashboard muestra 503 en las queries).

---

## 9. Verificación final

Después de instalar todo, correr estas verificaciones:

### 9.1 Build Go

```powershell
cd C:\Users\<tu-user>\Desktop\Viewred
go build ./...
# Debe compilar sin errores
```

### 9.2 Tests Go

```powershell
go test ./...
# Debe mostrar: ok 14/14 packages
```

### 9.3 Frontend

```powershell
cd web
npm install
pnpm build
# Debe buildear sin errores

pnpm vitest run
# Debe mostrar: 8 files, 91 tests passed
```

### 9.4 TypeScript

```powershell
pnpm tsc --noEmit
# Sin errores
```

### 9.5 (Opcional) Smoke test

```powershell
# Buildear binario
cd C:\Users\<tu-user>\Desktop\Viewred
go build -o viewred.exe ./cmd/viewred

# Iniciar (con DB o sin DB)
.\viewred.exe
# Debe mostrar: "ViewRed 0.1.0-dev listening on http://127.0.0.1:8080"

# En otra terminal, probar health endpoint
curl http://127.0.0.1:8080/api/v1/health
# Debe devolver 200 OK

# Matar el proceso
$process = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($process) { Stop-Process -Id $process.OwningProcess -Force }
```

---

## 10. Resumen: orden de instalación

```
 1. Git                          winget install Git.Git
 2. Go 1.26                      winget install GoLang.Go.1.26
  3. Node.js 24 LTS               winget install OpenJS.NodeJS.LTS
                                     → npm install -g pnpm@9.12.0
 4. MinGW-w64 (para CGO)         winget install MSYS2.MSYS2
                                    → pacman -S mingw-w64-x86_64-gcc
  5. Clonar repo                  git clone <url>
  6. Docker Desktop               winget install Docker.DockerDesktop
                                     → reiniciar, docker compose up -d
  7. ONNX Runtime DLL             Descargar de GitHub Releases
                                     → copiar a raíz del proyecto
  8. Ollama + qwen2.5:3b          winget install Ollama.Ollama
                                     → ollama pull qwen2.5:3b
  9. Npcap (opcional)             Descargar de npcap.com
 10. pnpm install                 cd web && pnpm install
```

**Tiempo estimado total**: 45-90 minutos dependiendo de velocidad de internet.

---

## Archivos grandes no commiteados

Estos archivos no están en el repo (`.gitignore`) y hay que generarlos
o descargarlos post-formateo:

| Archivo | Obtención |
|---|---|
| `onnxruntime.dll` | Descargar de GitHub Releases | 11 MB |
| `oui.txt` | Se genera automáticamente al correr `discovery.NewEngine()` | ~3 MB |
| `node_modules/` | `pnpm install` | ~200 MB |

Los modelos ONNX (`models/isolation_forest.onnx`, `models/random_forest.onnx`)
**SÍ están commiteados** (~1.2 MB + 42 KB).

---

> Si encontrás algún paso que falta o algo que no funciona, actualizá
> este documento y commitealo.
