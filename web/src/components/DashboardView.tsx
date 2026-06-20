import { useEffect, useState } from 'react';
import {
  Activity,
  Database,
  Radio,
  ExternalLink,
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
import { KPI, ViewType, NetworkAlert, IPVolume } from '../types';
import { initialTrafficHistory } from '../data/mockData';
import { getTopTalkers, getProtocols, getRecentAlerts } from '../lib/api';
import { useSSE } from '../lib/sse';
import { useSSEStore } from '../store/sse-store';
import {
  deriveDashboardViewState,
  type DashboardMetrics,
} from '../lib/view-state';

// SSE stream URL for live dashboard metrics. Matches the route
// registered in internal/api/router.go (`/api/v1/stream/metrics`).
// The same string is the key the useSSE hook uses to write payloads
// into the global Zustand store.
const METRICS_URL = '/api/v1/stream/metrics';

// Rolling window size for the traffic area chart. Spec §11.1 asks for
// "the last 60 data points from SSE events" — at the 1 Hz broker tick
// this is a one-minute history.
const TRAFFIC_WINDOW = 60;

// ---- number formatters -----------------------------------------------------
//
// The backend sends `bytes_per_sec` as raw bytes (float64). Networking
// conventions are 1000-based: 1 KB = 1000 B, 1 MB = 1000 KB, etc. We
// always show one decimal for the leading unit so a steady 1.2 MB/s
// stays readable instead of jumping to "1,234,567 B/s".
function formatPacketsRate(value: number): string {
  return value.toLocaleString('en-US', {
    maximumFractionDigits: 1,
  });
}

function formatBytesRate(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} GB`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} MB`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)} KB`;
  return `${value.toFixed(0)} B`;
}

function formatInteger(value: number): string {
  return value.toLocaleString('en-US');
}

interface DashboardViewProps {
  setView: (view: ViewType) => void;
  setSelectedAlert: (alert: NetworkAlert | null) => void;
}

export default function DashboardView({
  setView,
  setSelectedAlert,
}: DashboardViewProps) {
  const [topTalkers, setTopTalkers] = useState<IPVolume[]>([]);
  const [protocols, setProtocols] = useState<{name: string, value: string, color: string}[]>([]);
  const [recentAlerts, setRecentAlerts] = useState<NetworkAlert[]>([]);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [ttRes, protoRes, alertsRes] = await Promise.all([
          getTopTalkers(),
          getProtocols(),
          getRecentAlerts(4)
        ]);
        
        const maxBytes = Math.max(...ttRes.top_talkers.map(t => t.bytes), 1);
        setTopTalkers(ttRes.top_talkers.map(t => ({
          ip: t.src_ip,
          volume: formatBytesRate(t.bytes),
          percentage: (t.bytes / maxBytes) * 100
        })));
        
        const colors: Record<string, string> = { 'TCP': '#3b82f6', 'UDP': '#10b981', 'ICMP': '#f59e0b', 'HTTPS': '#8b5cf6', 'ARP': '#06b6d4' };
        setProtocols(protoRes.protocols.map(p => ({
          name: p.protocol,
          value: p.percentage.toFixed(1),
          color: colors[p.protocol.toUpperCase()] || '#9ba3b4'
        })));
        
        setRecentAlerts(alertsRes);
      } catch (err) {
        console.error("Error fetching dashboard data:", err);
      }
    };
    
    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, []);
  // Open the SSE stream. The hook handles EventSource lifecycle,
  // exponential-backoff reconnection, and writes incoming payloads
  // to the global Zustand store under METRICS_URL. We capture the
  // returned `reconnect` so the error banner can wire it up as the
  // spec §12 "Reintentar" (retry) callback.
  const { reconnect } = useSSE(METRICS_URL);

  // Selectors trigger re-renders when the messages map, connection
  // status, or last error change. Each selector returns a primitive
  // (or `undefined` for a missing key) so Zustand's default reference
  // equality is enough — no shallow comparator needed.
  const metrics = useSSEStore((s) =>
    s.messages.get(METRICS_URL),
  ) as DashboardMetrics | undefined;

  const connectionStatus = useSSEStore((s) => s.connectionStatus);
  const lastError = useSSEStore((s) => s.lastError);

  // Spec §12: derive a single ViewState<DashboardMetrics> from the
  // SSE inputs. The view branches (loading / error / empty / data)
  // are mutually exclusive at the type level — error wins over
  // partial data so the ViewState type never says "data" while a
  // connection error is in flight.
  const viewState = deriveDashboardViewState({
    metrics,
    connectionStatus,
    lastError,
    retry: reconnect,
  });

  // ---- traffic chart accumulator -----------------------------------------
  //
  // Spec §11.1: "Keep the traffic chart using local state that
  // accumulates the last 60 data points from SSE events." We append
  // one point per metrics event and drop the oldest when the window
  // fills up. The first event also seeds the chart with one row so the
  // area doesn't render an empty series.
  //
  // The accumulator lives in a `useEffect` keyed on the metrics object
  // (a new reference per broker tick) so we never call setState during
  // render. The functional `setTrafficData` form makes the dedup check
  // race-free without needing a ref to the previous state.
  const [trafficData, setTrafficData] = useState(initialTrafficHistory);
  useEffect(() => {
    if (metrics === undefined) return;
    setTrafficData((prev) => {
      const last = prev[prev.length - 1];
      // Only append when the value actually changed — the broker
      // ticks at 1 Hz, so a steady stream of identical values would
      // otherwise grow the history without adding information.
      if (
        last &&
        last.tcp === metrics.bytes_per_sec &&
        last.udp === metrics.packets_per_sec
      ) {
        return prev;
      }
      const nextTime = new Date().toLocaleTimeString('es-ES', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
      const nextItem = {
        time: nextTime,
        // The legacy mock chart used two arbitrary "tcp"/"udp"
        // series; we re-use those keys so the AreaChart keeps
        // rendering without any prop changes. `tcp` carries bytes/s,
        // `udp` carries packets/s. Future plans (Plan 4 / 5) will
        // replace these series with per-protocol breakdowns.
        tcp: Math.max(0, metrics.bytes_per_sec),
        udp: Math.max(0, metrics.packets_per_sec),
        other: last?.other ?? 0,
      };
      return prev.length >= TRAFFIC_WINDOW
        ? [...prev.slice(1), nextItem]
        : [...prev, nextItem];
    });
  }, [metrics]);

  // ---- KPI cards -----------------------------------------------------------
  //
  // The cards are derived from the live payload. The ViewState drives
  // which sub-tree to render:
  //   - loading → 4 skeleton placeholders (no value text).
  //   - data    → 4 real KPI cards.
  //   - empty   → "No hay tráfico" container; the KPI grid is
  //               REPLACED (spec: error/empty states must not render
  //               partial data underneath).
  //   - error   → if metrics is still in the store, keep the cards
  //               visible in the background so a transient drop
  //               doesn't wipe the operator's screen. The banner
  //               above still signals the failure.
  const showKpiCards =
    viewState.status === 'data' ||
    (viewState.status === 'error' && metrics !== undefined);
  const showKpiSkeletons = viewState.status === 'loading';
  const showEmptyState = viewState.status === 'empty';

  const kpiData: KPI[] | null = showKpiCards
    ? [
        {
          title: 'Paquetes Totales',
          value: formatPacketsRate(metrics!.packets_per_sec),
          change: '▲ en vivo',
          trend: 'up',
          color: 'text-[#3b82f6]',
          icon: 'Activity',
        },
        {
          title: 'Volumen (Bytes)',
          value: formatBytesRate(metrics!.bytes_per_sec),
          change: '▲ en vivo',
          trend: 'up',
          color: 'text-[#10b981]',
          icon: 'Database',
        },
        {
          title: 'Conexiones Activas',
          value: formatInteger(metrics!.active_connections),
          change: '▲ en vivo',
          trend: 'up',
          color: 'text-[#f59e0b]',
          icon: 'Activity',
        },
        {
          title: 'Dispositivos Únicos',
          value: formatInteger(metrics!.device_count),
          change: '▲ en vivo',
          trend: 'up',
          color: 'text-[#06b6d4]',
          icon: 'Database',
        },
      ]
    : null;

  const handleAlertInteraction = (alert: NetworkAlert) => {
    setSelectedAlert(alert);
    setView('alertas');
  };

  return (
    <div id="dashboard-view-wrapper" className="space-y-6">
      {/* SSE error banner — shown when the broker connection is
          explicitly down and a non-null last error has been recorded.
          Per spec §11.1 the copy is "Conexión SSE perdida.
          Reintentando...". The banner sits above the KPI grid so it
          is the first thing the operator sees when the stream goes
          away. Spec §12 also requires a "Reintentar" (retry) button
          that wires up to the SSE hook's `reconnect` callback. */}
      {viewState.status === 'error' && (
        <div
          id="sse-error-banner"
          role="alert"
          className="flex items-center gap-3 bg-[#3a1d1d] border border-[#ef4444]/40 text-[#fca5a5] px-4 py-2.5 rounded-md text-xs font-mono"
        >
          <WifiOff className="h-4 w-4 flex-shrink-0" />
          <span className="flex-1">
            Conexión SSE perdida. Reintentando…
          </span>
          <button
            id="sse-error-retry-btn"
            type="button"
            onClick={viewState.retry}
            className="font-mono text-[11px] font-semibold px-2.5 py-1 rounded border border-[#ef4444]/40 hover:bg-[#ef4444]/10 transition-colors"
          >
            Reintentar
          </button>
          <RefreshCw className="h-3.5 w-3.5 opacity-60 animate-spin" />
        </div>
      )}

      {/* 4 KPIs Highlight cards — driven by the ViewState. We render
          one of three branches in this slot:
            * empty   → "No hay tráfico" container (replaces grid).
            * loading → 4 skeleton placeholders.
            * data / error-with-data → 4 real KPI cards (error
              keeps the data visible in the background per spec §12).
            * error-no-data → no grid at all; the banner above is
              the only signal. We deliberately do NOT fall back to
              skeletons here because the spec says the error
              banner carries the retry affordance. */}
      {showEmptyState ? (
        <div
          id="dashboard-empty-state"
          role="status"
          className="bg-[#131820] border border-[#2a3242] border-dashed rounded-lg p-10 flex flex-col items-center justify-center text-center select-none"
        >
          <Inbox className="h-10 w-10 text-[#5c6473] mb-3" />
          <h3 className="font-sans font-semibold text-sm text-[#e4e7eb] mb-1">
            No hay tráfico en este momento
          </h3>
          <p className="font-mono text-xs text-[#9ba3b4] max-w-md">
            {(viewState as { message: string }).message}
          </p>
          <button
            type="button"
            onClick={() => setView('captura')}
            className="mt-4 font-mono text-xs font-semibold px-3 py-1.5 rounded border border-[#3b82f6]/40 text-[#3b82f6] hover:bg-[#3b82f6]/10 transition-colors"
          >
            Ir a Captura
          </button>
        </div>
      ) : showKpiCards || showKpiSkeletons ? (
        <div
          id="kpi-grid"
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4"
        >
          {showKpiSkeletons
            ? // Loading state: 4 skeleton placeholders, each with a
              // stable `kpi-card-N-skeleton` id and the same dimensions
              // as the real card so the layout doesn't shift when data
              // arrives.
              [0, 1, 2, 3].map((idx) => (
                <div
                  key={idx}
                  id={`kpi-card-${idx}-skeleton`}
                  aria-busy="true"
                  className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between text-left select-none"
                >
                  <div className="h-3 w-24 rounded bg-[#1c2230] animate-pulse" />
                  <div className="mt-3 h-7 w-20 rounded bg-[#1c2230] animate-pulse" />
                  <div className="mt-2 h-2.5 w-32 rounded bg-[#1c2230] animate-pulse" />
                </div>
              ))
            : kpiData!.map((kpi, idx) => (
                <div
                  key={idx}
                  id={`kpi-card-${idx}`}
                  className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between text-left hover:border-[#3b82f6]/40 transition-all select-none"
                >
                  <div className="flex justify-between items-center">
                    <span className="font-sans text-xs text-[#9ba3b4] font-medium tracking-wide uppercase">
                      {kpi.title}
                    </span>
                    <span className="text-[#5c6473] font-mono text-sm">[]</span>
                  </div>
                  <div className="mt-2.5 flex items-baseline gap-2.5">
                    <span className="font-sans font-bold text-[#e4e7eb] text-2xl tracking-tight">
                      {kpi.value}
                    </span>
                    <span
                      className={`font-mono text-xs font-semibold px-1.5 py-0.5 rounded ${
                        kpi.trend === 'up'
                          ? 'text-[#10b981]'
                          : kpi.trend === 'down'
                          ? 'text-[#ef4444]'
                          : 'text-[#9ba3b4]'
                      }`}
                    >
                      {kpi.change}
                    </span>
                  </div>
                  <div className="mt-1 text-[10px] font-mono text-[#5c6473]">
                    <span>Últimos 60s acumulados</span>
                  </div>
                </div>
              ))}
        </div>
      ) : null}

      {/* Main Charts area */}
      <div
        id="dashboard-charts-row"
        className="grid grid-cols-1 lg:grid-cols-3 gap-6"
      >
        {/* Line wave/area Chart */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between lg:col-span-2 text-left">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-3 mb-4">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Tráfico en Tiempo Real (Throughput)
            </h2>
            <div className="flex gap-2 text-[10px] font-mono">
              <span className="bg-[#1c2230] border border-[#2a3242] rounded px-2 py-0.5 text-[#e4e7eb] cursor-pointer">
                Últimos 60s
              </span>
              <span className="text-[#5c6473] hover:text-[#9ba3b4] cursor-pointer px-1 py-0.5">
                (Mbps)
              </span>
            </div>
          </div>

          <div className="h-56 w-full font-mono text-xs">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart
                data={trafficData}
                margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
              >
                <defs>
                  <linearGradient id="tcpColor" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.25} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="udpColor" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#10b981" stopOpacity={0.2} />
                    <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="#1c2230"
                  vertical={false}
                />
                <XAxis
                  dataKey="time"
                  stroke="#5c6473"
                  tickLine={false}
                  style={{ fontSize: 9 }}
                />
                <YAxis
                  stroke="#5c6473"
                  tickLine={false}
                  style={{ fontSize: 9 }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#131820',
                    borderColor: '#3a4356',
                    color: '#e4e7eb',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="tcp"
                  stroke="#3b82f6"
                  strokeWidth={2}
                  fillOpacity={1}
                  fill="url(#tcpColor)"
                  name="TCP (Mbps)"
                />
                <Area
                  type="monotone"
                  dataKey="udp"
                  stroke="#10b981"
                  strokeWidth={1.5}
                  fillOpacity={1}
                  fill="url(#udpColor)"
                  name="UDP (Mbps)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Protocol Distribution Donut Pie Chart */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between text-left">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-3 mb-4">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Distribución de Protocolos
            </h2>
            <Radio className="h-4 w-4 text-[#5c6473]" />
          </div>

          {/* Elegant Custom SVG Donut to match EXACT screenshot visuals */}
          <div className="flex flex-col items-center justify-center flex-1 my-3 select-none">
            <div className="relative w-36 h-36 flex items-center justify-center">
              {/* Radial Progress Layers representing L4 stats */}
              <svg
                className="absolute w-full h-full transform -rotate-90"
                viewBox="0 0 100 100"
              >
                {/* Background base Ring */}
                <circle
                  cx="50"
                  cy="50"
                  r="40"
                  stroke="#1c2230"
                  strokeWidth="8"
                  fill="transparent"
                />
                {(() => {
                  let cumulative = 0;
                  return protocols.map((p, i) => {
                    const percent = parseFloat(p.value);
                    if (percent === 0) return null;
                    const dashoffset = 251.3274 - (percent / 100) * 251.3274;
                    const rotate = (cumulative / 100) * 360;
                    cumulative += percent;
                    return (
                      <circle
                        key={i}
                        cx="50"
                        cy="50"
                        r="40"
                        stroke={p.color}
                        strokeWidth="8"
                        fill="transparent"
                        strokeDasharray="251.3274"
                        strokeDashoffset={dashoffset}
                        transform={`rotate(${rotate} 50 50)`}
                        className="transition-all duration-1000 ease-out"
                      />
                    );
                  });
                })()}
              </svg>
              {/* Inner Circle metrics */}
              <div className="text-center">
                <span className="font-mono text-xs text-[#9ba3b4] uppercase tracking-wider block">
                  Capa 4
                </span>
                <span className="font-sans font-bold text-lg text-[#e4e7eb] block mt-0.5">
                  L4 Total
                </span>
              </div>
            </div>

            {/* Legends */}
            <div className="grid grid-cols-2 gap-x-6 gap-y-2 mt-4 w-full px-2">
              {protocols.map((proto, index) => (
                <div key={index} className="flex items-center gap-2 text-xs">
                  <span
                    className="w-2.5 h-2.5 rounded-sm flex-shrink-0"
                    style={{ backgroundColor: proto.color }}
                  />
                  <span className="font-sans text-[#9ba3b4]">
                    {proto.name}
                  </span>
                  <span className="font-mono font-bold text-[#e4e7eb] ml-auto">
                    {proto.value}%
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Top Talkers & Recent Alerts */}
      <div
        id="lower-dashboard-row"
        className="grid grid-cols-1 lg:grid-cols-2 gap-6"
      >
        {/* Top talkers IPs */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between text-left">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-3 mb-4">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Top Talkers (IPs)
            </h2>
            <button
              onClick={() => setView('filtros')}
              className="text-[#3b82f6] hover:underline text-xs font-semibold flex items-center gap-1.5"
            >
              <span>Ver Todo</span>
              <ExternalLink className="h-3 w-3" />
            </button>
          </div>

          <div className="space-y-4">
            <div className="grid grid-cols-12 text-[10px] font-mono text-[#5c6473] border-b border-[#1c2230] pb-1.5 font-bold">
              <span className="col-span-4 uppercase">Origen</span>
              <span className="col-span-6 uppercase">Volumen</span>
              <span className="col-span-2 text-right uppercase">Total</span>
            </div>

            {topTalkers.map((talker, idx) => (
              <div
                key={idx}
                className="grid grid-cols-12 items-center text-xs"
              >
                {/* IP address cell */}
                <span className="col-span-4 font-mono font-medium text-[#e4e7eb] hover:text-[#3b82f6] cursor-pointer">
                  {talker.ip}
                </span>
                {/* Custom dynamic percentage horizontal bar */}
                <div className="col-span-6 pr-3 flex items-center">
                  <div className="w-full h-2 bg-[#0a0e14] rounded overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-[#3b82f6] to-[#adc6ff] rounded"
                      style={{ width: `${talker.percentage}%` }}
                    />
                  </div>
                </div>
                {/* Statistics Total Value Cell */}
                <span className="col-span-2 text-right font-mono font-semibold text-[#9ba3b4]">
                  {talker.volume}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Recent alerts section */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between text-left">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-3 mb-4">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Alertas Recientes
            </h2>
            <button
              onClick={() => setView('alertas')}
              className="text-[#3b82f6] hover:underline text-xs font-semibold flex items-center gap-1"
            >
              <span>Gestionar</span>
              <ExternalLink className="h-3 w-3" />
            </button>
          </div>

          <div className="space-y-3.5 max-h-64 overflow-y-auto pr-1">
            {recentAlerts.map((alert) => (
              <div
                key={alert.id}
                onClick={() => handleAlertInteraction(alert)}
                className="p-2 w-full hover:bg-[#1c2230] hover:border-[#3b82f6]/40 cursor-pointer border border-[#1c2230] rounded-md transition-all flex items-start gap-3"
              >
                {/* Severity indicator dots */}
                <span
                  className={`w-2 h-2 rounded-full mt-1.5 flex-shrink-0 ${
                    alert.severity === 'CRITICAL'
                      ? 'bg-red-500 animate-pulse'
                      : alert.severity === 'WARNING'
                      ? 'bg-amber-500'
                      : 'bg-cyan-500'
                  }`}
                />

                <div className="flex-1 flex flex-col min-w-0">
                  <div className="flex justify-between items-center">
                    <span className="font-sans font-semibold text-[#e4e7eb] text-xs truncate">
                      {alert.title}
                    </span>
                    <span className="font-mono text-[10px] text-[#5c6473] ml-2 shrink-0">
                      {alert.timeAgo}
                    </span>
                  </div>
                  <span className="font-sans text-[11px] text-[#9ba3b4] mt-0.5 line-clamp-1">
                    {alert.description}
                  </span>
                  <div className="flex gap-2.5 text-[10px] font-mono text-[#5c6473] mt-1.5">
                    <span>ORIGEN: {alert.source}</span>
                    <span>•</span>
                    <span>ID: {alert.id}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
