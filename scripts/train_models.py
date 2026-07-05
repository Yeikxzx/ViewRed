"""
Train Isolation Forest + Random Forest models for ViewRed ML engine.

Feature vector (7 dimensions, must match internal/ml/features.go):
  [0] Length         — packet length in bytes
  [1] Proto_Idx      — 0=TCP, 1=UDP, 2=ICMP, 3=OTHER
  [2] SrcPort        — source port
  [3] DstPort        — destination port
  [4] PacketsPerSec  — packet rate from src IP over last 60s
  [5] UniqueDstPorts — distinct dst ports contacted by src IP
  [6] PayloadEntropy — Shannon entropy of payload (0..8)

Random Forest classes (must match engine.go switch):
  0 = DoS/DDoS Flood
  1 = Recon / Scanning
  2 = Web Attack (SQLi, XSS, Command Injection)
  3 = Brute Force (SSH/Telnet)
  4 = MITM (ARP spoofing, impersonation)
  5 = Malware (Mirai botnet)

Usage:
    pip install numpy scikit-learn skl2onnx onnx onnxruntime
    python scripts/train_models.py

Output:
    models/isolation_forest.onnx  — anomaly detector (-1 anomalous, 1 normal)
    models/random_forest.onnx     — attack classifier (0-5)
"""

import os
import numpy as np
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType

os.makedirs("models", exist_ok=True)

NUM_FEATURES = 7
np.random.seed(42)

# ============================================================================
# 1. Isolation Forest — Anomaly Detection
# ============================================================================
# Trained ONLY on normal traffic patterns to learn what "normal" looks like.
# Anything that deviates significantly gets flagged as anomalous.
#
# Normal traffic classes:
#   - Web browsing: TCP, ports 80/443, length 64-1500, pps 0.5-30, 1-8 unique ports, entropy 3-6
#   - DNS: UDP, ports 53/5353, length 40-200, pps 0.2-10, 1-3 unique ports, entropy 2-4
#   - ICMP (ping): ICMP, length 40-100, pps 0.1-5, 1 unique port, entropy 0
#   - SSH: TCP, port 22, length 40-300, pps 0.5-5, 1-2 unique ports, entropy 4-6
#   - Email (SMTP/IMAP): TCP, ports 25/143/587/993, length 60-2000, pps 0.2-3, 1-2 ports, entropy 3-5
#   - File transfer / media: TCP/UDP, high variance, pps 1-50, 1-4 ports, entropy 3-7

print("Generating normal traffic samples for Isolation Forest...")
X_normal = []

