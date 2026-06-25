# ViewRed — Motor ML

## Feature Vector (7 dimensiones)

Extraído por `internal/ml/features.go` → `ExtractFeatures(pkt, ws)`:

| Índice | Feature | Descripción |
|--------|---------|-------------|
| 0 | Length | Tamaño del paquete en bytes |
| 1 | Proto_Idx | 0=TCP, 1=UDP, 2=ICMP, 3=OTHER |
| 2 | SrcPort | Puerto origen |
| 3 | DstPort | Puerto destino |
| 4 | PacketsPerSec | Tasa de paquetes de esta IP origen (ventana 60s) |
| 5 | UniqueDstPorts | Puertos destino únicos contactados por esta IP |
| 6 | PayloadEntropy | Shannon entropy de primeros 256 bytes (0-8) |

## WindowStats (`internal/ml/window.go`)

Sliding window de 60s por IP origen. Thread-safe (mutex). Cleanup cada 5min.

Tracking:
- Timestamps de paquetes (para packet rate)
- Puertos destino únicos
- IPs destino únicas
- Total de bytes

## Pipeline de inferencia

```
PacketEvent → WindowStats.Record() → ExtractFeatures() → ONNX tensor
  → Isolation Forest (anomaly detection)
    → si label == -1 (anomalía): Random Forest (classification)
      → alerta categorizada
    → si label != -1: descartado (tráfico normal)
```

## Pipeline de entrenamiento

```
CIC IIoT Dataset 2025 (CSV) → extract_features() [84 cols → 7-dim vector]
  → Isolation Forest (unsupervised, solo benigno)
    → export ONNX (models/isolation_forest.onnx)
  → Random Forest (supervised, ataques etiquetados)
    → export ONNX (models/random_forest.onnx)
```

## Modelos

| Modelo | Archivo | Propósito |
|--------|---------|-----------|
| Isolation Forest | `models/isolation_forest.onnx` | Detección de anomalías (unsupervised, 200 trees) |
| Random Forest | `models/random_forest.onnx` | Clasificación del tipo de ataque (200 trees, 6 clases) |

### Generación

**v1 — Sintética** (legacy): `scripts/train_models.py` — datos generados con `np.random`. Útil para probar el pipeline, no para detección real.

