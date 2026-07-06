# ViewRed

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Node](https://img.shields.io/badge/Node-22+-339933?logo=node.js)](https://nodejs.org)
[![License](https://img.shields.io/badge/License-MIT-blue)](#license)

**Network Detection & Response (NDR)** con IA local, para equipos chicos que gestionan redes de 20–100 dispositivos. Todo corre en tu máquina, sin enviar datos a la nube.

> **Estado**: V1 en desarrollo. Plan 4 (ML engine + alerts + reports) completado. Ver [`docs/plans/`](./docs/plans/).

## Características principales

- **Captura pasiva** de tráfico de red con gopacket (Npcap en Windows, AF_PACKET en Linux)
- **Detección avanzada de amenazas** con IA local y heurísticas (Fuerza Bruta, Escaneo/Recon, Inundación, Anomalías)
- **Dashboard en tiempo real** vía SSE con métricas, top talkers y gráficos
- **Escaneo de vulnerabilidades** con nmap (rápido, completo, detección de servicios)
- **Asistente IA** con Qwen 2.5 3B vía Ollama, responde en español
- **Topología de red** 2D interactiva con React Flow
- **Alertas y notificaciones** con severidad, score de confianza y acciones
- **Generación de reportes PDF** manuales y programados para auditorías de red
- **Cross-platform**: funciona idéntico en Windows y Linux

## Quick start (dev)

### Prerrequisitos

| Herramienta | Versión | Plataforma | Para qué |
|---|---|---|---|
| Go | 1.25+ | Windows / Linux | Backend |
| Node.js | 22+ | Windows / Linux | Frontend |
| pnpm | 9+ | Windows / Linux | Frontend |
| Npcap | [última estable](https://npcap.com/#download) | Windows | Captura de paquetes (gopacket) |
| libpcap-dev | `apt install libpcap-dev` | Linux (Debian/Ubuntu) | Captura de paquetes (gopacket) |
| libpcap-devel | `dnf install libpcap-devel` | Linux (RHEL/Fedora) | Captura de paquetes (gopacket) |
| nmap | En `PATH` | Windows / Linux | Escaneo de vulnerabilidades |
| ONNX Runtime | 1.19.2 (verificada 2026-06-28) | Windows / Linux | Inferencia de modelos ML |
| Docker | Cualquiera | Windows / Linux | Base de datos (PostgreSQL + TimescaleDB) |
| PostgreSQL 15+ | Con TimescaleDB | Windows / Linux | Almacenamiento |
| make | Opcional | Windows / Linux | Comandos helper |

**Notas de compatibilidad:**

- `CGO_ENABLED=1` es obligatorio para compilar el backend porque el runtime de ONNX se enlaza vía CGO. En Windows usá un toolchain compatible (por ejemplo, MinGW-w64 via `choco install mingw`).
- La biblioteca de ONNX Runtime tiene que estar disponible en tiempo de ejecución:
  - **Windows:** `onnxruntime.dll` en la raíz del repo o en un directorio del `PATH`.
  - **Linux:** `libonnxruntime.so` en el `LD_LIBRARY_PATH` o en rutas del sistema como `/usr/local/lib`.
- La captura de paquetes depende de Npcap en Windows y de libpcap en Linux. Sin estas bibliotecas el backend compila, pero la captura en vivo falla.
- El escáner invoca `nmap` como proceso externo; si no está en `PATH`, las funciones de vulnerabilidades devuelven error.
- El frontend se construye exclusivamente con `pnpm`; `npm` y `yarn` no están soportados en este repo.

### 1. Clonar e instalar

```bash
git clone https://github.com/Yeikxzx/ViewRed.git viewred
cd viewred
```

### 2. Base de datos

```bash
# Opción A — Docker (recomendado para dev)
docker compose -f deploy/docker-compose.dev.yml up -d

# Opción B — PostgreSQL local con TimescaleDB
#   Crear DB: createdb viewred
#   Migraciones: go run ./cmd/viewred migrate
```

### 3. Instalar frontend

```bash
cd web && pnpm install && cd ..
```

### 4. Ejecutar tests

```bash
make test
# o separado:
#   go test ./...
#   cd web && pnpm test
```

### 5. Iniciar (dos terminales)

```bash
# Terminal 1: Backend en :8080
go run ./cmd/viewred

# Terminal 2: Frontend en :3000
cd web && pnpm dev
```

Abrir [http://127.0.0.1:3000](http://127.0.0.1:3000). El backend sirve health check en [http://127.0.0.1:8080/api/v1/health](http://127.0.0.1:8080/api/v1/health).

## Build

```bash
# Binario para el OS actual
go build -o viewred.exe ./cmd/viewred

# Cross-compile (con make)
make build-all

# Manual
GOOS=linux   GOARCH=amd64 go build -o dist/viewred-linux-amd64        ./cmd/viewred
GOOS=windows GOARCH=amd64 go build -o dist/viewred-windows-amd64.exe  ./cmd/viewred
```

## Estructura del proyecto

```
cmd/viewred/          Entry point Go
internal/             Paquetes internos (api, capture, ml, sse, ...)
web/                  Frontend React + Vite + TypeScript
docs/
  ├── architecture/   Diseños y decisiones técnicas
  ├── specs/          Especificaciones técnicas
  ├── plans/          Planes de implementación
  └── decisions/      Propuestas y exploraciones
deploy/               Docker compose y despliegue
models/               Modelos ONNX pre-entrenados
scripts/              Scripts auxiliares (build, train, ...)
```

## Documentación

| Documento | Contenido |
|---|---|
| [`que es viewred.md`](./que%20es%20viewred.md) | Definición del producto (público objetivo, features, roadmap) |
| [`design.md`](./design.md) | Diseño de interfaz de usuario (UI/UX spec completo) |
| [`docs/specs/`](./docs/specs/) | Especificaciones técnicas |
| [`docs/architecture/`](./docs/architecture/) | Arquitectura y diseño del sistema |
| [`docs/plans/`](./docs/plans/) | Planes de implementación por fase |
| [`docs/SETUP.md`](./docs/SETUP.md) | Guía de instalación post-formateo |
| [`docs/DESARROLLO.md`](./docs/DESARROLLO.md) | Documento de desarrollo (español) |

## Stack

| Capa | Tecnología |
|---|---|
| Backend | Go 1.25+ con Gin |
| Frontend | React 18 + TypeScript + Tailwind + Vite |
| DB | PostgreSQL 15+ + TimescaleDB |
| Captura | gopacket (Npcap / AF_PACKET) |
| ML | ONNX Runtime (Isolation Forest + Random Forest) |
| LLM | Qwen 2.5 3B vía Ollama |
| Scanner | nmap |
| Reportes | gofpdf (PDF) + cron |
| Tiempo real | SSE |
| Tests | testify (Go) + Vitest + Testing Library (React) |

## Licencia

MIT.
