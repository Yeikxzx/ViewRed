import { useMemo, useState } from 'react';
import {
  AlertOctagon,
  AlertTriangle,
  Bell,
  Check,
  ChevronRight,
  Info,
  Search,
  ShieldCheck,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react';
import { NetworkAlert, ViewType } from '../types';

interface AlertasViewProps {
  alerts: NetworkAlert[];
  onAcknowledgeAlert: (id: string) => void;
  onDeleteAlert: (id: string) => void;
  onClearAlerts: () => void;
  setSelectedAlertToExplain: (alert: NetworkAlert | null) => void;
  setView: (view: ViewType) => void;
}

type SeverityFilter = 'ALL' | 'CRITICAL' | 'WARNING' | 'HIGH' | 'INFO';

const severityMeta = {
  CRITICAL: {
    label: 'CRITICAL',
    title: 'Criticas',
    Icon: AlertOctagon,
    text: 'text-red-300',
    badge: 'bg-red-500/12 text-red-300 border-red-500/30',
    border: 'border-red-500/55',
    accent: 'bg-red-500',
    shadow: 'shadow-[0_0_24px_rgba(239,68,68,0.12)]',
  },
  HIGH: {
    label: 'HIGH',
    title: 'Advertencias',
    Icon: AlertTriangle,
    text: 'text-amber-300',
    badge: 'bg-amber-500/12 text-amber-300 border-amber-500/30',
    border: 'border-amber-500/55',
    accent: 'bg-amber-400',
    shadow: 'shadow-[0_0_24px_rgba(245,158,11,0.10)]',
  },
  WARNING: {
    label: 'WARNING',
    title: 'Advertencias',
    Icon: AlertTriangle,
    text: 'text-amber-300',
    badge: 'bg-amber-500/12 text-amber-300 border-amber-500/30',
    border: 'border-amber-500/55',
    accent: 'bg-amber-400',
    shadow: 'shadow-[0_0_24px_rgba(245,158,11,0.10)]',
  },
  INFO: {
    label: 'INFO',
    title: 'Informativas',
    Icon: Info,
    text: 'text-cyan-300',
    badge: 'bg-cyan-500/12 text-cyan-300 border-cyan-500/30',
    border: 'border-cyan-500/55',
    accent: 'bg-cyan-400',
    shadow: 'shadow-[0_0_24px_rgba(6,182,212,0.10)]',
  },
} as const;

function getAlertMeta(alert: NetworkAlert) {
  return severityMeta[alert.severity] ?? severityMeta.INFO;
}

function normalizeDate(value: string) {
  if (!value) return 'Sin fecha';
  return value.includes('UTC') ? value : `${value} UTC`;
}

export default function AlertasView({
  alerts,
  onAcknowledgeAlert,
  onDeleteAlert,
  onClearAlerts,
  setSelectedAlertToExplain,
  setView,
}: AlertasViewProps) {
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('ALL');
  const [searchPhrase, setSearchPhrase] = useState('');
  const [showExplanationModal, setShowExplanationModal] = useState(false);
  const [explainingAlert, setExplainingAlert] = useState<NetworkAlert | null>(null);
  const [aiStreamingText, setAiStreamingText] = useState('');
  const [aiLoading, setAiLoading] = useState(false);

  const counts = useMemo(() => {
    const active = alerts.filter((alert) => !alert.acknowledged);
    return {
      critical: active.filter((alert) => alert.severity === 'CRITICAL').length,
      warning: active.filter((alert) => alert.severity === 'WARNING' || alert.severity === 'HIGH').length,
      info: active.filter((alert) => alert.severity === 'INFO').length,
      acknowledged: alerts.filter((alert) => alert.acknowledged).length,
    };
  }, [alerts]);

  const filteredAlerts = alerts.filter((alert) => {
    const matchesSeverity =
      severityFilter === 'ALL' ||
      alert.severity === severityFilter ||
      (severityFilter === 'WARNING' && alert.severity === 'HIGH');
    const phrase = searchPhrase.trim().toLowerCase();
    const matchesSearch =
      !phrase ||
      alert.title.toLowerCase().includes(phrase) ||
      alert.description.toLowerCase().includes(phrase) ||
      alert.source.toLowerCase().includes(phrase) ||
      alert.destination.toLowerCase().includes(phrase) ||
      String(alert.port ?? '').toLowerCase().includes(phrase);

    return matchesSeverity && matchesSearch;
  });

  const simulateAIServiceExplanation = (alert: NetworkAlert) => {
    setExplainingAlert(alert);
    setShowExplanationModal(true);
    setAiLoading(true);
    setAiStreamingText('');

    const textToStream = `[ANALISIS VIEWRED]\n\nIncidente: ${alert.title}\nOrigen: ${alert.source}\nDestino: ${alert.destination}\nConfianza: ${alert.confidence}%\n\nDiagnostico:\nLa alerta fue clasificada como ${alert.severity}. El evento coincide con patrones de ${alert.origin || 'deteccion local'} y debe revisarse contra paquetes recientes, puertos expuestos y actividad previa del host.\n\nAcciones sugeridas:\n1. Ver paquetes asociados al flujo ${alert.source} -> ${alert.destination}.\n2. Validar si el puerto ${alert.port || 'afectado'} forma parte de un servicio esperado.\n3. Ejecutar escaneo dirigido del origen si el patron se repite.\n4. Marcar como ACK solo cuando el evento quede documentado o mitigado.`;

    let i = 0;
    const interval = setInterval(() => {
      setAiStreamingText((prev) => prev + textToStream.charAt(i));
      i += 1;
      if (i >= textToStream.length) {
        clearInterval(interval);
        setAiLoading(false);
      }
    }, 7);
  };

  const handleExplainWithAI = (alert: NetworkAlert) => {
    setSelectedAlertToExplain(alert);
    simulateAIServiceExplanation(alert);
  };

  const statCards = [
    {
      key: 'CRITICAL' as SeverityFilter,
      label: 'Criticas',
      value: counts.critical,
      detail: 'Sin reconocer',
      meta: severityMeta.CRITICAL,
    },
    {
      key: 'WARNING' as SeverityFilter,
      label: 'Advertencias',
      value: counts.warning,
      detail: 'Incluye HIGH',
      meta: severityMeta.WARNING,
    },
    {
      key: 'INFO' as SeverityFilter,
      label: 'Informativas',
      value: counts.info,
      detail: 'Eventos de contexto',
      meta: severityMeta.INFO,
    },
    {
      key: 'ALL' as SeverityFilter,
      label: 'Total',
      value: alerts.length,
      detail: `${counts.acknowledged} reconocidas`,
      meta: {
        Icon: Bell,
        text: 'text-blue-300',
        badge: 'bg-blue-500/12 text-blue-300 border-blue-500/30',
        border: 'border-blue-500/55',
        accent: 'bg-blue-400',
        shadow: 'shadow-[0_0_24px_rgba(59,130,246,0.10)]',
      },
    },
  ];

  return (
    <div id="alerts-view-wrapper" className="space-y-5 text-left">
      <div id="alerts-status-counters-grid" className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3 select-none">
        {statCards.map(({ key, label, value, detail, meta }) => {
          const Icon = meta.Icon;
          const active = severityFilter === key || (key === 'WARNING' && severityFilter === 'HIGH');
          return (
            <button
              key={key}
              type="button"
              aria-label={`Contador ${key}`}
              onClick={() => setSeverityFilter(key)}
              className={`group relative overflow-hidden rounded-lg border bg-[#111720] p-4 text-left transition-all duration-200 ${
                active ? `${meta.border} ${meta.shadow}` : 'border-[#263041] hover:border-[#3a465c]'
              }`}
            >
              <div className={`absolute left-0 top-0 h-full w-1 ${meta.accent}`} />
              <div className="flex items-start justify-between gap-3">
                <div>
                  <span className="text-[10px] font-mono uppercase tracking-[0.18em] text-[#748099]">{label}</span>
                  <span className="mt-2 block font-mono text-3xl font-bold text-[#f8fafc]">{value}</span>
                </div>
                <div className={`rounded-md border p-2 ${meta.badge}`}>
                  <Icon className="h-4 w-4" />
                </div>
              </div>
              <div className="mt-3 flex items-center justify-between gap-2">
                <span className="text-[11px] font-mono text-[#8792a8]">{detail}</span>
                <span className={`h-1.5 w-12 rounded-full ${meta.accent} opacity-70`} />
              </div>
            </button>
          );
        })}
      </div>

      <div className="rounded-lg border border-[#263041] bg-[#111720] p-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap gap-1.5 rounded-lg border border-[#263041] bg-[#0a0e14] p-1.5 text-xs select-none">
            {[
              ['ALL', 'Todas'],
              ['CRITICAL', 'Criticas'],
              ['WARNING', 'Advertencia'],
              ['INFO', 'Info'],
            ].map(([key, label]) => (
              <button
                key={key}
                type="button"
                onClick={() => setSeverityFilter(key as SeverityFilter)}
                className={`rounded-md px-3 py-1.5 font-mono transition-all ${
                  severityFilter === key || (key === 'WARNING' && severityFilter === 'HIGH')
                    ? 'bg-[#202a3a] text-[#f8fafc] shadow-inner'
                    : 'text-[#8792a8] hover:bg-[#151c27] hover:text-[#e5edf8]'
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="flex w-full flex-col gap-2 sm:flex-row lg:w-auto">
            <div className="relative min-w-0 flex-1 sm:w-80">
              <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-[#657089]" />
              <input
                type="text"
                placeholder="Buscar IP, firma, puerto..."
                value={searchPhrase}
                onChange={(event) => setSearchPhrase(event.target.value)}
                className="h-9 w-full rounded-md border border-[#263041] bg-[#0a0e14] pl-9 pr-3 font-mono text-xs text-[#e5edf8] outline-none transition-colors placeholder:text-[#566176] focus:border-blue-400/70"
              />
            </div>
            <button
              type="button"
              onClick={onClearAlerts}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md border border-red-500/25 px-3 font-mono text-xs text-red-300 transition-all hover:bg-red-500/10"
              title="Eliminar todas las alertas"
            >
              <Trash2 className="h-3.5 w-3.5" />
              Limpiar Todo
            </button>
          </div>
        </div>
      </div>

      <div id="alerts-stream-list" className="space-y-3">
        {filteredAlerts.length > 0 ? (
          filteredAlerts.map((alert) => {
            const meta = getAlertMeta(alert);
            const Icon = meta.Icon;
            const flowConfidence = Math.max(0, Math.min(100, alert.confidence));
            return (
              <article
                key={alert.id}
                className={`group relative overflow-hidden rounded-lg border bg-[#111720] p-4 shadow-sm transition-all duration-200 ${
                  alert.acknowledged
                    ? 'border-[#263041]/70 opacity-70'
                    : `${meta.border} hover:bg-[#131b27] ${meta.shadow}`
                }`}
              >
                <div className={`absolute left-0 top-0 h-full w-1 ${meta.accent}`} />
                <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className={`inline-flex items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-[10px] font-bold tracking-wider ${meta.badge}`}>
                        <Icon className="h-3 w-3" />
                        {alert.severity}
                      </span>
                      <span className="font-mono text-[11px] text-[#6f7b93]">{normalizeDate(alert.timestamp)}</span>
                      {!alert.acknowledged && <span className={`h-2 w-2 rounded-full ${meta.accent} animate-pulse`} />}
                    </div>
                    <div>
                      <h3 className="text-sm font-bold text-[#f8fafc]">{alert.title}</h3>
                      <p className="mt-1 max-w-5xl text-xs leading-relaxed text-[#aab6c9]">{alert.description}</p>
                    </div>
                  </div>

                  <div className="min-w-[180px] rounded-lg border border-[#263041] bg-[#0a0e14] p-3">
                    <div className="flex items-center justify-between font-mono text-[10px] uppercase tracking-wider text-[#6f7b93]">
                      <span>Confianza</span>
                      <span className="text-[#f8fafc]">{flowConfidence}%</span>
                    </div>
                    <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[#1b2432]">
                      <div className={`h-full ${meta.accent}`} style={{ width: `${flowConfidence}%` }} />
                    </div>
                    <div className="mt-2 truncate font-mono text-[11px] text-[#8792a8]" title={alert.origin}>
                      {alert.origin || 'Deteccion local'}
                    </div>
                  </div>
                </div>

                <div className="mt-4 grid gap-2 rounded-lg border border-[#263041]/70 bg-[#0a0e14] p-3 font-mono text-xs sm:grid-cols-[1fr_auto_1fr_auto] sm:items-center">
                  <div>
                    <span className="block text-[10px] uppercase tracking-wider text-[#657089]">Origen</span>
                    <span className="font-semibold text-[#f8fafc]">{alert.source}</span>
                  </div>
                  <ChevronRight className="hidden h-4 w-4 text-[#657089] sm:block" />
                  <div>
                    <span className="block text-[10px] uppercase tracking-wider text-[#657089]">Destino</span>
                    <span className="font-semibold text-red-300">{alert.destination}</span>
                  </div>
                  <div className="sm:text-right">
                    <span className="block text-[10px] uppercase tracking-wider text-[#657089]">Puerto afectado</span>
                    <span className="font-semibold text-[#f8fafc]">{alert.port || 'Varios / Any'}</span>
                  </div>
                </div>

                <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-[#263041]/70 pt-3">
                  <button
                    type="button"
                    onClick={() => setView('filtros')}
                    className="rounded-md border border-[#263041] bg-[#182131] px-3 py-1.5 font-mono text-xs text-[#e5edf8] transition-all hover:border-blue-400/40 hover:bg-[#202a3a]"
                  >
                    Ver Paquetes
                  </button>
                  <button
                    type="button"
                    onClick={() => handleExplainWithAI(alert)}
                    className="inline-flex items-center gap-1.5 rounded-md border border-blue-400/30 bg-blue-500/10 px-3 py-1.5 font-mono text-xs font-medium text-blue-300 transition-all hover:bg-blue-500/15"
                  >
                    <Sparkles className="h-3.5 w-3.5" />
                    Explicar con IA
                  </button>

                  {!alert.acknowledged && (
                    <button
                      type="button"
                      onClick={() => onAcknowledgeAlert(alert.id)}
                      className="ml-auto inline-flex items-center gap-1.5 rounded-md border border-emerald-400/25 px-3 py-1.5 font-mono text-xs font-bold text-emerald-300 transition-all hover:bg-emerald-500/10"
                    >
                      <Check className="h-3.5 w-3.5" />
                      Aceptar (Ack)
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => onDeleteAlert(alert.id)}
                    className={`${alert.acknowledged ? 'ml-auto' : ''} inline-flex items-center gap-1.5 rounded-md border border-red-500/25 px-3 py-1.5 font-mono text-xs text-red-300 transition-all hover:bg-red-500/10`}
                    title="Eliminar alerta"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    Eliminar
                  </button>
                </div>
              </article>
            );
          })
        ) : (
          <div className="rounded-lg border border-[#263041] bg-[#111720] p-12 text-center">
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-lg border border-emerald-400/25 bg-emerald-500/10 text-emerald-300">
              <ShieldCheck className="h-6 w-6" />
            </div>
            <h3 className="font-bold text-[#f8fafc]">No se encontraron alertas</h3>
            <p className="mt-1 text-xs text-[#8792a8]">No hay eventos que coincidan con el filtro actual.</p>
          </div>
        )}
      </div>

      {showExplanationModal && explainingAlert && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm">
          <div className="flex max-h-[86vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg border border-[#334155] bg-[#111720] text-left shadow-2xl">
            <div className="flex items-center justify-between border-b border-[#263041] bg-[#0a0e14] p-4">
              <div className="flex min-w-0 items-center gap-3">
                <div className="rounded-md border border-blue-400/25 bg-blue-500/10 p-2 text-blue-300">
                  <Sparkles className="h-4 w-4" />
                </div>
                <div className="min-w-0">
                  <span className="block truncate text-sm font-bold text-[#f8fafc]">Analisis inteligente</span>
                  <span className="block truncate font-mono text-[10px] text-[#8792a8]">{explainingAlert.title}</span>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setShowExplanationModal(false)}
                className="rounded-md p-1.5 text-[#8792a8] transition-colors hover:bg-[#182131] hover:text-[#f8fafc]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="space-y-4 overflow-y-auto p-5">
              <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4">
                <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-[#657089]">Alerta analizada</span>
                <p className="mt-2 text-sm font-bold text-[#f8fafc]">{explainingAlert.title}</p>
                <p className="mt-1 border-l-2 border-[#334155] pl-3 text-xs leading-relaxed text-[#aab6c9]">{explainingAlert.description}</p>
              </div>

              <div className="h-72 overflow-y-auto rounded-lg border border-blue-400/20 bg-[#070b10] p-4 font-mono text-xs leading-relaxed text-blue-300 whitespace-pre-wrap select-text">
                {aiStreamingText}
                {aiLoading && <span className="ml-1 inline-block h-4 w-2 animate-pulse bg-blue-300" />}
              </div>
            </div>

            <div className="flex flex-col gap-2 border-t border-[#263041] bg-[#0a0e14] px-5 py-3 text-xs sm:flex-row sm:items-center sm:justify-between">
              <span className="font-mono text-[10px] text-[#657089]">ViewRed AI Local Model</span>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowExplanationModal(false)}
                  className="rounded-md border border-[#263041] px-3 py-1.5 font-mono text-xs text-[#aab6c9] transition-all hover:bg-[#182131] hover:text-white"
                >
                  Cerrar
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setShowExplanationModal(false);
                    setView('asistente');
                  }}
                  className="inline-flex items-center gap-1.5 rounded-md bg-blue-500 px-3.5 py-1.5 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400"
                >
                  Chatear en Asistente IA
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
