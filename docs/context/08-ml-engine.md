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
      → alerta categorizada (0=Web, 1=DNS, 2=Suspicious Port, 3=ICMP)
    → si label != -1: descartado (tráfico normal)
```

## Modelos

| Modelo | Archivo | Propósito |
|--------|---------|-----------|
| Isolation Forest | `models/isolation_forest.onnx` | Detección de anomalías (unsupervised) |
| Random Forest | `models/random_forest.onnx` | Clasificación del tipo de ataque |

Entrenados con datos sintéticos via `scripts/train_models.py`.

## Datasets recomendados para reentrenar

Prioridad:

1. **CIC-IoT-2023** — https://www.unb.ca/cic/datasets/iotdataset-2023.html (IoT real + ataques)
2. **Edge-IIoTset 2022** — https://ieee-dataport.org/open-access/edge-iiotset (IoT/IIoT, 14 ataques)
3. **CIC-IDS2017** — https://www.unb.ca/cic/datasets/ids-2017.html (gold standard)
4. **TON_IoT** — https://research.unsw.edu.au/projects/toniot-datasets (sensores + telemetría)
5. **CIC-Darknet2020** — https://www.unb.ca/cic/datasets/darknet2020.html (túneles/anonymización)

## Build constraints

- `internal/ml/engine.go` → `//go:build cgo` (requiere ONNX Runtime C library)
- `internal/ml/engine_stub.go` → `//go:build !cgo` (no-op, permite build sin CGO)

## Dependencias

- `github.com/yalue/onnxruntime_go` — Go bindings para ONNX Runtime
- `onnxruntime.dll` — runtime library (Windows), 11.5MB. Descargar de https://github.com/microsoft/onnxruntime/releases