**v2 — CIC IIoT Dataset 2025** (actual): `scripts/train_from_dataset.py` — entrenado con el dataset [DataSense CIC IIoT 2025](https://www.unb.ca/cic/datasets/iiot-dataset-2025.html). 27,360 muestras benignas + 17,695 muestras de ataque en ventanas de 5 segundos.

---

## Training v2 — CIC IIoT Dataset 2025

### Dataset

**Fuente:** Canadian Institute for Cybersecurity (CIC), UNB.
**Nombre:** DataSense: A Real-Time Sensor-Based Benchmark Dataset for Attack Analysis in IIoT.
**Descarga:** 294 MB (`all_attack_benign_samples.tar.xz`) en `D:\viewred-dataset\`.

Estructura:
```
processed_files/
  attack_data/attack_samples_5sec.csv   — 17,695 filas, 94 columnas
  benign_data/benign_samples_5sec.csv   — 27,360 filas, 94 columnas
raw_files/                               — PCAPs + JSONs (~40-60 GB estimado, no descargado)
```

Categorías de ataque:
| Categoría | Muestras | Paquetes | Descripción |
|-----------|----------|----------|-------------|
| Recon | 6,571 | 15.8M | Escaneo de puertos, host discovery, OS scan |
| DoS | 3,600 | 537.5M | SYN flood, UDP flood, ICMP flood, etc. |
| DDoS | 3,528 | 1.14B | DDoS multi-origen: SYN, UDP, HTTP, ICMP flood |
| MITM | 1,577 | 125.7M | ARP spoofing, IP spoofing, impersonación |
| Malware | 1,490 | 1.47M | Mirai botnet (SYN flood, UDP flood) |
| Web | 557 | 590K | SQLi, XSS, Command Injection |
| Bruteforce | 372 | 126.8K | SSH bruteforce, Telnet bruteforce |

### Feature Mapping (CSV → 7-dim)

| Feature | Columna CSV | Notas |
|---------|-------------|-------|
| Length | `network_ip-length_avg` | Promedio del tamaño IP en la ventana de 5s |
| Proto_Idx | `network_protocols_all` | Se parsea la lista. Prioridad: TCP→0, UDP→1, ICMP→2, OTHER→3 |
| SrcPort | `network_ports_src[0]` | Primer puerto de la lista de puertos fuente |
| DstPort | `network_ports_dst[0]` | Primer puerto de la lista de puertos destino |
| PacketsPerSec | `network_packets_all_count / 5` | Total de paquetes dividido por la ventana de 5s |
| UniqueDstPorts | `network_ports_dst_count` | Cantidad de puertos destino únicos en la ventana |
| PayloadEntropy | `min(8, payload-length-avg / 200)` | **Proxy** — el dataset no tiene Shannon entropy real |

### Hiperparámetros

| Modelo | Parámetro | Valor |
|--------|-----------|-------|
| Isolation Forest | n_estimators | 200 |
| | contamination | 0.05 |
| | max_samples | auto |
| Random Forest | n_estimators | 200 |
| | max_depth | 15 |
| | min_samples_leaf | 4 |
| | class_weight | balanced |
| Ambos | random_state | 42 |

### Resultados

#### Isolation Forest (anomaly detection)

| Métrica | Valor |
|---------|-------|
| True Positive Rate (ataques detectados) | **76.04%** |
| False Positive Rate (benigno marcado como anomalía) | **5.00%** |

De cada 100 ataques reales, 76 son detectados como anomalías. De cada 100 ventanas de tráfico normal, 5 son falsas alarmas.

#### Random Forest (attack classification)

**Accuracy: 91.27%**

| Clase | Precisión | Recall | F1-Score | Muestras |
|-------|-----------|--------|----------|----------|
| DoS/DDoS Flood | **0.99** | 0.93 | 0.96 | 1,426 |
| Recon / Scanning | **0.98** | 0.90 | 0.94 | 1,314 |
| Web Attack | **0.98** | 0.84 | 0.90 | 111 |
| Brute Force | **0.35** | 0.84 | 0.49 | 74 |
| MITM | **0.69** | 0.91 | 0.78 | 316 |
| Malware (Mirai) | **0.89** | 0.91 | 0.90 | 298 |

Las clases con más muestras (DDoS, Recon) tienen precisión excelente (>95%). Brute Force tiene pocas muestras (372) y sufre de baja precisión (35%).

#### Feature Importance

| Feature | Importancia |
|---------|------------|
| PacketsPerSec [4] | **21.4%** |
| SrcPort [2] | **17.6%** |
| Length [0] | **16.8%** |
| DstPort [3] | **16.1%** |
| UniqueDstPorts [5] | **15.1%** |
| PayloadEntropy [6] | **11.6%** |
| Proto_Idx [1] | **1.5%** |

La tasa de paquetes es la feature más discriminante para clasificar ataques — consistente con ataques de red donde el volumen es la señal principal.

### Decisión: CSV time-window vs PCAPs raw

El dataset ofrece dos formatos: `processed_files/` (CSV con features por ventana de tiempo) y `raw_files/` (PCAPs + JSONs). Se eligió el enfoque CSV por las siguientes razones:

#### Estimación de raw_files (PCAPs)

Categoría | Paquetes | PCAP estimado | .tar.xz estimado
----------|----------|--------------|-----------------
Bruteforce | 126K | 45 MB | **21 MB**
Web | 590K | ~210 MB | ~100 MB
Malware | 1.47M | ~525 MB | ~245 MB
Recon | 15.8M | ~5.6 GB | ~2.6 GB
MITM | 125.7M | ~25 GB | ~12 GB
DoS | 537.5M | ~107 GB | ~50 GB
DDoS | 1.14B | ~74 GB | ~35 GB
**Total** | **~1.8B** | **~212 GB** | **~100 GB**

> Referencia: bruteforce.tar.xz pesa 21 MB comprimido y contiene ~60 MB sin comprimir (PCAPs + JSONs de 8 dispositivos, 31 minutos de tráfico). El resto de las categorías se estimaron por proporción de paquetes.

#### Justificación

| Factor | CSV time-window (elegido) | PCAPs raw (descartado por ahora) |
|--------|--------------------------|----------------------------------|
| Descarga | **294 MB** (un archivo) | **~40-100 GB** estimado |
| Procesamiento | **Directo** (pandas) | Requiere scapy/tshark + extraer features |
| Features | Pre-calculadas (84 cols) | Hay que calcular cada feature manualmente |
| Precisión | **91.27%** en test | Potencialmente mayor (packet-level) |
| Tiempo total | **~30 min** (descarga + training) | **~3-4 horas** estimado |

Para un MVP, los CSV time-window ofrecen el mejor balance entre esfuerzo y calidad. Los PCAPs quedan como mejora futura si la precisión en producción no es suficiente.

### Limitaciones Conocidas

1. **Skew training/inference**: el modelo se entrenó con promedios por ventana de 5s (time-window), pero en producción recibe datos packet-by-packet con sliding window de 60s. Los patrones generales se mantienen, pero la precisión en producción puede ser menor que el 91% reportado.
2. **PayloadEntropy proxy**: el dataset no incluye Shannon entropy de payload. Se aproximó con `payload-length-avg / 200`. La detección de túneles/data exfiltration puede verse afectada.
3. **Brute Force**: baja precisión por falta de muestras (solo 372). Mejorable con data augmentation o más datos.
4. **Sin validación en hardware real**: los modelos se validaron con el split del dataset, no con tráfico capturado por ViewRed.

### Próximas mejoras

- [ ] Probar en producción y medir precisión real
- [ ] Entrenar con raw_files (PCAPs) para eliminar el skew time-window → packet-level
- [ ] Agregar más datos de Brute Force para mejorar precisión
- [ ] Implementar PayloadEntropy real extrayendo bytes de PCAPs

---

## Categorías de Alerta (engine.go)

| classLabel | Categoría | Severidad |
|------------|-----------|-----------|
| 0 | DoS/DDoS Flood | CRITICAL |
| 1 | Recon / Scanning | WARNING |
| 2 | Web Attack (SQLi, XSS, Cmd Injection) | HIGH |
| 3 | Brute Force | HIGH |
| 4 | MITM (ARP spoofing, impersonación) | CRITICAL |
| 5 | Malware (Mirai) | CRITICAL |

## Build constraints

- `internal/ml/engine.go` → `//go:build cgo` (requiere ONNX Runtime C library)
- `internal/ml/engine_stub.go` → `//go:build !cgo` (no-op, permite build sin CGO)

## Dependencias

- `github.com/yalue/onnxruntime_go` — Go bindings para ONNX Runtime
- `onnxruntime.dll` — runtime library (Windows), 11.5MB. Descargar de https://github.com/microsoft/onnxruntime/releases

## Scripts

| Script | Propósito |
|--------|-----------|
| `scripts/train_models.py` | Training v1 con datos sintéticos (legacy) |
| `scripts/train_from_dataset.py` | **Training v2** con CIC IIoT Dataset 2025 |
| `scripts/install-onnx-win.ps1` | Descarga e instala onnxruntime.dll en Windows |
| `scripts/build.ps1` | Build script para Windows (CGO + flags) |
