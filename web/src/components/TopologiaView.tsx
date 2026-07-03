import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Network,
  Smartphone,
  Server,
  HelpCircle,
  X,
  Play,
  Monitor,
  Printer,
  Tv,
  Wifi,
  Router,
  AlertTriangle,
  Activity,
  Globe,
  MapPin,
  Clock,
  Zap,
} from 'lucide-react';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import 'leaflet.markercluster';
import 'leaflet.markercluster/dist/MarkerCluster.css';
import 'leaflet.markercluster/dist/MarkerCluster.Default.css';

interface TopologiaViewProps {
  setSelectedDeviceIP: (ip: string) => void;
}

// ---- Types -----------------------------------------------------------------

interface Device {
  id: number;
  ip: string;
  mac: string;
  vendor: string;
  hostname: string;
  first_seen: string;
  last_seen: string;
  risk_score: number | null;
}

interface TopologyEdge {
  id: number;
  src_device_id: number;
  dst_device_id: number;
  bytes_count: number;
  last_seen: string;
}

interface GeoLocation {
  ip: string;
  lat: number;
  lon: number;
  city: string;
  region: string;
  country: string;
  country_code: string;
  isp: string;
}

function isPrivateIP(ipStr: string): boolean {
  const ip = ipStr.split('/')[0];
  const parts = ip.split('.');
  if (parts.length !== 4) return false;
  const p1 = parseInt(parts[0], 10);
  const p2 = parseInt(parts[1], 10);
  if (p1 === 10) return true;
  if (p1 === 172 && p2 >= 16 && p2 <= 31) return true;
  if (p1 === 192 && p2 === 168) return true;
  if (p1 === 127) return true;
  if (p1 === 169 && p2 === 254) return true; // link-local
  return false;
}

// ---- Device type heuristic -------------------------------------------------

type DeviceType = 'router' | 'phone' | 'laptop' | 'pc' | 'tv' | 'printer' | 'server' | 'iot' | 'unknown';

