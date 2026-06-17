# ViewRed — Diseño de Interfaz de Usuario (V1)

> **Versión**: 0.2 — alineado con spec técnico 2026-06-17 y producto v0.3
> **Audiencia**: implementador de la UI (humano o IA) que va a crear la interfaz
> **Stack de UI**: React 18 + TypeScript + Tailwind CSS
> **Librerías sugeridas**: shadcn/ui (componentes), Lucide (íconos), Recharts (gráficos), React Flow (topología), react-virtual (tablas grandes), React Hook Form + Zod (formularios)
> **Spec técnico**: `docs/superpowers/specs/2026-06-17-viewred-v1-design.md`
> **Spec producto**: `que es viewred.md`

---

## 1. Filosofía de diseño

### Principios
- **Información densa pero escaneable**: los admins de red miran la UI por horas. Todo dato importante debe ser visible sin hacer scroll innecesario.
- **Tiempo real siempre visible**: la app vive del stream. Indicadores de "live" deben ser obvios.
- **IA como ciudadana de primera clase**: el asistente no es un chatbot escondido. Es una pieza central, accesible desde cualquier vista.
- **Datos técnicos en monoespaciada**: IPs, MACs, puertos y bytes se ven mejor en mono.
- **Modo oscuro por default**: los admins trabajan en cuartos oscuros, sesiones largas.
- **Cero decoración sin función**: si un elemento no informa ni permite actuar, sobra.
- **Idioma**: español en toda la UI. Términos técnicos universales en inglés (IP, TCP, DNS).
- **Cross-platform**: la UI debe verse y comportarse idéntica en Windows y Linux.

### Anti-principios
- No usar gradientes decorativos.
- No usar animaciones largas (>200ms) en interacciones.
- No esconder métricas detrás de clicks.
- No usar colores arbitrarios — cada color tiene significado.
- No usar paths absolutos del filesystem en la UI (mostrar nombres relativos).

---

## 2. Tokens visuales

### Colores (modo oscuro default)

```css
/* Backgrounds */
--bg-primary:    #0a0e14   /* fondo principal */
--bg-secondary:  #131820   /* cards, paneles */
--bg-tertiary:   #1c2230   /* hover, active */
--bg-elevated:   #232a3a   /* modales, dropdowns */

/* Texto */
--text-primary:   #e4e7eb   /* texto principal */
--text-secondary: #9ba3b4   /* texto secundario */
--text-muted:     #5c6473   /* labels, hints */
--text-inverse:   #0a0e14   /* texto sobre fondos claros */

/* Bordes */
--border-subtle:  #1c2230
--border-default: #2a3242
--border-strong:  #3a4356

/* Acentos semánticos */
--accent-primary:    #3b82f6   /* azul — acciones principales, links */
--accent-success:    #10b981   /* verde — OK, conectado, sin amenazas */
--accent-warning:    #f59e0b   /* amarillo — advertencias, anomalías leves */
--accent-danger:     #ef4444   /* rojo — crítico, amenazas, bloqueos */
--accent-info:       #06b6d4   /* cyan — info neutral */

/* Severidad de vulnerabilidades (CVSS) */
--vuln-critical:    #dc2626   /* CVSS >= 9.0 */
--vuln-high:        #ef4444   /* CVSS 7.0-8.9 */
--vuln-medium:      #f59e0b   /* CVSS 4.0-6.9 */
--vuln-low:         #3b82f6   /* CVSS 0.1-3.9 */

/* Protocolos (para gráficos de tráfico) */
--proto-tcp:    #3b82f6
--proto-udp:    #10b981
--proto-http:   #f59e0b
--proto-https:  #8b5cf6
--proto-dns:    #06b6d4
--proto-icmp:   #ec4899
--proto-other:  #6b7280
```

### Tipografía

