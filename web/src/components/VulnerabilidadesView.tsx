import React, { useState, useEffect } from 'react';
import {
  ShieldAlert,
  Play,
  History,
  AlertOctagon,
  AlertTriangle,
  Info,
  Check,
  Search,
  Sliders,
  Sparkles,
  RefreshCw,
} from 'lucide-react';
import { Vulnerability, ScanHistory, ViewType } from '../types';
import { initialVulnerabilities, initialScanHistory } from '../data/mockData';
import { triggerScan } from '../lib/api';
import { toast } from 'sonner';

interface VulnerabilidadesViewProps {
  vulnerabilities: Vulnerability[];
  onMitigateVuln: (id: string) => void;
  selectedDeviceIP: string;
  setSelectedDeviceIP: (ip: string) => void;
  refreshVulnerabilities?: () => Promise<void>;
}

export default function VulnerabilidadesView({
  vulnerabilities,
  onMitigateVuln,
  selectedDeviceIP,
  setSelectedDeviceIP,
  refreshVulnerabilities,
}: VulnerabilidadesViewProps) {
  const [severityFilter, setSeverityFilter] = useState<'ALL' | 'CRITICAL' | 'HIGH' | 'MEDIUM'>('ALL');
  const [targetQuery, setTargetQuery] = useState(selectedDeviceIP || '192.168.1.0/24');
  const [scanProfile, setScanProfile] = useState('service-detect');
  const [scanScripts, setScanScripts] = useState(true);

  // Scan progress simulators
  const [scanInProgress, setScanInProgress] = useState(false);
  const [scanProgressPercent, setScanProgressPercent] = useState(0);
  const [hostsScannedCount, setHostsScannedCount] = useState(0);
  const [statusText, setStatusText] = useState('');

  const [localScansHistory, setLocalScansHistory] = useState<ScanHistory[]>(initialScanHistory);
  const [showExplanationModal, setShowExplanationModal] = useState(false);
  const [selectedVulnToExplain, setSelectedVulnToExplain] = useState<Vulnerability | null>(null);
  const [aiStreamingText, setAiStreamingText] = useState('');
  const [aiLoading, setAiLoading] = useState(false);

  // Autofill target query if selected device IP changed
  useEffect(() => {
    if (selectedDeviceIP) {
      setTargetQuery(selectedDeviceIP);
    }
  }, [selectedDeviceIP]);

  // Handle live Nmap scanning simulator
  const handleStartScan = async (e: React.FormEvent) => {
    e.preventDefault();
    if (scanInProgress) return;

    if (!targetQuery) {
      toast.error('Debe ingresar un objetivo a escanear');
      return;
    }

    setScanInProgress(true);
    setScanProgressPercent(0);
    setHostsScannedCount(0);
    setStatusText('Inicializando escáner Nmap y contactando objetivo...');

    // Fake progress interval while waiting for backend
    let percent = 0;
    const interval = setInterval(() => {
      percent += 2;
      if (percent > 90) percent = 90; // Wait at 90% until backend returns
      setScanProgressPercent(percent);
    }, 500);

    const startTime = Date.now();

    try {
      await triggerScan(targetQuery, scanProfile);
      
      clearInterval(interval);
      setScanProgressPercent(100);
      setStatusText('Escaneo terminado de forma exitosa.');
      
      const durationStr = Math.round((Date.now() - startTime) / 1000) + 's';

      // Insert new entry in local scan histories
      const newHistory: ScanHistory = {
        id: localScansHistory.length + 42,
        date: 'Hoy, ' + new Date().toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' }),
        target: targetQuery,
        profile: scanProfile === 'quick' ? 'Quick Scan' : scanProfile === 'full' ? 'Full Scan' : 'Service Detection',
        duration: durationStr,
        hostsScanned: 1,
        vulnsFound: 0,
        status: 'COMPLETED',
      };
      setLocalScansHistory([newHistory, ...localScansHistory]);
      
      toast.success(`Escaneo completado para el objetivo ${targetQuery}`);
      
      if (refreshVulnerabilities) {
        await refreshVulnerabilities();
      }
    } catch (err: any) {
      clearInterval(interval);
      setStatusText('Error en el escaneo: ' + err.message);
      toast.error('Fallo al ejecutar Nmap: ' + err.message);
    } finally {
      setTimeout(() => {
        setScanInProgress(false);
      }, 2000);
    }
  };

  const handleExplainVulnWithAI = (vuln: Vulnerability) => {
    setSelectedVulnToExplain(vuln);
    setShowExplanationModal(true);
    setAiLoading(true);
    setAiStreamingText('');

    let textToStream = `[ANÁLISIS DE INCIDENTE VIEWRED - VULNERABILIDAD CVE]\n\nSe ha evaluado el riesgo asociado a la vulnerabilidad '${vuln.cve}' en el dispositivo '${vuln.targetDevice}' (${vuln.targetIP}).\n\n🔴 GRAVEDAD (CVSS Score: ${vuln.cvss}):\nEsta vulnerabilidad está calificada como de severidad '${vuln.severity}'. Permite a atacantes remotos ejecutar payloads maliciosos comprometiendo el host.\n\n🛠️ INSTRUCCIONES DETALLADAS DE MITIGACIÓN:\n- Pasos inmediatos: ${vuln.mitigationSteps || 'Aplicar parches del distribuidor.'}\n\n¿Desea descargar el reporte integral PDF para compartir con el equipo de SecOps?`;

    let i = 0;
    const interval = setInterval(() => {
      setAiStreamingText((prev) => prev + textToStream.charAt(i));
      i++;
      if (i >= textToStream.length) {
        clearInterval(interval);
        setAiLoading(false);
      }
    }, 7);
  };

  const filteredVulns = vulnerabilities.filter((v) => {
    const matchesSeverity = severityFilter === 'ALL' || v.severity === severityFilter;
    return matchesSeverity;
  });

  return (
    <div id="vulnerabilities-view-wrapper" className="space-y-6 text-left font-sans">
      {/* Configuration Form + Running Progress row */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Nmap scan form parameters - 7 Cols */}
        <form
          onSubmit={handleStartScan}
          className="lg:col-span-7 bg-[#131820] border border-[#2a3242] p-5 rounded-lg space-y-4"
        >
          <div className="flex items-center gap-2 border-b border-[#2a3242] pb-3 mb-1">
            <Sliders className="h-4 w-4 text-[#3b82f6]" />
            <span className="font-sans font-bold text-sm text-[#e4e7eb]">
              Configurar Escaneo de Vulnerabilidades (Nmap / NSE)
            </span>
          </div>

          {/* Targets */}
          <div className="space-y-1">
            <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
              1. Selección de Objetivo (Target / Host / Subred)
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={targetQuery}
                onChange={(e) => {
                  setTargetQuery(e.target.value);
                  setSelectedDeviceIP('');
                }}
                disabled={scanInProgress}
                placeholder="Ej: 192.168.1.0/24 o 10.0.5.12"
                className="w-full bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] rounded px-3.5 py-1.5 text-xs text-[#e4e7eb] outline-none font-mono py-2 disabled:opacity-60"
              />
            </div>
            <span className="text-[9px] text-[#5c6473] font-mono block leading-tight">
              Asegúrese de poseer permisos de auditoría antes de escannear cualquier activo de red.
            </span>
          </div>

          {/* Profiles select */}
          <div className="space-y-2">
            <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
              2. Perfil de Escaneo
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
              <label
                className={`p-2.5 bg-[#0a0e14] border rounded flex items-center gap-2 cursor-pointer transition-all ${
                  scanProfile === 'quick' ? 'border-[#3b82f6] bg-[#3b82f6]/5' : 'border-[#2a3242] hover:border-[#3a4356]'
                }`}
              >
                <input
                  type="radio"
                  name="profile"
                  checked={scanProfile === 'quick'}
                  onChange={() => setScanProfile('quick')}
                  disabled={scanInProgress}
                  className="accent-[#3b82f6]"
                />
                <div className="flex flex-col text-left">
                  <span className="text-xs text-[#e4e7eb] font-bold leading-none">Quick Scan</span>
                  <span className="text-[9px] text-[#5c6473] font-mono mt-1 leading-none">Top 100 ports</span>
                </div>
              </label>

              <label
                className={`p-2.5 bg-[#0a0e14] border rounded flex items-center gap-2 cursor-pointer transition-all ${
                  scanProfile === 'service-detect'
                    ? 'border-[#3b82f6] bg-[#3b82f6]/5'
                    : 'border-[#2a3242] hover:border-[#3a4356]'
                }`}
              >
                <input
                  type="radio"
                  name="profile"
                  checked={scanProfile === 'service-detect'}
                  onChange={() => setScanProfile('service-detect')}
                  disabled={scanInProgress}
                  className="accent-[#3b82f6]"
                />
                <div className="flex flex-col text-left">
                  <span className="text-xs text-[#e4e7eb] font-bold leading-none">OS/Service</span>
                  <span className="text-[9px] text-[#5c6473] font-mono mt-1 leading-none">Recomendado</span>
                </div>
              </label>

              <label
                className={`p-2.5 bg-[#0a0e14] border rounded flex items-center gap-2 cursor-pointer transition-all ${
                  scanProfile === 'full' ? 'border-[#3b82f6] bg-[#3b82f6]/5' : 'border-[#2a3242] hover:border-[#3a4356]'
                }`}
              >
                <input
                  type="radio"
                  name="profile"
                  checked={scanProfile === 'full'}
                  onChange={() => setScanProfile('full')}
                  disabled={scanInProgress}
                  className="accent-[#3b82f6]"
                />
                <div className="flex flex-col text-left">
                  <span className="text-xs text-[#e4e7eb] font-bold leading-none">Full Comprehensive</span>
                  <span className="text-[9px] text-[#5c6473] font-mono mt-1 leading-none">Todos los puertos</span>
                </div>
              </label>
            </div>
          </div>

          {/* NSE Scripts Checkbox */}
          <div className="flex items-center gap-2 bg-[#0a0e14] border border-[#2a3242]/40 p-2.5 rounded">
            <input
              type="checkbox"
              id="sc_scripts"
              checked={scanScripts}
              onChange={(e) => setScanScripts(e.target.checked)}
              disabled={scanInProgress}
              className="accent-[#3b82f6]"
            />
            <label htmlFor="sc_scripts" className="text-xs font-mono text-[#9ba3b4]">
              Ejecutar auditoría agregada de scripts NSE (--script vuln)
            </label>
          </div>

          <div className="flex justify-between items-center border-t border-[#1c2230] pt-3.5">
            <span className="text-[10px] font-mono text-[#5c6473]">
              Tiempo Estimado: {scanProfile === 'quick' ? '15s - 30s' : '1m - 2m'}
            </span>
            <button
              type="submit"
              disabled={scanInProgress}
              className="px-5 py-2 rounded bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white font-mono text-xs font-bold disabled:opacity-60 flex items-center gap-2 transition-all cursor-pointer"
            >
              {scanInProgress ? (
                <>
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                  <span>Escaneando...</span>
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5 fill-current" />
                  <span>Iniciar Escaneo</span>
                </>
              )}
            </button>
          </div>
        </form>

        {/* Scan Running live progress panel or History - 5 Cols */}
        <div className="lg:col-span-5 bg-[#131820] border border-[#2a3242] p-5 rounded-lg flex flex-col justify-between">
          {scanInProgress ? (
            /* Live scanning metrics */
            <div className="space-y-6 flex-grow flex flex-col justify-center text-center py-6">
              <RefreshCw className="h-10 w-10 text-[#3b82f6] animate-spin mx-auto text-center" />
              <div className="space-y-2">
                <span className="font-sans font-bold text-sm text-[#e4e7eb] block uppercase tracking-wider">
                  Escaneo en Curso
                </span>
                <p className="text-xs font-mono text-[#9ba3b4] max-w-xs mx-auto text-center leading-snug">
                  "{statusText}"
                </p>
              </div>

              {/* Progress bar percentage */}
              <div className="space-y-1 max-w-sm mx-auto w-full">
                <div className="flex justify-between items-center text-[10px] font-mono text-[#5c6473]">
                  <span>PROGRESO NMAP (HOSTS)</span>
                  <span>{scanProgressPercent}%</span>
                </div>
                <div className="w-full h-2.5 bg-[#0a0e14] rounded-full overflow-hidden border border-[#2a3242]/50">
                  <div
                    className="h-full bg-[#3b82f6] transition-all duration-300 rounded-full"
                    style={{ width: `${scanProgressPercent}%` }}
                  />
                </div>
                <div className="flex justify-between text-[9px] font-mono text-[#5c6473]">
                  <span>Hosts escaneados: {hostsScannedCount}</span>
                  <span>Tiempo transcurrido: ~32s</span>
                </div>
              </div>
            </div>
          ) : (
            /* Scans History list */
            <div className="space-y-4 flex-grow flex flex-col justify-between h-full">
              <div>
                <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5 mb-2">
                  <History className="h-4 w-4 text-[#5c6473]" />
                  <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                    Historial de Escaneos Recientes
                  </span>
                </div>

                <div className="space-y-2 max-h-56 overflow-y-auto pr-1">
                  {localScansHistory.map((scan) => (
                    <div
                      key={scan.id}
                      className="bg-[#0a0e14] border border-[#2a3242]/40 p-2 px-3 rounded text-xs flex justify-between items-center font-mono"
                    >
                      <div className="flex flex-col gap-1 text-left">
                        <span className="font-bold text-[#e4e7eb] text-[11px] truncate max-w-[200px]">
                          {scan.target}
                        </span>
                        <span className="text-[9px] text-[#5c6473]">
                          {scan.date} • {scan.profile}
                        </span>
                      </div>
                      <div className="text-right flex flex-col items-end gap-1">
                        <span
                          className={`font-bold text-[9px] px-1.5 py-0.5 rounded leading-none ${
                            scan.vulnsFound > 5
                              ? 'bg-red-500/10 text-red-500'
                              : scan.vulnsFound > 0
                              ? 'bg-amber-500/10 text-amber-500'
                              : 'bg-[#10b981]/10 text-[#10b981]'
                          }`}
                        >
                          {scan.vulnsFound} vulns
                        </span>
                        <span className="text-[9px] text-[#5c6473]">
                          DURACIÓN: {scan.duration}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="text-[10px] text-[#5c6473] font-mono border-t border-[#1c2230] pt-2 mt-4">
                <span>Total de {localScansHistory.length} auditorías en base de datos.</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Detected Vulnerabilities title headers */}
      <div className="border-t border-[#2a3242] pt-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4">
          <div className="flex flex-col">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight flex items-center gap-2">
              <ShieldAlert className="h-4.5 w-4.5 text-[#ef4444]" />
              <span>Base de Vulnerabilidades Registradas</span>
            </h2>
            <span className="text-[11px] text-[#9ba3b4] font-sans mt-0.5">
              Auditoría combinada de debilidades lógicas de capa 3 a 7.
            </span>
          </div>

          {/* Quick tab filters by CVSS severity */}
          <div className="flex bg-[#0a0e14] p-1 border border-[#2a3242] rounded-md gap-1 text-[10px] font-mono select-none">
            <button
              onClick={() => setSeverityFilter('ALL')}
              className={`px-2.5 py-1 rounded transition-all ${
                severityFilter === 'ALL' ? 'bg-[#1c2230] text-white font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
              }`}
            >
              TODAS ({vulnerabilities.length})
            </button>
            <button
              onClick={() => setSeverityFilter('CRITICAL')}
              className={`px-2.5 py-1 rounded transition-all ${
                severityFilter === 'CRITICAL' ? 'bg-red-500 text-white font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
              }`}
            >
              CRÍTICAS ({vulnerabilities.filter((v) => v.severity === 'CRITICAL').length})
            </button>
            <button
              onClick={() => setSeverityFilter('HIGH')}
              className={`px-2.5 py-1 rounded transition-all ${
                severityFilter === 'HIGH' ? 'bg-amber-500 text-[#0a0e14] font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
              }`}
            >
              ALTAS ({vulnerabilities.filter((v) => v.severity === 'HIGH').length})
            </button>
          </div>
        </div>

        {/* Vulnerabilities grid cards list */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredVulns.map((vuln) => (
            <div
              key={vuln.id}
              className={`bg-[#131820] border p-4 rounded-lg flex flex-col justify-between text-left transition-all ${
                vuln.mitigated ? 'border-[#2a3242]/40 opacity-70' : 'border-[#2a3242] hover:border-[#3a4356]'
              }`}
            >
              <div>
                <div className="flex justify-between items-center border-b border-[#1c2230]/70 pb-2 mb-2.5">
                  <div className="flex items-center gap-2 font-mono">
                    <span
                      className={`font-bold text-[10px] px-1.5 py-0.5 rounded ${
                        vuln.severity === 'CRITICAL'
                          ? 'bg-red-500/15 text-red-500'
                          : vuln.severity === 'HIGH'
                          ? 'bg-amber-500/15 text-[#f59e0b]'
                          : 'bg-blue-500/15 text-[#3b82f6]'
                      }`}
                    >
                      {vuln.cve}
                    </span>
                    <span className="text-[10px] text-[#5c6473]">CVSS Score: {vuln.cvss}</span>
                  </div>
                  <span className="font-mono text-[9px] text-[#5c6473]">ID: {vuln.id}</span>
                </div>

                <h3 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                  {vuln.title}
                </h3>
                <p className="font-sans text-xs text-[#9ba3b4] mt-1.5 leading-relaxed line-clamp-3">
                  {vuln.description}
                </p>

                {/* impacted host */}
                <div className="bg-[#0a0e14] p-2 mt-3 rounded text-[11px] font-mono flex justify-between">
                  <span className="text-[#5c6473]">Dispositivo Expuesto:</span>
                  <span className="text-[#e4e7eb] font-semibold">
                    {vuln.targetDevice} ({vuln.targetIP})
                  </span>
                </div>
              </div>

              {/* Action buttons inside card */}
              <div className="flex gap-2 border-t border-[#1c2230] pt-3 mt-4">
                <button
                  onClick={() => handleExplainVulnWithAI(vuln)}
                  className="px-3 py-1 bg-[#3b82f6]/10 hover:bg-[#3b82f6]/20 border border-[#3b82f6]/30 text-[#3b82f6] rounded text-xs font-mono font-medium flex items-center gap-1 cursor-pointer"
                >
                  <Sparkles className="h-3 w-3" />
                  <span>Preguntar IA</span>
                </button>

                {!vuln.mitigated ? (
                  <button
                    onClick={() => onMitigateVuln(vuln.id)}
                    className="ml-auto px-3 py-1 text-[#10b981] hover:bg-[#10b981]/15 scale-100 hover:scale-[1.02] active:scale-[0.98] border border-[#10b981]/30 rounded text-xs font-mono transition-all font-bold cursor-pointer"
                  >
                    Mitigar Riesgo
                  </button>
                ) : (
                  <span className="ml-auto text-[#10b981] text-xs font-mono font-bold flex items-center gap-1">
                    <Check className="h-4.5 w-4.5" />
                    <span>Mitigado</span>
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* AI Explanation dialog */}
      {showExplanationModal && selectedVulnToExplain && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#131820] w-full max-w-xl border border-[#3a4356] shadow-2xl rounded-lg overflow-hidden flex flex-col text-left font-sans">
            {/* Modal Header */}
            <div className="bg-[#0a0e14] border-b border-[#2a3242] p-4 flex justify-between items-center">
              <div className="flex items-center gap-2">
                <Sparkles className="h-4.5 w-4.5 text-[#3b82f6]" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  Análisis de Parche Vulnerabilidad
                </span>
              </div>
              <button
                onClick={() => setShowExplanationModal(false)}
                className="text-[#9ba3b4] hover:text-white"
              >
                Cerrar
              </button>
            </div>

            {/* Modal response body */}
            <div className="p-5 overflow-y-auto max-h-[460px] space-y-4">
              <div className="space-y-1">
                <span className="text-[10px] font-mono font-bold text-[#5c6473] uppercase tracking-wider block">
                  {selectedVulnToExplain.cve}
                </span>
                <span className="font-sans font-bold text-[#e4e7eb] text-sm">
                  {selectedVulnToExplain.title}
                </span>
              </div>

              {/* Streaming box container */}
              <div className="bg-[#0a0e14] border border-[#2a3242] rounded-md p-4 font-mono text-xs whitespace-pre-wrap leading-relaxed text-[#3b82f6] h-60 overflow-y-auto select-text prose-invert select-text">
                {aiStreamingText}
                {aiLoading && (
                  <span className="inline-block w-2 h-4 bg-[#3b82f6] ml-1 animate-pulse" />
                )}
              </div>
            </div>

            {/* Modal Footer Controls */}
            <div className="bg-[#0a0e14] border-t border-[#2a3242] px-5 py-3 flex justify-end gap-2 text-xs">
              <button
                onClick={() => setShowExplanationModal(false)}
                className="px-3 py-1.5 border border-[#2a3242] rounded text-xs font-mono text-[#9ba3b4]"
              >
                Cerrar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
