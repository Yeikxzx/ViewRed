import { useEffect, useState } from 'react';
import { Archive, Trash2, HardDrive, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { getPcaps, deletePcap, deleteAllPcaps, PcapFile } from '../lib/api';
import { useSSEStore } from '../store/sse-store';

export default function HistorialView() {
  const [pcaps, setPcaps] = useState<PcapFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState<string | null>(null);
  
  const captureStatus = useSSEStore((s) => s.captureStatus);
  const isCapturing = captureStatus === 'running' || captureStatus === 'unknown';

  const fetchPcaps = async () => {
    try {
      const data = await getPcaps();
      // Sort newest first
      data.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());
      setPcaps(data);
    } catch (err) {
      console.error('Failed to fetch pcaps:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPcaps();
    const interval = setInterval(fetchPcaps, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleDelete = (filename: string) => {
    toast(`¿Eliminar el archivo ${filename}?`, {
      description: 'Esta acción no se puede deshacer.',
      action: {
        label: 'Eliminar',
        onClick: async () => {
          setDeleting(filename);
          try {
            await deletePcap(filename);
            await fetchPcaps();
            toast.success('Archivo eliminado');
          } catch (err) {
            console.error('Failed to delete pcap:', err);
            toast.error('Error al eliminar el archivo');
          } finally {
            setDeleting(null);
          }
        },
      },
      cancel: {
        label: 'Cancelar',
        onClick: () => {},
      },
      duration: 10000,
    });
  };

  const handleDeleteAll = () => {
    toast('¿Eliminar TODOS los archivos de captura?', {
      description: 'Esta acción no se puede deshacer.',
      action: {
        label: 'Eliminar todo',
        onClick: async () => {
          setDeleting('ALL');
          try {
            await deleteAllPcaps();
            await fetchPcaps();
            toast.success('Todos los archivos eliminados');
          } catch (err) {
            console.error('Failed to clear pcaps:', err);
            toast.error('Error al eliminar los archivos');
          } finally {
            setDeleting(null);
          }
        },
      },
      cancel: {
        label: 'Cancelar',
        onClick: () => {},
      },
      duration: 10000,
    });
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const totalSize = pcaps.reduce((acc, p) => acc + p.size, 0);

  return (
    <div className="flex-1 flex flex-col min-h-0 bg-[#0a0e14]">
      {/* Header */}
      <header className="h-14 border-b border-[#2a3242]/40 bg-[#0a0e14] flex items-center px-6 shrink-0 justify-between">
        <div className="flex items-center gap-3 text-[#e4e7eb]">
          <Archive className="h-5 w-5 text-[#3b82f6]" />
          <h1 className="font-sans font-semibold text-sm uppercase tracking-wide">
            Historial de Capturas
          </h1>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-xs text-[#5c6473] font-mono flex items-center gap-2">
            <HardDrive className="h-4 w-4" />
            Total: {formatBytes(totalSize)}
          </div>
          <button
            onClick={() => fetchPcaps()}
            className="text-[#5c6473] hover:text-[#e4e7eb] transition-colors"
            title="Actualizar"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
          {pcaps.length > 0 && (
            <button
              onClick={handleDeleteAll}
              disabled={deleting === 'ALL'}
              className="px-3 py-1.5 bg-red-500/10 hover:bg-red-500/20 text-red-400 text-xs font-medium rounded border border-red-500/20 transition-colors flex items-center gap-2 disabled:opacity-50"
            >
              <Trash2 className="h-3.5 w-3.5" />
              {deleting === 'ALL' ? 'Eliminando...' : 'Eliminar Todos'}
            </button>
          )}
        </div>
      </header>

      {/* Content */}
      <div className="flex-1 p-6 overflow-auto">
        <div className="max-w-5xl mx-auto space-y-6">
          <div className="bg-[#0f141d] rounded-lg border border-[#2a3242]/40 overflow-hidden shadow-sm">
            {loading ? (
              <div className="p-8 text-center text-[#5c6473] text-sm">
                Cargando historial...
              </div>
            ) : pcaps.length === 0 ? (
              <div className="p-12 text-center flex flex-col items-center">
                <Archive className="h-12 w-12 text-[#2a3242] mb-4" />
                <h3 className="text-[#e4e7eb] font-medium mb-1">No hay capturas</h3>
                <p className="text-[#5c6473] text-sm">Los archivos PCAP generados aparecerán aquí.</p>
              </div>
            ) : (
              <table className="w-full text-left text-sm text-[#e4e7eb]">
                <thead className="bg-[#141b25] border-b border-[#2a3242]/40 text-xs text-[#5c6473] uppercase tracking-wider font-semibold">
                  <tr>
                    <th className="px-6 py-4">Archivo</th>
                    <th className="px-6 py-4">Fecha de Modificación</th>
                    <th className="px-6 py-4">Tamaño</th>
                    <th className="px-6 py-4 text-right">Acciones</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#2a3242]/20">
                  {pcaps.map((file, idx) => {
                    const isActive = isCapturing && idx === 0; // Assuming sorted newest first
                    return (
                    <tr key={file.name} className="hover:bg-[#1a2332]/50 transition-colors group">
                      <td className="px-6 py-3">
                        <div className="flex items-center gap-3">
                          <Archive className={`h-4 w-4 ${isActive ? 'text-[#10b981]' : 'text-[#5c6473]'}`} />
                          <span className="font-mono text-xs">{file.name}</span>
                          {isActive && (
                            <span className="px-1.5 py-0.5 rounded text-[9px] font-bold bg-[#10b981]/10 text-[#10b981] uppercase tracking-wider animate-pulse flex items-center gap-1">
                              <div className="w-1.5 h-1.5 rounded-full bg-[#10b981]"></div>
                              Grabando
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-3 font-mono text-xs text-[#8b949e]">
                        {new Date(file.timestamp).toLocaleString()}
                      </td>
                      <td className="px-6 py-3 font-mono text-xs text-[#8b949e]">
                        {formatBytes(file.size)}
                      </td>
                      <td className="px-6 py-3 text-right">
                        <button
                          onClick={() => handleDelete(file.name)}
                          disabled={deleting === file.name || isActive}
                          className="text-[#5c6473] hover:text-red-400 p-1.5 rounded transition-colors disabled:opacity-20"
                          title={isActive ? 'No se puede eliminar el archivo activo' : 'Eliminar archivo'}
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </td>
                    </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
