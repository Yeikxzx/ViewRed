# ViewRed — Definición del Producto

> **Versión**: 0.3 (alineado con spec técnico 2026-06-17)
> **Estado**: Documento de producto — base consolidada para implementación
> **Spec técnico**: `docs/superpowers/specs/2026-06-17-viewred-v1-design.md`
> **Spec UI/UX**: `design.md`

---

## Resumen ejecutivo

ViewRed es una herramienta de **Network Detection & Response (NDR)** asistida por IA, pensada para **equipos chicos que gestionan su propia red local** (20–100 dispositivos, PyME, equipo TI de oficina, espacio de coworking). Escucha tráfico de red de forma pasiva (wireless o cableada), aplica modelos de **Machine Learning locales** para detectar anomalías y tráfico malicioso en tiempo real, ejecuta **escaneos de vulnerabilidades y puertos** bajo demanda o programados, y expone un asistente LLM local que explica en lenguaje natural lo que está ocurriendo. Todo corre 100% en una sola máquina, sin enviar datos a la nube.

**Cross-platform desde V1**: la app corre idéntica en Windows y Linux, con todos los datos locales.

El objetivo es bajar la barrera de entrada a un NDR con IA: detección automática + explicación en español + escaneo de vulnerabilidades + UI amigable, sin requerir un data scientist ni enviar tráfico a la nube.

---

## Problema que resuelve

Los equipos chicos no tienen acceso a herramientas enterprise de NDR (Darktrace, Vectra, Cisco Secure Network Analytics) por costo y complejidad. Las opciones gratuitas o de código abierto (Wireshark, Suricata, Zeek, Nmap) requieren expertise alto, están fragmentadas y no ofrecen:

- Detección inteligente automática sin escribir reglas manualmente.
- Explicaciones en lenguaje natural de lo que está pasando.
- Topología y descubrimiento de dispositivos.
- Escaneo de vulnerabilidades integrado con la detección.
- Una interfaz usable para no-expertos.
- Asistencia contextual ("¿qué pasó anoche?", "¿este dispositivo es normal?").

ViewRed cubre ese hueco: IA que detecta + IA que explica + scanner de vulnerabilidades, en una sola herramienta local, mantenible y multiplataforma.

---

## Usuario objetivo primario

**Perfil**: equipo chico (2–10 personas) que gestiona su propia red.
**Contexto**: redes LAN cableadas o wireless, una sola ubicación, sin SOC dedicado.
**Escala**: 20–100 dispositivos, 100 Mbps – 1 Gbps de tráfico.
**OS**: la red puede tener mezcla de Windows y Linux; ViewRed corre nativo en ambos.
**Conocimiento técnico**: intermedio. Conoce IP, puerto, protocolo. No es data scientist ni desarrollador de ML.
**Necesidad**: visibilidad rápida de la red, detección temprana de amenazas, escaneo de vulnerabilidades básico, asistencia para entender eventos sin volverse experto en seguridad.

### Out of target para V1

| Segmento | Por qué no en V1 |
|---|---|
| Hogar / SOHO (5–20 dispositivos) | UI muy simple, no justifica features. Diferido a V2+. |
| Enterprise (1000+ disp., 10 Gbps+) | Requiere arquitectura distribuida, RBAC avanzado, SIEM, HA. Diferido a V3. |
| Multi-tenant / SaaS | ViewRed es local, no cloud. Diferido a V3+. |

---

## Stack técnico V1

