import { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Radio, Search, Sparkles, SlidersHorizontal, Settings, Terminal } from 'lucide-react';
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
  const navigate = useNavigate();
  const [selectedIndex, setSelectedIndex] = useState(0);

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

  const quickCommands = [
    { label: 'Ir al Dashboard de Control', view: 'dashboard' as ViewType, desc: 'Métricas, throughput y resumen de amenazas.' },
    { label: 'Abrir Captura en Vivo (pcap)', view: 'captura' as ViewType, desc: 'Ver interfaces activas, rotación y buffers.' },
    { label: 'Constructor de Filtros Hex', view: 'filtros' as ViewType, desc: 'Búsquedas booleanas de payload hexadecimal.' },
    { label: 'Stream de Alertas IDS', view: 'alertas' as ViewType, desc: 'Ver alertas críticas emitidas por Suricata.' },
    { label: 'Auditoría Topológica 2D', view: 'topologia' as ViewType, desc: 'Interactuar con el grafo de conectividad de red.' },
    { label: 'Vulnerabilidades y Binarios', view: 'vulnerabilidades' as ViewType, desc: 'Configurar escaneos Nmap de largo alcance.' },
    { label: 'Chat con Asistente IA', view: 'asistente' as ViewType, desc: 'Explicación interactiva sobre malware a nivel de red.' },
  ];

  const filteredCommands = quickCommands.filter(item => item.label.toLowerCase().includes(searchQuery.toLowerCase()) || searchQuery === '');
  const totalItems = filteredCommands.length + (searchQuery ? 1 : 0);

  useEffect(() => {
    setSelectedIndex(0);
  }, [searchQuery]);

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex((prev) => (prev + 1) % totalItems);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex((prev) => (prev - 1 + totalItems) % totalItems);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (selectedIndex < filteredCommands.length) {
        handleQuickCommand(filteredCommands[selectedIndex].view);
      } else if (searchQuery) {
        setShowSearchModal(false);
        navigate('/filtros', { state: { query: searchQuery } });
      }
    }
  };

  return (
    <>
      <header
        id="app-header-bar"
        className="h-14 bg-[#131820] border-b border-[#2a3242] sticky top-0 z-40 px-6 flex items-center justify-between gap-4"
      >
        {/* Left spacer to perfectly center the search bar */}
        <div className="flex-1 hidden md:block"></div>

        {/* Quick command search input bar */}
        <div className="relative hidden md:block select-none w-full max-w-2xl">
          <button
            onClick={() => setShowSearchModal(true)}
            className="w-full bg-[#0a0e14] border border-[#2a3242] hover:border-[#3a4356] text-[#9ba3b4] hover:text-[#e4e7eb] rounded-md py-1.5 pl-3 pr-10 text-xs font-sans text-left flex items-center justify-between transition-all mx-auto"
          >
              <span className="flex items-center gap-2">
                <Search className="h-3.5 w-3.5 text-[#5c6473]" />
                <span>Buscar IPs, protocolos, CVEs...</span>
              </span>
              <kbd className="bg-[#1c2230] text-[9px] font-mono px-1.5 py-0.5 rounded text-[#5c6473] border border-[#2a3242]">
                {navigator.platform.toLowerCase().includes('mac') ? '⌘K' : 'Ctrl+K'}
              </kbd>
            </button>
        </div>

        {/* Global actions group */}
        <div className="flex items-center justify-end gap-4 flex-1">
          {/* Connection / Capture status badge */}
          <button
            onClick={toggleCapture}
            className={`group relative flex h-9 items-center gap-2 overflow-hidden rounded-lg border px-3 font-mono text-xs font-bold tracking-wide transition-all ${
              isCapturing
                ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-300 shadow-[0_0_22px_rgba(16,185,129,0.08)] hover:border-emerald-300/45 hover:bg-emerald-500/15'
                : 'border-red-400/30 bg-red-500/10 text-red-300 shadow-[0_0_22px_rgba(239,68,68,0.08)] hover:border-red-300/45 hover:bg-red-500/15'
            }`}
            title={isCapturing ? 'Pausar captura' : 'Reanudar captura'}
          >
            <span
              className={`absolute inset-x-0 bottom-0 h-px ${
                isCapturing ? 'bg-emerald-300/70' : 'bg-red-300/70'
              }`}
            />
            <span
              className={`flex h-6 w-6 items-center justify-center rounded-md border ${
                isCapturing
                  ? 'border-emerald-400/25 bg-emerald-500/10 text-emerald-300'
                  : 'border-red-400/25 bg-red-500/10 text-red-300'
              }`}
            >
              <Radio className="h-3.5 w-3.5" />
            </span>
            <span className="relative flex h-2.5 w-2.5 items-center justify-center">
              {isCapturing && (
                <span className="absolute inline-flex h-4 w-4 animate-ping rounded-full bg-emerald-400 opacity-45" />
              )}
              <span className={`relative inline-flex h-2 w-2 rounded-full ${isCapturing ? 'bg-emerald-300' : 'bg-red-300'}`} />
            </span>
            <span>{isCapturing ? 'LIVE CAPTURE' : 'CAPTURE OFF'}</span>
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

          {/* Reserved: user avatar area */}
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
                onKeyDown={handleKeyDown}
                className="w-full bg-transparent border-none outline-none text-[#e4e7eb] placeholder-[#5c6473] font-sans"
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

              {filteredCommands.map((item, index) => (
                  <button
                    key={item.view}
                    onClick={() => handleQuickCommand(item.view)}
                    className={`w-full text-left px-3.5 py-2 rounded-md flex items-center gap-3 transition-colors group ${
                      selectedIndex === index ? 'bg-[#1c2230]' : 'hover:bg-[#1c2230]'
                    }`}
                  >
                    <Terminal className={`h-3.5 w-3.5 ${selectedIndex === index ? 'text-[#3b82f6]' : 'text-[#5c6473] group-hover:text-[#3b82f6]'}`} />
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
                      setShowSearchModal(false);
                      navigate('/filtros', { state: { query: searchQuery } });
                    }}
                    className={`w-full text-left px-3.5 py-2 rounded-md flex items-center gap-3 text-xs text-[#e4e7eb] ${
                      selectedIndex === filteredCommands.length ? 'bg-[#1c2230]' : 'hover:bg-[#1c2230]'
                    }`}
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
