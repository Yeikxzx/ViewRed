import { useState, useEffect } from 'react';
import { Download, Zap, Save, Settings, FileText, AlertTriangle, ChevronDown, Trash2 } from 'lucide-react';
import { notify } from '../lib/notify';

interface ReportFile {
  name: string;
  size: number;
  timestamp: string;
}

interface AutoReportConfig {
  enabled: boolean;
  interval: string;
}

interface TestContext {
  scenario_name: string;
  victim_ip: string;
  attacker_ip: string;
  expected_result: string;
  notes: string;
}

// Custom Select Component to replace the native browser dropdown
function CustomSelect({ 
  value, 
  options, 
  onChange,
  disabled = false
}: { 
  value: string, 
  options: {value: string, label: string}[], 
  onChange: (val: string) => void,
  disabled?: boolean
}) {
  const [isOpen, setIsOpen] = useState(false);
  const selectedLabel = options.find(o => o.value === value)?.label || '';

  return (
    <div className={`relative ${disabled ? 'opacity-50 pointer-events-none' : ''}`}>
      <div 
        className="w-full bg-[#1a212d] border border-gray-700 rounded p-2 text-white flex justify-between items-center cursor-pointer hover:border-gray-500 transition-colors"
        onClick={() => setIsOpen(!isOpen)}
      >
        <span>{selectedLabel}</span>
        <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </div>
      
      {isOpen && (
        <div className="absolute z-10 w-full mt-1 bg-[#1a212d] border border-gray-700 rounded shadow-lg overflow-hidden">
          {options.map((option) => (
            <div
              key={option.value}
              className={`p-2 cursor-pointer hover:bg-[#ef233c] hover:text-white transition-colors ${
                value === option.value ? 'bg-gray-800 text-white' : 'text-gray-300'
              }`}
              onClick={() => {
                onChange(option.value);
                setIsOpen(false);
              }}
            >
              {option.label}
            </div>
          ))}
        </div>
      )}
      
      {/* Invisible overlay to close dropdown when clicking outside */}
      {isOpen && (
        <div 
          className="fixed inset-0 z-0" 
          onClick={() => setIsOpen(false)}
        />
      )}
    </div>
  );
}

