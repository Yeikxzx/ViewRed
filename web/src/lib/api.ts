import { NetworkAlert } from '../types';

export interface TopTalkerBackend {
  src_ip: string;
  bytes: number;
  packets: number;
}

export interface TopTalkersResponse {
  since: string;
  top_talkers: TopTalkerBackend[];
}

export interface ProtocolShareBackend {
  protocol: string;
  percentage: number;
}

export interface ProtocolsResponse {
  since: string;
  protocols: ProtocolShareBackend[];
}

export interface AlertBackend {
  id: number;
  time: string;
  severity: string;
  source: string;
  score: number;
  title: string;
  description?: string;
  context: any;
  acknowledged_at?: string;
}

export interface AlertsResponse {
  alerts: AlertBackend[];
  limit: number;
  offset: number;
  total: number;
}

const API_BASE = '/api/v1';

export interface ConfigBackend {
  version: string;
  features: string[];
}

export interface NetworkInterface {
  name: string;
  description: string;
  addresses: string[];
}

export interface CaptureConfig {
  interface: string;
  bpf: string;
  snaplen: number;
}

export async function getConfig(): Promise<ConfigBackend> {
  const res = await fetch('/api/v1/config');
  if (!res.ok) throw new Error('Failed to fetch config');
  return res.json();
}

export async function getInterfaces(): Promise<NetworkInterface[]> {
  const res = await fetch('/api/v1/interfaces');
  if (!res.ok) throw new Error('Failed to fetch interfaces');
  return res.json();
}

export async function getCaptureConfig(): Promise<CaptureConfig> {
  const res = await fetch('/api/v1/capture/config');
  if (!res.ok) throw new Error('Failed to fetch capture config');
  return res.json();
}

export async function restartCapture(): Promise<void> {
  const res = await fetch('/api/v1/capture/restart', { method: 'POST' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || 'Failed to restart capture');
  }
}

export async function stopCapture(): Promise<void> {
  const res = await fetch('/api/v1/capture/stop', { method: 'POST' });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || 'Failed to stop capture');
  }
}

