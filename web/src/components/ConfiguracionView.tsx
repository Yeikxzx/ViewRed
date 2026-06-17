import { useState } from 'react';
import {
  Settings,
  Tv,
  HardDrive,
  Cpu,
  Database,
  Terminal,
  Play,
  Check,
  RefreshCw,
} from 'lucide-react';

export default function ConfiguracionView() {
  const [activeTab, setActiveTab] = useState<'general' | 'captura' | 'integracion' | 'logs'>('general');
  const [ollamaURL, setOllamaURL] = useState('http://127.0.0.1:11434');
  const [ollamaConnected, setOllamaConnected] = useState(true);
  const [testingConnection, setTestingConnection] = useState(false);
  const [nmapPath, setNmapPath] = useState('/usr/bin/nmap');
  const [suricataPath, setSuricataPath] = useState('/etc/suricata/suricata.yaml');
  const [autoArchive, setAutoArchive] = useState(true);
  const [saveToast, setSaveToast] = useState(false);

  const handleTestOllamaConnection = () => {
    setTestingConnection(true);
    setTimeout(() => {
      setTestingConnection(false);
      setOllamaConnected(true);
      alert('Conexión con Ollama establecida con éxito: Modelo llama3 (8b) activo.');
    }, 1500);
  };

  const handleSaveConfig = () => {
    setSaveToast(true);
    setTimeout(() => setSaveToast(false), 2500);
  };

  const mockLogs = [
    '[10:45:01] INFO ViewRed Core Engine v2.4.1 started.',
    '[10:45:02] INFO Loading Suricata ruleset from /etc/suricata/rules...',
    '[10:45:05] OK 34,192 rules loaded successfully.',
    '[10:45:06] WARN Interface eth1 is down, skipping packet capture.',
    '[10:45:07] INFO Establishing connection to Ollama at http://127.0.0.1:11434...',
    '[10:45:08] OK AI Assistant ready. Model: llama3.',
    '[11:12:45] INFO User \'admin\' accessed settings endpoint.',
    '[14:02:11] OK Executing local ARP listener on eth0 node...',
    '[14:02:12] INFO Loaded active network topology node parameters.',
  ];

  return (
    <div id="settings-view-wrapper" className="grid grid-cols-1 lg:grid-cols-12 gap-6 text-left font-sans">
      {/* Sidebar internal tab options: 4 Cols */}
      <div className="lg:col-span-3 space-y-1 bg-[#131820] border border-[#2a3242] p-2.5 rounded-lg h-fit select-none">
        <button
          onClick={() => setActiveTab('general')}
          className={`w-full flex items-center gap-2.5 py-2 px-3.5 rounded text-xs font-mono text-left transition-all ${
            activeTab === 'general' ? 'bg-[#1c2230] text-[#3b82f6] font-bold' : 'text-[#9ba3b4] hover:text-white hover:bg-[#131820]/50'
          }`}
        >
          <Tv className="h-4 w-4" />
          <span>General & Visualización</span>
        </button>
        <button
          onClick={() => setActiveTab('captura')}
          className={`w-full flex items-center gap-2.5 py-2 px-3.5 rounded text-xs font-mono text-left transition-all ${
            activeTab === 'captura' ? 'bg-[#1c2230] text-[#3b82f6] font-bold' : 'text-[#9ba3b4] hover:text-white hover:bg-[#131820]/50'
          }`}
        >
          <HardDrive className="h-4 w-4" />
          <span>Almacenamiento & Retención</span>
        </button>
        <button
          onClick={() => setActiveTab('integracion')}
          className={`w-full flex items-center gap-2.5 py-2 px-3.5 rounded text-xs font-mono text-left transition-all ${
            activeTab === 'integracion' ? 'bg-[#1c2230] text-[#3b82f6] font-bold' : 'text-[#9ba3b4] hover:text-white hover:bg-[#131820]/50'
          }`}
        >
          <Cpu className="h-4 w-4" />
          <span>IA Integración & Rutas</span>
        </button>
        <button
          onClick={() => setActiveTab('logs')}
          className={`w-full flex items-center gap-2.5 py-2 px-3.5 rounded text-xs font-mono text-left transition-all ${
            activeTab === 'logs' ? 'bg-[#1c2230] text-[#3b82f6] font-bold' : 'text-[#9ba3b4] hover:text-white hover:bg-[#131820]/50'
          }`}
        >
          <Terminal className="h-4 w-4" />
          <span>Logs del Sistema</span>
        </button>
      </div>

      {/* Configurations actual form body: 8 Cols */}
      <div className="lg:col-span-9 space-y-6">
        <div className="bg-[#131820] border border-[#2a3242] p-5 rounded-lg h-fit space-y-5">
          {/* General Tab */}
          {activeTab === 'general' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5 mb-1">
                <Tv className="h-4 w-4 text-[#3b82f6]" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  Interfaz y Visualización
                </span>
              </div>

              {/* Theme selection (Disabled as per prompt specifications to match strict "Mode Dark by Default" philosophy) */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">TEMA PRINCIPAL</label>
                <div className="flex gap-2 select-none">
                  <span className="bg-[#0a0e14] border border-[#3a4356] text-xs font-semibold px-3 py-1.5 rounded text-white flex items-center gap-2">
                    <span>Oscuro Technical (Sentinel Default)</span>
                    <span className="w-2 h-2 rounded-full bg-[#10b981]" />
                  </span>
                  <span className="border border-[#2a3242] opacity-40 text-xs px-3 py-1.5 rounded text-[#5c6473] cursor-not-allowed">
                    Claro (Deshabilitado)
                  </span>
                </div>
                <span className="text-[9px] font-mono text-[#5c6473] block leading-tight pt-0.5">
                  El visualizador ViewRed prioriza la disminución ocular durante sesiones de SecOps continuas.
                </span>
              </div>

              {/* Language selection dropdown */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">IDIOMA DE LA INTERFAZ</label>
                <select className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 outline-none font-sans cursor-pointer">
                  <option value="es">Español (ES) - Sistema Central</option>
                  <option value="en">English (US)</option>
                </select>
              </div>

              {/* Timezone */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">ZONA HORARIA</label>
                <select className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 outline-none font-sans cursor-pointer">
                  <option value="utc">UTC (Coordinated Universal Time) - Recomendado</option>
                  <option value="local">Hora Local del Sistema</option>
                </select>
              </div>
            </div>
          )}

          {/* Almacenamiento & Retencion Tab */}
          {activeTab === 'captura' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5 mb-1">
                <HardDrive className="h-4 w-4 text-[#3b82f6]" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  Almacenamiento y Retención
                </span>
              </div>

              {/* PCAP limit */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">RETENCIÓN DE LOGS DE RED (PCAPs)</label>
                <div className="flex items-center gap-2">
                  <input
                    type="number"
                    defaultValue={30}
                    className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 w-24 outline-none font-mono"
                  />
                  <span className="text-xs text-[#9ba3b4]">días</span>
                </div>
                <span className="text-[9px] font-mono text-[#5c6473] block leading-tight">
                  Determina cuándo se barren las capturas completas en disco de forma automática.
                </span>
              </div>

              {/* Metadata limit */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">RETENCIÓN DE ALERTAS (METADATOS)</label>
                <div className="flex items-center gap-2">
                  <input
                    type="number"
                    defaultValue={90}
                    className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 w-24 outline-none font-mono"
                  />
                  <span className="text-xs text-[#9ba3b4]">días</span>
                </div>
                <span className="text-[9px] font-mono text-[#5c6473] block leading-tight">
                  Logs de alertas de Suricata e historiales guardados de anomalías lógicas.
                </span>
              </div>

              {/* AWS opt-in compression check */}
              <div className="flex items-center justify-between border-t border-[#1c2230] pt-4 select-none">
                <div className="space-y-1 text-left">
                  <span className="text-xs font-sans font-semibold text-[#e4e7eb] block">Auto-Archivado S3 / Cold Storage</span>
                  <span className="text-[10px] text-[#5c6473] font-mono block">Mueve logs antiguos a la nube en lugar de eliminarlos permanentemente.</span>
                </div>
                <input
                  type="checkbox"
                  checked={autoArchive}
                  onChange={(e) => setAutoArchive(e.target.checked)}
                  className="accent-[#3b82f6] h-4 w-4"
                />
              </div>
            </div>
          )}

          {/* AI Integrations Tab */}
          {activeTab === 'integracion' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5 mb-1">
                <Cpu className="h-4 w-4 text-[#3b82f6]" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  Integraciones y Rutas del Sistema
                </span>
              </div>

              {/* Ollama API Endpoint URL */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">OLLAMA ENDPOINT URL (LOCAL LLM)</label>
                <div className="flex gap-2.5">
                  <input
                    type="text"
                    value={ollamaURL}
                    onChange={(e) => setOllamaURL(e.target.value)}
                    className="bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] text-xs text-[#e4e7eb] rounded p-2 outline-none font-mono flex-1"
                  />
                  <button
                    type="button"
                    onClick={handleTestOllamaConnection}
                    disabled={testingConnection}
                    className="bg-transparent border border-[#2a3242] hover:bg-[#1c2230] text-[#9ba3b4] hover:text-white px-3 py-1.5 text-xs font-mono font-bold rounded flex items-center justify-center gap-2 transition-all cursor-pointer"
                  >
                    {testingConnection ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : null}
                    <span>Test Conexión</span>
                  </button>
                </div>
                <span className="flex items-center gap-1.5 text-[10px] font-mono pt-1 text-[#10b981]">
                  <span>🟢 Modelo llama3 (8b) detectado de forma exitosa.</span>
                </span>
              </div>

              {/* Nmap binary path */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">RUTA EJECUTABLE NMAP</label>
                <input
                  type="text"
                  value={nmapPath}
                  onChange={(e) => setNmapPath(e.target.value)}
                  className="bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] text-xs text-[#e4e7eb] rounded p-2 outline-none font-mono w-full"
                />
              </div>

              {/* Suricata config path */}
              <div className="space-y-1">
                <label className="text-[11px] font-mono text-[#9ba3b4] block">ARCHIVO CONFIGURACION SURICATA</label>
                <input
                  type="text"
                  value={suricataPath}
                  onChange={(e) => setSuricataPath(e.target.value)}
                  className="bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] text-xs text-[#e4e7eb] rounded p-2 outline-none font-mono w-full"
                />
              </div>
            </div>
          )}

          {/* Logs Tab */}
          {activeTab === 'logs' && (
            <div className="space-y-4">
              <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5 mb-1">
                <Terminal className="h-4 w-4 text-[#3b82f6]" />
                <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                  systemd_status.log
                </span>
              </div>

              <div className="bg-[#0a0e14] border border-[#2a3242] rounded p-3 font-mono text-[10.5px] leading-relaxed text-[#10b981] h-64 overflow-y-auto whitespace-pre-wrap select-text selection:bg-teal-500/20">
                {mockLogs.map((log, idx) => (
                  <div key={idx} className="hover:bg-[#131820] py-0.5 rounded px-1.5">
                    {log}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Footer Save Changes button */}
          {activeTab !== 'logs' && (
            <div className="flex justify-end gap-2 border-t border-[#1c2230] pt-4 mt-1 select-none">
              <span className="text-[#5c6473] font-mono text-[10px] mr-auto mt-2">
                *Los cambios requerirán reload de servicios para aplicarse.
              </span>
              <button
                type="button"
                onClick={handleSaveConfig}
                className="bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white py-1.5 px-4 rounded text-xs font-mono font-bold transition-all cursor-pointer shadow-lg shadow-[#3b82f6]/10"
              >
                Guardar Configuración
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Applied toast popup */}
      {saveToast && (
        <div className="fixed bottom-6 right-6 z-50 bg-[#10b981] text-white px-4 py-2.5 rounded shadow-xl font-sans text-xs font-semibold flex items-center gap-2">
          <Check className="h-3.5 w-3.5" />
          <span>Configuración guardada. Recargando subprocesos...</span>
        </div>
      )}
    </div>
  );
}
