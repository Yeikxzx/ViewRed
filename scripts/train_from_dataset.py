"""
Train Isolation Forest + Random Forest for ViewRed using the CIC IIoT Dataset 2025 (DataSense).

Reads time-windowed CSV features, maps them to the 7-dim feature vector, trains
both models, and exports ONNX files to models/.

Usage:
    python scripts/train_from_dataset.py

Output:
    models/isolation_forest.onnx  — anomaly detector (trained on benign data)
    models/random_forest.onnx     — attack classifier (trained on labeled attacks)

Feature mapping (7 dims):
  [0] Length         — network_ip-length_avg
  [1] Proto_Idx      — derived from network_protocols_all (0=TCP, 1=UDP, 2=ICMP, 3=OTHER)
  [2] SrcPort        — first port in network_ports_src
  [3] DstPort        — first port in network_ports_dst
  [4] PacketsPerSec  — network_packets_src_count / 5.0  (5-second window, per-source)
  [5] UniqueDstPorts — network_ports_src_count  (per-source unique ports)
  [6] PayloadEntropy — proxy: scaled payload-length-avg (0..8)
"""

import os
import ast
import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest, RandomForestClassifier
from skl2onnx import convert_sklearn
from skl2onnx.common.data_types import FloatTensorType
from sklearn.model_selection import train_test_split
from sklearn.metrics import classification_report
import onnx

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
DATASET_DIR = r"C:\Users\Yeikxz\Downloads\viewred-dataset\viewred-dataset"
WINDOW_SECONDS = 5
NUM_FEATURES = 7
RANDOM_STATE = 42

# Map CIC label2 categories to RF class IDs
# These match the updated engine.go switch statement
CATEGORY_MAP = {
    "ddos":       0,  # DDoS/DoS Flood
    "dos":        0,
    "recon":      1,  # Recon / Scanning
    "web":        2,  # Web Attack (SQLi, XSS, etc.)
    "bruteforce": 3,  # Brute Force
    "mitm":       4,  # MITM (ARP spoofing, etc.)
    "malware":    5,  # Malware (Mirai)
}

CATEGORY_NAMES = {
    0: "DoS/DDoS Flood",
    1: "Recon / Scanning",
    2: "Web Attack",
    3: "Brute Force",
    4: "MITM",
    5: "Malware (Mirai)",
}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
def parse_list_str(s: str) -> list:
    """Parse a string like \"['tcp', 'arp']\" into a Python list."""
    if pd.isna(s) or s == "[]" or s == "['']":
        return []
    try:
        return ast.literal_eval(s)
    except (ValueError, SyntaxError):
        return [s.strip("[]' ")]


def proto_idx(protocols_list: list) -> float:
    """Map protocol list to proto index: 0=TCP, 1=UDP, 2=ICMP, 3=OTHER."""
    if not protocols_list:
        return 3.0
    prots = [p.lower().strip() for p in protocols_list]
    # Priority order: first matching protocol wins
    for p in prots:
        if p in ("tcp",):
            return 0.0
    for p in prots:
        if p in ("udp",):
            return 1.0
    for p in prots:
        if p in ("icmp", "icmpv4", "icmpv6"):
            return 2.0
    return 3.0


def first_port(ports_list: list) -> float:
    """Return the first numeric port from a parsed list."""
    if not ports_list:
        return 0.0
    try:
        return float(ports_list[0])
    except (ValueError, IndexError):
        return 0.0


def estimate_entropy(row: pd.Series) -> float:
    """
    Estimate Shannon entropy (0..8) from available features.

    Uses a heuristic: if there is no meaningful payload, entropy is 0.
    Otherwise, scales the average payload length to the 0..8 range
    (capped at 8). This is a PROXY — real entropy requires byte-level
    analysis from raw PCAPs.
    """
    payload_avg = row.get("network_payload-length_avg", 0)
    if pd.isna(payload_avg) or payload_avg <= 1:
        return 0.0

    # Scale: typical payload max ~1500 bytes → 1500 / 200 = 7.5
    return min(8.0, payload_avg / 200.0)