export async function updateCaptureConfig(config: CaptureConfig): Promise<CaptureConfig> {
  const res = await fetch('/api/v1/capture/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
  if (!res.ok) throw new Error('Failed to update capture config');
  return res.json();
}

export async function getTopTalkers(): Promise<TopTalkersResponse> {
  const res = await fetch(`${API_BASE}/dashboard/top-talkers`);
  if (!res.ok) throw new Error('Error fetching top talkers');
  return res.json();
}

export async function getProtocols(): Promise<ProtocolsResponse> {
  const res = await fetch(`${API_BASE}/dashboard/protocols`);
  if (!res.ok) throw new Error('Error fetching protocols');
  return res.json();
}

export async function getRecentAlerts(limit = 4): Promise<NetworkAlert[]> {
  const res = await fetch(`${API_BASE}/alerts?limit=${limit}`);
  if (!res.ok) throw new Error('Error fetching alerts');
  
  const data: AlertsResponse = await res.json();
  
  // Map to frontend NetworkAlert type
  return data.alerts.map((a): NetworkAlert => ({
    id: String(a.id),
    timestamp: a.time,
    severity: (a.severity.toUpperCase() === 'CRITICAL' || a.severity.toUpperCase() === 'WARNING') 
      ? (a.severity.toUpperCase() as 'CRITICAL' | 'WARNING') 
      : 'INFO',
    title: a.title,
    description: a.description || '',
    source: a.source,
    destination: 'Desconocido', // API might not have this depending on Context
    confidence: a.score,
    origin: 'IA Detector',
    timeAgo: calculateTimeAgo(new Date(a.time)),
    acknowledged: !!a.acknowledged_at,
  }));
}

function calculateTimeAgo(date: Date): string {
  const diffMs = new Date().getTime() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  
  if (diffSecs < 60) return 'Hace un momento';
  const diffMins = Math.floor(diffSecs / 60);
  if (diffMins < 60) return `Hace ${diffMins} min`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `Hace ${diffHours} h`;
  return `Hace ${Math.floor(diffHours / 24)} d`;
}

export interface PacketBackend {
  time: string;
  src_ip: string;
  dst_ip: string;
  // Hostnames resolved from the backend's DNS cache. Optional —
  // the backend only populates these when it has a recent A/AAAA
  // record that mapped the IP to a name. The UI renders the IP
  // alone when both are empty.
  src_hostname?: string;
  dst_hostname?: string;
  src_port: number;
  dst_port: number;
  protocol: string;
  length: number;
  payload_preview: string | null;
}

export interface PacketsResponse {
  packets: PacketBackend[];
  total: number;
  limit: number;
  offset: number;
}

export interface PacketFilters {
  since?: string;
  until?: string;
  protocol?: string;
  src_ip?: string;
  dst_ip?: string;
  port?: number;
  payload_search?: string;
  limit?: number;
  offset?: number;
}

export async function getPackets(filters: PacketFilters = {}): Promise<PacketsResponse> {
  const params = new URLSearchParams();
  if (filters.since) params.append('since', filters.since);
  if (filters.until) params.append('until', filters.until);
  if (filters.protocol) params.append('protocol', filters.protocol);
  if (filters.src_ip) params.append('src_ip', filters.src_ip);
  if (filters.dst_ip) params.append('dst_ip', filters.dst_ip);
  if (filters.port) params.append('port', String(filters.port));
  if (filters.payload_search) params.append('payload_search', filters.payload_search);
  if (filters.limit) params.append('limit', String(filters.limit));
  if (filters.offset) params.append('offset', String(filters.offset));

  const res = await fetch(`${API_BASE}/packets?${params.toString()}`);
  if (!res.ok) throw new Error('Error fetching packets');
  return res.json();
}

export async function acknowledgeAlert(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/alerts/${id}/ack`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Error acknowledging alert');
}

export async function deleteAlert(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/alerts/${id}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Error deleting alert');
}

export async function clearAlerts(): Promise<void> {
  const res = await fetch(`${API_BASE}/alerts`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Error clearing alerts');
}

export interface VulnerabilityBackend {
  id: number;
  device_ip: string;
  cve_id: string | null;
  cvss_score: number | null;
  description: string;
  port: number | null;
  mitigated: boolean;
  discovered_at: string;
}

export async function getVulnerabilities(includeMitigated = false): Promise<VulnerabilityBackend[]> {
  const res = await fetch(`${API_BASE}/vulnerabilities?include_mitigated=${includeMitigated}`);
  if (!res.ok) throw new Error('Error fetching vulnerabilities');
  return res.json();
}

export async function triggerScan(ip: string, profile: string): Promise<void> {
  const res = await fetch(`${API_BASE}/scan`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ip, profile }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || 'Scan failed');
  }
}

export interface ScanHistoryBackend {
  id: number;
  target: string;
  profile: string;
  duration_ms: number;
  hosts_scanned: number;
  vulns_found: number;
  status: string;
  started_at: string;
}

export interface ScanHistoryResponse {
  history: ScanHistoryBackend[];
  limit: number;
}

export async function getScanHistory(limit = 20): Promise<ScanHistoryBackend[]> {
  const res = await fetch(`${API_BASE}/scan-history?limit=${limit}`);
  if (!res.ok) throw new Error('Error fetching scan history');
  const data: ScanHistoryResponse = await res.json();
  return data.history;
}

export async function mitigateVulnerability(id: number): Promise<void> {
  const res = await fetch(`${API_BASE}/vulnerabilities/${id}/mitigate`, {
    method: 'POST',
  });
  if (!res.ok) throw new Error('Error mitigating vulnerability');
}

export interface SystemMetrics {
  cpu_percent: number;
  ram_percent: number;
  ram_used_mb: number;
  ram_total_mb: number;
  uptime_seconds: number;
}

export async function getSystemMetrics(): Promise<SystemMetrics> {
  const res = await fetch(`${API_BASE}/system/metrics`);
  if (!res.ok) throw new Error('Error fetching system metrics');
  return res.json();
}

export interface StoragePolicy {
  max_pcap_size_mb: number;
  max_pcap_age_hours: number;
}

export async function getStoragePolicy(): Promise<StoragePolicy> {
  const res = await fetch(`${API_BASE}/capture/storage-policy`);
  if (!res.ok) throw new Error('Error fetching storage policy');
  return res.json();
}

export async function updateStoragePolicy(policy: StoragePolicy): Promise<StoragePolicy> {
  const res = await fetch(`${API_BASE}/capture/storage-policy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(policy),
  });
	if (!res.ok) throw new Error('Error updating storage policy');
	return res.json();
}

export interface PcapFile {
  name: string;
  size: number;
  timestamp: string;
}

export async function getPcaps(): Promise<PcapFile[]> {
  const res = await fetch(`${API_BASE}/pcaps`);
  if (!res.ok) throw new Error('Error fetching pcaps');
  return res.json();
}

export async function deletePcap(filename: string): Promise<void> {
  const res = await fetch(`${API_BASE}/pcaps/${encodeURIComponent(filename)}`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Error deleting pcap');
}

export async function deleteAllPcaps(): Promise<void> {
  const res = await fetch(`${API_BASE}/pcaps`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error('Error clearing pcaps');
}