| Capa | Tecnología | Justificación |
|---|---|---|
| **Lenguaje backend** | Go 1.22+ | Performance, binario único, fácil de mantener. |
| **Captura de paquetes** | gopacket con driver **Npcap (Windows)** o **AF_PACKET (Linux)** | Cross-platform con build tags. Mismo código, dos binarios. |
| **Framework HTTP** | Gin | Mismo lenguaje, alto rendimiento, soporte SSE. |
| **DB driver** | pgx v5 (pool nativo) | Más rápido y type-safe que `database/sql`. |
| **Base de datos** | PostgreSQL 15+ + TimescaleDB 2.x | Time-series para métricas, SQL estándar para entidades. |
| **Migraciones** | golang-migrate | Reproducibles, archivos SQL versionados. |
| **Web UI** | React 18 + TypeScript 5+ + Tailwind | Estándar, fácil de iterar y contratar devs. |
| **Build React** | Vite | Dev con HMR, build optimizado para producción. |
| **Tiempo real** | **SSE + REST** (no WebSockets) | Unidireccional, sobre HTTP, menor superficie de ataque, debug simple. |
| **ML detección (tiempo real)** | Isolation Forest + Random Forest en **ONNX Runtime Go** | CPU, sin nube, baja latencia, modelos intercambiables. |
| **LLM (bajo demanda)** | **Qwen 2.5 3B** vía Ollama 0.30+ | Local, multilingüe (excelente en español), suficiente para V1. Upgrade a 7B en V2. |
| **Scanner** | nmap 7.9x como subprocess (`os/exec`), parse XML | Battle-tested, cross-platform. |
| **Tests Go** | go test + testify | Estándar. |
| **Tests React** | Vitest + Testing Library | Estándar para Vite. |
| **Deploy** | Binario único cross-platform + installer nativo (MSI / deb / rpm) + first-run wizard | Sin bundlear deps pesadas. |

---

## Características de V1 (MVP)

Las 8 features del MVP, en orden de prioridad de implementación:

### 1. Captura y almacenamiento de paquetes
- Captura con Npcap (Windows) o AF_PACKET (Linux) sobre interfaz seleccionada.
- Almacenamiento rotativo en archivos pcap.
- Metadatos de cada paquete en PostgreSQL/TimescaleDB (hypertable).
- Retención configurable (default 7 días, particiones comprimidas después).
- Rotación automática por tamaño o tiempo.
- Modo promiscuo opcional.

### 2. Dashboard en tiempo real
- Top talkers (IPs con más tráfico).
- Throughput agregado por protocolo y puerto.
- Gráficos de tráfico en vivo (SSE push, 1 Hz).
- Métricas: paquetes/seg, bytes/seg, conexiones activas, dispositivos.
- Alertas recientes (5 últimas) inline.
- Toggle de rango temporal: 60s / 5m / 1h / 24h.

### 3. Filtros y búsqueda
- Por IP origen/destino, protocolo, puerto.
- Búsqueda de texto sobre payload.
- Filtros combinables (AND/OR) con constructor visual.
- Filtros guardables por el admin.
- Tabla virtualizada (decenas de miles de filas).
- Drawer con detalle del paquete (header + payload hex/ASCII).

### 4. Alertas con detección IA (feature central)
- Motor de reglas tipo Suricata (firmas de amenazas conocidas).
- **Anomaly detection** con Isolation Forest: detecta desvíos del baseline aprendido.
- **Clasificación de tráfico** con Random Forest: web, video, VoIP, P2P, sospechoso.
- Score de confianza por alerta.
- Severidad: info / warning / critical.
- Notificación en UI en tiempo real (SSE).
- Acciones: ver paquetes relacionados, explicar con IA, acknowledge, silenciar.

### 5. Descubrimiento y topología 2D
- Descubrimiento de dispositivos vía ARP, DHCP, mDNS.
- Identificación de vendor por MAC (OUI lookup).
- Grafo de conexiones entre dispositivos (2D, React Flow).
- Click en nodo → detalle del device (panel lateral).
- Color de nodo = nivel de riesgo.
- Sin 3D en V1.

### 6. Asistente IA en lenguaje natural (feature diferenciador)
- LLM local Qwen 2.5 3B vía Ollama.
- Corre **bajo demanda**, no en tiempo real.
- Capacidades:
  - Resumir alertas y eventos: "¿qué pasó entre las 2 y las 4 AM?"
  - Explicar tráfico sospechoso en lenguaje humano.
  - Responder preguntas sobre el estado de la red.
  - Sugerir próximos pasos ante una alerta.
  - Explicar un dispositivo específico.
- Responde en español por defecto.
- Streaming de tokens (SSE).
- Historial persistente en DB.
- Contexto automático según vista actual.

### 7. Alertas y notificaciones
- Configuración de umbrales y reglas.
- Notificación en UI (panel + toast).
- Sin email ni SMS en V1.
- Historial de alertas con búsqueda y filtros.

### 8. Escaneo de vulnerabilidades y puertos (nuevo en V1)
- Subprocess a `nmap` con perfiles: `quick`, `full`, `service-detect`.
- Perfiles:
  - `quick`: top 1000 puertos.
  - `full`: 1–65535.
  - `service-detect`: `-sV` con detección de versión.
