import { useEffect, useState, useRef } from 'react';
import { Search, Sparkles, SlidersHorizontal, Settings, HelpCircle, Terminal } from 'lucide-react';
import { ViewType } from '../types';

interface HeaderProps {
  currentView: ViewType;
  setView: (view: ViewType) => void;
  isCapturing: boolean;
  toggleCapture: () => void;
  openAIAssistant: () => void;
}

export default function Header({
  currentView,
  setView,
  isCapturing,
  toggleCapture,
  openAIAssistant,
}: HeaderProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearchModal, setShowSearchModal] = useState(false);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Command palette hotkey (⌘K or Ctrl+K)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setShowSearchModal((prev) => !prev);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Autofocus search input when modal opens
  useEffect(() => {
    if (showSearchModal && searchInputRef.current) {
      setTimeout(() => searchInputRef.current?.focus(), 50);
    }
  }, [showSearchModal]);

  const handleQuickCommand = (view: ViewType) => {
    setView(view);
    setShowSearchModal(false);
  };

  const navTitles: Record<ViewType, string> = {
    dashboard: 'Resumen de Red',
    captura: 'Control de Captura',
    filtros: 'Constructor de Filtros',
    alertas: 'Alertas de Seguridad',
    topologia: 'Topología de Red 2D',
    vulnerabilidades: 'Vulnerabilidades Detectadas',
    asistente: 'Asistente IA ViewRed',
    configuracion: 'Configuración del Sistema',
  };

  const navDescriptions: Record<ViewType, string> = {
    dashboard: 'Estado global de captura, throughput y análisis automático en tiempo real.',
    captura: 'Tráfico en interfaces de hardware, almacenamiento de PCAPs y políticas de retención.',
    filtros: 'Detección profunda de paquetes y mapeo hexadecimal con constructores de expresiones.',
    alertas: 'Disparadores e incidencias críticas por firmas Suricata IDS y algoritmos estocásticos.',
    topologia: 'Grafo de conectividad activa, detección de VLANs secundarias y auditoría de riesgos.',
    vulnerabilidades: 'Escaneo cibernético de endpoints mediante integraciones de binarios Nmap/NSE.',
    asistente: 'Modelado local LLM y explicaciones contextuales de vulnerabilidades detectadas.',
    configuracion: 'Parámetros del colector pcap, variables del host, rutas y umbrales de seguridad.',
  };

  return (
    <>
      <header
        id="app-header-bar"
        className="h-14 bg-[#131820] border-b border-[#2a3242] sticky top-0 z-40 px-6 flex items-center justify-between"
      >
        {/* Title Group */}
        <div className="flex flex-col text-left">
          <h1 className="font-sans font-bold text-[#e4e7eb] text-base leading-none tracking-tight">
            {navTitles[currentView]}
          </h1>
          <span className="font-sans text-[11px] text-[#9ba3b4] mt-1 hidden sm:inline">
            {navDescriptions[currentView]}
          </span>
        </div>

        {/* Global actions group */}
        <div className="flex items-center gap-4">
          {/* Quick command search input bar */}
          <div className="relative hidden md:block select-none">
            <button
              onClick={() => setShowSearchModal(true)}
              className="w-64 bg-[#0a0e14] border border-[#2a3242] hover:border-[#3a4356] text-[#9ba3b4] hover:text-[#e4e7eb] rounded-md py-1.5 pl-3 pr-10 text-xs font-sans text-left flex items-center justify-between transition-all"
            >
              <span className="flex items-center gap-2">
                <Search className="h-3.5 w-3.5 text-[#5c6473]" />
                <span>Buscar IPs, protocolos, CVEs...</span>
              </span>
              <kbd className="bg-[#1c2230] text-[9px] font-mono px-1.5 py-0.5 rounded text-[#5c6473] border border-[#2a3242]">
                ⌘K
              </kbd>
            </button>
          </div>

          {/* Connection / Capture status badge */}
          <button
            onClick={toggleCapture}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-full border text-[11px] font-mono font-medium tracking-wide transition-all ${
              isCapturing
                ? 'bg-[#10b981]/10 text-[#10b981] border-[#10b981]/30 hover:bg-[#10b981]/20'
                : 'bg-red-500/10 text-red-500 border-red-500/30 hover:bg-red-500/20'
            }`}
          >
            <span
              className={`w-1.5 h-1.5 rounded-full ${
                isCapturing ? 'bg-[#10b981] animate-ping' : 'bg-red-500'
              }`}
            />
            <span>{isCapturing ? 'CAPTURA ACTIVA' : 'PCAP DETENIDO'}</span>
          </button>

          {/* Sparkles AI button */}
          <button
            onClick={openAIAssistant}
            className={`p-2 rounded-md transition-all relative ${
              currentView === 'asistente'
                ? 'bg-[#3b82f6]/20 text-[#3b82f6]'
                : 'text-[#9ba3b4] hover:text-[#e4e7eb] hover:bg-[#1c2230]'
            }`}
            title="Soporte Inteligente ViewRed"
          >
            <Sparkles className="h-4 w-4" />
            <span className="absolute top-1 right-1 w-1.5 h-1.5 bg-[#3b82f6] rounded-full ring-2 ring-[#131820]" />
          </button>

          {/* Quick Config button */}
          <button
            onClick={() => setView('configuracion')}
            className={`p-2 rounded-md transition-all ${
              currentView === 'configuracion'
                ? 'bg-[#1c2230] text-[#e4e7eb]'
                : 'text-[#9ba3b4] hover:text-[#e4e7eb] hover:bg-[#1c2230]'
            }`}
            title="Ajustes de Red"
          >
            <Settings className="h-4 w-4" />
          </button>

          {/* Small User Info / Avatar */}
          <div className="flex items-center gap-2 border-l border-[#2a3242] pl-3.5">
            <div className="w-7 h-7 bg-[#1c2230] hover:bg-[#2a3242] text-[#3b82f6] ring-1 ring-[#2a3242] flex items-center justify-center font-bold text-xs rounded-full cursor-pointer select-none">
              OP
            </div>
            <div className="hidden lg:flex flex-col text-left">
              <span className="font-sans font-semibold text-xs text-[#e4e7eb]">SecOps Admin</span>
              <span className="font-sans text-[10px] text-[#5c6473]">Nivel Administrador</span>
            </div>
          </div>
        </div>
      </header>

      {/* ⌘K Command Palette Modal Overlay */}
      {showSearchModal && (
        <div className="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex items-start justify-center pt-24 px-4 font-sans">
          <div className="bg-[#131820] w-full max-w-lg rounded-lg border border-[#3a4356] shadow-2xl shadow-black/80 overflow-hidden flex flex-col">
            {/* Modal Input */}
            <div className="p-3.5 border-b border-[#2a3242] flex items-center gap-2.5">
              <Search className="h-4 w-4 text-[#9ba3b4]" />
              <input
                ref={searchInputRef}
                type="text"
                placeholder="Escribe un comando, IP, o sección..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="bg-transparent border-0 outline-none ring-0 w-full text-sm text-[#e4e7eb] placeholder-[#5c6473]"
              />
              <button
                onClick={() => setShowSearchModal(false)}
                className="text-[#9ba3b4] hover:text-[#e4e7eb] text-xs font-mono px-2 py-0.5 border border-[#2a3242] rounded-md bg-[#0a0e14]"
              >
                ESC
              </button>
            </div>

            {/* Quick Actions List */}
            <div className="p-2 max-h-72 overflow-y-auto">
              <div className="text-[10px] font-mono font-medium text-[#5c6473] px-3.5 py-1.5 uppercase tracking-wider">
                Navegación Rápida de Secciones
              </div>

              {[
                { label: 'Ir al Dashboard de Control', view: 'dashboard' as ViewType, desc: 'Métricas, throughput y resumen de amenazas.' },
                { label: 'Abrir Captura en Vivo (pcap)', view: 'captura' as ViewType, desc: 'Ver interfaces activas, rotación y buffers.' },
                { label: 'Constructor de Filtros Hex', view: 'filtros' as ViewType, desc: 'Búsquedas booleanas de payload hexadecimal.' },
                { label: 'Stream de Alertas IDS', view: 'alertas' as ViewType, desc: 'Ver alertas críticas emitidas por Suricata.' },
                { label: 'Auditoría Topológica 2D', view: 'topologia' as ViewType, desc: 'Interactuar con el grafo de conectividad de red.' },
                { label: 'Vulnerabilidades y Binarios', view: 'vulnerabilidades' as ViewType, desc: 'Configurar escaneos Nmap de largo alcance.' },
                { label: 'Chat con Asistente IA', view: 'asistente' as ViewType, desc: 'Explicación interactiva sobre malware a nivel de red.' },
              ]
                .filter(item => item.label.toLowerCase().includes(searchQuery.toLowerCase()) || searchQuery === '')
                .map((item) => (
                  <button
                    key={item.view}
                    onClick={() => handleQuickCommand(item.view)}
                    className="w-full text-left px-3.5 py-2 hover:bg-[#1c2230] rounded-md flex items-center gap-3 transition-colors group"
                  >
                    <Terminal className="h-3.5 w-3.5 text-[#5c6473] group-hover:text-[#3b82f6]" />
                    <div className="flex flex-col">
                      <span className="text-xs text-[#e4e7eb] font-medium">{item.label}</span>
                      <span className="text-[10px] text-[#5c6473] mt-0.5">{item.desc}</span>
                    </div>
                  </button>
                ))}

              {/* IP / Custom Queries */}
              {searchQuery && (
                <div id="search-results-query" className="border-t border-[#2a3242] mt-2 pt-2">
                  <div className="text-[10px] font-mono font-medium text-[#5c6473] px-3.5 py-1.5 uppercase">
                    Búsqueda de Datos
                  </div>
                  <button
                    onClick={() => {
                      setView('filtros');
                      setShowSearchModal(false);
                    }}
                    className="w-full text-left px-3.5 py-2 hover:bg-[#1c2230] rounded-md flex items-center gap-3 text-xs text-[#e4e7eb]"
                  >
                    <SlidersHorizontal className="h-3.5 w-3.5 text-[#3b82f6]" />
                    <span>Filtrar red para consulta: <strong className="text-[#3b82f6] font-mono">"{searchQuery}"</strong></span>
                  </button>
                </div>
              )}
            </div>
            <div className="bg-[#0a0e14] px-4 py-2 border-t border-[#2a3242] text-[10px] text-[#5c6473] flex justify-between items-center font-mono">
              <span>Usa ↑ ↓ para navegar, Enter para seleccionar</span>
              <span>ViewRed v0.2.1-beta</span>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