```css
/* Sans para UI */
--font-sans:    'Inter', -apple-system, BlinkMacSystemFont, system-ui, sans-serif;

/* Mono para datos técnicos */
--font-mono:    'JetBrains Mono', 'Fira Code', 'Consolas', monospace;

--text-xs:    0.75rem   /* 12px — labels secundarios */
--text-sm:    0.875rem  /* 14px — body secundario */
--text-base:  1rem      /* 16px — body principal */
--text-lg:    1.125rem  /* 18px — títulos de sección */
--text-xl:    1.25rem   /* 20px — títulos de card */
--text-2xl:   1.5rem    /* 24px — títulos de página */
--text-3xl:   1.875rem  /* 30px — KPIs grandes */
--text-4xl:   2.25rem   /* 36px — hero metrics */
```

### Espaciado
Grid base de 4px.
- `--space-1: 4px`, `--space-2: 8px`, `--space-3: 12px`, `--space-4: 16px`, `--space-6: 24px`, `--space-8: 32px`, `--space-12: 48px`

### Radios
- `--radius-sm: 4px` (botones, inputs)
- `--radius-md: 8px` (cards)
- `--radius-lg: 12px` (modales, panels grandes)
- `--radius-full: 9999px` (badges, avatares)

### Sombras
- `--shadow-sm: 0 1px 2px rgba(0,0,0,0.3)` (cards)
- `--shadow-md: 0 4px 8px rgba(0,0,0,0.4)` (dropdowns)
- `--shadow-lg: 0 12px 24px rgba(0,0,0,0.5)` (modales)

---

## 3. Layout general

```
┌──────────────────────────────────────────────────────────────────┐
│ ┌──────┐                                                          │
│ │ Logo │ ViewRed    [🔍 Búsqueda global ⌘K]    [🟢 Live] [IA] [⚙]│  Header (56px)
├─┴──────┴──────────────────────────────────────────────────────────┤
│          │                                                          │
│  Side    │                                                          │
│  Nav     │            Main Content Area                            │
│  (240px) │            (flex-1, scrollable)                        │
│          │                                                          │
│  • Dash  │                                                          │
│  • Capt  │                                                          │
│  • Filt  │                                                          │
│  • Alert │                                                          │
│  • Topo  │                                                          │
│  • Vuln  │                                                          │
│  • IA    │                                                          │
│  • Conf  │                                                          │
│          │                                                          │
│  ─────   │                                                          │
│  Status  │                                                          │
│  • OS    │                                                          │
│  • CPU   │                                                          │
│  • RAM   │                                                          │
└──────────┴──────────────────────────────────────────────────────────┘
                                                                [💬 IA]
                                                                (floating)
```

### Header (56px, sticky top)
- **Izquierda**: logo + nombre "ViewRed".
- **Centro**: barra de búsqueda global (modal con `⌘K` o `Ctrl+K`).
- **Derecha**:
  - Indicador de estado live (🟢 pulsante cuando hay stream activo).
  - Botón de IA (abre panel de asistente).
  - Avatar / menú de usuario.

### Sidebar (240px, fija izquierda)
- Navegación principal con iconos + label.
- **8 items** en este orden: **Dashboard, Captura, Filtros, Alertas, Topología, Vulnerabilidades, Asistente, Configuración**.
- En la parte inferior: mini status del host (OS, CPU, RAM, uptime).
- Colapsable a 64px (solo iconos) con botón en el header.

### Main content
- Scrollable vertical.
- Padding: `--space-6` (24px).
- Ancho máximo: 1600px, centrado.

### Asistente IA flotante
- Botón circular fijo en bottom-right (56px).
- Al click: abre panel lateral derecho (400px) o modal fullscreen en pantallas chicas.
- Muestra badge con número de respuestas nuevas.

---

## 4. Secciones detalladas

### 4.1 Dashboard (vista default)

