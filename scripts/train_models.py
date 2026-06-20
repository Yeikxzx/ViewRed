"""
Train Isolation Forest + Random Forest models for ViewRed ML engine.

Feature vector (7 dimensions):
  [0] Length         — packet length in bytes
  [1] Proto_Idx      — 0=TCP, 1=UDP, 2=ICMP, 3=OTHER
  [2] SrcPort        — source port
  [3] DstPort        — destination port
  [4] PacketsPerSec  — packet rate from src IP over last 60s
  [5] UniqueDstPorts — distinct dst ports contacted by src IP
  [6] PayloadEntropy — Shannon entropy of payload (0..8)

Usage:
    pip install numpy scikit-learn skl2onnx onnx onnxruntime
    python scripts/train_models.py

Output:
    models/isolation_forest.onnx  — anomaly detector
    models/random_forest.onnx     — attack classifier
"""

import os
import numpy as np
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType

os.makedirs("models", exist_ok=True)

NUM_FEATURES = 7

# ---------------------------------------------------------------------------
# 1. Isolation Forest — Anomaly Detection
# ---------------------------------------------------------------------------
# Normal traffic patterns:
#   - Web: TCP, ports 80/443/8080, length 64-1500, rate ~1-20 pps,
#           1-5 unique dst ports, entropy 3-6 (HTTP is somewhat structured)
#   - DNS: UDP, ports 53/5353, length 40-200, rate ~0.5-5 pps,
#           1-2 unique ports, entropy 2-4
#   - ICMP: ICMP, length 40-100, rate ~0.1-2 pps, 1 dst port (0),
#            entropy 0 (no payload)
np.random.seed(42)

print("Generating normal traffic samples for Isolation Forest...")
X_normal = []

