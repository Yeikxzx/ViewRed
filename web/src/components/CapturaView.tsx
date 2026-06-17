import { useState, useEffect } from 'react';
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
import { storageHistory } from '../data/mockData';

interface CapturaViewProps {
  isCapturing: boolean;
  toggleCapture: () => void;
}

export default function CapturaView({ isCapturing, toggleCapture }: CapturaViewProps) {
  const [selectedInterface, setSelectedInterface] = useState('eth0 (Activa)');
  const [bpfFilter, setBpfFilter] = useState('tcp port 80 or tcp port 443');
  const [snaplen, setSnaplen] = useState(65535);
  const [rotSize, setRotSize] = useState('500 MB');
  const [retention, setRetention] = useState('7 Días');
  const [showAppliedToast, setShowAppliedToast] = useState(false);

  // Stats Counters
  const [packetsTotal, setPacketsTotal] = useState(14234500);
  const [rate, setRate] = useState(1432);
  const [capturedBytes, setCapturedBytes] = useState(4.2);

  const interfacesList = [
    'eth0 (Activa)',
    'eth1 (Desconectada)',
    'wlan0 (Monitoreo)',
    'lo (Loopback)',
  ];

  useEffect(() => {
    let interval: NodeJS.Timeout;
    if (isCapturing) {
      interval = setInterval(() => {
        setPacketsTotal((prev) => prev + Math.floor(Math.random() * 30) + 10);
        setRate(() => Math.floor(Math.random() * 150) + 1350);
        setCapturedBytes((prev) => parseFloat((prev + 0.0001).toFixed(4)));
      }, 1000);
    }
    return () => clearInterval(interval);
  }, [isCapturing]);

  const handleApplyChanges = () => {
    setShowAppliedToast(true);
    setTimeout(() => setShowAppliedToast(false), 2500);
  };

  const handleClearBuffer = () => {
    if (confirm('¿Está absolutamente seguro de limpiar el búfer actual de paquetes capturados en el disco? Esta acción borrará todos los PCAPs temporales.')) {
      setPacketsTotal(0);
      setRate(0);
      setCapturedBytes(0);
    }
  };

  return (
    <div id="captura-view-wrapper" className="space-y-6">
      {/* Top Capture Control Status Card */}
      <div className="bg-[#131820] border border-[#2a3242] p-5 rounded-lg text-left">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-[#2a3242] pb-4 mb-4">
          <div className="flex items-center gap-3">
            <span
              className={`w-3.5 h-3.5 rounded-full ${
                isCapturing ? 'bg-[#10b981] animate-ping' : 'bg-red-500'
              }`}
            />
            <div className="flex flex-col">
              <span className="font-sans font-bold text-sm text-[#e4e7eb]">
                {isCapturing ? 'Captura Activa' : 'Captura Pausada'}
              </span>
              <span className="font-mono text-[11px] text-[#5c6473]">
                Interfaz: {selectedInterface.split(' ')[0]} • Iniciada hace 02:45:12
              </span>
            </div>
          </div>
          <div className="flex gap-2">
            <button
              id="pause-resume-btn"
              onClick={toggleCapture}
              className={`flex items-center gap-2 px-3 .5 py-1.5 rounded text-xs font-mono border transition-all ${
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
              className="flex items-center gap-2 px-3.5 py-1.5 rounded text-xs font-mono border border-red-500/30 bg-red-500/10 text-red-500 hover:bg-red-500/20 transition-all"
            >
              <StopCircle className="h-3.5 w-3.5" />
              <span>Detener</span>
            </button>
          </div>
        </div>

        {/* Counters Block */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mt-2">
          <div className="bg-[#0a0e14] border border-[#2a3242]/40 p-4 rounded text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Paquetes Totales
            </span>
            <span className="text-xl font-mono font-bold text-[#e4e7eb] block mt-1">
              {(packetsTotal / 1000000).toFixed(1)}M
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              {packetsTotal.toLocaleString('es-ES')} pkts
            </span>
          </div>
          <div className="bg-[#0a0e14] border border-[#2a3242]/40 p-4 rounded text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Tasa (pkts/s)
            </span>
            <span className="text-xl font-mono font-bold text-[#3b82f6] block mt-1">
              {rate.toLocaleString('es-ES')}
            </span>
            <span className="text-[9px] font-mono text-[#10b981] block mt-0.5">
              ▲ 12.1% en cola
            </span>
          </div>
          <div className="bg-[#0a0e14] border border-[#2a3242]/40 p-4 rounded text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Datos Capturados
            </span>
            <span className="text-xl font-mono font-bold text-[#e4e7eb] block mt-1">
              {capturedBytes.toFixed(1)} GB
            </span>
            <span className="text-[9px] font-mono text-[#5c6473] block mt-0.5">
              Compresión Gzip activa
            </span>
          </div>
          <div className="bg-[#0a0e14] border border-[#2a3242]/40 p-4 rounded text-left">
            <span className="text-[10px] font-mono text-[#5c6473] font-bold block uppercase tracking-wider">
              Descartados
            </span>
            <span className="text-xl font-mono font-bold text-amber-500 block mt-1">12</span>
            <span className="text-[9px] font-mono text-amber-500 block mt-0.5">(0.001%)</span>
          </div>
        </div>
      </div>

      {/* Split configurations and statistics */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left column: Storage progression area chart */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between lg:col-span-2 text-left">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-3 mb-4">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
              Uso de Almacenamiento PCAPs (Últimas 24h)
            </h2>
            <HardDrive className="h-4 w-4 text-[#5c6473]" />
          </div>

          <div className="h-52 w-full font-mono text-xs">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={storageHistory} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="storageColor" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2} />
                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
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
                  dataKey="used"
                  stroke="#3b82f6"
                  strokeWidth={2}
                  fillOpacity={1}
                  fill="url(#storageColor)"
                  name="Espacio (MB)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          <div className="flex justify-between items-center border-t border-[#1c2230] pt-3 text-[10px] font-mono text-[#5c6473]">
            <span>ESPACIO CONSUMIDO POR PCAPs</span>
            <span>CAPACIDAD MÁXIMA ASIGNADA: 500 GB</span>
          </div>
        </div>

        {/* Right column: Config settings side card */}
        <div className="space-y-4 text-left">
          {/* Form Capture Config */}
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg space-y-4">
            <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5">
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
                className="w-full bg-[#0a0e14] border border-[#2a3242] rounded px-3 py-1.5 text-xs text-[#e4e7eb] outline-none font-mono py-1.5 disabled:opacity-60 cursor-pointer"
              >
                {interfacesList.map((iface) => (
                  <option key={iface} value={iface}>
                    {iface}
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
                className="w-full bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] rounded px-3 py-1.5 text-xs text-[#e4e7eb] outline-none font-mono"
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
                className="w-full bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] rounded px-3 py-1.5 text-xs text-[#e4e7eb] outline-none font-mono"
              />
              <span className="text-[9px] text-[#5c6473] font-mono block">
                Tamaño máximo de paquete a capturar.
              </span>
            </div>

            {/* Save Buttons */}
            <button
              onClick={handleApplyChanges}
              className="w-full bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white py-1.5 rounded text-xs font-mono font-bold flex items-center justify-center gap-2 transition-all cursor-pointer"
            >
              <span>Aplicar Cambios</span>
            </button>
          </div>

          {/* Form Storage policy */}
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg space-y-4">
            <div className="flex items-center gap-2 border-b border-[#2a3242] pb-2.5">
              <Database className="h-4 w-4 text-[#3b82f6]" />
              <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                Políticas de Almacenamiento
              </h2>
            </div>

            {/* File rotation slider */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">Rotar archivo cada</span>
                <span className="text-[#e4e7eb] font-bold">{rotSize}</span>
              </div>
              <input
                type="range"
                min="100"
                max="1000"
                step="50"
                value={parseInt(rotSize)}
                onChange={(e) => setRotSize(`${e.target.value} MB`)}
                className="w-full h-1.5 bg-[#0a0e14] rounded-lg appearance-none cursor-pointer"
              />
            </div>

            {/* Max Retention slider */}
            <div className="space-y-1">
              <div className="flex justify-between text-[11px] font-mono">
                <span className="text-[#9ba3b4]">Retención Máxima</span>
                <span className="text-[#e4e7eb] font-bold">{retention}</span>
              </div>
              <input
                type="range"
                min="1"
                max="30"
                step="1"
                value={parseInt(retention)}
                onChange={(e) => setRetention(`${e.target.value} Días`)}
                className="w-full h-1.5 bg-[#0a0e14] rounded-lg appearance-none cursor-pointer"
              />
            </div>

            <div className="flex items-start gap-2 text-[10px] text-[#5c6473] font-mono border-t border-[#1c2230] pt-2 mt-1">
              <AlertTriangle className="h-3 w-3 text-amber-500 shrink-0 mt-0.5" />
              <span>
                Los archivos más antiguos serán deligados de forma automática cuando el disco supere
                el 90% de capacidad asignada.
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Applied toast popup */}
      {showAppliedToast && (
        <div className="fixed bottom-6 right-6 z-50 bg-[#10b981] text-white px-4 py-2.5 rounded shadow-xl shadow-black/60 font-sans text-xs font-semibold flex items-center gap-2 animate-bounce">
          <Check className="h-4 w-4" />
          <span>Configuración de Captura y BPF guardada con éxito.</span>
        </div>
      )}
    </div>
  );
}