**Propósito**: vista de un golpe del estado actual de la red. Lo primero que ve el admin al abrir la app.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Dashboard                          [Última act: hace 2 seg]  │
│                                                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐      │
│  │ Paquetes │  │  Bytes   │  │Conexiones│  │ Disposit.│      │
│  │  12,345  │  │ 4.2 MB/s │  │    234   │  │    47    │      │
│  │  ▲ 3%    │  │  ▼ 1%    │  │  ▲ 12    │  │  =       │      │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘      │
│                                                                │
│  ┌─────────────────────────┐  ┌──────────────────────────┐    │
│  │ Tráfico en tiempo real  │  │ Top Talkers              │    │
│  │ [gráfico líneas]        │  │ IP           Bytes       │    │
│  │                         │  │ 192.168.1.10  1.2 GB ████│    │
│  │ 60s | 5m | 1h | 24h    │  │ 192.168.1.20  800 MB ███ │    │
│  └─────────────────────────┘  └──────────────────────────┘    │
│                                                                │
│  ┌─────────────────────────┐  ┌──────────────────────────┐    │
│  │ Distribución protocolos │  │ Alertas recientes        │    │
│  │ [pie/donut]             │  │ [listado, 5 últimas]     │    │
│  │ TCP   45%               │  │ 14:32 [WARN] Tráfico     │    │
│  │ UDP   30%               │  │   anómalo desde 192...   │    │
│  │ HTTPS 20%               │  │ 14:30 [INFO] Nuevo disp. │    │
│  │ DNS   5%                │  │ 14:28 [CRIT] Conexión a  │    │
│  └─────────────────────────┘  │   IP conocida maliciosa  │    │
│                               └──────────────────────────┘    │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- **4 KPI cards** arriba: paquetes/s, bytes/s, conexiones activas, dispositivos. Cada uno con delta vs período anterior.
- **Gráfico de tráfico** (línea, 60s default): paquetes/s y bytes/s superpuestos o toggle.
- **Top Talkers** (lista virtualizada, 10 items): IP, bytes, mini bar.
- **Distribución por protocolo** (donut chart con leyenda).
- **Alertas recientes** (lista de 5, link a sección completa).

**Interacciones**:
- Click en KPI → drill-down a vista de detalle.
- Click en Top Talker IP → filtrar paquetes por esa IP.
- Click en alerta → abrir detalle + ofrecer acción "Explicar con IA".
- Toggle de rango temporal en gráfico: 60s / 5m / 1h / 24h.
- Todo se actualiza vía SSE (throttle 1s en UI).

**Eventos SSE**:
- `metrics.summary` (cada 1s) → KPIs y gráfico.
- `metrics.top_talkers` (cada 5s) → Top Talkers.
- `alerts.recent` (push) → Alertas recientes.

---

### 4.2 Captura (status + configuración)

**Propósito**: ver el estado de la captura y configurar la interfaz/interfaz de red.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Captura de paquetes                                            │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Estado: 🟢 Capturando                                    │   │
│  │ Interfaz: eth0                                          │   │
│  │ Inicio: 2024-01-15 14:32:10                             │   │
│  │ Paquetes capturados: 1,234,567                          │   │
│  │ Bytes en disco: 12.4 GB (de 50 GB)                      │   │
│  │                                                          │   │
│  │ [Pausar]  [Detener]  [Cambiar interfaz]                  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌────────────────────────┐  ┌────────────────────────┐        │
│  │ Configuración          │  │ Almacenamiento         │        │
│  │                        │  │                        │        │
│  │ Interfaz: [eth0 ▼]    │  │ Retención: [7 días ▼] │        │
│  │ Filtro BPF: [...]      │  │ Rotación: [1 GB ▼]    │        │
│  │ Snaplen: [65535]       │  │ Compresión: [Sí ▼]    │        │
│  │ Modo: [Promiscuo ▼]    │  │                        │        │
│  │                        │  │                        │        │
│  │ [Guardar]              │  │ [Limpiar paquetes]    │        │
│  └────────────────────────┘  └────────────────────────┘        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Uso de disco (últimas 24h)                                │   │
│  │ [gráfico de área]                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- Card de estado (grande, arriba): status actual, métricas acumuladas, acciones rápidas.
- Card de configuración: formulario para cambiar parámetros.
- Card de almacenamiento: políticas de retención y rotación.
- Gráfico de uso de disco.