# Web traffic (60%)
for _ in range(1200):
    length = np.random.randint(64, 1500)
    src_port = int(np.random.choice([1024, 33456, 49152, 54321, 65000]))
    dst_port = int(np.random.choice([80, 443, 8080]))
    pps = round(np.random.uniform(1, 20), 2)
    unique_ports = int(np.random.randint(1, 6))
    entropy = round(np.random.uniform(3, 6), 2)
    X_normal.append([float(length), 0.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# DNS traffic (20%)
for _ in range(400):
    length = np.random.randint(40, 200)
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([53, 5353]))
    pps = round(np.random.uniform(0.5, 5), 2)
    unique_ports = int(np.random.randint(1, 3))
    entropy = round(np.random.uniform(2, 4), 2)
    X_normal.append([float(length), 1.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# ICMP traffic (10%)
for _ in range(200):
    length = np.random.randint(40, 100)
    src_port = 0.0
    dst_port = 0.0
    pps = round(np.random.uniform(0.1, 2), 2)
    unique_ports = 1
    entropy = 0.0
    X_normal.append([float(length), 2.0, src_port, dst_port, pps, float(unique_ports), entropy])

# Other benign (10%)
for _ in range(200):
    length = np.random.randint(60, 500)
    proto = float(np.random.choice([0, 1]))
    src_port = float(np.random.randint(1024, 65535))
    dst_port = float(np.random.choice([22, 25, 110, 143, 993, 3306]))
    pps = round(np.random.uniform(0.5, 10), 2)
    unique_ports = int(np.random.randint(1, 4))
    entropy = round(np.random.uniform(3, 6), 2)
    X_normal.append([float(length), proto, src_port, dst_port, pps, float(unique_ports), entropy])

# Anomalous traffic (for IF training — contamination ~5%)
# Port scan: high pps, many unique ports, single src
# Data exfil: high entropy, odd dst port
# Large ICMP: oversized ICMP packets
print("Generating anomalous traffic samples...")
X_anomalous = []

# Port scan (40% of anomalies)
for _ in range(80):
    length = np.random.randint(40, 100)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = float(np.random.randint(1, 65535))
    pps = round(np.random.uniform(50, 500), 2)  # very high rate
    unique_ports = int(np.random.randint(50, 500))  # many dst ports
    entropy = round(np.random.uniform(2, 4), 2)
    X_anomalous.append([float(length), 0.0, src_port, dst_port, pps, float(unique_ports), entropy])

# Data exfiltration / tunnel (30% of anomalies)
for _ in range(60):
    length = np.random.randint(1000, 5000)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = float(np.random.choice([443, 53, 22, 9001]))
    pps = round(np.random.uniform(1, 10), 2)
    unique_ports = int(np.random.randint(1, 3))
    entropy = round(np.random.uniform(7, 8), 2)  # very high — encrypted
    X_anomalous.append([float(length), 0.0, src_port, dst_port, pps, float(unique_ports), entropy])

# ICMP flood (20% of anomalies)
for _ in range(40):
    length = np.random.randint(1000, 50000)  # oversized ICMP
    src_port = 0.0
    dst_port = 0.0
    pps = round(np.random.uniform(100, 1000), 2)
    unique_ports = 1
    entropy = 0.0
    X_anomalous.append([float(length), 2.0, src_port, dst_port, pps, float(unique_ports), entropy])

# DNS tunnel (10% of anomalies)
for _ in range(20):
    length = np.random.randint(200, 2000)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = 53.0
    pps = round(np.random.uniform(5, 50), 2)
    unique_ports = 1
    entropy = round(np.random.uniform(6, 8), 2)  # encoded data in DNS
    X_anomalous.append([float(length), 1.0, src_port, dst_port, pps, float(unique_ports), entropy])

# Combine for IF training (normal only + optional anomalous for unsupervised)
# Isolation Forest is unsupervised — train ONLY on normal data
X_train_if = np.array(X_normal, dtype=np.float32)
print(f"  Normal samples: {len(X_normal)}")

print("Training Isolation Forest...")
iso_forest = IsolationForest(
    n_estimators=100,
    contamination=0.05,
    random_state=42,
    max_samples='auto',
    bootstrap=False,
)
iso_forest.fit(X_train_if)

# Convert to ONNX
initial_type = [('float_input', FloatTensorType([None, NUM_FEATURES]))]
onnx_iso = convert_sklearn(
    iso_forest,
    initial_types=initial_type,
    target_opset={'': 15, 'ai.onnx.ml': 3},
)
with open("models/isolation_forest.onnx", "wb") as f:
    f.write(onnx_iso.SerializeToString())
print("Saved models/isolation_forest.onnx")

# ---------------------------------------------------------------------------
# 2. Random Forest — Attack Classification
# ---------------------------------------------------------------------------
# Classes:
#   0 = Unusual Web Traffic — anomalous but web-like (high entropy, odd ports)
#   1 = DNS Anomaly — possible DNS tunnel
#   2 = Suspicious Port Traffic — scan or high-port comms
#   3 = ICMP Anomaly — oversized ping, flood
print("\nGenerating classified traffic samples for Random Forest...")

X_rf = []
y_rf = []

# Web anomalies (class 0) — HTTP/HTTPS with abnormal characteristics
for _ in range(400):
    length = np.random.randint(64, 3000)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = float(np.random.choice([80, 443, 8080]))
    pps = round(np.random.uniform(1, 100), 2)
    unique_ports = int(np.random.randint(1, 10))
    entropy = round(np.random.uniform(5, 8), 2)
    X_rf.append([float(length), 0.0, src_port, dst_port, pps, float(unique_ports), entropy])
    y_rf.append(0)

# DNS anomalies (class 1) — high entropy DNS, large DNS payloads
for _ in range(300):
    length = np.random.randint(200, 2000)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = 53.0
    pps = round(np.random.uniform(1, 50), 2)
    unique_ports = int(np.random.randint(1, 3))
    entropy = round(np.random.uniform(5, 8), 2)
    X_rf.append([float(length), 1.0, src_port, dst_port, pps, float(unique_ports), entropy])
    y_rf.append(1)

# Suspicious port traffic (class 2) — scans, high-port comms
for _ in range(400):
    length = np.random.randint(40, 500)
    src_port = float(np.random.randint(1024, 65535))
    dst_port = float(np.random.randint(1024, 65535))  # non-standard
    pps = round(np.random.uniform(10, 500), 2)
    unique_ports = int(np.random.randint(5, 500))
    entropy = round(np.random.uniform(2, 6), 2)
    X_rf.append([float(length), float(np.random.choice([0, 1])), src_port, dst_port, pps, float(unique_ports), entropy])
    y_rf.append(2)

# ICMP anomalies (class 3) — large ping, flood
for _ in range(300):
    length = np.random.randint(500, 50000)
    src_port = 0.0
    dst_port = 0.0
    pps = round(np.random.uniform(50, 1000), 2)
    unique_ports = 1
    entropy = 0.0
    X_rf.append([float(length), 2.0, src_port, dst_port, pps, float(unique_ports), entropy])
    y_rf.append(3)

X_train_rf = np.array(X_rf, dtype=np.float32)
y_train_rf = np.array(y_rf, dtype=np.int64)
print(f"  Class distribution: {np.bincount(y_train_rf)}")

print("Training Random Forest Classifier...")
rf = RandomForestClassifier(
    n_estimators=100,
    max_depth=10,
    min_samples_leaf=4,
    random_state=42,
)
rf.fit(X_train_rf, y_train_rf)

# Convert to ONNX
onnx_rf = convert_sklearn(
    rf,
    initial_types=initial_type,
    target_opset={'': 15, 'ai.onnx.ml': 3},
)
with open("models/random_forest.onnx", "wb") as f:
    f.write(onnx_rf.SerializeToString())
print("Saved models/random_forest.onnx")

print("\nDone! Both models trained and exported with 7 features.")
print("Feature order: [Length, Proto_Idx, SrcPort, DstPort, PacketsPerSec, UniqueDstPorts, PayloadEntropy]")
