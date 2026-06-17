import { useState } from 'react';
import {
  Network,
  Tv,
  Database,
  Smartphone,
  Server,
  Activity,
  ChevronRight,
  Shield,
  HelpCircle,
  TrendingUp,
  X,
  Play,
} from 'lucide-react';
import { NetworkDevice, ViewType } from '../types';

interface TopologiaViewProps {
  devices: NetworkDevice[];
  setView: (view: ViewType) => void;
  setSelectedDeviceIP: (ip: string) => void;
}

export default function TopologiaView({ devices, setView, setSelectedDeviceIP }: TopologiaViewProps) {
  const [selectedDevice, setSelectedDevice] = useState<NetworkDevice | null>(devices[3]); // Default select web-srv-prod

  const handleDeviceClick = (dev: NetworkDevice) => {
    setSelectedDevice(dev);
  };

  const handleScanDevice = (ip: string) => {
    setSelectedDeviceIP(ip);
    setView('vulnerabilidades');
  };

  const getDeviceIcon = (type: string) => {
    switch (type) {
      case 'router':
        return <Network className="h-4.5 w-4.5" />;
      case 'switch':
        return <Activity className="h-4.5 w-4.5" />;
      case 'server':
        return <Server className="h-4.5 w-4.5" />;
      case 'database':
        return <Database className="h-4.5 w-4.5" />;
      case 'pc':
        return <Tv className="h-4.5 w-4.5" />;
      case 'mobile':
        return <Smartphone className="h-4.5 w-4.5" />;
      default:
        return <HelpCircle className="h-4.5 w-4.5" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'online':
        return 'border-[#10b981] text-[#10b981] shadow-lg shadow-[#10b981]/10 bg-[#10b981]/5';
      case 'warning':
        return 'border-[#f59e0b] text-[#f59e0b] shadow-lg shadow-[#f59e0b]/10 bg-[#f59e0b]/5';
      case 'critical':
        return 'border-red-500 text-red-500 shadow-lg shadow-red-500/10 bg-red-500/5';
      default:
        return 'border-gray-500 text-gray-400 bg-gray-500/5';
    }
  };

  return (
    <div id="topologia-view-wrapper" className="grid grid-cols-1 lg:grid-cols-12 gap-6 text-left">
      {/* 2D interactive canvas: 8 Cols */}
      <div className="lg:col-span-8 space-y-4">
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between h-[520px] relative">
          {/* Canvas Tool Header */}
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 z-10">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight flex items-center gap-2">
              <Network className="h-4 w-4 text-[#3b82f6]" />
              <span>Grafo de Topología Activo</span>
            </h2>
            <div className="text-[10px] font-mono text-[#5c6473]">
              <span>Haga click en los nodos para inspeccionar endpoints</span>
            </div>
          </div>

          {/* Canvas Background with detailed dotted grid */}
          <div
            id="topology-graph-canvas"
            className="flex-1 w-full bg-[#0a0e14] border border-[#1c2230]/70 rounded mt-3 relative overflow-hidden select-none"
            style={{
              backgroundImage: 'radial-gradient(#1c2230 1px, transparent 1px)',
              backgroundSize: '16px 16px',
            }}
          >
            {/* Connection Edges lines */}
            <svg className="absolute inset-0 w-full h-full pointer-events-none">
              {/* CORE-RT-01 (400,120) -> DIST-SW-A (250,240) */}
              <line x1="400" y1="120" x2="250" y2="240" stroke="#3a4356" strokeWidth="1.5" strokeDasharray="4 4" />
              {/* CORE-RT-01 (400,120) -> DIST-SW-B (550,240) */}
              <line x1="400" y1="120" x2="550" y2="240" stroke="#3a4356" strokeWidth="1.5" />

              {/* DIST-SW-A (250,240) -> WEB-SRV-PROD (250,380) */}
              <line x1="250" y1="240" x2="250" y2="380" stroke="#ef4444" strokeWidth="2" className="animate-pulse" />
              {/* DIST-SW-A (250,240) -> DB-SRV-01 (400,380) */}
              <line x1="250" y1="240" x2="400" y2="380" stroke="#3a4356" strokeWidth="1.5" />

              {/* DIST-SW-B (550,240) -> gateway-edge-01 (650,380) */}
              <line x1="550" y1="240" x2="650" y2="380" stroke="#3a4356" strokeWidth="1.5" />
              {/* DIST-SW-B (550,240) -> api-gateway-us (100,380) */}
              <line x1="550" y1="240" x2="100" y2="380" stroke="#3a4356" strokeWidth="1.5" />
            </svg>

            {/* Nodes representation */}
            {devices.map((dev) => {
              const iconObj = getDeviceIcon(dev.type);
              const statusBorderClass = getStatusColor(dev.status);
              const isSelected = selectedDevice?.id === dev.id;

              return (
                <div
                  key={dev.id}
                  onClick={() => handleDeviceClick(dev)}
                  className={`absolute rounded px-3 py-1.5 border flex items-center gap-2 cursor-pointer transition-all ${statusBorderClass} ${
                    isSelected
                      ? 'ring-2 ring-[#3b82f6] scale-105 z-20'
                      : 'hover:scale-102 hover:brightness-110'
                  }`}
                  style={{ left: `${dev.x}px`, top: `${dev.y}px`, transform: 'translate(-50%, -50%)' }}
                >
                  <div className="flex-shrink-0">{iconObj}</div>
                  <div className="flex flex-col text-left">
                    <span className="font-sans font-bold text-[10px] text-white tracking-tight leading-none uppercase">
                      {dev.name}
                    </span>
                    <span className="font-mono text-[9px] text-[#9ba3b4] mt-0.5 leading-none">
                      {dev.ip}
                    </span>
                  </div>

                  {/* Red/amber warning tag counting vulnerabilities (Vulnerabilities flag badge on node) */}
                  {dev.vulnsCount > 0 && (
                    <span className="absolute -top-1.5 -right-1.5 bg-red-500 font-mono text-[9px] font-bold text-white px-1 h-3.5 rounded-full flex items-center justify-center min-w-[14px]">
                      {dev.vulnsCount}
                    </span>
                  )}
                </div>
              );
            })}

            {/* Canvas overlay instruction card */}
            <div className="absolute bottom-3 left-3 bg-[#131820]/90 border border-[#2a3242] p-2.5 rounded text-[10px] font-mono leading-tight space-y-1 select-none pointer-events-none">
              <span className="text-white block font-bold uppercase pb-0.5 mb-1 border-b border-[#2a3242]/50">
                LEYENDA DE RIESGO
              </span>
              <div className="flex items-center gap-1.5 text-[#10b981]">
                <span className="w-1.5 h-1.5 bg-[#10b981] rounded-full" />
                <span>OPERATIVO / ESTABLE</span>
              </div>
              <div className="flex items-center gap-1.5 text-[#f59e0b]">
                <span className="w-1.5 h-1.5 bg-[#f59e0b] rounded-full" />
                <span>ADVERTENCIA / CVEs DETECTADOS</span>
              </div>
              <div className="flex items-center gap-1.5 text-red-500">
                <span className="w-1.5 h-1.5 bg-red-500 rounded-full" />
                <span>CRÍTICO / AMENAZAS EN CURSO</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Selected Node Inspector side panel: 4 Cols */}
      <div className="lg:col-span-4 space-y-4">
        {selectedDevice ? (
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col h-[520px] justify-between">
            <div>
              {/* Header */}
              <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 mb-3.5">
                <div className="flex items-center gap-2">
                  <span
                    className={`w-2 h-2 rounded-full ${
                      selectedDevice.status === 'online'
                        ? 'bg-[#10b981]'
                        : selectedDevice.status === 'warning'
                        ? 'bg-[#f59e0b]'
                        : 'bg-red-500 animate-pulse'
                    }`}
                  />
                  <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                    Inspección: {selectedDevice.name}
                  </h2>
                </div>
                <span className="text-[10px] font-mono text-[#5c6473] uppercase tracking-wider">
                  {selectedDevice.type}
                </span>
              </div>

              {/* Endpoint Telemetry Properties list */}
              <div className="space-y-3 font-mono text-xs">
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">IP Address</span>
                  <span className="text-[#e4e7eb] font-semibold">{selectedDevice.ip}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">MAC Address</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.mac}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">Distribuidor (OEM)</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.vendor}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">Último Registro</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.lastSeen}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">Ancho rx / tx</span>
                  <span className="text-[#e4e7eb] font-semibold text-right">
                    {selectedDevice.trafficRx} / {selectedDevice.trafficTx}
                  </span>
                </div>
              </div>

              {/* Vulnerabilities counts list associated to endpoint */}
              <div className="mt-5 space-y-2.5">
                <span className="text-[10px] uppercase font-bold tracking-wider text-[#5c6473] font-mono block">
                  Auditoría de Vulnerabilidades
                </span>

                <div className="bg-[#0a0e14] border border-[#2a3242] p-3 rounded-md space-y-2 text-xs">
                  <div className="flex justify-between items-center">
                    <span className="text-[#9ba3b4] font-sans">Nivel de Vulnerabilidades</span>
                    <span
                      className={`font-mono font-bold px-1.5 py-0.5 rounded text-[10px] ${
                        selectedDevice.vulnsCount > 5
                          ? 'bg-red-500/10 text-red-500'
                          : selectedDevice.vulnsCount > 0
                          ? 'bg-amber-500/10 text-amber-500'
                          : 'bg-[#10b981]/10 text-[#10b981]'
                      }`}
                    >
                      {selectedDevice.vulnsCount} CVEs encontradas
                    </span>
                  </div>

                  {selectedDevice.vulnsCount > 0 ? (
                    <div className="pt-1.5 space-y-1.5 border-t border-[#1c2230]/70 font-sans">
                      <div className="flex gap-2 items-center text-[11px] text-[#e4e7eb]">
                        <Shield className="h-3 w-3.5 text-red-500" />
                        <span>Log4Shell CVE-2021-44228 (Crítico, CVSS 10.0)</span>
                      </div>
                      <div className="flex gap-2 items-center text-[11px] text-[#e4e7eb]">
                        <Shield className="h-3 w-3.5 text-amber-500" />
                        <span>HTTP/2 Rapid Reset CVE-2023-44487 (Alto)</span>
                      </div>
                    </div>
                  ) : (
                    <div className="pt-1.5 border-t border-[#1c2230]/70 text-[10px] font-mono text-[#5c6473]">
                      No hay brechas de seguridad registradas en este host.
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Quick scanning actions */}
            <div className="space-y-2 pt-2">
              <button
                onClick={() => handleScanDevice(selectedDevice.ip)}
                className="w-full bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white py-1.5 rounded text-xs font-mono font-bold flex items-center justify-center gap-2 transition-all cursor-pointer"
              >
                <Play className="h-3.5 w-3.5 fill-current" />
                <span>Escanear este dispositivo</span>
              </button>
              <button
                onClick={() => setView('asistente')}
                className="w-full bg-transparent border border-[#2a3242] hover:bg-[#1c2230] text-[#9ba3b4] hover:text-white py-1.5 rounded text-xs font-mono transition-all cursor-pointer"
              >
                <span>Preguntar sobre Host al Asistente IA</span>
              </button>
            </div>
          </div>
        ) : (
          <div className="bg-[#131820] border border-[#2a3242] p-12 rounded-lg flex flex-col items-center justify-center h-[520px] text-center">
            <X className="h-10 w-10 text-[#5c6473] mb-3" />
            <h3 className="font-sans font-semibold text-sm text-[#e4e7eb]">Seleccione un Endpoint</h3>
            <p className="text-xs text-[#9ba3b4] mt-1">Interactúe con el mapa de topología para auditar en profundidad.</p>
          </div>
        )}
      </div>
    </div>
  );
}
