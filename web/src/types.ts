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
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW';
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
  id: number;
  time: string;
  source: string;
  destination: string;
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
