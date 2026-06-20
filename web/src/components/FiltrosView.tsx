import { useState, useEffect } from 'react';
import {
  Filter,
  Search,
  Check,
  Copy,
  ChevronRight,
  HelpCircle,
} from 'lucide-react';
import { CapturedPacket } from '../types';
import { getPackets, PacketFilters } from '../lib/api';

// Utility to convert base64 to hex and ascii dumps
function decodePayload(base64Str: string | null) {
  if (!base64Str) return { hexDump: '', asciiDump: '' };
  try {
    const rawStr = atob(base64Str);
    const bytes = new Uint8Array(rawStr.length);
    for (let i = 0; i < rawStr.length; i++) {
      bytes[i] = rawStr.charCodeAt(i);
    }
    
    let hexDump = '';
    let asciiDump = '';
    for (let i = 0; i < bytes.length; i += 16) {
      const chunk = bytes.slice(i, i + 16);
      
      const hexChunk = Array.from(chunk)
        .map(b => b.toString(16).padStart(2, '0'))
        .join(' ');
        
      const asciiChunk = Array.from(chunk)
        .map(b => (b >= 32 && b <= 126) ? String.fromCharCode(b) : '.')
        .join('');
        
      hexDump += '00 ' + hexChunk.padEnd(48, ' ') + '\n';
      asciiDump += asciiChunk + '\n';
    }
    return { hexDump: hexDump.trimEnd(), asciiDump: asciiDump.trimEnd() };
  } catch(e) {
    return { hexDump: 'Error decoding payload', asciiDump: '' };
  }
}

