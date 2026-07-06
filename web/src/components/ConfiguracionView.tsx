import { useEffect, useState, type ReactNode } from 'react';
import {
  Check,
  Cpu,
  HardDrive,
  Info,
  Plus,
  RefreshCw,
  ShieldAlert,
  Terminal,
  Trash2,
  X,
} from 'lucide-react';
import { notify } from '../lib/notify';

type SettingsTab = 'captura' | 'integracion' | 'reglas' | 'logs';

interface SettingFieldProps {
  label: string;
  hint?: string;
  children: ReactNode;
}

function SettingField({ label, hint, children }: SettingFieldProps) {
  return (
    <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4">
      <label className="mb-2 block font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[#748099]">
        {label}
      </label>
      {children}
      {hint && <p className="mt-2 font-mono text-[10px] leading-relaxed text-[#657089]">{hint}</p>}
    </div>
  );
}

export default function ConfiguracionView() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('captura');
  const [ollamaURL, setOllamaURL] = useState(localStorage.getItem('viewred.ollama_url') || 'http://127.0.0.1:11434');
  const [ollamaConnected, setOllamaConnected] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [nmapPath, setNmapPath] = useState(localStorage.getItem('viewred.nmap_path') || '');
  const [suricataPath, setSuricataPath] = useState(localStorage.getItem('viewred.suricata_path') || '');
  const [autoArchive, setAutoArchive] = useState(localStorage.getItem('viewred.auto_archive') !== 'false');
  const [pcapRetention, setPcapRetention] = useState(localStorage.getItem('viewred.pcap_retention') || '30');
  const [metadataRetention, setMetadataRetention] = useState(localStorage.getItem('viewred.metadata_retention') || '90');
  const [showSyntaxHelp, setShowSyntaxHelp] = useState(false);
  const [saveToast, setSaveToast] = useState(false);
  const [customRules, setCustomRules] = useState<{ id: number; rule_text: string; enabled: boolean }[]>([]);
  const [newRuleText, setNewRuleText] = useState('');

  useEffect(() => {
    fetch('/api/v1/rules')
      .then((res) => res.json())
      .then((data) => setCustomRules(data))
      .catch(console.error);
  }, []);

  const handleAddRule = async () => {
    if (!newRuleText.trim()) return;
    try {
      const res = await fetch('/api/v1/rules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rule_text: newRuleText, enabled: true }),
      });
      if (res.ok) {
        const rule = await res.json();
        setCustomRules((prev) => [...prev, rule]);
        setNewRuleText('');
        notify.success('Regla anadida con exito');
      }
    } catch (err) {
      console.error(err);
      notify.error('Error al anadir la regla');
    }
  };

  const handleDeleteRule = async (id: number) => {
    try {
      const res = await fetch(`/api/v1/rules/${id}`, { method: 'DELETE' });
      if (res.ok) {
        setCustomRules((prev) => prev.filter((r) => r.id !== id));
        notify.success('Regla eliminada con exito');
      }
    } catch (err) {
      console.error(err);
      notify.error('Error al eliminar la regla');
    }
  };

  const handleTestOllamaConnection = async () => {
    setTestingConnection(true);
    setOllamaConnected(false);
    try {
      const res = await fetch('/api/v1/ollama/test');
      const data = await res.json();
      if (res.ok && data.available) {
        setOllamaConnected(true);
        notify.success(`Conexion exitosa: ${data.model} disponible`);
      } else {
        setOllamaConnected(false);
        notify.error(`Modelo no disponible: ${data.error || 'ver consola'}`);
        console.error('Ollama test failed:', data);
      }
    } catch (err) {
      setOllamaConnected(false);
      notify.error(`Error de conexion: ${err instanceof Error ? err.message : 'desconocido'}`);
      console.error('Ollama test error:', err);
    } finally {
      setTestingConnection(false);
    }
  };

  const handleSaveConfig = () => {
    try {
      localStorage.setItem('viewred.ollama_url', ollamaURL);
      localStorage.setItem('viewred.nmap_path', nmapPath);
      localStorage.setItem('viewred.suricata_path', suricataPath);
      localStorage.setItem('viewred.auto_archive', String(autoArchive));
      localStorage.setItem('viewred.pcap_retention', pcapRetention);
      localStorage.setItem('viewred.metadata_retention', metadataRetention);
    } catch {
      // localStorage may be disabled; settings still remain in memory.
    }
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
    '[11:12:45] INFO User admin accessed settings endpoint.',
    '[14:02:11] OK Executing local ARP listener on eth0 node...',
    '[14:02:12] INFO Loaded active network topology node parameters.',
  ];

  const tabs = [
    { key: 'captura' as const, label: 'Almacenamiento', detail: 'Retencion y archivado', Icon: HardDrive },
    { key: 'integracion' as const, label: 'Integraciones', detail: 'Ollama, Nmap, Suricata', Icon: Cpu },
    { key: 'reglas' as const, label: 'Reglas', detail: `${customRules.length} personalizadas`, Icon: ShieldAlert },
    { key: 'logs' as const, label: 'Logs', detail: 'Estado del sistema', Icon: Terminal },
  ];

  return (
    <div id="settings-view-wrapper" className="grid grid-cols-1 gap-5 text-left font-sans lg:grid-cols-12">
      <aside className="lg:col-span-3">
        <div className="sticky top-0 rounded-lg border border-[#263041] bg-[#111720] p-2.5 select-none">
          <div className="mb-2 border-b border-[#263041] px-2 py-2">
            <span className="font-mono text-[10px] font-bold uppercase tracking-[0.18em] text-[#657089]">Centro de control</span>
            <p className="mt-1 text-xs text-[#aab6c9]">Preferencias locales y reglas operativas.</p>
          </div>
          <nav className="space-y-1">
            {tabs.map(({ key, label, detail, Icon }) => {
              const active = activeTab === key;
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => setActiveTab(key)}
                  className={`w-full rounded-lg border p-3 text-left transition-all ${
                    active
                      ? 'border-blue-400/35 bg-blue-500/10 text-[#f8fafc]'
                      : 'border-transparent text-[#8792a8] hover:border-[#263041] hover:bg-[#151c27] hover:text-[#e5edf8]'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className={`rounded-md border p-2 ${active ? 'border-blue-400/30 bg-blue-500/10 text-blue-300' : 'border-[#263041] bg-[#0a0e14] text-[#657089]'}`}>
                      <Icon className="h-4 w-4" />
                    </div>
                    <div className="min-w-0">
                      <span className="block text-xs font-bold">{label}</span>
                      <span className="block truncate font-mono text-[10px] text-[#657089]">{detail}</span>
                    </div>
                  </div>
                </button>
              );
            })}
          </nav>
        </div>
      </aside>

      <main className="space-y-5 lg:col-span-9">
        <section className="rounded-lg border border-[#263041] bg-[#111720]">
          <header className="flex flex-col gap-2 border-b border-[#263041] bg-[#0a0e14] p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="text-sm font-bold text-[#f8fafc]">
                {tabs.find((tab) => tab.key === activeTab)?.label}
              </h2>
              <p className="mt-1 font-mono text-[11px] text-[#657089]">
                Cambios guardados en preferencias locales; rutas de backend requieren reinicio del servicio.
              </p>
            </div>
            {activeTab === 'integracion' && (
              <button
                type="button"
                onClick={() => setShowSyntaxHelp(true)}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-[#263041] px-3 font-mono text-xs text-[#aab6c9] transition-all hover:border-blue-400/35 hover:bg-[#182131] hover:text-white"
              >
                <Info className="h-3.5 w-3.5" />
                Sintaxis OS
              </button>
            )}
          </header>

          <div className="space-y-4 p-4">
            {activeTab === 'captura' && (
              <>
                <div className="grid gap-4 md:grid-cols-2">
                  <SettingField
                    label="Retencion de logs de red"
                    hint="Determina cuando se barren las capturas completas en disco de forma automatica."
                  >
                    <div className="flex items-center gap-2">
                      <input
                        type="number"
                        value={pcapRetention}
                        onChange={(event) => setPcapRetention(event.target.value)}
                        className="h-10 w-28 rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none focus:border-blue-400/70"
                      />
                      <span className="font-mono text-xs text-[#aab6c9]">dias</span>
                    </div>
                  </SettingField>

                  <SettingField
                    label="Retencion de alertas"
                    hint="Controla cuanto tiempo se conservan metadatos, historiales y alertas."
                  >
                    <div className="flex items-center gap-2">
                      <input
                        type="number"
                        value={metadataRetention}
                        onChange={(event) => setMetadataRetention(event.target.value)}
                        className="h-10 w-28 rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none focus:border-blue-400/70"
                      />
                      <span className="font-mono text-xs text-[#aab6c9]">dias</span>
                    </div>
                  </SettingField>
                </div>

                <div className="flex flex-col gap-3 rounded-lg border border-[#263041] bg-[#0a0e14] p-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <h3 className="text-xs font-bold text-[#f8fafc]">Auto-Archivado S3 / Cold Storage</h3>
                    <p className="mt-1 font-mono text-[10px] text-[#657089]">Mueve logs antiguos a la nube en lugar de eliminarlos permanentemente.</p>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={autoArchive}
                    onClick={() => setAutoArchive((value) => !value)}
                    className={`relative h-7 w-12 shrink-0 rounded-full border transition-all ${
                      autoArchive ? 'border-emerald-400/30 bg-emerald-500/25' : 'border-[#334155] bg-[#111720]'
                    }`}
                  >
                    <span
                      className={`absolute left-1 top-1 h-5 w-5 rounded-full bg-white transition-transform ${
                        autoArchive ? 'translate-x-5' : 'translate-x-0'
                      }`}
                    />
                  </button>
                </div>
              </>
            )}

            {activeTab === 'integracion' && (
              <>
                <SettingField label="Ollama endpoint URL" hint="Se usa para comprobar disponibilidad del modelo local configurado.">
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <input
                      type="text"
                      value={ollamaURL}
                      onChange={(event) => setOllamaURL(event.target.value)}
                      className="h-10 min-w-0 flex-1 rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none focus:border-blue-400/70"
                    />
                    <button
                      type="button"
                      onClick={handleTestOllamaConnection}
                      disabled={testingConnection}
                      className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-blue-400/30 bg-blue-500/10 px-3 font-mono text-xs font-bold text-blue-300 transition-all hover:bg-blue-500/15 disabled:opacity-60"
                    >
                      {testingConnection && <RefreshCw className="h-3.5 w-3.5 animate-spin" />}
                      Test Conexion
                    </button>
                  </div>
                  <div className={`mt-3 inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-[10px] ${
                    ollamaConnected
                      ? 'border-emerald-400/25 bg-emerald-500/10 text-emerald-300'
                      : 'border-amber-400/25 bg-amber-500/10 text-amber-300'
                  }`}>
                    {ollamaConnected ? <Check className="h-3 w-3" /> : <Info className="h-3 w-3" />}
                    {ollamaConnected ? 'Ollama accesible' : 'Pendiente de verificacion'}
                  </div>
                </SettingField>

                <SettingField label="Ruta ejecutable Nmap">
                  <input
                    type="text"
                    value={nmapPath}
                    onChange={(event) => setNmapPath(event.target.value)}
                    placeholder="C:\\Program Files (x86)\\Nmap\\nmap.exe  o  /usr/bin/nmap"
                    className="h-10 w-full rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none placeholder:text-[#566176] focus:border-blue-400/70"
                  />
                </SettingField>

                <SettingField label="Archivo configuracion Suricata">
                  <input
                    type="text"
                    value={suricataPath}
                    onChange={(event) => setSuricataPath(event.target.value)}
                    placeholder="C:\\Program Files\\Suricata\\suricata.yaml  o  /etc/suricata/suricata.yaml"
                    className="h-10 w-full rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none placeholder:text-[#566176] focus:border-blue-400/70"
                  />
                </SettingField>
              </>
            )}

            {activeTab === 'reglas' && (
              <>
                <SettingField label="Anadir nueva regla">
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <input
                      type="text"
                      value={newRuleText}
                      onChange={(event) => setNewRuleText(event.target.value)}
                      placeholder="alert tcp any any -> any 80 (msg:&quot;HTTP Traffic&quot;; severity:INFO;)"
                      className="h-10 min-w-0 flex-1 rounded-md border border-[#263041] bg-[#111720] px-3 font-mono text-xs text-[#e5edf8] outline-none placeholder:text-[#566176] focus:border-blue-400/70"
                    />
                    <button
                      type="button"
                      onClick={handleAddRule}
                      className="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-blue-500 px-3 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400"
                    >
                      <Plus className="h-4 w-4" />
                      Anadir
                    </button>
                  </div>
                </SettingField>

                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[#748099]">Reglas activas</span>
                    <span className="rounded-md border border-[#263041] px-2 py-1 font-mono text-[10px] text-[#8792a8]">{customRules.length}</span>
                  </div>
                  {customRules.length === 0 ? (
                    <div className="rounded-lg border border-dashed border-[#263041] bg-[#0a0e14] p-8 text-center font-mono text-xs text-[#657089]">
                      No hay reglas personalizadas cargadas.
                    </div>
                  ) : (
                    <div className="max-h-72 space-y-2 overflow-y-auto pr-1">
                      {customRules.map((rule) => (
                        <div key={rule.id} className="flex items-start gap-3 rounded-lg border border-[#263041] bg-[#0a0e14] p-3">
                          <span className={`mt-1 h-2 w-2 rounded-full ${rule.enabled ? 'bg-emerald-400' : 'bg-[#657089]'}`} />
                          <code className="min-w-0 flex-1 break-all font-mono text-[11px] leading-relaxed text-[#dbe6f7]">{rule.rule_text}</code>
                          <button
                            type="button"
                            onClick={() => handleDeleteRule(rule.id)}
                            className="rounded-md p-1.5 text-red-300 transition-all hover:bg-red-500/10"
                            title="Eliminar regla"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </>
            )}

            {activeTab === 'logs' && (
              <div className="rounded-lg border border-[#263041] bg-[#070b10] p-3">
                <div className="mb-3 flex items-center justify-between border-b border-[#263041] pb-2">
                  <span className="font-mono text-[10px] font-bold uppercase tracking-[0.16em] text-[#748099]">systemd_status.log</span>
                  <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 font-mono text-[10px] text-emerald-300">stream local</span>
                </div>
                <div className="h-72 overflow-y-auto font-mono text-[11px] leading-relaxed text-emerald-300 selection:bg-emerald-500/20">
                  {mockLogs.map((log) => (
                    <div key={log} className="rounded px-2 py-1 hover:bg-[#111720]">{log}</div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {activeTab !== 'logs' && (
            <footer className="flex flex-col gap-3 border-t border-[#263041] bg-[#0a0e14] px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <span className="font-mono text-[10px] text-[#657089]">
                Las rutas de Nmap, Suricata y Ollama se aplican desde config.yaml al reiniciar el backend.
              </span>
              <button
                type="button"
                onClick={handleSaveConfig}
                className="inline-flex h-9 items-center justify-center rounded-md bg-blue-500 px-4 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400"
              >
                Guardar Configuracion
              </button>
            </footer>
          )}
        </section>
      </main>

      {saveToast && (
        <div className="fixed bottom-6 right-6 z-50 inline-flex items-center gap-2 rounded-lg border border-emerald-400/30 bg-emerald-500 px-4 py-2.5 text-xs font-semibold text-white shadow-xl">
          <Check className="h-3.5 w-3.5" />
          Configuracion guardada.
        </div>
      )}

      {showSyntaxHelp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-lg border border-[#334155] bg-[#111720] shadow-2xl">
            <div className="flex items-center justify-between border-b border-[#263041] bg-[#0a0e14] p-4">
              <h3 className="flex items-center gap-2 text-sm font-bold text-[#f8fafc]">
                <Info className="h-4 w-4 text-blue-300" />
                Sintaxis de rutas por SO
              </h3>
              <button
                type="button"
                onClick={() => setShowSyntaxHelp(false)}
                className="rounded-md p-1.5 text-[#8792a8] hover:bg-[#182131] hover:text-white"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="grid gap-3 p-4 text-xs md:grid-cols-2">
              <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-3">
                <h4 className="mb-2 font-mono font-bold text-blue-300">Windows</h4>
                <code className="block break-all rounded border border-[#263041] bg-[#111720] p-2 font-mono text-[#dbe6f7]">
                  C:\Program Files (x86)\Nmap\nmap.exe
                </code>
                <code className="mt-2 block break-all rounded border border-[#263041] bg-[#111720] p-2 font-mono text-[#dbe6f7]">
                  C:\Program Files\Suricata\suricata.yaml
                </code>
              </div>
              <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-3">
                <h4 className="mb-2 font-mono font-bold text-blue-300">Linux / macOS</h4>
                <code className="block break-all rounded border border-[#263041] bg-[#111720] p-2 font-mono text-[#dbe6f7]">
                  /usr/bin/nmap
                </code>
                <code className="mt-2 block break-all rounded border border-[#263041] bg-[#111720] p-2 font-mono text-[#dbe6f7]">
                  /etc/suricata/suricata.yaml
                </code>
              </div>
            </div>
            <div className="border-t border-[#263041] bg-[#0a0e14] p-4">
              <button
                type="button"
                onClick={() => setShowSyntaxHelp(false)}
                className="h-9 w-full rounded-md bg-blue-500 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400"
              >
                Entendido
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
