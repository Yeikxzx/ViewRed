import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Network,
  Smartphone,
  Server,
  HelpCircle,
  X,
  Play,
  Monitor,
  Printer
} from 'lucide-react';
import { 
  ReactFlow, 
  useNodesState, 
  useEdgesState, 
  Background, 
  Controls, 
  Node, 
  Edge,
  Handle,
  Position
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import { NetworkDevice, ViewType } from '../types';

interface TopologiaViewProps {
  devices: NetworkDevice[];
  setView: (view: ViewType) => void;
  setSelectedDeviceIP: (ip: string) => void;
}

// Custom Node for devices
const DeviceNode = ({ data, isConnectable }: any) => {
  const { device } = data;
  
  // Decide icon based on vendor heuristics
  let Icon = HelpCircle;
  const vendor = (device.vendor || '').toLowerCase();
  if (vendor.includes('apple') || vendor.includes('samsung') || vendor.includes('huawei')) Icon = Smartphone;
  else if (vendor.includes('cisco') || vendor.includes('juniper') || vendor.includes('ubiquiti')) Icon = Server;
  else if (vendor.includes('hp') || vendor.includes('epson')) Icon = Printer;
  else if (vendor.includes('intel') || vendor.includes('dell')) Icon = Monitor;

  // Decide if offline (> 5 mins without traffic)
  const lastSeenDate = new Date(device.last_seen);
  const isOffline = (new Date().getTime() - lastSeenDate.getTime()) > 5 * 60 * 1000;

  return (
    <div className={`flex items-center gap-2 px-3 py-2 border rounded-lg bg-[#1c2230] shadow-sm transition-all ${isOffline ? 'opacity-50 border-[#5c6473]' : 'border-[#3b82f6] shadow-[#3b82f6]/20'}`}>
      <Handle type="target" position={Position.Top} isConnectable={isConnectable} className="w-2 h-2 bg-[#5c6473] border-none" />
      <div className="flex-shrink-0">
        <Icon className={`h-5 w-5 ${isOffline ? 'text-[#5c6473]' : 'text-[#3b82f6]'}`} />
      </div>
      <div className="flex flex-col text-left">
        <span className="text-[#e4e7eb] font-mono text-[11px] font-bold leading-tight">
          {device.hostname || device.ip}
        </span>
        {device.hostname && (
          <span className="text-[#9ba3b4] font-mono text-[9px] leading-tight">
            {device.ip}
          </span>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} isConnectable={isConnectable} className="w-2 h-2 bg-[#5c6473] border-none" />
    </div>
  );
};

export default function TopologiaView({ setView, setSelectedDeviceIP }: TopologiaViewProps) {
  const [selectedDevice, setSelectedDevice] = useState<any>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Memoize nodeTypes so React Flow doesn't recreate them every render
  const nodeTypes = useMemo(() => ({ device: DeviceNode }), []);

  useEffect(() => {
    const fetchTopology = async () => {
      try {
        const res = await fetch('/api/v1/topology');
        if (!res.ok) throw new Error('Failed to fetch topology');
        const data = await res.json();
        
        const radius = 250;
        const cx = 350;
        const cy = 250;

        // Use a functional state update to preserve dragged node positions
        setNodes((currentNodes) => {
          // Copy current nodes to easily find them
          const nextNodes = [...currentNodes];
          
          (data.nodes || []).forEach((dev: any, index: number, arr: any[]) => {
            const existingNodeIndex = nextNodes.findIndex(n => n.id === String(dev.id));
            if (existingNodeIndex >= 0) {
              // Node exists, just update data
              nextNodes[existingNodeIndex] = {
                ...nextNodes[existingNodeIndex],
                data: { ...nextNodes[existingNodeIndex].data, device: dev, label: dev.hostname || dev.ip }
              };
            } else {
              // New node, place it in a circular layout
              const angle = (index / arr.length) * 2 * Math.PI;
              nextNodes.push({
                id: String(dev.id),
                type: 'device',
                position: {
                  x: cx + radius * Math.cos(angle),
                  y: cy + radius * Math.sin(angle),
                },
                data: { label: dev.hostname || dev.ip, device: dev },
              });
            }
          });
          return nextNodes;
        });

        // Edges can be fully replaced since React Flow matches them by ID naturally
        setEdges((data.edges || []).map((edge: any) => ({
          id: String(edge.id),
          source: String(edge.src_device_id),
          target: String(edge.dst_device_id),
          animated: true,
          style: { 
            stroke: edge.bytes_count > 1000000 ? '#ef4444' : '#3b82f6', 
            strokeWidth: Math.max(1, Math.min(edge.bytes_count / 10000, 5)) 
          }
        })));

      } catch (err) {
        console.error('Topology fetch error:', err);
      }
    };
    
    fetchTopology();
    const interval = setInterval(fetchTopology, 5000);
    return () => clearInterval(interval);
  }, [setNodes, setEdges]);

  const onNodeClick = useCallback((event: any, node: Node) => {
    setSelectedDevice(node.data.device);
  }, []);

  const handleScanDevice = (ip: string) => {
    setSelectedDeviceIP(ip);
    setView('vulnerabilidades');
  };

  return (
    <div id="topologia-view-wrapper" className="grid grid-cols-1 lg:grid-cols-12 gap-6 text-left">
      {/* 2D interactive canvas: 8 Cols */}
      <div className="lg:col-span-8 space-y-4">
        <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col justify-between h-[520px] relative">
          <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 z-10">
            <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight flex items-center gap-2">
              <Network className="h-4 w-4 text-[#3b82f6]" />
              <span>Grafo de Topología Activo</span>
            </h2>
          </div>

          <div className="flex-1 w-full bg-[#0a0e14] border border-[#1c2230]/70 rounded mt-3 relative overflow-hidden">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onNodeClick={onNodeClick}
              colorMode="dark"
              fitView
            >
              <Background color="#1c2230" gap={16} />
              <Controls />
            </ReactFlow>
          </div>
        </div>
      </div>

      {/* Selected Node Inspector side panel: 4 Cols */}
      <div className="lg:col-span-4 space-y-4">
        {selectedDevice ? (
          <div className="bg-[#131820] border border-[#2a3242] p-4 rounded-lg flex flex-col h-[520px] justify-between">
            <div>
              <div className="flex justify-between items-center border-b border-[#2a3242] pb-2.5 mb-3.5">
                <h2 className="font-sans font-bold text-sm text-[#e4e7eb] tracking-tight">
                  Inspección: {selectedDevice.hostname || selectedDevice.ip}
                </h2>
              </div>

              <div className="space-y-3 font-mono text-xs">
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">IP Address</span>
                  <span className="text-[#e4e7eb] font-semibold">{selectedDevice.ip}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">MAC Address</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.mac}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">Distribuidor (OEM)</span>
                  <span className="text-[#9ba3b4]">{selectedDevice.vendor || 'Desconocido'}</span>
                </div>
                <div className="flex justify-between border-b border-[#1c2230]/40 pb-1.5">
                  <span className="text-[#5c6473]">Último Registro</span>
                  <span className="text-[#9ba3b4]">{new Date(selectedDevice.last_seen).toLocaleString()}</span>
                </div>
              </div>
            </div>

            <div className="space-y-2 pt-2">
              <button
                onClick={() => handleScanDevice(selectedDevice.ip)}
                className="w-full bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white py-1.5 rounded text-xs font-mono font-bold flex items-center justify-center gap-2 transition-all cursor-pointer"
              >
                <Play className="h-3.5 w-3.5 fill-current" />
                <span>Escanear este dispositivo</span>
              </button>
            </div>
          </div>
        ) : (
          <div className="bg-[#131820] border border-[#2a3242] p-12 rounded-lg flex flex-col items-center justify-center h-[520px] text-center">
            <X className="h-10 w-10 text-[#5c6473] mb-3" />
            <h3 className="font-sans font-semibold text-sm text-[#e4e7eb]">Seleccione un Endpoint</h3>
            <p className="text-xs text-[#9ba3b4] mt-1">Interactúe con el mapa de topología para auditar en profundidad.</p>
          </div>
        )}
      </div>
    </div>
  );
}
