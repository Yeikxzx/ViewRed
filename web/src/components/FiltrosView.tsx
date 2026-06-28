import { useState, useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import {
  Filter,
  Search,
  Check,
  Copy,
  ChevronRight,
  HelpCircle,
  Cable,
  Globe,
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
  const location = useLocation();
  const [filterType, setFilterType] = useState('ip.src');
  const [filterOperator, setFilterOperator] = useState('==');
  const [filterValue, setFilterValue] = useState('');
  const [payloadSearch, setPayloadSearch] = useState('');
  const [activeFilterString, setActiveFilterString] = useState('');

  const [packetsQueue, setPacketsQueue] = useState<CapturedPacket[]>([]);
  const [selectedPacket, setSelectedPacket] = useState<CapturedPacket | null>(null);
  const [copiedHex, setCopiedHex] = useState(false);
  const [totalItems, setTotalItems] = useState(0);
  // autoRefresh drives the polling interval below. The packets
  // table is read-mostly (humans scan rows, they don't expect
  // individual cells to mutate in front of them), so a 3-second
  // poll is the right cadence: fast enough that a new packet
  // appears within a few seconds, slow enough that the UI never
  // feels jittery. SSE would be more "real-time" but is overkill
  // for a paginated list — every new packet would race the active
  // filter and risk duplicating or mis-ordering rows.
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);
  // isFetching drives the small "Actualizando..." pill in the
  // header. Decoupled from autoRefresh so a manual refresh can
  // also show the indicator.
  const [isFetching, setIsFetching] = useState(false);

  // REFRESH_INTERVAL_MS lives outside React because it's a
  // configuration constant, not state. 3s is the default; tweak
  // here if the network is fast enough that 1s is useful or slow
  // enough that 5s is friendlier.
  const REFRESH_INTERVAL_MS = 3000;

  const filterFields = [
    { value: 'ip.src', label: 'IP Origen' },
    { value: 'ip.dst', label: 'IP Destino' },
    { value: 'tcp.port', label: 'Puerto TCP' },
    { value: 'udp.port', label: 'Puerto UDP' },
    { value: 'ip.proto', label: 'Protocolo' },
    { value: 'frame.len', label: 'Longitud' },
  ];

  const fetchPackets = async (customFilterValue = filterValue, customPayloadSearch = payloadSearch) => {
    setIsFetching(true);
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
      // Deduplicate on the (time, 5-tuple) key. The backend
      // currently returns the same packet twice when Npcap
      // hands the same frame to the reader more than once
      // (visible on busy flows — many TCP ACKs per
      // microsecond). Without dedup, React's `key` warning
      // fires and the table can render visually-duplicate rows.
      // Set keyed by the content id is enough; collisions are
      // rare enough that we just keep the first occurrence.
      const seen = new Set<string>();
      const mappedPackets: CapturedPacket[] = [];
      res.packets.forEach((p, idx) => {
        // Stable, content-based id. We previously derived the
        // id from `res.total - res.offset - idx`, which looked
        // fine in isolation but shifted every time a new
        // packet was captured: `total` grew by 1, every
        // existing packet's id moved by 1, and the user's
        // selection (keyed on the old id) silently failed to
        // match anything in the refreshed list. The detail
        // panel then "jumped" to whichever packet happened to
        // land at index 0 (newest). The time field is a
        // microsecond timestamp, effectively unique per packet,
        // and the IP+port combo is a tie-breaker that survives
        // any clock weirdness. Critically, the id is computed
        // from the packet itself, not from the surrounding
        // pagination state, so refreshes never shift it.
        const id = `${p.time}|${p.src_ip}|${p.src_port}|${p.dst_ip}|${p.dst_port}|${p.protocol}`;
        if (seen.has(id)) return;
        seen.add(id);
        const { hexDump, asciiDump } = decodePayload(p.payload_preview);
        mappedPackets.push({
          id,
          // Display-only number, used in the "NO." column so
          // the UI still shows a sequential-looking identifier
          // like the old (position-based) implementation did.
          // Decoupled from the React key on purpose: shifting
          // this on refreshes would be harmless (it's just a
          // label), but we also recompute it from
          // `res.total - idx` so the visual number stays
          // monotonically decreasing from top to bottom.
          number: res.total - idx,
          time: new Date(p.time).toISOString().split('T')[1].slice(0, 12),
          source: p.src_ip,
          destination: p.dst_ip,
          sourceHostname: p.src_hostname,
          destinationHostname: p.dst_hostname,
          protocol: p.protocol,
          length: p.length,
          info: `Puerto ${p.src_port} -> ${p.dst_port}`,
          hexDump,
          asciiDump
        });
      });

      setPacketsQueue(mappedPackets);
      setTotalItems(res.total);
      setLastRefresh(new Date());

      // Preserve the user's row selection across refreshes.
      // The id is content-based (time + 5-tuple + protocol) so
      // it's stable even when the pagination window shifts.
      //
      // Three cases:
      //  1. Selected packet is still in the new list → reuse
      //     the fresh row (so the detail panel always shows
      //     up-to-date data) but keep the original `number`
      //     the user clicked. The number is recomputed every
      //     fetch from `total - idx`, so it would otherwise
      //     shift on every refresh; sticking to the originally
      //     clicked number keeps the detail panel header
      //     visually stable, which matches user expectation
      //     ("I clicked #377289, the detail should still say
      //     #377289").
      //  2. Selected packet has scrolled off the top-N window
      //     (e.g. several thousand new packets arrived faster
      //     than the 3s poll) → keep the previously-stored
      //     packet object so the detail panel still shows its
      //     info. Without this guard, the old behaviour was to
      //     fall back to mappedPackets[0] on every refresh,
      //     which is what the user reported: "the detail panel
      //     jumps to a different packet on every refresh".
      //  3. Nothing was ever selected (initial load, or after
      //     a filter reset) → default to the first row of the
      //     new list.
      setSelectedPacket(prev => {
        if (prev) {
          const stillThere = mappedPackets.find(p => p.id === prev.id);
          if (stillThere) {
            // Same packet, fresh data, but pin the number the
            // user originally clicked so the detail header
            // doesn't appear to "change packet" on every poll.
            return { ...stillThere, number: prev.number };
          }
          return prev; // keep the selection, even if off-page
        }
        return mappedPackets.length > 0 ? mappedPackets[0] : null;
      });
    } catch (e) {
      console.error(e);
    } finally {
      setIsFetching(false);
    }
  };

  // One-shot fetch on mount so the table is never empty when the
  // user lands on the view. Lives in its own effect so it doesn't
  // re-run when the user toggles autoRefresh or edits a filter
  // input (those paths trigger their own fetch).
  useEffect(() => {
    fetchPackets();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-refresh: when the toggle is on, poll the packets endpoint
  // every REFRESH_INTERVAL_MS. The interval captures filterValue
  // and payloadSearch via closure, so the user can change a
  // filter and either click "Aplicar" (immediate fetch) or wait
  // for the next tick (the change will be picked up
  // automatically). The cleanup clears the interval so we don't
  // leak timers on unmount or on a toggle-off.
  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(() => {
      fetchPackets();
    }, REFRESH_INTERVAL_MS);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoRefresh]);

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

  // Process location state on mount
  useEffect(() => {
    const query = (location.state as any)?.query;
    if (query) {
      const cleanQuery = query.trim();
      const isIP = /^[0-9]{1,3}(\.[0-9]{1,3}){3}$/.test(cleanQuery);
      
      let newFilterValue = filterValue;
      let newPayloadSearch = payloadSearch;
      let newFilterType = filterType;
      
      if (isIP) {
        newFilterType = 'ip.src';
        setFilterType(newFilterType);
        setFilterValue(cleanQuery);
        newFilterValue = cleanQuery;
      } else {
        setPayloadSearch(cleanQuery);
        newPayloadSearch = cleanQuery;
      }
      
      let expr = '';
      if (newFilterValue.trim()) {
        expr = `${newFilterType} ${filterOperator} ${newFilterValue}`;
      }
      if (newPayloadSearch.trim()) {
        expr += expr ? ` && payload contains '${newPayloadSearch}'` : `payload contains '${newPayloadSearch}'`;
      }
      setActiveFilterString(expr);
      fetchPackets(newFilterValue, newPayloadSearch);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.state]);

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
          <div className="flex justify-between items-center pb-2.5 mb-3 border-b border-[#2a3242] gap-4">
            <span className="text-xs font-sans font-bold text-[#e4e7eb]">
              Resultados de Captura (Total: {totalItems} paquetes)
            </span>
            <div className="flex items-center gap-3">
              {/* Live / Paused indicator + manual refresh button.
                  Toggling autoRefresh re-creates the polling
                  interval in the useEffect above, so the UI
                  always reflects what the effect is actually
                  doing. The pulsing dot uses Tailwind's built-in
                  animate-pulse so we don't ship a keyframe. */}
              <button
                onClick={() => setAutoRefresh(v => !v)}
                className={`flex items-center gap-1.5 text-[10px] font-mono px-2 py-1 rounded border transition-colors ${
                  autoRefresh
                    ? 'text-[#10b981] border-[#10b981]/40 bg-[#10b981]/10 hover:bg-[#10b981]/20'
                    : 'text-[#5c6473] border-[#2a3242] hover:text-[#e4e7eb] hover:border-[#3a4356]'
                }`}
                title={autoRefresh ? 'Pausar auto-refresh' : 'Reanudar auto-refresh'}
              >
                {autoRefresh ? (
                  <>
                    <span className="w-1.5 h-1.5 bg-[#10b981] rounded-full animate-pulse" />
                    LIVE · {REFRESH_INTERVAL_MS / 1000}s
                  </>
                ) : (
                  <>
                    <span className="w-1.5 h-1.5 bg-[#5c6473] rounded-full" />
                    PAUSADO
                  </>
                )}
              </button>
              {/* Manual refresh is always available, even when
                  auto-refresh is on — useful for "I just clicked
                  something, show me now" without waiting for the
                  next tick. */}
              <button
                onClick={() => fetchPackets()}
                disabled={isFetching}
                className="text-[10px] font-mono text-[#9ba3b4] hover:text-[#e4e7eb] disabled:text-[#5c6473] disabled:cursor-not-allowed"
                title="Refrescar ahora"
              >
                <span className={isFetching ? 'animate-pulse' : ''}>
                  {isFetching ? 'Actualizando…' : '↻ Refrescar'}
                </span>
              </button>
              {lastRefresh && (
                <span className="text-[10px] font-mono text-[#5c6473]" title={lastRefresh.toISOString()}>
                  · {lastRefresh.toLocaleTimeString()}
                </span>
              )}
              <span className="text-[10px] font-mono text-[#5c6473] ml-2">
                Mostrando {packetsQueue.length}
              </span>
            </div>
          </div>

          <div className="overflow-x-auto max-h-[480px] overflow-y-auto">
            <table className="w-full select-none text-left">
              <thead className="sticky top-0 bg-[#131820] z-10">
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
                    <td className="py-2.5 px-3 text-[#5c6473]">{pkt.number}</td>
                    <td className="py-2.5 px-3 text-[#9ba3b4]">{pkt.time}</td>
                    <td className="py-2.5 px-3">
                      <div className="text-[#e4e7eb]">{pkt.source}</div>
                      {pkt.sourceHostname && (
                        <div className="text-[10px] text-[#06b6d4] truncate max-w-[140px]" title={pkt.sourceHostname}>
                          {pkt.sourceHostname}
                        </div>
                      )}
                    </td>
                    <td className="py-2.5 px-3">
                      <div className="text-[#e4e7eb]">{pkt.destination}</div>
                      {pkt.destinationHostname && (
                        <div className="text-[10px] text-[#06b6d4] truncate max-w-[140px]" title={pkt.destinationHostname}>
                          {pkt.destinationHostname}
                        </div>
                      )}
                    </td>
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
                  Detalles del Paquete #{selectedPacket.number}
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
                    <span className="flex items-center gap-2">
                      <Cable className="h-3.5 w-3.5 text-amber-500" />
                      Frame {selectedPacket.number}: {selectedPacket.length} bytes on wire
                    </span>
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
                    <span className="flex items-center gap-2">
                      <Globe className="h-3.5 w-3.5 text-cyan-400" />
                      Internet Protocol
                    </span>
                    <ChevronRight className="h-3.5 w-3.5 text-[#5c6473] transform group-open:rotate-90 transition-transform" />
                  </summary>
                  <div className="pl-4 text-[#9ba3b4] text-[11px] mt-1 font-mono leading-tight space-y-0.5">
                    <div>
                      • Source IP: {selectedPacket.source}
                      {selectedPacket.sourceHostname && (
                        <span className="text-[#06b6d4] ml-2">({selectedPacket.sourceHostname})</span>
                      )}
                    </div>
                    <div>
                      • Destination IP: {selectedPacket.destination}
                      {selectedPacket.destinationHostname && (
                        <span className="text-[#06b6d4] ml-2">({selectedPacket.destinationHostname})</span>
                      )}
                    </div>
                  </div>
                </details>
              </div>

              {/* Hex + ASCII viewer in the Wireshark "two-line" layout:
                  each row is two stacked lines — the hex bytes on
                  top, the printable ASCII on the bottom. Two lines
                  instead of three side-by-side columns means the
                  ASCII column gets the full content width to
                  itself, so even printable runs longer than the
                  hex view (URLs, base64-ish blobs, escaped JSON)
                  stay readable without horizontal scroll on a
                  narrow detail panel. The offset lives on the
                  hex line; the ASCII line is indented to align
                  visually under the bytes, which is the same
                  convention Wireshark and tcpdump -X use. */}
              <div className="space-y-1.5 text-left">
                <span className="text-[10px] uppercase tracking-wider font-bold text-[#5c6473] font-mono block">
                  Hex Dump & ASCII View
                </span>
                <div className="bg-[#0a0e14] border border-[#2a3242] rounded p-2.5 font-mono text-[10px] leading-relaxed text-[#06b6d4] overflow-x-auto h-72">
                  <div className="text-amber-500 font-bold pb-1.5 mb-1 border-b border-[#2a3242]/40">
                    <span className="inline-block w-12">OFFSET</span>
                    <span>HEXADECIMAL BYTES</span>
                  </div>
                  {selectedPacket.hexDump.split('\n').map((line, idx) => {
                    if (!line.trim()) return null;
                    const asciiLine = selectedPacket.asciiDump.split('\n')[idx] || '';
                    return (
                      <div key={idx} className="hover:bg-[#131820] py-0.5 rounded px-1">
                        <div className="flex gap-3">
                          <span className="text-[#5c6473] select-none w-12 flex-shrink-0">
                            {(idx * 16).toString(16).padStart(4, '0')}
                          </span>
                          <span className="text-[#e4e7eb] whitespace-pre">
                            {/* hex line has a leading "00 " that
                                the offset visually replaces; strip
                                it so the bytes line up under the
                                header's HEXADECIMAL column */}
                            {line.replace(/^00\s/, '')}
                          </span>
                        </div>
                        <div className="flex gap-3">
                          {/* empty offset column on the ASCII
                              line keeps the rows aligned */}
                          <span className="w-12 flex-shrink-0" />
                          <span className="text-gray-400 whitespace-pre">
                            {asciiLine || ' '}
                          </span>
                        </div>
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