- Script `--script=vulners` para detección de CVEs.
- Parse XML → struct Go → INSERT en DB.
- Persistencia de vulnerabilidades por device con CVE + CVSS + descripción + puerto.
- Scheduler con rate limiting (default 5 hosts concurrentes).
- Trigger manual (botón "Escanear") + programable.
- UI: lista de vulnerabilidades, vista por device, vista por severidad.
- Integración con topología: ver vulns en contexto del device.

---

## Fuera de alcance (explícito)

Esto **NO** entra en V1. Diferido a versiones futuras:

| Feature | Versión objetivo | Motivo |
|---|---|---|
| Topología 3D inmersiva | V3 | Caro de hacer bien, valor cuestionable vs grafo 2D |
| Control activo: bloquear dispositivos, filtrar tráfico, bloquear sitios | V2 | Convierte a ViewRed en firewall/IPS, distinto objetivo |
| RBAC granular (gestión por roles) | V2 | V1 asume usuario único |
| Reportes exportables (PDF, CSV) | V2 | Nice-to-have, no crítico |
| Integración SIEM / Syslog / NetFlow / sFlow | V2 | Necesario para enterprise |
| Multi-tenant / alta disponibilidad | V3 | Enterprise-only |
| Actualización automática de firmas de malware | V2 | Depende de fuentes, validar primero |
| App móvil | V3 | Diferido |
| Auth interna de la app (usuario/contraseña) | V2 | V1 asume acceso local con permisos del SO |
| ML fine-tuneado por red | V2 | V1 usa modelos pre-entrenados genéricos |
| Acceso remoto seguro (SSH tunnel / mTLS) | V2 | V1 asume acceso local; remoto se hace por LAN con firewall del OS |

---

## Lo que ViewRed NO es

- **No es un firewall/IPS**: ViewRed detecta y alerta. No bloquea por defecto en V1.
- **No es un IDS puro basado en firmas**: combina firmas con ML.
- **No es una herramienta de pentesting**: no explota vulnerabilidades, solo las detecta.
- **No es un SIEM**: no correlaciona eventos de múltiples fuentes, no guarda logs de aplicación.
- **No es cloud-native**: 100% local. Los datos no salen del equipo del cliente.
- **No es para redes ajenas**: requiere consentimiento explícito del dueño de la red.
- **No es multi-usuario en V1**: corre con permisos del SO, un usuario a la vez.

---

## Modelo de despliegue

### Principio: cross-platform, una sola máquina

ViewRed se instala y corre en una sola máquina (Windows o Linux) que tenga visibilidad del tráfico de la red. La app corre con permisos elevados (Admin en Windows, root o `CAP_NET_RAW` en Linux) para poder capturar en modo promiscuo.

### Installer strategy (layered, no bundle)

Por restricciones de licencia y tamaño, ViewRed **no bundlea todas las dependencias** en un solo binario. En su lugar:

**Capa 1: Installer nativo** (~30 MB).
- Windows: MSI que instala `viewred.exe` + wrapper de Windows Service.
- Linux: `.deb` / `.rpm` + unit de systemd.
- Detecta Npcap (Windows) y abre navegador a npcap.com si falta.

**Capa 2: First-run wizard dentro de la app**.
- Detecta: PostgreSQL+TimescaleDB, Ollama, nmap, Npcap.
- Ofrece instalar las que falten, con instrucciones claras.
- Sugiere `ollama pull qwen2.5:3b` si el modelo no está descargado.

**Capa 3: Docker compose alternativo**.
- `docker-compose.yml` provisto levanta Postgres+TimescaleDB y (opcional) Ollama.
- ViewRed apunta a ellos. Una línea: `docker compose up -d`.

**Capa 4: Detección continua**.
- Health check por subsistema.
- Si algo se cae, lo muestra en UI y log.

### Acceso

- **Local**: el admin abre `http://localhost:8080` en la misma máquina.
- **Remoto (V1 simple)**: la máquina donde corre ViewRed expone el puerto (con firewall del OS) y se accede por LAN.
- **Remoto seguro (V2)**: SSH tunnel o reverse proxy con mTLS. No en V1.

### Paths por OS (cross-platform via `os.UserConfigDir()`)

