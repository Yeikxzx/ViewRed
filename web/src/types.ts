export type ViewType =
  | 'dashboard'
  | 'captura'
  | 'filtros'
  | 'alertas'
  | 'topologia'
  | 'vulnerabilidades'
  | 'asistente'
  | 'configuracion';

export interface KPI {
  title: string;
  value: string;
  change: string;
  trend: 'up' | 'down' | 'neutral';
  color: string;
  icon: string;
  action?: () => void;
}

export interface IPVolume {
  ip: string;
  volume: string;
  percentage: number;
}

export interface NetworkAlert {
  id: string;
  timestamp: string;
  severity: 'CRITICAL' | 'WARNING' | 'INFO';
  title: string;
  description: string;
  source: string;
  destination: string;
  confidence: number;
  origin: string;
  timeAgo: string;
  acknowledged: boolean;
  port?: string | number;
}

export interface NetworkDevice {
  id: string;
  name: string;
  ip: string;
  mac: string;
  vendor: string;
  type: 'router' | 'switch' | 'server' | 'database' | 'pc' | 'mobile' | 'iot';
  status: 'online' | 'warning' | 'critical' | 'offline';
  vulnsCount: number;
  lastSeen: string;
  trafficRx: string;
  trafficTx: string;
  x?: number; // Visual placement
  y?: number; // Visual placement
}

export interface Vulnerability {
  id: string;
  cve: string;
  cvss: number;
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO-PORT';
  title: string;
  description: string;
  targetDevice: string;
  targetIP: string;
  detectedAt: string;
  scanId: number;
  mitigated: boolean;
  mitigationSteps: string;
}

export interface CapturedPacket {
  // String id, content-derived (time + 5-tuple + protocol) so the
  // selected packet survives auto-refreshes that would otherwise
  // shift pagination-based numeric ids. The old numeric id was
  // `total - offset - idx`, which silently broke the selection
  // every time a new packet arrived.
  id: string;
  // Display-only number rendered in the "NO." column.
  // Recomputed per-fetch from `total - idx` so it stays
  // monotonically decreasing as the user scrolls; decoupled
  // from `id` because the display number shifting across
  // refreshes is harmless (it's just a label).
  number: number;
  time: string;
  source: string;
  destination: string;
  // Optional resolved hostnames. Populated by the backend's DNS
  // resolver when a recent A/AAAA record mapped the IP to a
  // hostname. Empty string when the cache has no entry — the UI
  // renders nothing in that case rather than a placeholder.
  sourceHostname?: string;
  destinationHostname?: string;
  protocol: string;
  length: number;
  info: string;
  hexDump: string;
  asciiDump: string;
}

export interface ChatMessage {
  id: string | number;
  sender: 'admin' | 'ia' | 'bot';
  timestamp?: string;
  content: string;
}

export interface ScanHistory {
  id: number;
  date: string;
  target: string;
  profile: string;
  duration: string;
  hostsScanned: number;
  vulnsFound: number;
  status: 'COMPLETED' | 'STOPPED' | 'RUNNING';
}
