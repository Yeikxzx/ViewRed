import { useEffect, useState } from 'react';
import { NavLink } from 'react-router-dom';
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
  Tv,
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
}

export default function Sidebar({
  collapsed,
  setCollapsed,
  activeAlertCount,
}: SidebarProps) {
  const [cpuUsage, setCpuUsage] = useState<number>(12.4);
  const [ramUsage, setRamUsage] = useState<number>(30.2);

  // Live telemetry updates
  useEffect(() => {
    const timer = setInterval(() => {
      setCpuUsage((prev) => {
        const delta = (Math.random() - 0.5) * 4;
        return Math.max(5, Math.min(95, parseFloat((prev + delta).toFixed(1))));
      });
      setRamUsage((prev) => {
        const delta = (Math.random() - 0.5) * 0.2;
        return Math.max(28, Math.min(35, parseFloat((prev + delta).toFixed(1))));
      });
    }, 2000);
    return () => clearInterval(timer);
  }, []);

  const menuItems: MenuItem[] = [
    { id: 'dashboard', path: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { id: 'captura', path: '/captura', label: 'Captura', icon: Radio },
    { id: 'filtros', path: '/filtros', label: 'Filtros', icon: Filter },
    {
      id: 'alertas',
      path: '/alertas',
      label: 'Alertas',
      icon: Bell,
      badge: activeAlertCount > 0 ? activeAlertCount : undefined,
    },
    { id: 'topologia', path: '/topologia', label: 'Topología', icon: Network },
    { id: 'vulnerabilidades', path: '/vulnerabilidades', label: 'Vulnerabilidades', icon: ShieldAlert },
    { id: 'asistente', path: '/asistente', label: 'Asistente', icon: Sparkles },
    { id: 'configuracion', path: '/configuracion', label: 'Configuración', icon: Settings },
  ];

  // Shared link className; `isActive` is supplied by react-router-dom's
  // NavLink to drive the active-route styling without local state.
  const navLinkClass = ({ isActive }: { isActive: boolean }): string =>
    `w-full flex items-center gap-3 py-2 px-3 rounded-md transition-all duration-150 text-left relative ${
      isActive
        ? 'bg-[#1c2230] text-[#e4e7eb] font-medium border-l-2 border-[#3b82f6]'
        : 'text-[#9ba3b4] hover:text-[#e4e7eb] hover:bg-[#131820]/75'
    }`;

  const iconClass = (isActive: boolean): string =>
    `h-4.5 w-4.5 flex-shrink-0 ${isActive ? 'text-[#3b82f6]' : ''}`;

  return (
    <aside
      id="sidebar-shell"
      className={`bg-[#0a0e14] border-r border-[#2a3242] flex flex-col justify-between transition-all duration-300 select-none ${
        collapsed ? 'w-16' : 'w-60'
      }`}
    >
      <div>
        {/* Brand/Logo Section */}
        <div
          id="brand-header"
          className="h-14 border-b border-[#2a3242] flex items-center px-4 justify-between gap-3 overflow-hidden"
        >
          <div className="flex items-center gap-2.5">
            <div
              id="brand-icon-container"
              className="bg-[#3b82f6] text-white p-1.5 rounded-md flex items-center justify-center shadow-[0_0_10px_rgba(59,130,246,0.3)] animate-pulse"
            >
              <Tv className="h-4 w-4" />
            </div>
            {!collapsed && (
              <div className="flex flex-col">
                <span className="font-sans font-bold text-[#e4e7eb] tracking-tight text-sm">
                  ViewRed Admin
                </span>
                <span className="font-sans font-medium text-xs text-[#5c6473] tracking-wide uppercase">
                  Network Intelligence
                </span>
              </div>
            )}
          </div>
          <button
            id="toggle-sidebar-btn"
            onClick={() => setCollapsed(!collapsed)}
            className="text-[#9ba3b4] hover:text-[#e4e7eb] rounded-md p-1 hover:bg-[#1c2230] transition-colors"
          >
            {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </button>
        </div>

        {/* Navigation Menu Links */}
        <nav id="sidebar-nav-links" className="mt-4 px-2 space-y-1">
          {menuItems.map((item) => {
            const Icon = item.icon;
            return (
              <NavLink
                key={item.id}
                id={`nav-${item.id}`}
                to={item.path}
                className={navLinkClass}
              >
                {({ isActive }) => (
                  <>
                    <Icon className={iconClass(isActive)} />
                    {!collapsed && (
                      <span className="font-sans text-xs tracking-wide">{item.label}</span>
                    )}
                    {item.badge && !collapsed && (
                      <span className="ml-auto bg-red-500 font-sans text-[10px] text-white font-bold h-4 px-1.5 rounded-full flex items-center justify-center shadow-lg shadow-red-500/30 animate-pulse">
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
        </nav>
      </div>

      {/* System Host Status Telemetry Box */}
      <div id="system-telemetry-panel" className="border-t border-[#1c2230] p-3 text-left">
        {collapsed ? (
          <div className="flex flex-col items-center gap-3 text-[#5c6473]">
            <Cpu className="h-4 w-4 animate-spin-slow" />
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex justify-between items-center text-[10px] text-[#5c6473] font-mono uppercase tracking-wider">
              <span>HOST: DEBIAN-SRV</span>
              <span className="text-[#10b981]">🟢 STABLE</span>
            </div>
            {/* CPU Bar */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">CPU</span>
                <span className="text-[#e4e7eb]">{cpuUsage}%</span>
              </div>
              <div className="w-full h-1 bg-[#131820] rounded-full overflow-hidden">
                <div
                  className="h-full bg-[#10b981] transition-all duration-1000"
                  style={{ width: `${cpuUsage}%` }}
                />
              </div>
            </div>
            {/* RAM Bar */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">RAM</span>
                <span className="text-[#e4e7eb]">{ramUsage}%</span>
              </div>
              <div className="w-full h-1 bg-[#131820] rounded-full overflow-hidden">
                <div
                  className="h-full bg-[#3b82f6] transition-all duration-1000"
                  style={{ width: `${ramUsage}%` }}
                />
              </div>
            </div>
            {/* Extra Host info */}
            <div className="text-[10px] font-mono text-[#5c6473] pt-1">
              <span>UPTIME: 02:45:12</span>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
}