# Web browsing (35%)
for _ in range(2100):
    length = np.random.randint(64, 1500)
    src_port = int(np.random.choice([49152, 33456, 54321, 65000, 1024]))
    dst_port = int(np.random.choice([80, 443, 8080, 8443]))
    pps = round(np.random.uniform(0.5, 30), 2)
    unique_ports = int(np.random.randint(1, 8))
    entropy = round(np.random.uniform(3.0, 6.0), 2)
    X_normal.append([float(length), 0.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# DNS (15%)
for _ in range(900):
    length = np.random.randint(40, 200)
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([53, 5353]))
    pps = round(np.random.uniform(0.2, 10), 2)
    unique_ports = int(np.random.randint(1, 3))
    entropy = round(np.random.uniform(2.0, 4.0), 2)
    X_normal.append([float(length), 1.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# SSH (10%)
for _ in range(600):
    length = np.random.randint(40, 300)
    src_port = int(np.random.randint(1024, 65535))
    dst_port = 22
    pps = round(np.random.uniform(0.5, 5), 2)
    unique_ports = int(np.random.randint(1, 2))
    entropy = round(np.random.uniform(4.0, 6.0), 2)
    X_normal.append([float(length), 0.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# ICMP / ping (8%)
for _ in range(480):
    length = np.random.randint(40, 100)
    src_port = 0.0
    dst_port = 0.0
    pps = round(np.random.uniform(0.1, 5), 2)
    unique_ports = 1
    entropy = 0.0
    X_normal.append([float(length), 2.0, src_port, dst_port, pps, float(unique_ports), entropy])

# Email (12%)
for _ in range(720):
    length = np.random.randint(60, 2000)
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([25, 143, 587, 993]))
    pps = round(np.random.uniform(0.2, 3), 2)
    unique_ports = int(np.random.randint(1, 2))
    entropy = round(np.random.uniform(3.0, 5.0), 2)
    X_normal.append([float(length), 0.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# Media streaming / file transfer (10%)
for _ in range(600):
    length = int(np.random.choice([np.random.randint(60, 500), np.random.randint(1000, 3000)]))
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([443, 1935, 8080, 3333]))
    pps = round(np.random.uniform(1, 50), 2)
    unique_ports = int(np.random.randint(1, 4))
    entropy = round(np.random.uniform(3.0, 7.0), 2)
    X_normal.append([float(length), float(np.random.choice([0, 1])), float(src_port), float(dst_port), pps, float(unique_ports), entropy])

# DHCP / NTP / other benign UDP (10%)
for _ in range(600):
    length = np.random.randint(40, 500)
    src_port = int(np.random.choice([68, 123, 161, 514, np.random.randint(1024, 65535)]))
    dst_port = int(np.random.choice([67, 123, 162, 514]))
    pps = round(np.random.uniform(0.1, 3), 2)
    unique_ports = int(np.random.randint(1, 3))
    entropy = round(np.random.uniform(1.0, 4.0), 2)
    X_normal.append([float(length), 1.0, float(src_port), float(dst_port), pps, float(unique_ports), entropy])

X_train_if = np.array(X_normal, dtype=np.float32)
print(f"  Normal samples: {len(X_normal)}")
print()

print("Training Isolation Forest...")
iso_forest = IsolationForest(
    n_estimators=200,
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
print("  Saved -> models/isolation_forest.onnx")
print()

# ============================================================================
# 2. Random Forest — Attack Classification
# ============================================================================
# Classes (MUST match engine.go switch statement):
#   0 = DoS/DDoS Flood
#   1 = Recon / Scanning
#   2 = Web Attack
#   3 = Brute Force
#   4 = MITM
#   5 = Malware (Mirai)
#
# Each class gets 400 samples with realistic attack characteristics.

N_PER_CLASS = 500
print("Generating classified attack samples for Random Forest...")

X_rf = []
y_rf = []

# --- Class 0: DoS/DDoS Flood ---
# Characteristics: very high packet rate (>100 pps), high byte rate,
# low unique dst ports (targeting specific services), low entropy
# (no payload or garbage), often same protocol (TCP SYN flood or ICMP flood)
for _ in range(N_PER_CLASS):
    # Type A: SYN flood (70%)
    if np.random.random() < 0.7:
        length = np.random.randint(40, 80)         # SYN packets are small
        proto = 0.0                                 # TCP
        src_port = int(np.random.randint(1024, 65535))
        dst_port = int(np.random.choice([80, 443, 22, 53]))  # targeting specific service
        pps = round(np.random.uniform(200, 2000), 2)         # very high rate
        unique_ports = int(np.random.randint(1, 5))           # few dst ports
        entropy = round(np.random.uniform(0.0, 2.0), 2)       # no payload or garbage
    # Type B: ICMP flood (20%)
    else:
        length = np.random.randint(40, 1500)
        proto = 2.0                                 # ICMP
        src_port = 0.0
        dst_port = 0.0
        pps = round(np.random.uniform(300, 3000), 2)
        unique_ports = 1
        entropy = 0.0
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(0)

# --- Class 1: Recon / Scanning ---
# Characteristics: moderate to high unique dst ports (port scan),
# moderate packet rate, various protocols, low payload
for _ in range(N_PER_CLASS):
    length = np.random.randint(40, 200)
    proto = float(np.random.choice([0.0, 1.0]))       # TCP or UDP scan
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.randint(1, 65535))        # many different ports
    pps = round(np.random.uniform(10, 200), 2)         # moderate to high
    unique_ports = int(np.random.randint(20, 1000))    # many unique ports
    entropy = round(np.random.uniform(0.0, 3.0), 2)    # low payload
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(1)

# --- Class 2: Web Attack (SQLi, XSS, Cmd Injection) ---
# Characteristics: TCP port 80/443, unusual patterns in payload (higher entropy),
# medium packet rate, specific source ports, larger packets with payloads
for _ in range(N_PER_CLASS):
    length = int(np.random.choice([np.random.randint(60, 500), np.random.randint(500, 3000)]))
    proto = 0.0                                        # TCP
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([80, 443, 8080, 8443]))
    pps = round(np.random.uniform(1, 50), 2)           # moderate rate
    unique_ports = int(np.random.randint(1, 6))         # few targets
    # Higher entropy because SQL/XSS payloads have varied byte distribution
    entropy = round(np.random.uniform(5.0, 8.0), 2)
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(2)

# --- Class 3: Brute Force (SSH/Telnet) ---
# Characteristics: TCP ports 22/23, many connections from same src,
# moderate rate, small packets (auth attempts), low-medium entropy
for _ in range(N_PER_CLASS):
    length = np.random.randint(40, 300)
    proto = 0.0                                        # TCP
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([22, 23, 2323]))    # SSH or Telnet
    pps = round(np.random.uniform(2, 30), 2)            # moderate rate (human-ish)
    unique_ports = int(np.random.randint(1, 3))          # targeting only auth port
    entropy = round(np.random.uniform(4.0, 6.5), 2)
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(3)

# --- Class 4: MITM (ARP spoofing, impersonation) ---
# Characteristics: often small packets, could be TCP or ARP-like (proto OTHER),
# unusual src/dst port combos, medium entropy, often same-subnet IP patterns
# NOTE: ARP isn't TCP/UDP/ICMP, so it maps to proto=3 (OTHER)
for _ in range(N_PER_CLASS):
    # Type A: ARP spoofing (60%) — maps to proto OTHER
    if np.random.random() < 0.6:
        length = np.random.randint(28, 60)             # ARP packets are tiny
        proto = 3.0                                    # OTHER
        src_port = 0.0
        dst_port = 0.0
        pps = round(np.random.uniform(1, 20), 2)
        unique_ports = 1
        entropy = round(np.random.uniform(0.0, 1.0), 2)
    # Type B: TCP impersonation / session hijack (40%)
    else:
        length = np.random.randint(40, 200)
        proto = 0.0                                    # TCP
        src_port = int(np.random.randint(1024, 65535))
        dst_port = int(np.random.choice([80, 443, 22]))
        pps = round(np.random.uniform(1, 15), 2)
        unique_ports = int(np.random.randint(1, 3))
        entropy = round(np.random.uniform(3.0, 5.0), 2)
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(4)

# --- Class 5: Malware (Mirai botnet) ---
# Characteristics: targets IoT ports (23/2323/80/8080), specific scan patterns,
# moderate-high rate, many unique IPs, SYN scans, often low-medium entropy
for _ in range(N_PER_CLASS):
    proto = float(np.random.choice([0.0, 1.0]))       # TCP or UDP
    length = np.random.randint(40, 500)
    src_port = int(np.random.randint(1024, 65535))
    dst_port = int(np.random.choice([23, 2323, 80, 8080, 7547, 5555, 37215]))
    pps = round(np.random.uniform(5, 100), 2)          # moderate-high rate
    unique_ports = int(np.random.randint(5, 100))       # scans multiple ports
    entropy = round(np.random.uniform(2.0, 5.0), 2)
    X_rf.append([float(length), proto, float(src_port), float(dst_port), pps, float(unique_ports), entropy])
    y_rf.append(5)

X_train_rf = np.array(X_rf, dtype=np.float32)
y_train_rf = np.array(y_rf, dtype=np.int64)
print(f"  Class distribution: {np.bincount(y_train_rf)}")
print()

print("Training Random Forest Classifier...")
rf = RandomForestClassifier(
    n_estimators=200,
    max_depth=15,
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
print("  Saved -> models/random_forest.onnx")
print()

print("=== TRAINING COMPLETE ===")
print(f"Feature vector ({NUM_FEATURES} dims): [Length, Proto_Idx, SrcPort, DstPort, PacketsPerSec, UniqueDstPorts, PayloadEntropy]")
print()
print("RF Classes:")
print("  0 = DoS/DDoS Flood")
print("  1 = Recon / Scanning")
print("  2 = Web Attack")
print("  3 = Brute Force")
print("  4 = MITM")
print("  5 = Malware (Mirai)")
