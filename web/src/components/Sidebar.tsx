import { useEffect, useState } from 'react';
import { NavLink } from 'react-router-dom';
import { getSystemMetrics } from '../lib/api';
import {
  LayoutDashboard,
  Radio,
  Filter,
  Bell,
  Network,
  ShieldAlert,
  Sparkles,
  Settings,
  ChevronLeft,
  ChevronRight,
  Cpu,
  Activity,
  Clock,
  Archive,
} from 'lucide-react';

interface SidebarProps {
  collapsed: boolean;
  setCollapsed: (collapsed: boolean) => void;
  activeAlertCount: number;
}

interface MenuItem {
  id: string;
  path: string;
  label: string;
  icon: typeof LayoutDashboard;
  badge?: number;
  section?: 'monitor' | 'analysis' | 'system';
}

export default function Sidebar({
  collapsed,
  setCollapsed,
  activeAlertCount,
}: SidebarProps) {
  const [cpuUsage, setCpuUsage] = useState<number>(0);
  const [ramUsage, setRamUsage] = useState<number>(0);
  const [uptimeSecs, setUptimeSecs] = useState<number>(0);

  // Live telemetry — real data from backend
  useEffect(() => {
    const fetchMetrics = async () => {
      try {
        const m = await getSystemMetrics();
        setCpuUsage(m.cpu_percent);
        setRamUsage(m.ram_percent);
        setUptimeSecs(m.uptime_seconds);
      } catch {
        // keep last known values
      }
    };
    fetchMetrics();
    const timer = setInterval(fetchMetrics, 5000);
    return () => clearInterval(timer);
  }, []);

  const menuItems: MenuItem[] = [
    { id: 'dashboard', path: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, section: 'monitor' },
    { id: 'captura', path: '/captura', label: 'Captura', icon: Radio, section: 'monitor' },
    { id: 'historial', path: '/historial', label: 'Historial', icon: Archive, section: 'monitor' },
    { id: 'filtros', path: '/filtros', label: 'Filtros', icon: Filter, section: 'analysis' },
    {
      id: 'alertas',
      path: '/alertas',
      label: 'Alertas',
      icon: Bell,
      badge: activeAlertCount > 0 ? activeAlertCount : undefined,
      section: 'analysis',
    },
    { id: 'topologia', path: '/topologia', label: 'Topología', icon: Network, section: 'analysis' },
    { id: 'vulnerabilidades', path: '/vulnerabilidades', label: 'Vulnerabilidades', icon: ShieldAlert, section: 'analysis' },
    { id: 'asistente', path: '/asistente', label: 'Asistente', icon: Sparkles, section: 'system' },
    { id: 'configuracion', path: '/configuracion', label: 'Configuración', icon: Settings, section: 'system' },
  ];

  const sections: { key: string; label: string; items: MenuItem[] }[] = [
    { key: 'monitor', label: 'Monitoreo', items: menuItems.filter((i) => i.section === 'monitor') },
    { key: 'analysis', label: 'Análisis', items: menuItems.filter((i) => i.section === 'analysis') },
    { key: 'system', label: 'Sistema', items: menuItems.filter((i) => i.section === 'system') },
  ];

  // Color logic for system metrics — actual hex colors for SVG strokes
  const cpuStroke = cpuUsage > 80 ? '#ef4444' : cpuUsage > 50 ? '#f59e0b' : '#10b981';
  const ramStroke = ramUsage > 85 ? '#ef4444' : ramUsage > 60 ? '#3b82f6' : '#06b6d4';
  const cpuGlow = cpuUsage > 80 ? 'rgba(239,68,68,0.35)' : cpuUsage > 50 ? 'rgba(245,158,11,0.25)' : 'rgba(16,185,129,0.25)';
  const ramGlow = ramUsage > 85 ? 'rgba(239,68,68,0.35)' : ramUsage > 60 ? 'rgba(59,130,246,0.3)' : 'rgba(6,182,212,0.25)';

  const formatUptime = (secs: number) => {
    const h = Math.floor(secs / 3600);
    const m = Math.floor(secs / 60) % 60;
    const s = secs % 60;
    if (h > 0) return `${h}h ${m.toString().padStart(2, '0')}m`;
    return `${m.toString().padStart(2, '0')}m ${s.toString().padStart(2, '0')}s`;
  };

  // SVG ring gauge component for the system panel
  const RingGauge = ({
    value,
    size = 54,
    strokeWidth = 4,
    strokeColor,
    glowColor,
    icon: Icon,
    label,
  }: {
    value: number;
    size?: number;
    strokeWidth?: number;
    strokeColor: string;
    glowColor: string;
    icon: typeof Cpu;
    label: string;
  }) => {
    const radius = (size - strokeWidth) / 2;
    const circumference = 2 * Math.PI * radius;
    const offset = circumference - (Math.min(value, 100) / 100) * circumference;

    return (
      <div className="flex flex-col items-center gap-1.5">
        <div className="relative" style={{ width: size, height: size }}>
          <svg width={size} height={size} className="-rotate-90">
            {/* Background track */}
            <circle
              cx={size / 2}
              cy={size / 2}
              r={radius}
              fill="none"
              stroke="#1c2230"
              strokeWidth={strokeWidth}
            />
            {/* Value arc */}
            <circle
              cx={size / 2}
              cy={size / 2}
              r={radius}
              fill="none"
              stroke={strokeColor}
              strokeWidth={strokeWidth}
              strokeLinecap="round"
              strokeDasharray={circumference}
              strokeDashoffset={offset}
              style={{
                transition: 'stroke-dashoffset 1.2s cubic-bezier(0.4, 0, 0.2, 1)',
                filter: `drop-shadow(0 0 4px ${glowColor})`,
              }}
            />
          </svg>
          {/* Center icon */}
          <div className="absolute inset-0 flex items-center justify-center">
            <Icon className="h-3.5 w-3.5 text-[#9ba3b4]" />
          </div>
        </div>
        <span className="text-[10px] font-mono font-bold text-[#e4e7eb]">{value}%</span>
        <span className="text-[8px] font-mono uppercase tracking-widest text-[#5c6473]">{label}</span>
      </div>
    );
  };

  return (
    <aside
      id="sidebar-shell"
      className={`bg-[#0a0e14] border-r border-[#1c2230]/80 flex flex-col justify-between transition-all duration-300 select-none ${
        collapsed ? 'w-[60px]' : 'w-[220px]'
      }`}
    >
      <div className="flex flex-col h-full">
        {/* Brand Header */}
        <div
          id="brand-header"
          className="h-14 flex items-center px-3 justify-between gap-2 overflow-hidden"
        >
          <div className="flex items-center gap-2 min-w-0">
            {/* Brand mark — always visible */}
            <div className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0">
              <img src="/logo.png" alt="ViewRed Logo" className="w-full h-full object-contain" />
            </div>
            {!collapsed && (
              <div className="flex flex-col min-w-0">
                <span className="font-bold tracking-wide text-[15px] text-[#e4e7eb] leading-tight">
                  ViewRed
                </span>
                <span className="text-[8px] font-mono uppercase tracking-[0.15em] text-[#5c6473] leading-tight">
                  SecOps Console
                </span>
              </div>
            )}
          </div>
          <button
            id="toggle-sidebar-btn"
            onClick={() => setCollapsed(!collapsed)}
            className="text-[#5c6473] hover:text-[#e4e7eb] rounded-md p-1 hover:bg-[#1c2230] transition-colors flex-shrink-0"
          >
            {collapsed ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronLeft className="h-3.5 w-3.5" />}
          </button>
        </div>

        {/* Subtle separator */}
        <div className="mx-3 h-px bg-gradient-to-r from-transparent via-[#2a3242]/60 to-transparent" />

        {/* Navigation Menu Links — sectioned */}
        <nav id="sidebar-nav-links" className="mt-3 px-2 flex-1 overflow-y-auto space-y-4">
          {sections.map((section) => (
            <div key={section.key}>
              {/* Section label — hidden when collapsed */}
              {!collapsed && (
                <span className="text-[9px] font-mono font-semibold uppercase tracking-[0.2em] text-[#3b82f6]/60 px-2 mb-1.5 block">
                  {section.label}
                </span>
              )}
              {collapsed && <div className="h-px mx-2 bg-[#1c2230] mb-1.5" />}
              <div className="space-y-0.5">
                {section.items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.id}
                      id={`nav-${item.id}`}
                      to={item.path}
                      className={({ isActive }) =>
                        `w-full flex items-center ${collapsed ? 'justify-center' : ''} gap-2.5 py-[7px] px-2.5 rounded-lg transition-all duration-200 text-left relative group ${
                          isActive
                            ? 'bg-[#3b82f6]/10 text-[#e4e7eb]'
                            : 'text-[#6b7280] hover:text-[#e4e7eb] hover:bg-[#ffffff]/[0.03]'
                        }`
                      }
                    >
                      {({ isActive }) => (
                        <>
                          {/* Active indicator bar */}
                          {isActive && (
                            <div className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 bg-[#3b82f6] rounded-r-full shadow-[0_0_8px_rgba(59,130,246,0.5)]" />
                          )}
                          <Icon
                            className={`h-[18px] w-[18px] flex-shrink-0 transition-colors duration-200 ${
                              isActive ? 'text-[#3b82f6]' : 'group-hover:text-[#9ba3b4]'
                            }`}
                          />
                          {!collapsed && (
                            <span className={`text-[12px] tracking-wide ${isActive ? 'font-semibold' : 'font-medium'}`}>
                              {item.label}
                            </span>
                          )}
                          {item.badge && !collapsed && (
                            <span className="ml-auto bg-red-500/90 text-[9px] text-white font-bold h-[18px] min-w-[18px] px-1 rounded-full flex items-center justify-center shadow-lg shadow-red-500/25">
                              {item.badge}
                            </span>
                          )}
                          {item.badge && collapsed && (
                            <span className="absolute top-1 right-1 bg-red-500 w-2 h-2 rounded-full ring-2 ring-[#0a0e14] shadow" />
                          )}
                        </>
                      )}
                    </NavLink>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </div>

      {/* System Telemetry Panel */}
      <div id="system-telemetry-panel" className="p-2.5 mt-auto">
        {collapsed ? (
          /* Collapsed: compact vertical stack */
          <div className="flex flex-col items-center gap-2 bg-[#0f1319] rounded-xl border border-[#1c2230]/60 py-3 px-1">
            <div className="relative">
              <div
                className="w-2 h-2 rounded-full bg-emerald-500"
                style={{ boxShadow: '0 0 6px rgba(16,185,129,0.6)' }}
              />
            </div>
            <Cpu className="h-3.5 w-3.5 text-[#5c6473]" />
            <span className="text-[9px] font-mono font-bold text-[#9ba3b4]">{cpuUsage}%</span>
            <div className="h-px w-4 bg-[#1c2230]" />
            <Activity className="h-3.5 w-3.5 text-[#5c6473]" />
            <span className="text-[9px] font-mono font-bold text-[#9ba3b4]">{ramUsage}%</span>
          </div>
        ) : (
          /* Expanded: ring gauge panel */
          <div className="bg-[#0f1319] rounded-xl border border-[#1c2230]/60 p-3.5 relative overflow-hidden">
            {/* Ambient glow */}
            <div
              className="absolute -bottom-10 -left-10 w-32 h-32 rounded-full blur-3xl pointer-events-none opacity-[0.04]"
              style={{ background: 'linear-gradient(135deg, #3b82f6, #6366f1)' }}
            />

            {/* Header */}
            <div className="flex items-center justify-center mb-3.5">
              <span className="text-[9px] font-mono font-bold uppercase tracking-[0.2em] text-[#5c6473]">
                System Monitor
              </span>
            </div>

            {/* Ring gauges row */}
            <div className="flex items-center justify-around">
              <RingGauge
                value={cpuUsage}
                strokeColor={cpuStroke}
                glowColor={cpuGlow}
                icon={Cpu}
                label="CPU"
              />
              <RingGauge
                value={ramUsage}
                strokeColor={ramStroke}
                glowColor={ramGlow}
                icon={Activity}
                label="RAM"
              />
            </div>

            {/* Uptime footer */}
            <div className="flex items-center justify-center gap-1.5 mt-3.5 pt-2.5 border-t border-[#1c2230]/40">
              <Clock className="h-3 w-3 text-[#3b82f6]/50" />
              <span className="text-[9px] font-mono text-[#5c6473]">
                Uptime <span className="text-[#9ba3b4] font-bold">{formatUptime(uptimeSecs)}</span>
              </span>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
}