def extract_features(df: pd.DataFrame) -> np.ndarray:
    """
    Extract the 7-dim feature vector from the CIC dataset DataFrame.

    Returns a float32 numpy array of shape (n_samples, 7).
    """
    n = len(df)
    features = np.zeros((n, NUM_FEATURES), dtype=np.float32)

    for i in range(n):
        row = df.iloc[i]

        # [0] Length — average IP packet length
        features[i, 0] = float(row.get("network_ip-length_avg", 0) or 0)

        # [1] Proto_Idx
        protocols = parse_list_str(row.get("network_protocols_all", "[]"))
        features[i, 1] = proto_idx(protocols)

        # [2] SrcPort — first src port
        src_ports = parse_list_str(row.get("network_ports_src", "[]"))
        features[i, 2] = first_port(src_ports)

        # [3] DstPort — first dst port
        dst_ports = parse_list_str(row.get("network_ports_dst", "[]"))
        features[i, 3] = first_port(dst_ports)

        # [4] PacketsPerSec (per-source rate)
        pkt_count = float(row.get("network_packets_src_count", 0) or 0)
        features[i, 4] = pkt_count / WINDOW_SECONDS

        # [5] UniqueDstPorts (per-source unique ports)
        features[i, 5] = float(row.get("network_ports_src_count", 0) or 0)

        # [6] PayloadEntropy (proxy)
        features[i, 6] = estimate_entropy(row)

    return features


def _shape_spec(shape: list) -> list:
    """Normalize ONNX shape: integer dims stay ints, dynamic dims become None."""
    out = []
    for d in shape:
        if isinstance(d, int):
            out.append(d)
        elif isinstance(d, str) and d.strip() in ("", "None"):
            out.append(None)
        else:
            out.append(d)
    return out


def validate_onnx(path: str, expected_inputs: list, expected_outputs: list) -> None:
    """Load an ONNX model and assert expected tensor names and shapes."""
    model = onnx.load(path)
    onnx.checker.check_model(model)

    graph = model.graph
    inputs = {i.name: [d.dim_value if d.dim_value else d.dim_param for d in i.type.tensor_type.shape.dim] for i in graph.input}
    outputs = {o.name: [d.dim_value if d.dim_value else d.dim_param for d in o.type.tensor_type.shape.dim] for o in graph.output}

    print(f"\n  ONNX validation: {path}")
    print(f"    Inputs:  {inputs}")
    print(f"    Outputs: {outputs}")

    for name, shape in expected_inputs:
        assert name in inputs, f"Expected input '{name}' not found in {path}"
        actual = _shape_spec(inputs[name])
        expected = _shape_spec(shape)
        assert actual == expected, f"Input '{name}' shape {actual} != expected {expected} in {path}"

    for name, shape in expected_outputs:
        assert name in outputs, f"Expected output '{name}' not found in {path}"
        actual = _shape_spec(outputs[name])
        expected = _shape_spec(shape)
        assert actual == expected, f"Output '{name}' shape {actual} != expected {expected} in {path}"

    print("    ONNX contract OK")


# ---------------------------------------------------------------------------
# 1. Load data
# ---------------------------------------------------------------------------
print("=" * 60)
print("ViewRed — ML Training with CIC IIoT Dataset 2025")
print("=" * 60)

print("\nLoading datasets...")
attack_path = os.path.join(DATASET_DIR, "processed_files", "attack_data", "attack_samples_5sec.csv")
benign_path = os.path.join(DATASET_DIR, "processed_files", "benign_data", "benign_samples_5sec.csv")

df_attack = pd.read_csv(attack_path, low_memory=False)
df_benign = pd.read_csv(benign_path, low_memory=False)

print(f"  Attack samples: {len(df_attack)}")
print(f"  Benign samples:  {len(df_benign)}")
print(f"  Attack categories:\n{df_attack['label2'].value_counts()}")

# ---------------------------------------------------------------------------
# 2. Extract features
# ---------------------------------------------------------------------------
print("\nExtracting 7-dim feature vectors...")
X_benign = extract_features(df_benign)
X_attack = extract_features(df_attack)
print(f"  X_benign shape: {X_benign.shape}")
print(f"  X_attack shape: {X_attack.shape}")