| Recurso | Windows | Linux |
|---|---|---|
| Config | `%APPDATA%\viewred\config.yaml` | `~/.config/viewred/config.yaml` |
| DB data | `%APPDATA%\viewred\data\` | `~/.local/share/viewred/data/` |
| Logs | `%APPDATA%\viewred\logs\` | `~/.local/share/viewred/logs/` |

### Requisitos mínimos

| Recurso | Mínimo | Recomendado |
|---|---|---|
| CPU | 4 cores | 8 cores |
| RAM | 8 GB (4 para app, 4 para LLM 3B) | 16 GB |
| Disco | 100 GB SSD | 500 GB SSD (pcap files crecen) |
| OS | Windows 10/11, Ubuntu 22.04+, Debian 12+, RHEL 9+ | latest LTS |
| Red | Interfaz con modo promiscuo o monitor mode | Gigabit Ethernet |
| Permisos | Admin (Win) o root (Linux) | dedicado |

---

## Privacidad y seguridad

- **Consentimiento**: capturar tráfico de una red requiere autorización explícita del dueño. ViewRed NO debe usarse en redes ajenas.
- **Datos locales**: ML, LLM, almacenamiento y procesamiento corren 100% en el equipo. Cero llamadas a APIs externas.
- **Tráfico cifrado (HTTPS/TLS)**: ViewRed solo analiza metadatos (IPs, puertos, SNI, timing, tamaños). No intenta descifrar contenido (no hace MITM).
- **Auditoría**: log de acciones del admin en la UI + log de acceso al servicio.
- **Autenticación**: en V1, el único control de acceso es el del SO (permisos de archivo del binario, firewall). Auth interna en V2.
- **Hardening**: responsabilidad del admin (firewall del OS, actualizaciones, etc.).
- **Modelo de amenazas asumido**: el host de ViewRed es confiable; no se asume atacante en el mismo host.
- **Tráfico en red**: en V1, si ViewRed escucha en `0.0.0.0` lo hace en LAN confiable. Por default bind a `127.0.0.1` y se accede por LAN solo si el admin configura firewall.

---

## Métricas de éxito

### Funcionales
- Captura tráfico de 1 Gbps sin perder paquetes (medido con `tcpreplay`).
- Dashboard actualiza métricas en menos de 2 segundos.
- Filtros responden en menos de 1 segundo sobre 24h de datos.
- LLM responde en menos de 15 segundos por consulta (CPU).
- Scanner `quick` (top 1000 puertos) completa en menos de 2 minutos para una subred /24.

### IA
- Anomaly detection: tasa de detección > 80% sobre dataset público (CICIDS, UNSW-NB15).
- Falsos positivos: < 5% en tráfico normal.
- Clasificación de tráfico: accuracy > 90% en categorías principales.
- LLM: respuestas coherentes en español en > 90% de las consultas de prueba.

### Adopción (medible en beta con 3-5 equipos)
- Admin puede instalar y empezar a capturar en menos de 30 minutos.
- Admin identifica al menos 1 problema real en su red en la primera semana.
- 0 caídas del servicio en el primer mes.
- Funciona idéntico en Windows y Linux.

---

## Riesgos y supuestos

### Riesgos técnicos

| Riesgo | Mitigación |
|---|---|
| Performance de captura a 1 Gbps puede requerir tuning o AF_XDP en Linux | Empezar con 100 Mbps, dejar headroom, DPDK/AF_XDP en V2 |
| gopacket en Windows requiere Npcap (driver con licencia especial) | Installer detecta y guía al usuario a npcap.com |
| Modelos ML pre-entrenados pueden no adaptarse al tráfico de redes específicas | Baseline por red en V1, fine-tuning en V2 con datos del cliente |
| Qwen 2.5 3B puede dar respuestas imprecisas o alucinar en casos complejos | Validar output, mostrar disclaimer en UI, upgrade a 7B en V2 |
| Disco lleno por pcap files | Rotación automática, alertas de capacidad, default conservador de retención |
| nmap binary no instalado en Windows/Linux | First-run wizard detecta y sugiere instalación |
| Cross-platform bugs sutiles | Tests por OS con build tags, CI matrix Windows + Linux |
| Diferencias de permisos Windows Admin vs Linux root | Abstracción clara, mensajes de error específicos por OS |

### Supuestos

- Admin tiene permisos para instalar el binario (Admin en Windows, root en Linux).
- Host tiene al menos 8 GB RAM.
- Disco suficiente para retención de pcap (~50 GB/día a 1 Gbps con compresión).
- Red es IPv4 principalmente (IPv6 soportado pero no optimizado en V1).
- El admin acepta correr LLM en el mismo host (8 GB RAM compartidos).
- El admin instala dependencias (PostgreSQL, Ollama, nmap, Npcap) cuando el wizard lo sugiere.
- El host tiene acceso de red a la interfaz que quiere monitorear.

---

## Timeline realista

| Versión | Scope | Duración estimada (1 dev senior solo) |
|---|---|---|
| **V1 MVP** | Las 8 features listadas arriba | **4–6 meses** |
| **V2** | ML fine-tuneado, LLM 7B, reportes, SIEM básico, topología 3D, control activo, RBAC, auth interna, SOHO target | 6–12 meses después de V1 |
| **V3** | Enterprise-ready, RBAC avanzado, multi-tenant, HA, app móvil, cloud-hybrid | 12+ meses después de V2 |

Con equipo de 2–3 devs: dividir estos tiempos por ~2.

---

## Roadmap visual

```
V1 (MVP)                          V2                              V3
───────────────────────────────────────────────────────────────────────
- Captura + storage               - 7B LLM                        - RBAC avanzado
- Dashboard tiempo real           - ML fine-tuneado               - Multi-tenant
- Filtros y búsqueda              - Topología 3D                  - HA / clustering
- Alertas IA (Isolation + RF)     - Reportes exportables          - SIEM enterprise
- Topología 2D                    - Syslog / NetFlow / sFlow      - App móvil
- Asistente IA (Qwen 3B)          - Control activo (bloquear)     - Cloud-hybrid
- Alertas en UI                   - RBAC básico
- Escaneo de vulnerabilidades     - Auth interna
- Cross-platform Win+Linux        - Update automático firmas
- Installer nativo + wizard       - SOHO target
```

---

## Glosario

- **NDR** — Network Detection & Response. Categoría de herramientas de seguridad que detectan amenazas en el tráfico de red.
- **gopacket** — Librería Go para captura y parsing de paquetes. Abstrae libpcap/Npcap/AF_PACKET.
- **Npcap** — Versión Windows de libpcap. Driver + librería compatible.
- **AF_PACKET** — Socket de Linux para captura de paquetes a alta velocidad sin pasar por libpcap en userspace.
- **SSE** — Server-Sent Events. Mecanismo unidireccional de push sobre HTTP.
- **ONNX** — Open Neural Network Exchange. Formato estándar de intercambio de modelos de ML.
- **Ollama** — Runtime para ejecutar LLMs localmente con API HTTP simple.
- **nmap** — Scanner de red y puertos. Línea de comandos, salida XML parseable.
- **TimescaleDB** — Extensión de PostgreSQL para datos time-series con hypertables y compresión.
- **Isolation Forest** — Algoritmo de ML no supervisado para detección de anomalías.
- **Random Forest** — Algoritmo de ML supervisado para clasificación.
- **PyME** — Pequeña y mediana empresa.
- **IDS / IPS** — Intrusion Detection System / Intrusion Prevention System.
- **SIEM** — Security Information and Event Management.
- **CVE / CVSS** — Common Vulnerabilities and Exposures / Common Vulnerability Scoring System.
- **OUI** — Organizationally Unique Identifier. Primeros 3 bytes de una MAC que identifican al fabricante.
- **TDD** — Test-Driven Development. Disciplina de escribir tests antes del código de producción.
- **msi / deb / rpm** — Formatos de installer nativo de Windows / Debian-Ubuntu / Red Hat.

---

## Changelog

- **v0.1** — Lista de 21 features sin contexto técnico, priorización ni restricciones. Borrador inicial.
- **v0.2** — Redefinición tras sesión de planning. Target PyME 50-500, stack Linux server, MVP de 7 features, deploy con SSH tunnel.
- **v0.3 (actual)** — Alineado con spec técnico 2026-06-17. Target 20-100 (equipo chico), cross-platform Windows+Linux, MVP de 8 features (suma escaneo de vulnerabilidades), installer layered, architecture A1 monolith con build tags, stack actualizado con gopacket+Npcap/AF_PACKET, pgx v5, Vite.