**Interacciones**:
- Pausar/detener captura con confirmación.
- Cambiar interfaz requiere confirmar y reinicia captura.
- Limpiar paquetes requiere doble confirmación (acción destructiva).
- Formularios con validación inline.

**Cross-platform**:
- Las interfaces disponibles en el dropdown se auto-detectan con `pcap.FindAllDevs()` (gopacket) — funciona en Windows y Linux.
- La ruta de almacenamiento se muestra como relativa al config dir (no como path absoluto), ejemplo: `datos/packets/` en vez de `C:\Users\...\data\packets\`.

---

### 4.3 Filtros y búsqueda

**Propósito**: buscar paquetes en el histórico y aplicar filtros guardados.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Filtros y búsqueda           [+ Nuevo filtro] [Guardados (5)]│
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ Constructor de filtros                                   │   │
│  │                                                          │   │
│  │ ┌──────────┐ ┌────┐ ┌────────┐ ┌────────┐              │   │
│  │ │ IP orig  │▼│ = │ │ 192... │ │  AND ▼ │              │   │
│  │ └──────────┘ └────┘ └────────┘ └────────┘              │   │
│  │ [+ condición]                                            │   │
│  │                                                          │   │
│  │ [Texto libre: "buscar en payload..."]                    │   │
│  │                                                          │   │
│  │ [Ejecutar]  [Guardar filtro]                            │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Resultados: 12,345 paquetes (en 0.4s)         [Exportar]       │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ # │ Tiempo     │ Proto │ IP orig     │ IP dest    │ ... │   │
│  ├───┼────────────┼───────┼─────────────┼────────────┼─────┤   │
│  │ 1 │ 14:32:10.1 │ TCP   │ 192.168.1.10│ 8.8.8.8    │ ... │   │
│  │ 2 │ 14:32:10.3 │ UDP   │ 192.168.1.20│ 1.1.1.1    │ ... │   │
│  │ 3 │ 14:32:10.5 │ TCP   │ 192.168.1.10│ 93.184.216.│ ... │   │
│  │ ...                                                      │   │
│  │ (tabla virtualizada)                                     │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Click en fila → detalle del paquete (drawer lateral)          │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- Constructor visual de filtros (dropdowns: campo + operador + valor + AND/OR).
- Input de texto libre para búsqueda en payload.
- Tabla virtualizada de resultados (react-virtual, importante con miles de filas).
- Drawer lateral para detalle del paquete (header, payload hex/ASCII).

**Filtros guardados** (modal):
- Lista de filtros guardados.
- Nombre, descripción, query.
- Acciones: ejecutar, editar, eliminar, compartir.

**Interacciones**:
- Cada cambio en el constructor aplica el filtro con debounce de 300ms.
- Click en fila → abre drawer con detalle.
- "Exportar" → CSV (V1) o PCAP.

---

### 4.4 Alertas

**Propósito**: ver, gestionar y configurar alertas de seguridad.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Alertas                          [Crít: 2] [Warn: 5] [Info:12]│
│                                                                  │
│  Filtros: [Todas ▼] [Severidad ▼] [Origen ▼] [Fecha ▼]        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ 🔴 CRÍT  14:32  Tráfico a IP maliciosa conocida        │   │
│  │            192.168.1.45 → 185.x.x.x:443                │   │
│  │            Confianza: 92%  Origen: ML                  │   │
│  │            [Ver paquetes] [Explicar con IA] [Ack]      │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │ 🟡 WARN  14:28  Anomalía en patrón de tráfico         │   │
│  │            Dispositivo: servidor-db (192.168.1.20)     │   │
│  │            Confianza: 67%  Origen: Isolation Forest    │   │
│  │            [Ver] [Explicar con IA] [Ack]               │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │ 🔵 INFO  14:25  Nuevo dispositivo detectado            │   │
│  │            MAC: aa:bb:cc:dd:ee:ff                      │   │
│  │            [Ver topología] [Ack]                       │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  (lista virtualizada)                                           │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- Header con contadores por severidad.
- Filtros rápidos.
- Lista de alertas (card por alerta).
- Cada card: severidad (color), timestamp, título, contexto, score, origen, acciones.

**Acciones por alerta**:
- **Ver paquetes relacionados** (filtro pre-armado).
- **Explicar con IA** (abre asistente con contexto).
- **Acknowledge** (marca como vista).
- **Silenciar** (esconde del listado, configurable).

**Detalle de alerta** (modal o drawer):
- Toda la info.
- Gráfico de la métrica que disparó.
- Paquetes relacionados.
- Botón de "Explicar con IA".

**SSE**: nuevas alertas hacen push al top de la lista (con animación sutil de entrada).

---

### 4.5 Topología 2D

**Propósito**: visualizar los dispositivos de la red y sus conexiones.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Topología                          [Vista: Grafo] [Lista]      │
│  Filtros: [Todos ▼] [Vendor ▼] [Riesgo ▼] [Vulns ▼]           │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                                                          │   │
│  │     [Internet]                                           │   │
│  │         |                                                │   │
│  │     [Router] 🔴 3 vulns                                  │   │
│  │      / | \                                               │   │
│  │   [S1] [AP] [Server] 🟡 1 vuln                          │   │
│  │   / \    |    \                                          │   │
│  │ [PC][PC][Móvil] [DB] 🟢 0 vulns                         │   │
│  │                                                          │   │
│  │  (grafo interactivo con React Flow)                      │   │
│  │  - arrastrar nodos                                      │   │
│  │  - zoom con scroll                                      │   │
│  │  - click nodo → detalle                                 │   │
│  │  - badge con número de vulns por nodo                    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  Leyenda:                                                        │
│  🟢 Bajo riesgo  🟡 Medio  🔴 Alto  ⬜ Sin clasificar          │
│  🔴 Crítico (≥9 CVSS)  🟠 Alto (7-8.9) 🟡 Medio (4-6.9)      │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- Grafo interactivo (React Flow).
- Nodos = dispositivos (icono según tipo: PC, server, móvil, router, IoT, desconocido).
- Edges = conexiones observadas (grosor = bytes, color = protocolo).
- Color del nodo = nivel de riesgo.
- **Badge con número de vulnerabilidades** en el nodo (link a sección de vulns filtrada por device).
- Click en nodo → panel lateral con detalle del dispositivo.
- Toggle grafo/lista.

**Panel de detalle del dispositivo**:
- Info básica: IP, MAC, vendor, hostname.
- Métricas: tráfico actual, conexiones, último visto.
- Alertas asociadas.
- Lista de vulnerabilidades detectadas.
- Lista de conexiones (top talkers).
- Botón "Escanear este dispositivo" (lanza scan con target = IP).
- Botón "Explicar este dispositivo con IA".

**SSE**: cambios en la topología (nuevo dispositivo, conexión nueva) animan la entrada del nodo/edge.

---

### 4.6 Vulnerabilidades

**Propósito**: ver, gestionar y disparar escaneos de vulnerabilidades y puertos.

**Layout**:

```
┌────────────────────────────────────────────────────────────────┐
│  Vulnerabilidades                  [Crít: 3] [Alt: 7] [Med: 12]│
│  Filtros: [Severidad ▼] [Device ▼] [Fecha ▼] [CVE ▼]         │
│                                                                  │
│  [🔍 Escanear red]  [📋 Historial de escaneos]                │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ 🔴 CVE-2024-1234   CVSS 9.8   Crítico                   │   │
│  │    Apache Log4j RCE en servidor-web (192.168.1.50:80)  │   │
│  │    Detectado: 2024-01-15 14:32  por escaneo #42        │   │
│  │    [Ver device] [Ver detalle CVE] [Marcar revisado]    │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │ 🟠 CVE-2023-5678   CVSS 7.5   Alto                      │   │
│  │    OpenSSL vulnerability en servidor-db (192.168.1.20) │   │
│  │    [Ver device] [Ver detalle] [Marcar revisado]        │   │
│  ├──────────────────────────────────────────────────────────┤   │
│  │ 🟡 Puerto 23 (Telnet) abierto en switch-1 (.1.10)     │   │
│  │    CVSS 5.0  Medio                                     │   │
│  │    [Ver device] [Ver detalle] [Marcar revisado]        │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  (lista virtualizada)                                           │
└────────────────────────────────────────────────────────────────┘
```

**Componentes**:
- Header con contadores por severidad.
- Filtros rápidos.
- Botón "Escanear red" (abre modal de configuración de scan).
- Botón "Historial de escaneos" (abre lista de scans pasados).
- Lista de vulnerabilidades (card por vuln).
- Cada card: severidad (color por CVSS), CVE/descripción, device afectado, score CVSS, fecha, scan que la detectó, acciones.

**Modal "Nuevo escaneo"**:
```
┌────────────────────────────────────────────┐
│  Configurar escaneo                  [✕]  │
│                                            │
│  Target:                                   │
│  ○ Subred completa (192.168.1.0/24)      │
│  ○ Rango IP: [192.168.1.10] a [.50]      │
│  ○ IP específica: [192.168.1.45]         │
│                                            │
│  Perfil:                                   │
│  ○ Quick (top 1000 puertos)               │
│  ○ Full (1-65535)                         │
│  ● Service-detect (recomendado)           │
│                                            │
│  ☐ Incluir script de vulnerabilidades     │
│                                            │
│  Estimado: ~2 min para 50 hosts           │
│                                            │
│             [Cancelar]  [Iniciar escaneo]  │
└────────────────────────────────────────────┘
```

**Modal "Detalle de vulnerabilidad"**:
- Descripción completa.
- Vector de ataque (CVSS vector).
- Referencias externas (links a NVD, MITRE).
- Lista de devices afectados.
- Botón "Marcar como mitigada" (con comentario).
- Botón "Explicar con IA" (abre asistente con contexto).

**Historial de escaneos** (modal o vista):
- Lista de scans pasados: fecha, target, perfil, duración, # hosts, # vulns encontradas.
- Click en un scan → ver vulns encontradas en ese scan.
- Botón "Re-escanear" (con la misma config).

**Eventos SSE**:
- `vulns.new` (push) → nueva vuln detectada.
- `scan.progress` (periódico) → % de progreso del scan activo.
- `scan.completed` (push) → scan terminado, refrescar lista.

---

### 4.7 Asistente IA

**Propósito**: chat con el LLM local para entender qué pasa en la red.

**Layout** (panel lateral derecho, 400px):

```
┌──────────────────────────────────────┐
│  Asistente IA              [✕]      │
│  Modelo: Qwen 2.5 3B · Local        │
├──────────────────────────────────────┤
│                                       │
│  [Admin, 14:30]                      │
│  ¿Qué pasó en la red entre las      │
│  2 y las 4 de la madrugada?          │
│                                       │
│  [IA, 14:30]                         │
│  En ese período se detectaron 3      │
│  eventos relevantes:                │
│                                       │
│  1. Conexión desde 192.168.1.45 a   │
│     una IP listada como maliciosa   │
│     (...).                          │
│                                       │
│  2. Pico de tráfico anómalo en el   │
│     servidor DB (3x lo normal).     │
│                                       │
│  3. Se conectó un nuevo dispositivo │
│     no visto antes.                 │
│                                       │
│  ¿Querés que profundice en alguno?  │
│                                       │
│  [Admin, 14:32]                      │
│  Sí, el segundo.                     │
│                                       │
│  [IA, 14:32 — typing...]            │
│                                       │
├──────────────────────────────────────┤
│  Sugerencias:                         │
│  [Resumen últimas 24h]              │
│  [Tráfico anómalo reciente]         │
│  [Vulnerabilidades nuevas]          │
│  [Dispositivos nuevos esta semana]  │
├──────────────────────────────────────┤
│  [Preguntá algo a tu red...    ] [➤]│
└──────────────────────────────────────┘
```

**Componentes**:
- Header: nombre, modelo, indicador "local".
- Historial de chat (scrollable).
- Burbujas: admin (derecha) vs IA (izquierda).
- Markdown rendering (negrita, listas, código).
- Sugerencias de preguntas (cambian según contexto de la vista actual).
- Input con botón enviar.
- Indicador "typing..." durante generación.

**Comportamiento**:
- **Streaming de respuesta** vía SSE.
- **Contexto automático** según vista actual ("estás viendo vulnerabilidades" → sugiere preguntas sobre vulns).
- **Al click en "Explicar con IA" desde una alerta/vuln/device** → abre chat con prompt pre-armado y contexto.
- **Historial persistente** (guardado en DB).
- **Memoria de conversación** dentro de la sesión (no entre sesiones en V1).

**Estados**:
- **Idle**: esperando pregunta.
- **Generando**: spinner + texto parcial.
- **Error**: mensaje claro + retry.
- **Sin LLM disponible**: mensaje "Iniciá Ollama para usar el asistente".

---

### 4.8 Configuración

**Propósito**: configurar la app en general.

**Layout** (tabs o sidebar interno):

```
Configuración
├─ General
│  - Idioma
│  - Tema (oscuro/claro)
│  - Zona horaria
├─ Captura
│  - Interfaz default
│  - Retención
│  - Snaplen
│  - Filtro BPF default
├─ Alertas
│  - Reglas activas
│  - Umbrales
│  - Notificaciones
├─ Scanner
│  - Path de nmap
│  - Rate limit (hosts concurrentes)
│  - Timeout default
│  - Auto-scan programado
├─ IA
│  - URL de Ollama
│  - Modelo activo
│  - Temperature
│  - System prompt
├─ Cuenta (V2)
└─ Sistema
   - Versión
   - OS y recursos
   - Logs
   - Reiniciar servicios
   - Exportar config