# Feature statistics
print("\nFeature stats (benign):")
for j in range(NUM_FEATURES):
    col = X_benign[:, j]
    print(f"  [{j}] mean={col.mean():.4f}  std={col.std():.4f}  min={col.min():.4f}  max={col.max():.4f}")

# Labels for RF
y_attack = np.array([CATEGORY_MAP.get(str(v).strip().lower(), 0) for v in df_attack["label2"]], dtype=np.int64)
print(f"\n  RF class distribution: {np.bincount(y_attack)}")

# ---------------------------------------------------------------------------
# 3. Train Isolation Forest (unsupervised — only benign data)
# ---------------------------------------------------------------------------
print("\n" + "-" * 60)
print("Training Isolation Forest (anomaly detection)...")
print("-" * 60)

iso_forest = IsolationForest(
    n_estimators=200,
    contamination=0.05,
    random_state=RANDOM_STATE,
    max_samples="auto",
    bootstrap=False,
    n_jobs=-1,
)
iso_forest.fit(X_benign)

# Quick validation: predict on both benign and attack
benign_preds = iso_forest.predict(X_benign)
attack_preds = iso_forest.predict(X_attack)
anomaly_rate_benign = (benign_preds == -1).mean() * 100
anomaly_rate_attack = (attack_preds == -1).mean() * 100
print(f"  Anomaly rate on BENIGN (false positive): {anomaly_rate_benign:.2f}%")
print(f"  Anomaly rate on ATTACK (true positive):  {anomaly_rate_attack:.2f}%")

# Export to ONNX
initial_type = [("float_input", FloatTensorType([None, NUM_FEATURES]))]
onnx_iso = convert_sklearn(
    iso_forest,
    initial_types=initial_type,
    target_opset={"": 15, "ai.onnx.ml": 3},
)

model_dir = os.path.join(os.path.dirname(os.path.dirname(__file__)), "models")
os.makedirs(model_dir, exist_ok=True)
iso_path = os.path.join(model_dir, "isolation_forest.onnx")
with open(iso_path, "wb") as f:
    f.write(onnx_iso.SerializeToString())
print(f"  Saved {iso_path}")
validate_onnx(
    iso_path,
    expected_inputs=[("float_input", ["None", 7])],
    expected_outputs=[("label", ["None", 1]), ("scores", ["None", 1])],
)

# ---------------------------------------------------------------------------
# 4. Train Random Forest (classification — only attack data)
# ---------------------------------------------------------------------------
print("\n" + "-" * 60)
print("Training Random Forest (attack classification)...")
print("-" * 60)

# Split for validation
X_train, X_test, y_train, y_test = train_test_split(
    X_attack, y_attack, test_size=0.2, random_state=RANDOM_STATE, stratify=y_attack
)

rf = RandomForestClassifier(
    n_estimators=200,
    max_depth=15,
    min_samples_leaf=4,
    random_state=RANDOM_STATE,
    n_jobs=-1,
    class_weight="balanced",
)
rf.fit(X_train, y_train)

# Evaluate
y_pred = rf.predict(X_test)
print(f"\n  Test accuracy: {(y_pred == y_test).mean() * 100:.2f}%")
print(f"\n  Classification report:")
print(classification_report(y_test, y_pred, target_names=[CATEGORY_NAMES[i] for i in sorted(CATEGORY_NAMES.keys())]))

# Feature importance
print(f"\n  Feature importance:")
for j in range(NUM_FEATURES):
    print(f"    [{j}] {rf.feature_importances_[j]:.4f}")

# Export to ONNX
onnx_rf = convert_sklearn(
    rf,
    initial_types=initial_type,
    target_opset={"": 15, "ai.onnx.ml": 3},
)
rf_path = os.path.join(model_dir, "random_forest.onnx")
with open(rf_path, "wb") as f:
    f.write(onnx_rf.SerializeToString())
print(f"\n  Saved {rf_path}")
validate_onnx(
    rf_path,
    expected_inputs=[("float_input", ["None", 7])],
    expected_outputs=[("output_label", ["None"])],
)

print("\n" + "=" * 60)
print("Training complete! Both models exported to models/")
print("=" * 60)
