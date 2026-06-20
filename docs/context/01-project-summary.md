# ViewRed — Resumen del Proyecto

## Qué es

ViewRed es un **monitor de red local** que captura paquetes, almacena metadata en TimescaleDB, muestra una UI React con datos en tiempo real vía SSE, y usa ML local (Isolation Forest + Random Forest en ONNX) más un LLM local (Qwen 2.5 3B vía Ollama) para detección de anomalías y asistente en lenguaje natural.

## Stack

| Capa | Tecnología |
|---|---|
| Captura | Go + gopacket (AF_PACKET Linux, Npcap Windows) |
| Backend | Go 1.26.3, Gin, pgx/v5 |
| DB | PostgreSQL + TimescaleDB |
| UI | React 18 + TypeScript + Tailwind CSS, Vite |
| Real-time | SSE (Server-Sent Events) |
| ML | ONNX Runtime (Isolation Forest + Random Forest) |
| LLM | Qwen 2.5 3B via Ollama |
| Bind | `127.0.0.1:8080`, acceso admin vía SSH tunnel |

## Plan General (8 planes)

| # | Plan | Estado |
|---|------|--------|
| 1 | Bootstrap (monorepo Go + React) | ✅ DONE en `main` |
| 2 | Capture-and-storage (pcap, TimescaleDB, health, shutdown) | ✅ DONE en `main` |
| 3 | Core API and UI Shell (REST, SSE, React Router, Zustand) | ✅ DONE en `main` |
| 4 | ML and Alerts (Isolation Forest, Random Forest, Suricata rules, alert CRUD) | 🚧 EN PROGRESO en `feat/plan4-ml-and-alerts` |
| 5 | Topology and Discovery (ARP/DHCP/mDNS discovery, React Flow 2D) | 🚧 PARCIAL en `feat/plan4-ml-and-alerts` |
| 6 | LLM Assistant (Qwen 2.5 3B via Ollama) | 🚧 PARCIAL en `feat/plan4-ml-and-alerts` |
| 7 | Scanner and Vulns (nmap-style) | ❌ No iniciado |
| 8 | Installer and Polish (Windows installer, shadcn/ui, UX) | ❌ No iniciado |

## Host

Windows 11. Sin Docker, sin WSL. Go 1.26.3, Node 24.16.0, Ollama 0.30.9 con `qwen2.5:3b`.