export default function FiltrosView() {
  const [filterType, setFilterType] = useState('ip.src');
  const [filterOperator, setFilterOperator] = useState('==');
  const [filterValue, setFilterValue] = useState('');
  const [payloadSearch, setPayloadSearch] = useState('');
  const [activeFilterString, setActiveFilterString] = useState('');

  const [packetsQueue, setPacketsQueue] = useState<CapturedPacket[]>([]);
  const [selectedPacket, setSelectedPacket] = useState<CapturedPacket | null>(null);
  const [copiedHex, setCopiedHex] = useState(false);
  const [totalItems, setTotalItems] = useState(0);

  const filterFields = [
    { value: 'ip.src', label: 'IP Origen' },
    { value: 'ip.dst', label: 'IP Destino' },
    { value: 'tcp.port', label: 'Puerto TCP' },
    { value: 'udp.port', label: 'Puerto UDP' },
    { value: 'ip.proto', label: 'Protocolo' },
    { value: 'frame.len', label: 'Longitud' },
  ];

  const fetchPackets = async (customFilterValue = filterValue, customPayloadSearch = payloadSearch) => {
    try {
      const apiFilters: PacketFilters = { limit: 100 };
      
      if (customPayloadSearch.trim() !== '') {
        apiFilters.payload_search = customPayloadSearch.trim();
      }

      if (customFilterValue.trim() !== '') {
         if (filterType === 'ip.src') apiFilters.src_ip = customFilterValue.trim();
         if (filterType === 'ip.dst') apiFilters.dst_ip = customFilterValue.trim();
         if (filterType === 'ip.proto') apiFilters.protocol = customFilterValue.trim().toUpperCase();
         if (filterType === 'tcp.port' || filterType === 'udp.port') {
            apiFilters.port = parseInt(customFilterValue.trim());
         }
      }

      const res = await getPackets(apiFilters);
      const mappedPackets: CapturedPacket[] = res.packets.map((p, idx) => {
        const { hexDump, asciiDump } = decodePayload(p.payload_preview);
        return {
          id: res.total - res.offset - idx,
          time: new Date(p.time).toISOString().split('T')[1].slice(0, 12),
          source: p.src_ip,
          destination: p.dst_ip,
          protocol: p.protocol,
          length: p.length,
          info: `Puerto ${p.src_port} -> ${p.dst_port}`,
          hexDump,
          asciiDump
        };
      });

      setPacketsQueue(mappedPackets);
      setTotalItems(res.total);
      if (mappedPackets.length > 0) {
        setSelectedPacket(mappedPackets[0]);
      } else {
        setSelectedPacket(null);
      }
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    fetchPackets();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleApplyVisualFilter = () => {
    let expr = '';
    if (filterValue.trim()) {
      expr = `${filterType} ${filterOperator} ${filterValue}`;
    }
    if (payloadSearch.trim()) {
      expr += expr ? ` && payload contains '${payloadSearch}'` : `payload contains '${payloadSearch}'`;
    }
    setActiveFilterString(expr);
    fetchPackets(filterValue, payloadSearch);
  };

  const handleResetFilters = () => {
    setActiveFilterString('');
    setFilterValue('');
    setPayloadSearch('');
    fetchPackets('', '');
  };

  const handleCopyHex = () => {
    if (selectedPacket) {
      navigator.clipboard.writeText(selectedPacket.hexDump);
      setCopiedHex(true);
      setTimeout(() => setCopiedHex(false), 2000);
    }
  };

  return (
    <div id="filtros-view-wrapper" className="grid grid-cols-1 lg:grid-cols-12 gap-6 text-left">
      {/* Visual constructor & results: 7 Cols */}
      <div className="lg:col-span-8 space-y-4">
        {/* Visual filter constructor widget */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg space-y-4">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-2">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight flex items-center gap-2">
              <Filter className="h-4 w-4 text-[#3b82f6]" />
              <span>Constructor de Filtros</span>
            </h2>
            <button
              onClick={handleResetFilters}
              className="text-[#9ba3b4] hover:text-[#e4e7eb] text-xs font-mono"
            >
              [Reestablecer]
            </button>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            {/* Field dropdown selector */}
            <select
              value={filterType}
              onChange={(e) => setFilterType(e.target.value)}
              className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 focus:border-[#3b82f6] outline-none font-mono"
            >
              {filterFields.map((field) => (
                <option key={field.value} value={field.value}>
                  {field.label}
                </option>
              ))}
            </select>

            {/* Operator selector */}
            <select
              value={filterOperator}
              onChange={(e) => setFilterOperator(e.target.value)}
              className="bg-[#0a0e14] border border-[#2a3242] text-xs text-[#e4e7eb] rounded p-2 focus:border-[#3b82f6] outline-none font-mono"
            >
              <option value="==">==</option>
              <option value="!=">!=</option>
              <option value="contains">contains</option>
              <option value=">">&gt;</option>
              <option value="<">&lt;</option>
            </select>

            {/* Value parameter text field */}
            <input
              type="text"
              value={filterValue}
              onChange={(e) => setFilterValue(e.target.value)}
              placeholder="Ex. 192.168.1.100"
              className="bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] text-xs text-[#e4e7eb] placeholder-[#5c6473] rounded p-2 outline-none font-mono flex-1 min-w-[150px]"
            />

            <span>AND</span>

            <span className="font-mono text-xs text-[#5c6473] bg-[#0a0e14] border border-[#2a3242] p-2 rounded">
              payload LIKE
            </span>

            <button
              onClick={handleApplyVisualFilter}
              className="bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white font-mono text-xs font-bold py-2 px-4 rounded transition-all cursor-pointer"
            >
              Aplicar Filtro
            </button>
          </div>

          {/* Active Filter query preview bar or plain text payload search bar */}
          <div className="flex items-center gap-2 bg-[#0a0e14] border border-[#2a3242] rounded px-3 py-2 text-xs font-mono">
            <span className="text-[#5c6473]">Filtro Activo:</span>
            <span className="text-[#10b981] font-semibold">
              {activeFilterString || 'Ninguno (Captura Completa)'}
            </span>
            {activeFilterString && (
              <span className="ml-auto text-[#5c6473] text-[10px]">Expresión Compilada</span>
            )}
          </div>

          <div className="space-y-1 bg-[#131820] py-1">
            <label className="text-[11px] font-mono text-[#9ba3b4] font-medium block">
              Búsqueda de texto libre en Payload (ASCII ILIKE)
            </label>
            <div className="relative">
              <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-[#5c6473]" />
              <input
                type="text"
                placeholder="Ex. 'password', 'HTTP/1.1 200 OK', 'admin'..."
                value={payloadSearch}
                onChange={(e) => setPayloadSearch(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleApplyVisualFilter()}
                className="w-full bg-[#0a0e14] border border-[#2a3242] focus:border-[#3b82f6] rounded pl-9 pr-4 py-2 text-xs text-[#e4e7eb] outline-none font-mono"
              />
            </div>
          </div>
        </div>

        {/* Results grid Table */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg">
          <div className="flex justify-between items-center pb-2.5 mb-3 border-b border-[#2a3242]">
            <span className="text-xs font-sans font-bold text-[#e4e7eb]">
              Resultados de Captura (Total: {totalItems} paquetes)
            </span>
            <span className="text-[10px] font-mono text-[#5c6473]">
              Mostrando {packetsQueue.length}
            </span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full select-none text-left">
              <thead>
                <tr className="border-b border-[#2a3242] text-[10px] font-mono font-bold text-[#5c6473] uppercase tracking-wider">
                  <th className="py-2 px-3">No.</th>
                  <th className="py-2 px-3">Tiempo</th>
                  <th className="py-2 px-3">Origen</th>
                  <th className="py-2 px-3">Destino</th>
                  <th className="py-2 px-3">Proto</th>
                  <th className="py-2 px-3">Longitud</th>
                  <th className="py-2 px-3">Información</th>
                </tr>
              </thead>
              <tbody>
                {packetsQueue.map((pkt) => (
                  <tr
                    key={pkt.id}
                    onClick={() => setSelectedPacket(pkt)}
                    className={`border-b border-[#1c2230]/40 text-xs font-mono hover:bg-[#1c2230] cursor-pointer transition-colors ${
                      selectedPacket?.id === pkt.id ? 'bg-[#1c2230]' : ''
                    }`}
                  >
                    <td className="py-2.5 px-3 text-[#5c6473]">{pkt.id}</td>
                    <td className="py-2.5 px-3 text-[#9ba3b4]">{pkt.time}</td>
                    <td className="py-2.5 px-3 text-[#e4e7eb]">{pkt.source}</td>
                    <td className="py-2.5 px-3 text-[#e4e7eb]">{pkt.destination}</td>
                    <td className="py-2.5 px-3">
                      <span
                        className={`px-1.5 py-0.5 rounded text-[10px] font-bold ${
                          pkt.protocol === 'TCP'
                            ? 'bg-[#3b82f6]/10 text-[#3b82f6]'
                            : pkt.protocol === 'UDP'
                            ? 'bg-[#10b981]/10 text-[#10b981]'
                            : pkt.protocol === 'DNS'
                            ? 'bg-[#06b6d4]/10 text-[#06b6d4]'
                            : 'bg-[#8b5cf6]/10 text-[#8b5cf6]'
                        }`}
                      >
                        {pkt.protocol}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-[#9ba3b4]">{pkt.length}</td>
                    <td className="py-2.5 px-3 text-[#e4e7eb] max-w-xs truncate">{pkt.info}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Hex Dump inspector right details panel: 4 Cols */}
      <div className="lg:col-span-4 space-y-4">
        {selectedPacket ? (
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col h-full justify-between">
            {/* Package details header content */}
            <div>
              <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 mb-3.5">
                <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                  Detalles del Paquete #{selectedPacket.id}
                </h2>
                <button
                  onClick={handleCopyHex}
                  className="text-[#9ba3b4] hover:text-[#e4e7eb] text-2xs font-mono flex items-center gap-1.5 bg-[#0a0e14] px-2 py-0.5 border border-[#2a3242] rounded-md transition-all cursor-pointer"
                >
                  {copiedHex ? <Check className="h-3 w-3 text-[#10b981]" /> : <Copy className="h-3 w-3" />}
                  <span>{copiedHex ? 'Copiado' : 'Copiar Hex'}</span>
                </button>
              </div>

              {/* L2/L3 Decoded headers metrics */}
              <div className="space-y-3.5 mb-5 text-xs font-mono">
                {/* Physical Frame info */}
                <details open className="group">
                  <summary className="outline-none filter brightness-90 hover:brightness-110 cursor-pointer text-[#e4e7eb] font-semibold list-none flex items-center justify-between">
                    <span>💡 Frame {selectedPacket.id}: {selectedPacket.length} bytes on wire</span>
                    <ChevronRight className="h-3.5 w-3.5 text-[#5c6473] transform group-open:rotate-90 transition-transform" />
                  </summary>
                  <p className="pl-4 text-[#9ba3b4] text-[11px] mt-1 space-y-0.5">
                    <span>• Protocolo detectado: {selectedPacket.protocol}</span>
                    <br />
                    <span>• Información: {selectedPacket.info}</span>
                  </p>
                </details>

                <details open className="group">
                  <summary className="outline-none filter brightness-90 hover:brightness-110 cursor-pointer text-[#e4e7eb] font-semibold list-none flex items-center justify-between">
                    <span>🌐 Internet Protocol</span>
                    <ChevronRight className="h-3.5 w-3.5 text-[#5c6473] transform group-open:rotate-90 transition-transform" />
                  </summary>
                  <div className="pl-4 text-[#9ba3b4] text-[11px] mt-1 font-mono leading-tight space-y-0.5">
                    <div>• Source IP: {selectedPacket.source}</div>
                    <div>• Destination IP: {selectedPacket.destination}</div>
                  </div>
                </details>
              </div>

              {/* Standard Side-by-Side Hex and ASCII viewer. Absolutely gorgeous! */}
              <div className="space-y-1.5 text-left">
                <span className="text-[10px] uppercase tracking-wider font-bold text-[#5c6473] font-mono block">
                  Hex Dump & ASCII View
                </span>
                <div className="bg-[#0a0e14] border border-[#2a3242] rounded p-2.5 font-mono text-[10px] leading-relaxed text-[#06b6d4] overflow-x-auto whitespace-pre h-72">
                  <span className="text-amber-500 font-bold block pb-1 border-b border-[#2a3242]/40 mb-1.5">
                    OFFSET    HEXADECIMAL BYTES                 ASCII DECODED
                  </span>
                  {selectedPacket.hexDump.split('\n').map((line, idx) => {
                    if (!line.trim()) return null;
                    const asciiLine = selectedPacket.asciiDump.split('\n')[idx] || '';
                    return (
                      <div key={idx} className="flex justify-between font-mono gap-1 hover:bg-[#131820] py-0.5 rounded px-1">
                        <span className="text-[#5c6473] select-none">{(idx * 16).toString(16).padStart(4, '0')}</span>
                        <span className="text-[#e4e7eb]">{line.substring(2)}</span>
                        <span className="text-gray-500 border-l border-[#2a3242] pl-2">{asciiLine}</span>
                      </div>
                    );
                  })}
                  {selectedPacket.hexDump.trim() === '' && (
                    <span className="text-[#5c6473] italic">No payload available.</span>
                  )}
                </div>
              </div>
            </div>

            <div className="bg-[#0a0e14] p-2.5 border border-[#2a3242]/40 rounded mt-4 text-[10px] font-mono text-[#5c6473]">
              <span>Mostrando datos en vivo desde el backend ViewRed.</span>
            </div>
          </div>
        ) : (
          <div className="bg-[#131820] border border-[#2a3242] p-12 rounded-lg flex flex-col items-center justify-center h-full text-center">
            <HelpCircle className="h-10 w-10 text-[#5c6473] mb-3 animate-pulse" />
            <h3 className="font-sans font-semibold text-sm text-[#e4e7eb]">No hay paquetes seleccionados</h3>
            <p className="text-xs text-[#9ba3b4] mt-1">Seleccione un paquete de la lista para ver su representación Hex Dump.</p>
          </div>
        )}
      </div>
    </div>
  );
}