function detectDeviceType(device: Device): DeviceType {
  const vendor = (device.vendor || '').toLowerCase();
  const hostname = (device.hostname || '').toLowerCase();

  // Hostname hints are strongest
  if (hostname.includes('tv')) return 'tv';
  if (hostname.includes('iphone') || hostname.includes('android') || hostname.includes('phone') ||
      hostname.includes('pixel') || hostname.includes('galaxy')) return 'phone';
  if (hostname.includes('laptop') || hostname.includes('notebook') || hostname.includes(' nb ') ||
      hostname.includes('victus') || hostname.includes('thinkpad') || hostname.includes('ideapad') ||
      hostname.includes('yoga') || hostname.includes('envy') || hostname.includes('pavilion') ||
      hostname.includes('inspiron') || hostname.includes('xps') || hostname.includes('macbook')) return 'laptop';
  if (hostname.includes('pc') || hostname.includes('desktop') || hostname.includes('workstation')) return 'pc';
  if (hostname.includes('printer') || hostname.includes('print')) return 'printer';
  if (hostname.includes('router') || hostname.includes('gateway') || hostname.includes('ap-')) return 'router';
  if (hostname.includes('tv-') || hostname.includes('livingroom')) return 'tv';
  if (hostname.includes('cam') || hostname.includes('camara')) return 'iot';

  // Vendor heuristics — the IP itself can also hint at the device role.
  // Some heuristics (e.g. HUAWEI) match both phone AND router vendors,
  // so the more specific checks come first.
  const ip = device.ip.replace(/\/\d+$/, ''); // strip /32 CIDR suffix

  // --- ROUTER / NETWORKING ---
  // IP ending in .1 often indicates a gateway or router.
  if (/\.1$/.test(ip)) return 'router';

  if (vendor.includes('cisco') || vendor.includes('juniper') || vendor.includes('ubiquiti') ||
      vendor.includes('tp-link') || vendor.includes('tplink') || vendor.includes('netgear') ||
      vendor.includes('mikrotik') || vendor.includes('aruba') || vendor.includes('fortinet') ||
      vendor.includes('meraki') || vendor.includes('linksys') || vendor.includes('draytek') ||
      vendor.includes('peplink') || vendor.includes('dlink') || vendor.includes('zyxel') ||
      vendor.includes('ruijie') || vendor.includes('tenda') || vendor.includes('mercusys') ||
      (vendor.includes('huawei') && vendor.includes('technologies')) ||
      (vendor.includes('asus') && vendor.includes('network'))) return 'router';

  // --- PC / LAPTOP ---
  // Common laptop NIC vendors (Broadcom, Qualcomm/Atheros, etc.) are included
  // because most PCs ship with chips from these manufacturers.
  if (vendor.includes('intel') || vendor.includes('dell') || vendor.includes('lenovo') ||
      vendor.includes('hp') || vendor.includes('hewlett') || vendor.includes('packard') ||
      vendor.includes('acer') || vendor.includes('microsoft') || vendor.includes('gigabyte') ||
      vendor.includes('giga-byte') || vendor.includes('msi') || vendor.includes('micro-star') ||
      vendor.includes('amd') || vendor.includes('realtek') || vendor.includes('asustek') ||
      vendor.includes('emachines') || vendor.includes('alienware') || vendor.includes('cyberpowerpc') ||
      vendor.includes('ibuypower') || vendor.includes('liteon') || vendor.includes('lite-on') || 
      vendor.includes('wnc') || vendor.includes('azurewave') || vendor.includes('broadcom') || 
      vendor.includes('qualcomm') || vendor.includes('atheros') || vendor.includes('marvell') || 
      vendor.includes('mediatek') || vendor.includes('ralink') || vendor.includes('rivet') || 
      vendor.includes('killer') || vendor.includes('foxconn') || vendor.includes('hon hai') || 
      vendor.includes('wistron') || vendor.includes('compal') || vendor.includes('quanta') || 
      vendor.includes('pegatron') || vendor.includes('inventec') || vendor.includes('chicony') ||
      vendor.includes('clevo') || vendor.includes('hefei') || vendor.includes('fujitsu') || 
      vendor.includes('panasonic')) return 'pc';

  // --- PRINTER ---
  if (vendor.includes('epson') || vendor.includes('canon') || vendor.includes('brother') ||
      vendor.includes('xerox') || vendor.includes('ricoh') || vendor.includes('kyocera') ||
      vendor.includes('lexmark') || vendor.includes('sharp') || vendor.includes('okidata') ||
      vendor.includes('konica')) return 'printer';

  // --- PHONE / TABLET ---
  if (vendor.includes('apple') || vendor.includes('samsung') || vendor.includes('huawei') ||
      vendor.includes('xiaomi') || vendor.includes('oneplus') || vendor.includes('motorola') ||
      vendor.includes('google') || vendor.includes('honor') || vendor.includes('oppo') ||
      vendor.includes('vivo') || vendor.includes('realme') || vendor.includes('zte') ||
      vendor.includes('nokia') || vendor.includes('nothing') || vendor.includes('meizu') ||
      vendor.includes('bbk') || vendor.includes('sony mobile') || vendor.includes('htc') ||
      vendor.includes('blackberry')) return 'phone';

  // --- TV / CONSOLES ---
  if (vendor.includes('lg') || vendor.includes('roku') || vendor.includes('tcl') ||
      vendor.includes('hisense') || vendor.includes('sony') || vendor.includes('panasonic') ||
      vendor.includes('philips') || vendor.includes('vestel') || vendor.includes('changhong') ||
      vendor.includes('nintendo') || vendor.includes('vizio')) return 'tv';

  // --- IoT / EMBEDDED ---
  if (vendor.includes('raspberry') || vendor.includes('arduino') || vendor.includes('espressif') ||
      vendor.includes('esp32') || vendor.includes('sonoff') || vendor.includes('shelly') ||
      vendor.includes('tuya') || vendor.includes('alexa') || vendor.includes('amazon') ||
      vendor.includes('earda') || vendor.includes('hichip') || vendor.includes('hikvision') ||
      vendor.includes('dahua') || vendor.includes('reolink') || vendor.includes('annke') ||
      vendor.includes('amcrest') || vendor.includes('wyze') || vendor.includes('ring') ||
      vendor.includes('belkin') || vendor.includes('murata') || vendor.includes('wemos') ||
      vendor.includes('nest') || vendor.includes('roomba') || vendor.includes('irobot') ||
      (vendor.includes('google') && (vendor.includes('home') || vendor.includes('nest'))) ||
      (vendor.includes('philips') && vendor.includes('hue')) ||
      (vendor.includes('samsung') && vendor.includes('smart')) ||
      vendor.includes('cloud network technology')) return 'iot';

  // --- SERVER / CLOUD ---
  if (vendor.includes('amazon') || vendor.includes('aws') || vendor.includes('vmware') ||
      vendor.includes('digitalocean') || vendor.includes('oracle') ||
      vendor.includes('microsoft') && vendor.includes('azure')) return 'server';

  return 'unknown';
}

