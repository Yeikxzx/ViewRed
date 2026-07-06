import { useState, useEffect, useRef, useCallback } from 'react';
import {
  Radio,
  Pause,
  Play,
  StopCircle,
  HardDrive,
  Database,
  Sliders,
  Check,
  AlertTriangle,
  WifiOff,
  RefreshCw,
  Inbox,
} from 'lucide-react';
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from 'recharts';
import { useSSEStore, type CaptureStatus } from '../store/sse-store';
import { deriveCapturaViewState } from '../lib/view-state';
import { getInterfaces, getCaptureConfig, updateCaptureConfig, restartCapture, getPackets, getStoragePolicy, updateStoragePolicy, NetworkInterface } from '../lib/api';
import { notify } from '../lib/notify';

interface CapturaViewProps {
  isCapturing: boolean;
  toggleCapture: () => void;
}

// Health endpoint registered in internal/api/router.go. The Captura
// view polls this every POLL_INTERVAL_MS to derive the real capture
// status (running/paused/unknown) instead of the previous local
// counter fake.
const HEALTH_URL = '/api/v1/health';
const POLL_INTERVAL_MS = 2000;

interface HealthReport {
  status: 'ok' | 'degraded' | 'down';
  uptime_seconds: number;
  subsystems: Array<{ name: string; status: 'ok' | 'down'; detail?: string }>;
}

/**
 * Map a health Report to the CapturaView's coarse-grained status.
 *
 * - "ok" capture subsystem → "running" (the operator is sniffing).
 * - "down" capture subsystem → "paused" (the broker is up but the
 *   capture worker reported an error, which the legacy UI surfaces
 *   as "paused" — a separate error banner carries the detail).
 * - no capture subsystem in the payload (e.g. binary started
 *   without capture wired, or response was 503) → "unknown".
 */
function deriveCaptureStatus(report: HealthReport | null): CaptureStatus {
  if (!report) return 'unknown';
  const capture = report.subsystems.find((s) => s.name === 'capture');
  if (!capture) return 'unknown';
  if (capture.status === 'ok') return 'running';
  if (capture.status === 'down') return 'paused';
  return 'unknown';
}