```

**Componentes**:
- Tabs o sidebar interno.
- Formularios con guardado automático.
- Sección de IA: configurar URL de Ollama, cambiar modelo.
- Sección de Scanner: path de nmap, rate limit, timeout.
- Sección de sistema: info del host, logs, acciones destructivas (reiniciar).

---

## 5. Patrones transversales

### 5.1 Datos en tiempo real
- Indicador "🟢 Live" en header cuando hay conexión SSE activa.
- Auto-reconnect con backoff exponencial.
- Throttle de updates en UI: máximo 1 vez por segundo por componente.
- Optimistic UI donde aplique (ack de alerta, marcar vuln revisada, etc.).

### 5.2 Estados de carga
- **Skeleton loaders** para contenido que se carga por primera vez.
- **Spinners sutiles** para acciones puntuales.
- **Barra de progreso** para escaneos (con % y tiempo estimado).
- **Nunca** pantalla completa de "cargando".

### 5.3 Estados vacíos
- Mensaje claro + ilustración simple + CTA.
- Ejemplos:
  - "No hay alertas. Tu red está tranquila 🟢"
  - "No se encontraron vulnerabilidades. ¡Buenas noticias! 🛡️"
  - "Aún no hay devices descubiertos. Esperá unos minutos..."

### 5.4 Errores
- Toast en bottom-right para errores no críticos.
- Modal para errores bloqueantes.
- Siempre con mensaje claro y acción ("Reintentar", "Ver logs", "Contactar soporte").

### 5.5 Notificaciones
- Toast no intrusivo (bottom-right, 5s).
- Badge en icono de alertas con contador.
- Sonido: OFF por default, opt-in.

### 5.6 Indicadores de operación larga
- Escaneos: barra de progreso con "Escaneando X de Y hosts" + botón "Cancelar".
- Carga inicial de DB: skeleton + texto "Cargando base de datos...".
- Reconstrucción de topología: spinner con texto "Actualizando grafo de red...".

---

## 6. Accesibilidad

- Contraste mínimo WCAG AA en todos los textos.
- Navegación completa por teclado.
- Focus visible en todos los interactivos.
- ARIA labels en componentes custom.
- Tooltips con descripción completa (no solo el icono).
- Anuncios de nuevas alertas/vulns para screen readers (configurable).

---

## 7. Responsive

- **Desktop (1024px+)**: layout completo, todos los paneles.
- **Tablet (768–1024px)**: sidebar colapsada a iconos, paneles laterales como modales.
- **Mobile (<768px)**: NO priorizado en V1, pero:
  - Vista de alertas funciona (scroll, ver detalle).
  - Dashboard básico funciona.
  - Asistente IA funciona.
  - Vista de vulns funciona.
  - Topología, captura, scanner: redirigir a desktop.

---

## 8. Workflow cuando el usuario pase la interfaz

Una vez que el usuario cree la interfaz (código React, Figma, o wireframes), yo voy a:

1. **Revisarla contra este spec**, sección por sección.
2. **Marcar desviaciones**: lo que no cumpla con esta especificación.
3. **Sugerir ajustes concretos** con diffs de código o props a cambiar.
4. **Validar accesibilidad y consistencia visual** contra los tokens.
5. **Devolver el código ajustado** listo para integrar.

El usuario puede traer:
- Código React/TSX.
- Capturas de Figma.
- Wireframes en cualquier formato.
- Descripción textual.

En todos los casos voy a tratar de llevarlo al estado descrito aquí.

---

## 9. Librerías y stack recomendado

| Necesidad | Librería | Notas |
|---|---|---|
| Componentes base | shadcn/ui | Copia el código a tu repo, customizable. |
| Íconos | lucide-react | Tree-shakeable, consistente. |
| Gráficos | Recharts | Simple, suficiente para V1. |
| Grafo topología | React Flow | Estándar para node-based UIs. |
| Tablas grandes | react-virtual o TanStack Virtual | Crítico para listas de paquetes/alertas/vulns. |
| Forms | React Hook Form + Zod | Validación type-safe. |
| Markdown (chat) | react-markdown | Renderizado de respuestas del LLM. |
| Syntax highlight | shiki o highlight.js | Para payloads y código. |
| Fechas | date-fns | Tree-shakeable, liviano. |
| HTTP client | fetch nativo + wrapper | Evitar axios. |
| State global | Zustand o Context API | Zustand para V1, simple y suficiente. |
| SSE client | EventSource nativo + wrapper | Estándar del navegador. |
| Progress bar | shadcn/ui Progress o sonner | Para scans largos. |
| Toast | sonner | Buena DX, customizable. |

---

## 10. Orden de implementación sugerido

1. Layout base + navegación + tema (header, sidebar, main, theme).
2. Dashboard con datos mockeados (KPI cards + gráfico).
3. Conexión SSE real + reemplazo de mocks.
4. Alertas (lista + detalle).
5. Filtros y búsqueda (tabla).
6. Topología (grafo con React Flow).
7. Asistente IA (chat + sugerencias).
8. **Vulnerabilidades** (lista + modal de scan + detalle de vuln + integración con topología).
9. Configuración.
10. Polish: estados, errores, animaciones, accesibilidad.

---

## Changelog

- **v0.1** — Especificación inicial. 7 features de UI, paths Linux.
- **v0.2 (actual)** — Alineado con spec técnico 2026-06-17 y producto v0.3. Agregada sección 4.6 Vulnerabilidades. Sidebar pasa a 8 items. Cross-platform: paths relativos en UI. Tokens de color para CVSS severidad. Indicadores de operación larga (progress en scans). Actualizado orden de implementación (paso 8 = scanner). Actualizada sección 9 con librerías nuevas (sonner, progress bar, SSE client).