const DEVICE_ICONS: Record<DeviceType, typeof HelpCircle> = {
  router: Router,
  phone: Smartphone,
  laptop: Monitor,
  pc: Monitor,
  tv: Tv,
  printer: Printer,
  server: Server,
  iot: Activity,
  unknown: HelpCircle,
};

const DEVICE_LABELS: Record<DeviceType, string> = {
  router: 'Router/Modem',
  phone: 'Telefono',
  laptop: 'Laptop',
  pc: 'PC',
  tv: 'TV',
  printer: 'Impresora',
  server: 'Servidor',
  iot: 'Dispositivo IoT',
  unknown: 'Desconocido',
};

const DEVICE_COLORS: Record<DeviceType, string> = {
  router: '#f59e0b',
  phone: '#10b981',
  laptop: '#3b82f6',
  pc: '#3b82f6',
  tv: '#a855f7',
  printer: '#ec4899',
  server: '#6366f1',
  iot: '#14b8a6',
  unknown: '#6b7280',
};

// ---- Component -------------------------------------------------------------

export default function TopologiaView({ setSelectedDeviceIP }: TopologiaViewProps) {
  const navigate = useNavigate();
  const mapRef = useRef<L.Map | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const markersRef = useRef<Map<number, L.Marker>>(new Map());
  const linesRef = useRef<L.Polyline[]>([]);
  const clusterGroupRef = useRef<L.MarkerClusterGroup | null>(null);
  const locationRef = useRef<GeoLocation | null>(null);
  const geoCacheRef = useRef<Map<string, GeoLocation>>(new Map());
  const devicesRef = useRef<Device[]>([]);
  const edgesRef = useRef<TopologyEdge[]>([]);
  const dataKeyRef = useRef('');
  const retryUpdateRef = useRef<() => void>(() => {});

  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const selectedDeviceRef = useRef<Device | null>(null);
  const [selectedEdges, setSelectedEdges] = useState<TopologyEdge[]>([]);
  const [networkLocation, setNetworkLocation] = useState<GeoLocation | null>(null);
  const [geoError, setGeoError] = useState<string | null>(null);
  const [stats, setStats] = useState({ devices: 0, connections: 0, suspicious: 0, totalBytes: 0 });
  const [showExternal, setShowExternal] = useState(false);
  const showExternalRef = useRef(false);

  const toggleExternal = () => {
    const next = !showExternal;
    setShowExternal(next);
    showExternalRef.current = next;
    retryUpdateRef.current();
  };

  // ---- Initialize map once ----
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = L.map(containerRef.current, {
      center: [4.6, -74.1], // Bogota fallback
      zoom: 14,
      zoomControl: true,
      attributionControl: true,
      preferCanvas: true, // Render thousands of lines instantly without DOM freeze
    });

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
      attribution: '&copy; OpenStreetMap &copy; CARTO',
      subdomains: 'abcd',
      maxZoom: 20,
    }).addTo(map);

    mapRef.current = map;

    // Fix for Leaflet sizing issues after container resize
    setTimeout(() => map.invalidateSize(), 200);

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // ---- Fetch GeoIP ----
  useEffect(() => {
    fetch('/api/v1/geoip')
      .then((res) => {
        if (!res.ok) throw new Error('GeoIP unavailable');
        return res.json();
      })
      .then((data: GeoLocation) => {
        locationRef.current = data;
        setNetworkLocation(data);
        if (mapRef.current) {
          mapRef.current.setView([data.lat, data.lon], 15);
        }
        // GeoIP just arrived — retry the map render in case the first
        // topology poll beat us here and updateMap bailed on !loc.
        retryUpdateRef.current();
      })
      .catch((err) => {
        console.error('GeoIP fetch error:', err);
        setGeoError(err.message);
      });
  }, []);

  // ---- Fetch topology + update map markers/lines ----
  useEffect(() => {
    const updateMap = () => {
      const map = mapRef.current;
      if (!map) return;

      let devices = devicesRef.current;
      let edges = edgesRef.current;
      const loc = locationRef.current;

      if (!loc || devices.length === 0) return;

      if (!showExternalRef.current) {
        devices = devices.filter(d => isPrivateIP(d.ip));
        const validIDs = new Set(devices.map(d => d.id));
        edges = edges.filter(e => validIDs.has(e.src_device_id) && validIDs.has(e.dst_device_id));
      }

      // Clear old markers
      markersRef.current.forEach((m) => m.remove());
      markersRef.current.clear();

      // Clear old lines
      linesRef.current.forEach((l) => l.remove());
      linesRef.current = [];
      
      if (clusterGroupRef.current) {
        map.removeLayer(clusterGroupRef.current);
      }
      
      const clusterGroup = L.markerClusterGroup({
        chunkedLoading: true,
        maxClusterRadius: 50,
      });
      clusterGroupRef.current = clusterGroup;

      // Place devices around the network center with deterministic offsets
      // based on device count. Use a sunflower spiral so devices spread
      // out without overlapping as the count grows.
      const totalDevices = devices.length;
      const goldenAngle = Math.PI * (3 - Math.sqrt(5)); // ~137.5°
      devices.forEach((dev, i) => {
        // Sunflower spiral: spreads points evenly in a disc.
        // radius grows with sqrt of index so density stays uniform.
        const r = 0.0008 * Math.sqrt(i + 1) * 2.5;
        const angle = i * goldenAngle;
        // Add a small deterministic offset per device ID for stability
        // across polls when device order changes.
        const jitter = ((dev.id * 13) % 100) / 100 * 0.0001;
        let lat = loc.lat + (r + jitter) * Math.sin(angle);
        let lon = loc.lon + (r + jitter) * Math.cos(angle);

        // If public IP, use real GeoIP coordinates if available
        if (!isPrivateIP(dev.ip)) {
          const geo = geoCacheRef.current.get(dev.ip);
          if (geo) {
            // Apply a small jitter for IPs at the exact same location (e.g. same ISP city)
            lat = geo.lat + jitter;
            lon = geo.lon + jitter;
          }
        }

        const type = detectDeviceType(dev);
        const color = DEVICE_COLORS[type];
        const Icon = DEVICE_ICONS[type];

        // Create a custom divIcon with the precision dot design
        const iconHtml = `
          <div style="position: relative;">
            <div style="
              position: absolute; bottom: 12px; left: 50%; transform: translateX(-50%);
              font-size: 10px; font-family: monospace; color: #fff;
              background: rgba(0,0,0,0.6); padding: 2px 4px; border-radius: 4px;
              white-space: nowrap; border: 1px solid ${color}44;
            ">${dev.ip}</div>
            <div style="
              width: 12px; height: 12px;
              background: ${color};
              border: 2px solid #0f172a;
              border-radius: 50%;
              box-shadow: 0 0 10px ${color}aa;
              margin: 0 auto;
            "></div>
          </div>
        `;

        const marker = L.marker([lat, lon], {
          icon: L.divIcon({ html: iconHtml, className: 'topo-marker', iconSize: [12, 12], iconAnchor: [6, 6] }),
        });

        marker.on('click', () => {
          setSelectedDevice(dev);
          selectedDeviceRef.current = dev;
          const connected = edges.filter(
            (e) => e.src_device_id === dev.id || e.dst_device_id === dev.id
          );
          setSelectedEdges(connected);
          retryUpdateRef.current(); // Redraw map to show/hide edges for this device
        });

        clusterGroup.addLayer(marker);
        markersRef.current.set(dev.id, marker);
      });
      
      map.addLayer(clusterGroup);

      // Draw edges as polylines
      // Limit to top 250 edges by volume to avoid visual clutter and saturation
      const maxEdgesToDraw = 250;
      const sortedEdges = [...edges].sort((a, b) => b.bytes_count - a.bytes_count);
      const edgesToDraw = sortedEdges.slice(0, maxEdgesToDraw);

      const suspiciousThreshold = 1000000; // 1MB
      const selDev = selectedDeviceRef.current;
      
      edgesToDraw.forEach((edge) => {
        const srcMarker = markersRef.current.get(edge.src_device_id);
        const dstMarker = markersRef.current.get(edge.dst_device_id);
        if (!srcMarker || !dstMarker) return;

        const srcDev = devices.find(d => d.id === edge.src_device_id);
        const dstDev = devices.find(d => d.id === edge.dst_device_id);
        if (!srcDev || !dstDev) return;

        const isExternalEdge = !isPrivateIP(srcDev.ip) || !isPrivateIP(dstDev.ip);

        // Hide external edges unless they belong to the selected device to prevent visual saturation
        if (isExternalEdge) {
          if (!selDev || (edge.src_device_id !== selDev.id && edge.dst_device_id !== selDev.id)) {
            return;
          }
        }

        const srcLatLng = srcMarker.getLatLng();
        const dstLatLng = dstMarker.getLatLng();

        const isSuspicious = edge.bytes_count > suspiciousThreshold;
        const color = isSuspicious ? '#ef4444' : '#38bdf8';
        const weight = Math.max(1, Math.min(4, Math.log10(edge.bytes_count + 1) * 0.5));
        const opacity = isSuspicious ? 0.8 : 0.15; // Reduce visual clutter for normal traffic

        const line = L.polyline([srcLatLng, dstLatLng], {
          color,
          weight,
          opacity,
          dashArray: '8 6',
          className: isSuspicious ? 'topo-edge topo-edge-suspicious' : 'topo-edge',
        });

        line.addTo(map);
        linesRef.current.push(line);
      });

      // Update stats
      const suspiciousCount = edges.filter((e) => e.bytes_count > suspiciousThreshold).length;
      const totalBytes = edges.reduce((sum, e) => sum + e.bytes_count, 0);
      setStats({
        devices: devices.length,
        connections: edges.length,
        suspicious: suspiciousCount,
        totalBytes,
      });
    };

    const fetchTopology = async () => {
      try {
        const res = await fetch('/api/v1/topology');
        if (!res.ok) throw new Error('Failed to fetch topology');
        const data = await res.json();

        // Only skip when the map already rendered markers AND the data
        // is identical. This avoids the race where the first poll arrives
        // before GeoIP — markers are still 0, so we always retry.
        const key = (data.nodes || []).map((d: Device) => d.id).join(',')
          + '|' + (data.edges || []).map((e: TopologyEdge) => e.id).join(',');
        if (markersRef.current.size > 0 && key === dataKeyRef.current) return;
        dataKeyRef.current = key;

        devicesRef.current = (data.nodes || []) as Device[];
        edgesRef.current = (data.edges || []) as TopologyEdge[];

        // Batch GeoIP for public IPs
        if (showExternalRef.current) {
          const publicIPs = devicesRef.current
            .filter(d => !isPrivateIP(d.ip))
            .filter(d => !geoCacheRef.current.has(d.ip))
            .map(d => d.ip);
            
          if (publicIPs.length > 0) {
            try {
              const geoRes = await fetch('/api/v1/geoip/batch', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ ips: publicIPs }),
              });
              if (geoRes.ok) {
                const geoData = await geoRes.json();
                geoData.forEach((g: GeoLocation) => {
                  geoCacheRef.current.set(g.ip, g);
                });
              }
            } catch (err) {
              console.error('GeoIP batch fetch error:', err);
            }
          }
        }

        updateMap();
      } catch (err) {
        console.error('Topology fetch error:', err);
      }
    };

    fetchTopology();
    retryUpdateRef.current = updateMap;
    const interval = setInterval(fetchTopology, 15000);
    return () => {
      clearInterval(interval);
      retryUpdateRef.current = () => {};
    };
  }, []);

  // ---- Helpers ----

  const formatBytes = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const timeAgo = (dateStr: string): string => {
    const diff = Date.now() - new Date(dateStr).getTime();
    const secs = Math.floor(diff / 1000);
    if (secs < 60) return `Hace ${secs}s`;
    const mins = Math.floor(secs / 60);
    if (mins < 60) return `Hace ${mins}m`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `Hace ${hours}h`;
    return `Hace ${Math.floor(hours / 24)}d`;
  };

  const handleScanDevice = (ip: string) => {
    setSelectedDeviceIP(ip);
    navigate('/vulnerabilidades');
  };

  const selectedDeviceType = selectedDevice ? detectDeviceType(selectedDevice) : 'server';
  const SelectedDeviceIcon = DEVICE_ICONS[selectedDeviceType];

  return (
    <div id="topologia-view-wrapper" className="grid grid-cols-1 lg:grid-cols-12 gap-6 text-left">
      {/* Map: 8 cols */}
      <div className="lg:col-span-8 space-y-4">
        {/* Stats bar */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div className="bg-[#131820] border border-[#2a3242] p-3 rounded-lg flex items-center gap-2.5">
            <div className="bg-sky-500/10 p-2 rounded border border-sky-500/20">
              <Wifi className="h-4 w-4 text-sky-400" />
            </div>
            <div>
              <span className="text-[10px] font-mono text-[#5c6473] block uppercase">Dispositivos</span>
              <span className="text-lg font-mono font-bold text-[#e4e7eb]">{stats.devices}</span>
            </div>
          </div>
          <div className="bg-[#131820] border border-[#2a3242] p-3 rounded-lg flex items-center gap-2.5">
            <div className="bg-indigo-500/10 p-2 rounded border border-indigo-500/20">
              <Network className="h-4 w-4 text-indigo-400" />
            </div>
            <div>
              <span className="text-[10px] font-mono text-[#5c6473] block uppercase">Conexiones</span>
              <span className="text-lg font-mono font-bold text-[#e4e7eb]">{stats.connections}</span>
            </div>
          </div>
          <div className="bg-[#131820] border border-[#2a3242] p-3 rounded-lg flex items-center gap-2.5">
            <div className="bg-red-500/10 p-2 rounded border border-red-500/20">
              <AlertTriangle className="h-4 w-4 text-red-400" />
            </div>
            <div>
              <span className="text-[10px] font-mono text-[#5c6473] block uppercase">Sospechosas</span>
              <span className="text-lg font-mono font-bold text-red-400">{stats.suspicious}</span>
            </div>
          </div>
          <div className="bg-[#131820] border border-[#2a3242] p-3 rounded-lg flex items-center gap-2.5">
            <div className="bg-emerald-500/10 p-2 rounded border border-emerald-500/20">
              <Activity className="h-4 w-4 text-emerald-400" />
            </div>
            <div>
              <span className="text-[10px] font-mono text-[#5c6473] block uppercase">Trafico Total</span>
              <span className="text-lg font-mono font-bold text-[#e4e7eb]">{formatBytes(stats.totalBytes)}</span>
            </div>
          </div>
        </div>

        {/* Map container */}
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col h-[560px] relative">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 mb-3 z-[1000]">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight flex items-center gap-2">
              <Network className="h-4 w-4 text-[#3b82f6]" />
              <span>Mapa de Topologia en Tiempo Real</span>
            </h2>
            <div className="flex items-center gap-4">
              <button
                onClick={toggleExternal}
                className={`text-[11px] font-mono px-3 py-1 rounded transition-colors ${
                  showExternal
                    ? 'bg-[#3b82f6]/20 text-[#3b82f6] border border-[#3b82f6]/30'
                    : 'bg-[#1c2230] text-[#9ba3b4] border border-[#2a3242] hover:bg-[#2a3242]'
                }`}
              >
                {showExternal ? 'Ocultar Externos' : 'Mostrar Externos'}
              </button>
              {networkLocation && (
                <div className="flex items-center gap-2 text-[11px] font-mono text-[#9ba3b4]">
                  <MapPin className="h-3 w-3 text-[#3b82f6]" />
                  <span>{networkLocation.city}, {networkLocation.country_code}</span>
                </div>
              )}
            </div>
          </div>

          <div
            ref={containerRef}
            className="flex-1 w-full rounded overflow-hidden border border-[#1c2230]"
            style={{ background: '#0a0e14' }}
          />

          {geoError && (
            <div className="absolute bottom-4 left-4 z-[1000] bg-[#3a1d1d] border border-red-500/40 text-[#fca5a5] px-3 py-1.5 rounded text-xs font-mono">
              GeoIP no disponible: {geoError}. Mostrando ubicacion por defecto.
            </div>
          )}

          {/* Legend */}
          <div className="absolute bottom-4 right-4 z-[1000] bg-[#0a0e14]/95 border border-[#2a3242] rounded-lg p-2.5 space-y-1.5">
            <span className="text-[9px] font-mono text-[#5c6473] block uppercase tracking-wider mb-1">Leyenda</span>
            {(['router', 'phone', 'laptop', 'pc', 'tv', 'printer', 'iot', 'server', 'unknown'] as DeviceType[]).map((t) => (
              <div key={t} className="flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full" style={{ background: DEVICE_COLORS[t] }} />
                <span className="text-[10px] font-mono text-[#9ba3b4]">{DEVICE_LABELS[t]}</span>
              </div>
            ))}
            <div className="border-t border-[#2a3242] pt-1.5 mt-1.5 space-y-1">
              <div className="flex items-center gap-2">
                <span className="w-4 h-0.5 bg-sky-400" />
                <span className="text-[10px] font-mono text-[#9ba3b4]">Normal</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="w-4 h-0.5 bg-red-500" />
                <span className="text-[10px] font-mono text-[#9ba3b4]">Sospechosa</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Device inspector panel: 4 cols */}
      <div className="lg:col-span-4 space-y-4">
        {selectedDevice ? (
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col h-[620px] justify-between">
            {/* Header */}
            <div>
              <div className="flex items-center justify-between border-b border-[#2a3242] pb-3 mb-3">
                <div className="flex items-center gap-3">
                  <div
                    className="p-2.5 rounded-lg border"
                    style={{
                      background: `${DEVICE_COLORS[selectedDeviceType]}22`,
                      borderColor: `${DEVICE_COLORS[selectedDeviceType]}66`,
                    }}
                  >
                    <SelectedDeviceIcon
                      className="h-6 w-6"
                      style={{ color: DEVICE_COLORS[selectedDeviceType] }}
                    />
                  </div>
                  <div>
                    <h2 className="font-sans font-bold text-sm text-[#e4e7eb]">
                      {selectedDevice.hostname || selectedDevice.ip}
                    </h2>
                    <span className="text-[10px] font-mono text-[#5c6473]">
                      {DEVICE_LABELS[selectedDeviceType]}
                    </span>
                  </div>
                </div>
                <button
                  onClick={() => setSelectedDevice(null)}
                  className="text-[#5c6473] hover:text-[#e4e7eb] transition-colors"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>

              {/* Device info */}
              <div className="space-y-2 font-mono text-xs">
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><Zap className="h-3 w-3" />IP</span>
                  <span className="text-[#e4e7eb] font-semibold">{selectedDevice.ip}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><Globe className="h-3 w-3" />MAC</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.mac}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><Network className="h-3 w-3" />Vendor</span>
                  <span className="text-[#9ba3b4] text-right max-w-[180px] truncate" title={selectedDevice.vendor}>
                    {selectedDevice.vendor || 'Desconocido'}
                  </span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><Clock className="h-3 w-3" />Ult. Actividad</span>
                  <span className="text-[#9ba3b4]">{timeAgo(selectedDevice.last_seen)}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><Clock className="h-3 w-3" />Visto por 1ra vez</span>
                  <span className="text-[#9ba3b4]">{new Date(selectedDevice.first_seen).toLocaleDateString()}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 py-1.5">
                  <span className="text-[#5c6473] flex items-center gap-1.5"><AlertTriangle className="h-3 w-3" />Risk Score</span>
                  <span className={selectedDevice.risk_score ? 'text-amber-400' : 'text-[#9ba3b4]'}>
                    {selectedDevice.risk_score !== null ? selectedDevice.risk_score.toFixed(2) : 'N/A'}
                  </span>
                </div>
              </div>

              {/* Connections */}
              <div className="mt-4">
                {(() => {
                  // Group edges by the "other" device — A→B and B→A aggregate
                  // into a single row with combined bytes_count.
                  const grouped = new Map<number, number>();
                  selectedEdges.forEach((edge) => {
                    const otherId = edge.src_device_id === selectedDevice!.id
                      ? edge.dst_device_id
                      : edge.src_device_id;
                    grouped.set(otherId, (grouped.get(otherId) || 0) + edge.bytes_count);
                  });
                  const unique = Array.from(grouped.entries())
                    .sort((a, b) => b[1] - a[1])
                    .slice(0, 10);

                  return (
                    <>
                      <span className="text-[10px] font-mono text-[#5c6473] block uppercase tracking-wider mb-2">
                        Conexiones ({unique.length})
                      </span>
                      {unique.length === 0 ? (
                        <p className="text-xs text-[#5c6473] font-mono">Sin conexiones registradas.</p>
                      ) : (
                        <div className="space-y-1.5 max-h-32 overflow-y-auto pr-1">
                          {unique.map(([deviceId, totalBytes]) => {
                            const otherDev = devicesRef.current.find((d) => d.id === deviceId);
                            const isSusc = totalBytes > 1000000;
                            return (
                              <div
                                key={deviceId}
                                className={`flex items-center justify-between p-1.5 rounded text-[10px] font-mono ${
                                  isSusc ? 'bg-red-500/10 border border-red-500/20' : 'bg-[#0a0e14] border border-[#2a3242]/40'
                                }`}
                              >
                                <span className={isSusc ? 'text-red-400' : 'text-[#9ba3b4]'}>
                                  {otherDev?.ip || `#${deviceId}`}
                                </span>
                                <span className={isSusc ? 'text-red-400 font-bold' : 'text-[#5c6473]'}>
                                  {formatBytes(totalBytes)}
                                </span>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </>
                  );
                })()}
              </div>
            </div>

            {/* Action button */}
            <div className="pt-3">
              <button
                onClick={() => handleScanDevice(selectedDevice.ip)}
                className="w-full bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white py-2 rounded text-xs font-mono font-bold flex items-center justify-center gap-2 transition-all cursor-pointer"
              >
                <Play className="h-3.5 w-3.5 fill-current" />
                <span>Escanear este dispositivo</span>
              </button>
            </div>
          </div>
        ) : (
          <div className="bg-[#131820] border border-[#2a3242] p-12 rounded-lg flex flex-col items-center justify-center h-[620px] text-center">
            <div className="bg-[#0a0e14] p-4 rounded-full border border-[#2a3242] mb-4">
              <Wifi className="h-8 w-8 text-[#5c6473]" />
            </div>
            <h3 className="font-sans font-semibold text-sm text-[#e4e7eb]">Seleccione un Dispositivo</h3>
            <p className="text-xs text-[#9ba3b4] mt-2 max-w-[200px]">
              Cliquea un marcador en el mapa para ver la informacion completa del dispositivo y sus conexiones.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// ---- SVG path helper for Leaflet divIcons ----------------------------------

function getIconSvgPath(Icon: typeof HelpCircle): string {
  // Map lucide icon components to their SVG path data.
  // We hardcode the paths to avoid needing to render React components
  // inside a Leaflet divIcon (which uses raw HTML).
  const paths: Record<string, string> = {
    Router: '<path d="M5 10a10 10 0 0 1 14 0"/><path d="M8.5 13.5a5 5 0 0 1 7 0"/><path d="M12 17h.01"/><path d="M5 4a16 16 0 0 1 14 0"/>',
    Smartphone: '<rect width="14" height="20" x="5" y="2" rx="2" ry="2"/><path d="M12 18h.01"/>',
    Monitor: '<rect width="20" height="14" x="2" y="3" rx="2"/><line x1="8" x2="16" y1="21" y2="21"/><line x1="12" x2="12" y1="17" y2="21"/>',
    Tv: '<rect width="20" height="15" x="2" y="3" rx="2"/><polyline points="8 21 12 17 16 21"/>',
    Printer: '<polyline points="6 9 6 2 18 2 18 9"/><path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/><rect width="12" height="8" x="6" y="14"/>',
    Server: '<rect width="20" height="8" x="2" y="2" rx="2" ry="2"/><rect width="20" height="8" x="2" y="14" rx="2" ry="2"/><line x1="6" x2="6.01" y1="6" y2="6"/><line x1="6" x2="6.01" y1="18" y2="18"/>',
    HelpCircle: '<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>',
  };

  const name = Icon.displayName || 'HelpCircle';
  return paths[name] || paths.HelpCircle;
}