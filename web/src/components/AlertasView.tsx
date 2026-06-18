import { useState } from 'react';
import {
  Bell,
  Check,
  AlertOctagon,
  AlertTriangle,
  Info,
  ChevronRight,
  Sparkles,
  SlidersHorizontal,
  X,
  Search,
} from 'lucide-react';
import { NetworkAlert, ViewType } from '../types';

interface AlertasViewProps {
  alerts: NetworkAlert[];
  onAcknowledgeAlert: (id: string) => void;
  setSelectedAlertToExplain: (alert: NetworkAlert | null) => void;
  setView: (view: ViewType) => void;
}

export default function AlertasView({
  alerts,
  onAcknowledgeAlert,
  setSelectedAlertToExplain,
  setView,
}: AlertasViewProps) {
  const [severityFilter, setSeverityFilter] = useState<'ALL' | 'CRITICAL' | 'WARNING' | 'INFO'>('ALL');
  const [searchPhrase, setSearchPhrase] = useState('');
  const [showExplanationModal, setShowExplanationModal] = useState(false);
  const [explainingAlert, setExplainingAlert] = useState<NetworkAlert | null>(null);

  // AI Streaming Simulator variables
  const [aiStreamingText, setAiStreamingText] = useState('');
  const [aiLoading, setAiLoading] = useState(false);

  const simulateAIServicExplanation = (alert: NetworkAlert) => {
    setExplainingAlert(alert);
    setShowExplanationModal(true);
    setAiLoading(true);
    setAiStreamingText('');

    let textToStream = `[ANÁLISIS DE SEGUREDAD VIEWRED - ASISTENTE IA]\n\nSe ha evaluado la amenaza '${alert.title}' originada desde el host ${alert.source}.\n\n🟢 DIAGNÓSTICO:\nSe clasifica este incidente con un nivel de confianza del ${alert.confidence}%. Creemos que el host origen está ejecutando un script de emulación con firmas anómalas. Hay rastro de ${alert.origin || 'firmas automáticas'}.\n\n🛡️ PASOS DE MITIGACIÓN RECOMENDADOS:\n1. Aislar inmediatamente el endpoint origen ${alert.source} de la VLAN de producción mediante una regla del WAF.\n2. Ejecutar un escaneo completo de vulnerabilidades (Nmap Vuln/NSE) contra ${alert.source} desde el panel de Vulnerabilidades.\n3. Auditar los registros de autenticación del puerto ${alert.port || 'TCP'} en busca de inyecciones de comandos LDAP o SSH.\n\n¿Desea bloquear la IP en el firewall ahora de manera automática?`;

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

  const handleExplainWithAIBtn = (alert: NetworkAlert) => {
    // Also bind it to global state so it transfers over to the AI chat tab natively!
    setSelectedAlertToExplain(alert);
    simulateAIServicExplanation(alert);
  };

  const handleOpenAIChatTab = () => {
    setShowExplanationModal(false);
    setView('asistente');
  };

  const filteredAlerts = alerts.filter((alert) => {
    const matchesSeverity = severityFilter === 'ALL' || alert.severity === severityFilter;
    const matchesSearch =
      alert.title.toLowerCase().includes(searchPhrase.toLowerCase()) ||
      alert.source.includes(searchPhrase) ||
      alert.destination.toLowerCase().includes(searchPhrase.toLowerCase());
    return matchesSeverity && matchesSearch;
  });

  const criticalCount = alerts.filter((a) => a.severity === 'CRITICAL' && !a.acknowledged).length;
  const warningCount = alerts.filter((a) => a.severity === 'WARNING' && !a.acknowledged).length;
  const infoCount = alerts.filter((a) => a.severity === 'INFO' && !a.acknowledged).length;

  return (
    <div id="alerts-view-wrapper" className="space-y-6 text-left">
      {/* Alert Severity Summary Cards */}
      <div id="alerts-status-counters-grid" className="grid grid-cols-1 sm:grid-cols-4 gap-4 select-none">
        {/* Critical Card */}
        <div
          onClick={() => setSeverityFilter('CRITICAL')}
          className={`bg-[#131820] border p-4 rounded-lg text-left cursor-pointer transition-all ${
            severityFilter === 'CRITICAL' ? 'border-[#ef4444] shadow-lg shadow-red-500/10' : 'border-[#2a3242]'
          }`}
        >
          <div className="flex justify-between items-center">
            <span className="text-xs font-mono text-[#9ba3b4] font-bold uppercase tracking-wider">
              CRÍTICO
            </span>
            <AlertOctagon className="h-4 w-4 text-[#ef4444]" />
          </div>
          <span className="text-3xl font-mono font-bold text-white block mt-2">
            {criticalCount}
          </span>
          <span className="text-[10px] font-mono text-red-400 block mt-1">
            +3 en la última hora
          </span>
        </div>

        {/* Warning Card */}
        <div
          onClick={() => setSeverityFilter('WARNING')}
          className={`bg-[#131820] border p-4 rounded-lg text-left cursor-pointer transition-all ${
            severityFilter === 'WARNING' ? 'border-[#f59e0b] shadow-lg shadow-amber-500/10' : 'border-[#2a3242]'
          }`}
        >
          <div className="flex justify-between items-center">
            <span className="text-xs font-mono text-[#9ba3b4] font-bold uppercase tracking-wider">
              ADVERTENCIA
            </span>
            <AlertTriangle className="h-4 w-4 text-[#f59e0b]" />
          </div>
          <span className="text-3xl font-mono font-bold text-white block mt-2">
            {warningCount}
          </span>
          <span className="text-[10px] font-mono text-[#f59e0b] block mt-1">
            -5 vs ayer
          </span>
        </div>

        {/* Info Card */}
        <div
          onClick={() => setSeverityFilter('INFO')}
          className={`bg-[#131820] border p-4 rounded-lg text-left cursor-pointer transition-all ${
            severityFilter === 'INFO' ? 'border-[#06b6d4] shadow-lg shadow-cyan-500/10' : 'border-[#2a3242]'
          }`}
        >
          <div className="flex justify-between items-center">
            <span className="text-xs font-mono text-[#9ba3b4] font-bold uppercase tracking-wider">
              INFO / NEUTRO
            </span>
            <Info className="h-4 w-4 text-[#06b6d4]" />
          </div>
          <span className="text-3xl font-mono font-bold text-white block mt-2">
            {infoCount}
          </span>
          <span className="text-[10px] font-mono text-[#06b6d4] block mt-1">
            Tráfico normal observativo
          </span>
        </div>

        {/* Config / Custom rules Card */}
        <div
          onClick={() => setSeverityFilter('ALL')}
          className={`bg-[#131820] border p-4 rounded-lg text-left flex flex-col justify-between cursor-pointer transition-all ${
            severityFilter === 'ALL' ? 'border-[#3b82f6]' : 'border-[#2a3242]'
          }`}
        >
          <div className="flex justify-between items-center">
            <span className="text-xs font-mono text-[#9ba3b4] font-bold uppercase tracking-wider">
              TOTAL ALERTAS
            </span>
            <SlidersHorizontal className="h-4 w-4 text-[#9ba3b4]" />
          </div>
          <span className="text-3xl font-mono font-bold text-[#e4e7eb] block mt-2">
            {alerts.length}
          </span>
          <span className="text-[10px] font-mono text-[#5c6473] block mt-1">
            Ver todas las incidencias
          </span>
        </div>
      </div>

      {/* Aler Stream Filters toolbar */}
      <div className="bg-[#131820] border border-[#2a3242] p-3 rounded-lg flex flex-col sm:flex-row gap-3 items-center justify-between">
        {/* Left Side Quick Toggle tabs */}
        <div className="flex bg-[#0a0e14] p-1.5 border border-[#2a3242] rounded-md gap-1 text-xs select-none">
          <button
            onClick={() => setSeverityFilter('ALL')}
            className={`px-3 py-1 rounded transition-all ${
              severityFilter === 'ALL' ? 'bg-[#1c2230] text-white font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
            }`}
          >
            Todas
          </button>
          <button
            onClick={() => setSeverityFilter('CRITICAL')}
            className={`px-3 py-1 rounded transition-all ${
              severityFilter === 'CRITICAL' ? 'bg-red-500 text-white font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
            }`}
          >
            Crítica
          </button>
          <button
            onClick={() => setSeverityFilter('WARNING')}
            className={`px-3 py-1 rounded transition-all ${
              severityFilter === 'WARNING' ? 'bg-amber-500 text-[#0a0e14] font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
            }`}
          >
            Advertencia
          </button>
          <button
            onClick={() => setSeverityFilter('INFO')}
            className={`px-3 py-1 rounded transition-all ${
              severityFilter === 'INFO' ? 'bg-cyan-500 text-[#0a0e14] font-semibold' : 'text-[#9ba3b4] hover:text-[#e4e7eb]'
            }`}
          >
            Info
          </button>
        </div>

        {/* Search input in Alerts */}
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-[#5c6473]" />
          <input
            type="text"
            placeholder="Buscar por IPs, firmas o tags..."
            value={searchPhrase}
            onChange={(e) => setSearchPhrase(e.target.value)}
            className="w-full bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] rounded pl-9 pr-4 py-1.5 text-xs text-[#e4e7eb] outline-none font-mono"
          />
        </div>
      </div>

      {/* Aler Stream List list */}
      <div id="alerts-stream-list" className="space-y-3">
        {filteredAlerts.length > 0 ? (
          filteredAlerts.map((alert) => (
            <div
              key={alert.id}
              className={`bg-[#131820] border rounded-lg p-4 text-left shadow duration-200 transition-all ${
                alert.acknowledged
                  ? 'border-[#2a3242]/40 opacity-70'
                  : alert.severity === 'CRITICAL'
                  ? 'border-l-4 border-l-red-500 border-[#2a3242] hover:border-red-500/50'
                  : alert.severity === 'WARNING'
                  ? 'border-l-4 border-l-amber-500 border-[#2a3242] hover:border-amber-500/50'
                  : 'border-l-4 border-l-cyan-500 border-[#2a3242] hover:border-cyan-500/50'
              }`}
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                {/* Header context */}
                <div className="flex items-center gap-2.5">
                  <span
                    className={`font-mono font-bold text-[10px] px-2 py-0.5 rounded tracking-wider ${
                      alert.severity === 'CRITICAL'
                        ? 'bg-red-500/10 text-red-500'
                        : alert.severity === 'WARNING'
                        ? 'bg-amber-500/10 text-amber-500'
                        : 'bg-cyan-500/10 text-cyan-500'
                    }`}
                  >
                    {alert.severity}
                  </span>
                  <span className="font-mono text-xs text-[#5c6473]">{alert.timestamp} UTC</span>
                  {!alert.acknowledged && (
                    <span className="w-1.5 h-1.5 bg-red-500 rounded-full animate-ping" />
                  )}
                </div>

                {/* Score badge / trust info */}
                <div className="text-[11px] font-mono text-[#5c6473]">
                  <span>Confianza: <strong className="text-[#e4e7eb] font-bold">{alert.confidence}%</strong></span>
                  <span className="mx-2">•</span>
                  <span>Origen: <strong className="text-[#e4e7eb] font-bold">{alert.origin}</strong></span>
                </div>
              </div>

              {/* Title and descriptions */}
              <div className="mt-2.5">
                <h3 className="font-sans font-bold text-[#e4e7eb] text-sm tracking-tight">
                  {alert.title}
                </h3>
                <p className="font-sans text-xs text-[#9ba3b4] mt-1 leading-relaxed">
                  {alert.description}
                </p>
              </div>

              {/* Network flow IP targets */}
              <div className="bg-[#0a0e14] border border-[#2a3242]/30 p-2.5 mt-3.5 rounded flex flex-wrap items-center justify-between text-xs font-mono gap-4">
                <div className="flex items-center gap-6">
                  <div>
                    <span className="text-[10px] text-[#5c6473] block">ORIGEN</span>
                    <span className="text-[#e4e7eb] font-semibold">{alert.source}</span>
                  </div>
                  <ChevronRight className="h-4 w-4 text-[#5c6473] hidden sm:block mt-2.5" />
                  <div>
                    <span className="text-[10px] text-[#5c6473] block">DESTINO</span>
                    <span className="text-red-400 font-semibold">{alert.destination}</span>
                  </div>
                </div>

                <div>
                  <span className="text-[10px] text-[#5c6473] block">PUERTO AFECTADO</span>
                  <span className="text-[#e4e7eb] font-semibold">{alert.port || 'Varios / Any'}</span>
                </div>
              </div>

              {/* Action buttons inside aler */}
              <div className="flex flex-wrap items-center gap-2 mt-4 border-t border-[#1c2230]/70 pt-3">
                <button
                  onClick={() => setView('filtros')}
                  className="px-3 py-1 bg-[#1c2230] hover:bg-[#232a3a] text-[#e4e7eb] rounded text-xs font-mono transition-colors cursor-pointer"
                >
                  Ver Paquetes
                </button>
                <button
                  onClick={() => handleExplainWithAIBtn(alert)}
                  className="px-3 py-1 bg-[#3b82f6]/10 hover:bg-[#3b82f6]/20 border border-[#3b82f6]/30 text-[#3b82f6] rounded text-xs font-mono font-medium flex items-center gap-1.5 transition-colors cursor-pointer"
                >
                  <Sparkles className="h-3 w-3" />
                  <span>Explicar con IA</span>
                </button>

                {!alert.acknowledged && (
                  <button
                    onClick={() => onAcknowledgeAlert(alert.id)}
                    className="ml-auto px-3 py-1 bg-transparent hover:bg-[#1c2230] text-[#10b981] rounded text-xs font-mono font-bold flex items-center gap-1.5 border border-[#10b981]/20 transition-all cursor-pointer"
                  >
                    <Check className="h-3.5 w-3.5" />
                    <span>Aceptar (Ack)</span>
                  </button>
                )}
              </div>
            </div>
          ))
        ) : (
          <div className="bg-[#131820] border border-[#2a3242] p-16 rounded-lg text-center flex flex-col items-center justify-center">
            <Check className="h-12 w-12 text-[#10b981] mb-3 bg-[#10b981]/10 p-2 rounded-full animate-bounce" />
            <h3 className="font-sans font-bold text-[#e4e7eb]">No se encontraron alertas</h3>
            <p className="text-xs text-[#9ba3b4] mt-1.5">Todo está tranquilo en la red. ¡Excelente trabajo!</p>
          </div>
        )}
      </div>

      {/* Interactive AI stream description popup overlay */}
      {showExplanationModal && explainingAlert && (
        <div className="fixed inset-0 z-50 bg-black/75 backdrop-blur-xs flex items-center justify-center p-4">
          <div className="bg-[#131820] w-full max-w-2xl border border-[#3a4356] shadow-2xl rounded-lg overflow-hidden flex flex-col text-left font-sans">
            {/* Modal Header */}
            <div className="bg-[#0a0e14] border-b border-[#2a3242] p-4 flex justify-between items-center">
              <div className="flex items-center gap-2">
                <Sparkles className="h-4.5 w-4.5 text-[#3b82f6] animate-pulse" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  Análisis Inteligente: {explainingAlert.id}
                </span>
              </div>
              <button
                onClick={() => setShowExplanationModal(false)}
                className="text-[#9ba3b4] hover:text-[#e4e7eb] rounded-full p-1 hover:bg-[#1c2230] transition-colors"
              >
                <X className="h-4.5 w-4.5" />
              </button>
            </div>

            {/* Modal response body */}
            <div className="p-5 overflow-y-auto max-h-[460px] space-y-4">
              <div className="space-y-1">
                <span className="text-[10px] font-mono font-bold text-[#5c6473] uppercase tracking-wider block">
                  ALERTA ANALIZADA
                </span>
                <span className="font-sans font-bold text-[#e4e7eb] text-sm">
                  {explainingAlert.title}
                </span>
                <p className="text-xs text-[#9ba3b4] italic mt-0.5 font-sans leading-tight pl-3 border-l-2 border-[#5c6473]">
                  "{explainingAlert.description}"
                </p>
              </div>

              {/* Streaming box container */}
              <div className="bg-[#0a0e14] border border-[#2a3242] rounded-md p-4 font-mono text-xs whitespace-pre-wrap leading-relaxed text-[#3b82f6] h-72 overflow-y-auto select-text prose-invert select-text">
                {aiStreamingText}
                {aiLoading && (
                  <span className="inline-block w-2 h-4 bg-[#3b82f6] ml-1 animate-pulse" />
                )}
              </div>
            </div>

            {/* Modal Footer Controls */}
            <div className="bg-[#0a0e14] border-t border-[#2a3242] px-5 py-3 flex justify-between items-center text-xs">
              <span className="text-[#5c6473] font-mono text-[10px]">
                Powered by ViewRed AI Local Model (Qwen 2.5 3B)
              </span>
              <div className="flex gap-2.5">
                <button
                  onClick={() => setShowExplanationModal(false)}
                  className="px-3 py-1.5 border border-[#2a3242] rounded text-xs font-mono text-[#9ba3b4] hover:text-white hover:bg-[#1c2230] transition-all cursor-pointer"
                >
                  Cerrar
                </button>
                <button
                  onClick={handleOpenAIChatTab}
                  className="px-3.5 py-1.5 bg-[#3b82f6] text-white rounded text-xs font-mono font-bold hover:bg-[#3b82f6]/95 flex items-center gap-1.5 shadow-lg shadow-[#3b82f6]/20 transition-all cursor-pointer"
                >
                  <span>Chatear en Asistente IA</span>
                  <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