export default function ReportesView() {
  const [reports, setReports] = useState<ReportFile[]>([]);
  const [config, setConfig] = useState<AutoReportConfig>({ enabled: false, interval: '24h' });
  const [isGenerating, setIsGenerating] = useState(false);
  const [range, setRange] = useState('24h'); // '5h', '24h', '7d'
  
  const [showContext, setShowContext] = useState(false);
  const [testContext, setTestContext] = useState<TestContext>({
    scenario_name: '',
    victim_ip: '',
    attacker_ip: '',
    expected_result: 'Normal',
    notes: ''
  });

  useEffect(() => {
    fetchReports();
    fetchConfig();
  }, []);

  const fetchReports = async () => {
    try {
      const res = await fetch('/api/v1/reports');
      if (res.ok) {
        const data = await res.json();
        setReports(data || []);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const fetchConfig = async () => {
    try {
      const res = await fetch('/api/v1/reports/config');
      if (res.ok) {
        const data = await res.json();
        setConfig(data);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const handleSaveConfig = async () => {
    try {
      const res = await fetch('/api/v1/reports/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      if (res.ok) {
        notify.success('Configuración de reportes guardada');
      } else {
        notify.error('Error al guardar configuración');
      }
    } catch (e) {
      console.error(e);
      notify.error('Error de red');
    }
  };

  const handleGenerate = async () => {
    setIsGenerating(true);
    try {
      const until = new Date();
      const since = new Date(until);
      if (range === '5h') since.setHours(since.getHours() - 5);
      else if (range === '24h') since.setHours(since.getHours() - 24);
      else if (range === '7d') since.setDate(since.getDate() - 7);
      
      const reqBody: any = {
        since: since.toISOString(),
        until: until.toISOString()
      };
      
      if (showContext && testContext.scenario_name.trim() !== '') {
        reqBody.test_context = testContext;
      }

      const res = await fetch('/api/v1/reports/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(reqBody),
      });
      
      if (res.ok) {
        notify.success('Reporte generado exitosamente');
        fetchReports(); // refresh list
      } else {
        notify.error('Error al generar reporte');
      }
    } catch (e) {
      console.error(e);
      notify.error('Error de red al generar reporte');
    } finally {
      setIsGenerating(false);
    }
  };

  const handleDownload = (filename: string) => {
    window.open(`/api/v1/reports/download/${filename}`, '_blank');
  };

  const handleDelete = (filename: string) => {
    notify.confirm(`¿Eliminar reporte ${filename}?`, {
      description: 'Esta acción no se puede deshacer.',
      actionLabel: 'Eliminar',
      onAction: async () => {
        try {
          const res = await fetch(`/api/v1/reports/${filename}`, {
            method: 'DELETE',
          });
          if (res.ok) {
            notify.success('Reporte eliminado exitosamente');
            fetchReports();
          } else {
            notify.error('Error al eliminar el reporte');
          }
        } catch (e) {
          console.error(e);
          notify.error('Error de red al eliminar el reporte');
        }
      },
      duration: 10000,
    });
  };

  return (
    <div className="flex h-full space-x-6 text-white p-6">
      {/* Left Col: Actions & Config */}
      <div className="w-1/3 flex flex-col space-y-6">
        
        {/* Generar Bajo Demanda */}
        <div className="bg-[#121820] border border-gray-800 rounded-lg shadow-xl p-6">
          <div className="flex items-center space-x-3 mb-6">
            <Zap className="w-6 h-6 text-[#ef233c]" />
            <h2 className="text-xl font-semibold">Generar Bajo Demanda</h2>
          </div>
          <div className="mb-4 relative z-20">
            <label className="block text-sm text-gray-400 mb-2">Rango de Tiempo</label>
            <CustomSelect
              value={range}
              onChange={setRange}
              options={[
                { value: '5h', label: 'Últimas 5 horas' },
                { value: '24h', label: 'Últimas 24 horas' },
                { value: '7d', label: 'Última semana' }
              ]}
            />
          </div>

          <div className="mb-4">
            <button 
              onClick={() => setShowContext(!showContext)}
              className="text-sm flex items-center text-gray-400 hover:text-white transition-colors"
            >
              <ChevronDown className={`w-4 h-4 mr-1 transition-transform ${showContext ? 'rotate-180' : ''}`} />
              Contexto de la prueba (Opcional)
            </button>
            
            {showContext && (
              <div className="mt-4 p-4 bg-[#1a212d] border border-gray-700 rounded-lg space-y-3 relative z-10">
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Escenario</label>
                  <input 
                    type="text" 
                    value={testContext.scenario_name}
                    onChange={e => setTestContext({...testContext, scenario_name: e.target.value})}
                    placeholder="Ej: Ataque DoS SYN Flood"
                    className="w-full bg-[#121820] border border-gray-700 rounded p-2 text-white text-sm"
                  />
                </div>
                <div className="flex space-x-2">
                  <div className="flex-1">
                    <label className="block text-xs text-gray-400 mb-1">IP Víctima</label>
                    <input 
                      type="text" 
                      value={testContext.victim_ip}
                      onChange={e => setTestContext({...testContext, victim_ip: e.target.value})}
                      className="w-full bg-[#121820] border border-gray-700 rounded p-2 text-white text-sm"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="block text-xs text-gray-400 mb-1">IP Atacante</label>
                    <input 
                      type="text" 
                      value={testContext.attacker_ip}
                      onChange={e => setTestContext({...testContext, attacker_ip: e.target.value})}
                      className="w-full bg-[#121820] border border-gray-700 rounded p-2 text-white text-sm"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Resultado Esperado</label>
                  <CustomSelect
                    value={testContext.expected_result}
                    onChange={v => setTestContext({...testContext, expected_result: v})}
                    options={[
                      { value: 'Normal', label: 'Tráfico Normal (Sin Alertas)' },
                      { value: 'Anomalia/Ataque', label: 'Anomalía/Ataque (Se esperan Alertas)' }
                    ]}
                  />
                </div>
              </div>
            )}
          </div>

          <button
            onClick={handleGenerate}
            disabled={isGenerating}
            className={`w-full flex items-center justify-center py-2 px-4 rounded font-medium transition-colors ${
              isGenerating 
                ? 'bg-gray-600 cursor-not-allowed' 
                : 'bg-[#ef233c] hover:bg-[#d90429] text-white'
            }`}
          >
            {isGenerating ? (
              'Generando...'
            ) : (
              <>
                <FileText className="w-4 h-4 mr-2" /> Generar Reporte PDF
              </>
            )}
          </button>
        </div>

        {/* Configuracion Automatica */}
        <div className="bg-[#121820] border border-gray-800 rounded-lg shadow-xl p-6">
          <div className="flex items-center space-x-3 mb-6">
            <Settings className="w-6 h-6 text-[#ef233c]" />
            <h2 className="text-xl font-semibold">Reportes Automáticos</h2>
          </div>
          
          <div className="flex items-center justify-between mb-4">
            <span className="text-gray-300">Habilitar generación automática</span>
            <label className="relative inline-flex items-center cursor-pointer">
              <input 
                type="checkbox" 
                className="sr-only peer" 
                checked={config.enabled}
                onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
              />
              <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-[#ef233c]"></div>
            </label>
          </div>

          <div className="mb-6 relative z-10">
            <label className="block text-sm text-gray-400 mb-2">Frecuencia</label>
            <CustomSelect
              value={config.interval}
              onChange={(val) => setConfig({ ...config, interval: val })}
              disabled={!config.enabled}
              options={[
                { value: '6h', label: 'Cada 6 horas' },
                { value: '8h', label: 'Cada 8 horas' },
                { value: '24h', label: 'Diario (Cada 24 horas)' }
              ]}
            />
          </div>

          <button
            onClick={handleSaveConfig}
            className="w-full flex items-center justify-center py-2 px-4 bg-gray-700 hover:bg-gray-600 rounded font-medium transition-colors"
          >
            <Save className="w-4 h-4 mr-2" /> Guardar Configuración
          </button>
        </div>
      </div>

      {/* Right Col: List of Reports */}
      <div className="flex-1 bg-[#121820] border border-gray-800 rounded-lg shadow-xl p-6 flex flex-col">
        <div className="flex justify-between items-center mb-6">
          <h2 className="text-xl font-semibold flex items-center">
            <FileText className="w-6 h-6 mr-2 text-[#ef233c]" />
            Archivos Generados
          </h2>
        </div>

        <div className="flex-1 overflow-auto pr-2">
          {reports.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <AlertTriangle className="w-12 h-12 mb-2 opacity-50" />
              <p>No hay reportes generados aún.</p>
            </div>
          ) : (
            <ul className="space-y-3">
              {reports.map((r, i) => (
                <li key={i} className="flex items-center justify-between p-4 bg-[#1a212d] border border-gray-800 rounded-lg hover:border-gray-700 transition-colors">
                  <div className="flex items-center">
                    <FileText className="w-8 h-8 text-red-400 mr-4" />
                    <div>
                      <p className="font-medium text-gray-200">{r.name}</p>
                      <p className="text-xs text-gray-500 mt-1">
                        {new Date(r.timestamp).toLocaleString()} • {(r.size / 1024).toFixed(1)} KB
                      </p>
                    </div>
                  </div>
                  <div className="flex space-x-2">
                    <button 
                      onClick={() => handleDownload(r.name)}
                      className="p-2 bg-gray-800 hover:bg-gray-700 rounded-full transition-colors"
                      title="Descargar PDF"
                    >
                      <Download className="w-5 h-5 text-gray-300" />
                    </button>
                    <button 
                      onClick={() => handleDelete(r.name)}
                      className="p-2 bg-gray-800 hover:bg-red-900/50 rounded-full transition-colors group"
                      title="Eliminar Reporte"
                    >
                      <Trash2 className="w-5 h-5 text-gray-400 group-hover:text-red-400" />
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}
