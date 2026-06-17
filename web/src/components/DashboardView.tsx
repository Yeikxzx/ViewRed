import { useState, useEffect } from 'react';
import {
  Activity,
  ArrowUp,
  ArrowDown,
  Shield,
  Zap,
  HelpCircle,
  Database,
  Radio,
  ExternalLink,
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
import { KPI, ViewType, NetworkAlert } from '../types';
import {
  topTalkersData,
  protocolDist,
  initialTrafficHistory,
} from '../data/mockData';

interface DashboardViewProps {
  setView: (view: ViewType) => void;
  alerts: NetworkAlert[];
  setSelectedAlert: (alert: NetworkAlert | null) => void;
}

export default function DashboardView({
  setView,
  alerts,
  setSelectedAlert,
}: DashboardViewProps) {
  const [trafficData, setTrafficData] = useState(initialTrafficHistory);
  const [throughput, setThroughput] = useState({ rx: '45.2 GB', tx: '5.4%' });

  // Dynamically update traffic chart lines to give it a stunning live feeling
  useEffect(() => {
    const interval = setInterval(() => {
      setTrafficData((prev) => {
        const nextTime = new Date().toLocaleTimeString('es-ES', {
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
        });
        const tcpNoise = Math.floor((Math.random() - 0.4) * 80);
        const udpNoise = Math.floor((Math.random() - 0.4) * 50);
        const otherNoise = Math.floor((Math.random() - 0.5) * 20);

        const lastItem = prev[prev.length - 1];
        const nextItem = {
          time: nextTime,
          tcp: Math.max(150, Math.min(800, lastItem.tcp + tcpNoise)),
          udp: Math.max(60, Math.min(350, lastItem.udp + udpNoise)),
          other: Math.max(20, Math.min(100, lastItem.other + otherNoise)),
        };

        return [...prev.slice(1), nextItem];
      });
    }, 3000);

    return () => clearInterval(interval);
  }, []);

  const kpiData: KPI[] = [
    {
      title: 'Paquetes Totales',
      value: '1.45M',
      change: '▲ 12%',
      trend: 'up',
      color: 'text-[#3b82f6]',
      icon: 'Activity',
    },
    {
      title: 'Volumen (Bytes)',
      value: '45.2 GB',
      change: '▲ 5.4%',
      trend: 'up',
      color: 'text-[#10b981]',
      icon: 'Database',
    },
    {
      title: 'Conexiones Activas',
      value: '12,450',
      change: '▼ 2.1%',
      trend: 'down',
      color: 'text-[#f59e0b]',
      icon: 'Zap',
    },
    {
      title: 'Dispositivos Únicos',
      value: '342',
      change: '- 0%',
      trend: 'neutral',
      color: 'text-[#06b6d4]',
      icon: 'Shield',
    },
  ];

  const handleAlertInteraction = (alert: NetworkAlert) => {
    setSelectedAlert(alert);
    setView('alertas');
  };

  return (
    <div id="dashboard-view-wrapper" className="space-y-6">
      {/* 4 KPIs Highlight cards */}
      <div id="kpi-grid" className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {kpiData.map((kpi, idx) => (
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

      {/* Main Charts area */}
      <div id="dashboard-charts-row" className="grid grid-cols-1 lg:grid-cols-3 gap-6">
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
                <CartesianGrid strokeDasharray="3 3" stroke="#1c2230" vertical={false} />
                <XAxis dataKey="time" stroke="#5c6473" tickLine={false} style={{ fontSize: 9 }} />
                <YAxis stroke="#5c6473" tickLine={false} style={{ fontSize: 9 }} />
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
              <svg className="absolute w-full h-full transform -rotate-90" viewBox="0 0 100 100">
                {/* Background base Ring */}
                <circle
                  cx="50"
                  cy="50"
                  r="40"
                  stroke="#1c2230"
                  strokeWidth="8"
                  fill="transparent"
                />
                {/* TCP Section (blue) - 50% */}
                <circle
                  cx="50"
                  cy="50"
                  r="40"
                  stroke="#3b82f6"
                  strokeWidth="8"
                  fill="transparent"
                  strokeDasharray="251.2"
                  strokeDashoffset="125.6"
                />
                {/* HTTPS Section (purple) - 25% */}
                <circle
                  cx="50"
                  cy="50"
                  r="40"
                  stroke="#8b5cf6"
                  strokeWidth="8"
                  fill="transparent"
                  strokeDasharray="251.2"
                  strokeDashoffset="188.4"
                  className="transform rotate-[180deg] origin-center"
                />
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
              {protocolDist.map((proto, index) => (
                <div key={index} className="flex items-center gap-2 text-xs">
                  <span
                    className="w-2.5 h-2.5 rounded-sm flex-shrink-0"
                    style={{ backgroundColor: proto.color }}
                  />
                  <span className="font-sans text-[#9ba3b4]">{proto.name}</span>
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
      <div id="lower-dashboard-row" className="grid grid-cols-1 lg:grid-cols-2 gap-6">
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

            {topTalkersData.map((talker, idx) => (
              <div key={idx} className="grid grid-cols-12 items-center text-xs">
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
            {alerts.slice(0, 4).map((alert) => (
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