export default function CapturaView({ isCapturing, toggleCapture }: CapturaViewProps) {
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [selectedInterface, setSelectedInterface] = useState('');
  const [bpfFilter, setBpfFilter] = useState('');
  const [snaplen, setSnaplen] = useState(65535);
  const [rotSizeMB, setRotSizeMB] = useState(500);
  const [retentionHours, setRetentionHours] = useState(168);
  const [showAppliedToast, setShowAppliedToast] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isSavingStorage, setIsSavingStorage] = useState(false);
  const [storageHealth, setStorageHealth] = useState<string | null>(null);
  // Storage chart data computed from real packet DB.
  const [storageData, setStorageData] = useState<{ time: string; used: number }[]>([]);
  const [totalPackets, setTotalPackets] = useState<number | null>(null);
  const [totalBytes, setTotalBytes] = useState<number | null>(null);
  const [activeSessions, setActiveSessions] = useState<number | null>(null);

  // Fetch initial config, interfaces, and storage policy
  useEffect(() => {
    async function loadData() {
      // Load interfaces independently — if pcap fails (no Npcap), the
      // user can still see and change the saved config.
      try {
        const ifaces = await getInterfaces();
        setInterfaces(ifaces);
      } catch (err) {
        console.error('Failed to load interfaces (pcap may be unavailable):', err);
      }

      // Load saved capture config separately so a pcap failure does
      // not prevent the UI from restoring the user's saved interface.
      try {
        const config = await getCaptureConfig();
        if (config) {
          if (config.interface) setSelectedInterface(config.interface);
          if (config.bpf) setBpfFilter(config.bpf);
          if (config.snaplen) setSnaplen(config.snaplen);
        }
      } catch (err) {
        console.error('Failed to load capture config:', err);
      }

      // Load storage policy (rotation size and retention).
      try {
        const policy = await getStoragePolicy();
        setRotSizeMB(policy.max_pcap_size_mb);
        setRetentionHours(policy.max_pcap_age_hours);
      } catch (err) {
        console.error('Failed to load storage policy:', err);
      }
    }
    loadData();
  }, []);

  // Fetch storage history (last 24h) by aggregating packet bytes into
  // hourly buckets. Updates every 30s.
  useEffect(() => {
    async function loadStorageHistory() {
      try {
        const since = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
        const resp = await getPackets({ since, limit: 500 });
        // Initialize 24 hourly buckets
        const buckets: { time: string; used: number; hour: number }[] = [];
        const now = new Date();
        for (let i = 23; i >= 0; i--) {
          const h = new Date(now.getTime() - i * 60 * 60 * 1000);
          buckets.push({
            time: h.getHours().toString().padStart(2, '0') + 'h',
            used: 0,
            hour: h.getHours(),
          });
        }
        // Sum packet bytes per hour bucket
        let sumBytes = 0;
        const uniqueIps = new Set<string>();
        
        for (const pkt of resp.packets) {
          const pktHour = new Date(pkt.time).getHours();
          const b = buckets.find((x) => x.hour === pktHour);
          if (b) b.used += pkt.length;
          
          sumBytes += pkt.length;
          if (pkt.src_ip) uniqueIps.add(pkt.src_ip);
          if (pkt.dst_ip) uniqueIps.add(pkt.dst_ip);
        }
        setStorageData(buckets.map((b) => ({ time: b.time, used: b.used })));
        setTotalPackets(resp.total);
        setTotalBytes(sumBytes);
        setActiveSessions(uniqueIps.size > 0 ? uniqueIps.size - 1 : 0);
      } catch (err) {
        console.error('Failed to load storage history:', err);
      }
    }
    loadStorageHistory();
    const interval = setInterval(loadStorageHistory, POLL_INTERVAL_MS);
    return () => clearInterval(interval);
  }, []);

  // Health-polling state. `healthError` is the local "last poll
  // failed" flag that drives the error banner; it is intentionally
  // separate from `useSSEStore.lastError` because the SSE store
  // tracks SSE connection errors, while this view tracks HTTP
  // health-poll errors. Both write to the same store field via
  // `setLastError` for any future consumer that wants to surface a
  // generic "backend unreachable" state.
  const [healthError, setHealthError] = useState<string | null>(null);
  // `hasPolled` flips to true the first time a poll finishes — even
  // if it errored. We need this to disambiguate "loading" (no poll
  // has resolved yet) from "empty" (we polled and the backend has
  // no `capture` subsystem). Without this flag both branches would
  // map to the same `captureStatus === 'unknown'` and we could not
  // satisfy spec §12.
  const [hasPolled, setHasPolled] = useState(false);
  // `retryCount` is incremented by the "Reintentar" button in the
  // error banner. Including it in the effect's dependency list
  // forces an immediate re-poll, so clicking retry is a true
  // "retry now" (it does not wait for the next 2s tick).
  const [retryCount, setRetryCount] = useState(0);
  const setStoreCaptureStatus = useSSEStore((s) => s.setCaptureStatus);
  const setStoreLastError = useSSEStore((s) => s.setLastError);
  const captureStatus = useSSEStore((s) => s.captureStatus);
  const inFlightRef = useRef(false);

  // ---- Health polling effect ---------------------------------------------
  //
  // Fetches /api/v1/health immediately on mount (or on retry) and
  // then every 2s. The captured promise is held in a ref so a slow
  // response from a previous tick can never overwrite a fresher
  // result. The interval is cleared on unmount to avoid leaking
  // timers into the next test or the next route the user navigates
  // to.
  //
  // Failure handling: a failed poll or a non-2xx response records the
  // error in `healthError` / `lastError` so the UI shows it, but does
  // NOT clobber a previously trusted 'paused' SSE state. Only when
  // the existing state is something other than 'paused' do we degrade
  // to 'unknown'.
  useEffect(() => {
    let cancelled = false;

    const degradeStatus = (): CaptureStatus => {
      const current = useSSEStore.getState().captureStatus;
      return current === 'paused' ? 'paused' : 'unknown';
    };

    const poll = async () => {
      if (cancelled || inFlightRef.current) return;
      inFlightRef.current = true;
      try {
        const res = await fetch(HEALTH_URL);
        if (cancelled) return;
        if (!res.ok) {
          // Non-2xx: record the HTTP error truthfully. Preserve a
          // previously trusted 'paused' state across transient 503s.
          setHealthError(`HTTP ${res.status}`);
          setStoreLastError(`Health endpoint returned ${res.status}`);
          setStoreCaptureStatus(degradeStatus());
          return;
        }
        const body = (await res.json()) as HealthReport;
        if (cancelled) return;
        setHealthError(null);
        setStoreLastError(null);
        setStoreCaptureStatus(deriveCaptureStatus(body));
        // Surface the storage subsystem health for the policy UI.
        const storage = body.subsystems.find((s) => s.name === 'storage');
        if (storage) {
          setStorageHealth(storage.status === 'ok' ? 'ok' : storage.detail || 'error');
        } else {
          setStorageHealth('unknown');
        }
      } catch (err) {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : 'fetch failed';
        setHealthError(msg);
        setStoreLastError(msg);
        setStoreCaptureStatus(degradeStatus());
      } finally {
        inFlightRef.current = false;
        // Mark "we have polled" regardless of success/failure so the
        // ViewState can distinguish loading from empty.
        if (!cancelled) setHasPolled(true);
      }
    };

    poll();
    const interval = setInterval(poll, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [setStoreCaptureStatus, setStoreLastError, retryCount]);

  // "Reintentar" callback used by the spec §12 error banner. We
  // bump `retryCount` so the polling effect re-runs; the actual
  // fetch is the same `poll()` function defined inside the effect.
  const retry = useCallback(() => {
    setRetryCount((c) => c + 1);
  }, []);

  // Spec §12 ViewState. Drives which branch of the view is rendered
  // (loading / error / empty / data). Captura's "empty" means
  // "backend is healthy but has no capture subsystem" — distinct
  // from "loading", which means "we haven't polled yet".
  const viewState = deriveCapturaViewState({
    captureStatus,
    hasPolled,
    healthError,
    retry,
  });

  const handleApplyChanges = async () => {
    if (isCapturing) {
      notify.warning("No se puede aplicar mientras la captura está activa.");
      return;
    }
    
    setIsSaving(true);
    try {
      await updateCaptureConfig({
        interface: selectedInterface,
        bpf: bpfFilter,
        snaplen: snaplen,
      });

      if (captureStatus === 'paused') {
        // La captura está pausada — solo guardamos la config. El cambio
        // aplica cuando el usuario reanude manualmente la captura.
        notify.success("Configuración guardada. Se aplicará al reanudar la captura.");
      } else {
        // La captura no está corriendo (estado unknown / nunca arrancó)
        // o el backend no reporta estado — reiniciamos para que el cambio
        // tenga efecto de inmediato.
        await restartCapture();
        setShowAppliedToast(true);
        setTimeout(() => setShowAppliedToast(false), 3000);
      }
    } catch (err) {
      notify.error("Error al guardar la configuración");
    } finally {
      setIsSaving(false);
    }
  };

  const handleSaveStoragePolicy = async () => {
    setIsSavingStorage(true);
    try {
      await updateStoragePolicy({
        max_pcap_size_mb: rotSizeMB,
        max_pcap_age_hours: retentionHours,
      });
      notify.success("Política de almacenamiento guardada. Los cambios aplicarán al reiniciar la captura.");
    } catch (err) {
      notify.error("Error al guardar la política de almacenamiento");
    } finally {
      setIsSavingStorage(false);
    }
  };

  const handleClearBuffer = () => {
    if (confirm('¿Está absolutamente seguro de limpiar el búfer actual de paquetes capturados en el disco? Esta acción borrará todos los PCAPs temporales.')) {
      // TODO(plan-4): POST /api/v1/capture/clear instead of the local
      // "confirmation" only path. The server is the source of truth
      // for captured bytes; this is a UI affordance for now.
    }
  };

  // Status pill copy + dot color derived from the polled health
  // status, falling back to the prop while the first poll is still
  // in flight. The status badge has a stable id so tests can target
  // it precisely.
  const badgeText =
    captureStatus === 'running'
      ? 'Captura Activa'
      : captureStatus === 'paused'
      ? 'Captura Pausada'
      : 'Estado desconocido';
  const dotClass =
    captureStatus === 'running'
      ? 'bg-[#10b981] animate-ping'
      : captureStatus === 'paused'
      ? 'bg-amber-500'
      : 'bg-red-500';

  return (
    <div id="captura-view-wrapper" className="space-y-5">
      {/* Health-unreachable banner — shown when the last /api/v1/health
          poll threw. Distinct from the SSE banner on the dashboard
          because the failure source is different (HTTP, not EventSource).
          Spec §12 also requires a "Reintentar" (retry) button that
          forces an immediate re-poll (we bump `retryCount` to trigger
          the polling effect). */}
      {viewState.status === 'error' && (
        <div
          id="captura-health-error-banner"
          role="alert"
          className="flex items-center gap-3 rounded-lg border border-red-500/35 bg-red-500/10 px-4 py-3 font-mono text-xs text-red-200"
        >
          <WifiOff className="h-4 w-4 flex-shrink-0" />
          <span className="flex-1">
            No se pudo contactar el servicio de salud ({viewState.message}). Reintentando cada 2s…
          </span>
          <button
            id="captura-health-retry-btn"
            type="button"
            onClick={viewState.retry}
            className="rounded-md border border-red-400/35 px-2.5 py-1 font-mono text-[11px] font-semibold transition-colors hover:bg-red-500/10"
          >
            Reintentar
          </button>
          <RefreshCw className="h-3.5 w-3.5 opacity-60 animate-spin" />
        </div>
      )}

      {/* Empty state (spec §12) — shown when the backend is healthy
          but has no `capture` subsystem. Replaces the status card so
          we don't leak the "Estado desconocido" placeholder on top
          of a real, deliberate configuration. */}
      {viewState.status === 'empty' && (
        <div
          id="captura-empty-state"
          role="status"
          className="flex items-center gap-4 rounded-lg border border-dashed border-[#263041] bg-[#111720] p-8 select-none"
        >
          <Inbox className="h-8 w-8 text-[#5c6473] flex-shrink-0" />
          <div>
            <h3 className="font-sans font-semibold text-sm text-[#e4e7eb]">
              No hay captura configurada
            </h3>
            <p className="font-mono text-xs text-[#9ba3b4] mt-1">
              {viewState.message}
            </p>
          </div>
        </div>
      )}

      {/* Top Capture Control Status Card */}
      <div className="overflow-hidden rounded-lg border border-[#263041] bg-[#111720] text-left">
        <div className="flex flex-col gap-4 border-b border-[#263041] bg-[#0a0e14] p-5 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <span
              id="captura-status-dot"
              data-capture-status={captureStatus}
              className={`w-3.5 h-3.5 rounded-full ${dotClass}`}
            />
            <div
              id="captura-status-badge"
              data-capture-status={captureStatus}
              className="flex flex-col"
            >
              <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                {badgeText}
              </span>
              <span className="font-mono text-[11px] text-[#5c6473]">
                Interfaz: {interfaces.find(i => i.name === selectedInterface)?.description || selectedInterface.split(' ')[0]} • Fuente: /api/v1/health (polling 2s)
              </span>
            </div>
          </div>
          <div className="flex gap-2">
            <button
              id="pause-resume-btn"
              onClick={toggleCapture}
              className={`flex items-center gap-2 rounded-md border px-3.5 py-1.5 font-mono text-xs transition-all ${
                isCapturing
                  ? 'bg-transparent border-[#2a3242] text-[#9ba3b4] hover:bg-[#1c2230]'
                  : 'bg-[#10b981] text-white border-[#10b981] hover:bg-[#10b981]/90'
              }`}
            >
              {isCapturing ? (
                <>
                  <Pause className="h-3.5 w-3.5" />
                  <span>Pausar</span>
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5" />
                  <span>Reanudar</span>
                </>
              )}
            </button>
            <button
              id="stop-capture-btn"
              onClick={handleClearBuffer}
              className="flex items-center gap-2 rounded-md border border-red-500/30 bg-red-500/10 px-3.5 py-1.5 font-mono text-xs text-red-300 transition-all hover:bg-red-500/20"
            >
              <StopCircle className="h-3.5 w-3.5" />
              <span>Detener</span>
            </button>
          </div>
        </div>

        {/* Counters Block — Plan 4 will replace these with real
            values from /api/v1/capture/stats. For Plan 3 we surface
            the live capture status via the badge above and leave the
            counter cards as placeholders that reflect the most recent
            derived status. */}
        <div className="grid grid-cols-2 gap-3 p-5 lg:grid-cols-4">
          <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4 text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Estado del Worker
            </span>
            <span
              className={`text-xl font-mono font-bold block mt-1 ${
                captureStatus === 'running'
                  ? 'text-[#10b981]'
                  : captureStatus === 'paused'
                  ? 'text-amber-500'
                  : 'text-[#e4e7eb]'
              }`}
              id="captura-worker-status"
            >
              {captureStatus === 'running'
                ? 'RUNNING'
                : captureStatus === 'paused'
                ? 'PAUSED'
                : 'UNKNOWN'}
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              Reportado por /api/v1/health
            </span>
          </div>
          <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4 text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Paquetes Procesados
            </span>
            <span className="text-xl font-mono font-bold text-[#3b82f6] block mt-1">
              {totalPackets !== null ? totalPackets.toLocaleString() : '--'}
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              {totalPackets !== null ? 'Últimas 24h' : 'Esperando métricas...'}
            </span>
          </div>
          <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4 text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Tráfico Total
            </span>
            <span className="text-xl font-mono font-bold text-[#e4e7eb] block mt-1">
              {totalBytes !== null ? (totalBytes / (1024 * 1024)).toFixed(2) : '--'} MB
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              {totalBytes !== null ? 'Últimas 24h' : 'Esperando métricas...'}
            </span>
          </div>
          <div className="rounded-lg border border-[#263041] bg-[#0a0e14] p-4 text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Sesiones Activas
            </span>
            <span
              className="text-xl font-mono font-bold text-[#e4e7eb] block mt-1"
            >
              {activeSessions !== null ? activeSessions.toLocaleString() : '--'}
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              {activeSessions !== null ? 'Endpoints detectados' : 'Esperando métricas...'}
            </span>
          </div>
        </div>
      </div>

      <div className="space-y-4">
        {/* Storage progression area chart */}
        <div className="flex h-fit flex-col overflow-hidden rounded-lg border border-[#263041] bg-[#111720] text-left">
          <div className="flex items-center justify-between border-b border-[#263041] bg-[#0a0e14] px-4 py-3">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Uso de Almacenamiento PCAPs (Últimas 24h)
            </h2>
            <HardDrive className="h-4 w-4 text-[#5c6473]" />
          </div>

          <div className="h-52 w-full px-4 py-4 font-mono text-xs">
            {storageData.every((d) => d.used === 0) ? (
              <div className="h-full flex items-center justify-center text-[#5c6473]">
                <span className="text-[10px]">Sin datos de uso de almacenamiento en las últimas 24h</span>
              </div>
            ) : (
            <ResponsiveContainer width="100%" height="100%" minWidth={0}>
              <AreaChart data={storageData} margin={{ top: 4, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="storageColor" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#263041" vertical={false} />
                <XAxis dataKey="time" stroke="#5c6473" tickLine={false} style={{ fontSize: 9 }} />
                <YAxis stroke="#5c6473" tickLine={false} style={{ fontSize: 9 }} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#111720',
                    borderColor: '#334155',
                    color: '#e5edf8',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="used"
                  stroke="#3b82f6"
                  strokeWidth={2}
                  fillOpacity={1}
                  fill="url(#storageColor)"
                  name="Espacio (MB)"
                />
              </AreaChart>
            </ResponsiveContainer>
            )}
          </div>

          <div className="flex items-center justify-between border-t border-[#263041] bg-[#0a0e14] px-4 py-2 font-mono text-[10px] text-[#657089]">
            <span>ESPACIO CONSUMIDO POR PCAPs</span>
            <span>CAPACIDAD MÁXIMA ASIGNADA: 500 GB</span>
          </div>
        </div>

        {/* Configuration cards */}
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
          {/* Form Capture Config */}
          <div className="space-y-3 rounded-lg border border-[#263041] bg-[#111720] p-4">
            <div className="flex items-center gap-2 border-b border-[#263041] pb-2">
              <Sliders className="h-4 w-4 text-[#3b82f6]" />
              <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                Configuración de Captura
              </h2>
            </div>

            {/* Interface Select */}
            <div className="space-y-1">
              <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
                Interfaz de Red
              </label>
              <select
                value={selectedInterface}
                onChange={(e) => setSelectedInterface(e.target.value)}
                disabled={isCapturing}
                className="h-9 w-full cursor-pointer rounded-md border border-[#263041] bg-[#0a0e14] px-3 font-mono text-xs text-[#e4e7eb] outline-none disabled:opacity-60"
              >
                {interfaces.map((iface) => (
                  <option key={iface.name} value={iface.name}>
                    {iface.description ? `${iface.description} (${iface.name})` : iface.name}
                  </option>
                ))}
              </select>
              {isCapturing && (
                <span className="text-[9px] font-mono text-amber-500 block leading-tight mt-1">
                  ⚠ La interfaz no se puede cambiar mientras la captura está activa.
                </span>
              )}
            </div>

            {/* BPF Filters */}
            <div className="space-y-1">
              <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
                Filtro BPF
              </label>
              <input
                type="text"
                value={bpfFilter}
                onChange={(e) => setBpfFilter(e.target.value)}
                placeholder="Ex. tcp port 80 or udp"
                className="h-9 w-full rounded-md border border-[#263041] bg-[#0a0e14] px-3 font-mono text-xs text-[#e4e7eb] outline-none focus:border-blue-400/70"
              />
            </div>

            {/* Snaplen size */}
            <div className="space-y-1">
              <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
                Snaplen (Bytes)
              </label>
              <input
                type="number"
                value={snaplen}
                onChange={(e) => setSnaplen(Number(e.target.value))}
                className="h-9 w-full rounded-md border border-[#263041] bg-[#0a0e14] px-3 font-mono text-xs text-[#e4e7eb] outline-none focus:border-blue-400/70"
              />
              <span className="text-[9px] text-[#5c6473] font-mono block">
                Tamaño máximo de paquete a capturar.
              </span>
            </div>

            {/* Save Buttons */}
            <button
              onClick={handleApplyChanges}
              disabled={isSaving || isCapturing}
              className="flex h-8 w-full cursor-pointer items-center justify-center gap-2 rounded-md bg-blue-500 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400 disabled:opacity-50"
            >
              <span>{isSaving ? "Guardando..." : "Aplicar Cambios"}</span>
            </button>
          </div>

          {/* Form Storage policy */}
          <div className="space-y-3 rounded-lg border border-[#263041] bg-[#111720] p-4">
            <div className="flex items-center gap-2 border-b border-[#263041] pb-2">
              <Database className="h-4 w-4 text-[#3b82f6]" />
              <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                Políticas de Almacenamiento
              </h2>
            </div>

            {/* File rotation slider */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">Rotar archivo cada</span>
                <span className="text-[#e4e7eb] font-bold">{rotSizeMB} MB</span>
              </div>
              <input
                type="range"
                min="100"
                max="1000"
                step="50"
                value={rotSizeMB}
                onChange={(e) => setRotSizeMB(Number(e.target.value))}
                className="w-full h-1.5 bg-[#0a0e14] rounded-lg appearance-none cursor-pointer"
              />
            </div>

            {/* Max Retention slider */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">Retención Máxima</span>
                <span className="text-[#e4e7eb] font-bold">{retentionHours}h ({Math.round(retentionHours / 24)} días)</span>
              </div>
              <input
                type="range"
                min="24"
                max="720"
                step="24"
                value={retentionHours}
                onChange={(e) => setRetentionHours(Number(e.target.value))}
                className="w-full h-1.5 bg-[#0a0e14] rounded-lg appearance-none cursor-pointer"
              />
            </div>

            {/* Storage health indicator */}
            <div className="flex items-center gap-2 text-[10px] font-mono">
              <span className={`w-2 h-2 rounded-full ${
                storageHealth === 'ok' ? 'bg-[#10b981]' : storageHealth === 'unknown' ? 'bg-[#5c6473]' : 'bg-red-500'
              }`} />
              <span className={storageHealth === 'ok' ? 'text-[#10b981]' : 'text-[#9ba3b4]'}>
                {storageHealth === 'ok'
                  ? 'Base de datos conectada'
                  : storageHealth === 'unknown'
                  ? 'Estado de almacenamiento desconocido'
                  : `Storage: ${storageHealth}`}
              </span>
            </div>

            <div className="flex items-start gap-2 text-[10px] text-[#5c6473] font-mono">
              <AlertTriangle className="h-3 w-3 text-amber-500 shrink-0 mt-0.5" />
              <span>
                Los archivos más antiguos serán eliminados de forma automática cuando el disco supere
                el 90% de capacidad asignada.
              </span>
            </div>

            {/* Save button for storage policy */}
            <button
              onClick={handleSaveStoragePolicy}
              disabled={isSavingStorage}
              className="flex h-8 w-full cursor-pointer items-center justify-center gap-2 rounded-md border border-blue-400/30 bg-blue-500/10 font-mono text-xs font-bold text-blue-300 transition-all hover:bg-blue-500/15 disabled:opacity-50"
            >
              <span>{isSavingStorage ? "Guardando..." : "Guardar Política de Almacenamiento"}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Applied toast popup */}
      {showAppliedToast && (
        <div className="fixed bottom-6 right-6 z-50 bg-[#10b981] text-white px-4 py-2.5 rounded shadow-xl shadow-black/60 font-sans text-xs flex flex-col gap-1 animate-bounce">
          <div className="flex items-center gap-2 font-semibold">
            <Check className="h-4 w-4" />
            <span>Configuración aplicada. Captura reiniciada con la nueva interfaz.</span>
          </div>
        </div>
      )}
    </div>
  );
}
