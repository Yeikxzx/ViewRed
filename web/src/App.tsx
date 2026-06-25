import { useState, useEffect, useCallback } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import Sidebar from './components/Sidebar';
import Header from './components/Header';
import DashboardView from './components/DashboardView';
import CapturaView from './components/CapturaView';
import FiltrosView from './components/FiltrosView';
import AlertasView from './components/AlertasView';
import TopologiaView from './components/TopologiaView';
import VulnerabilidadesView from './components/VulnerabilidadesView';
import AsistenteView from './components/AsistenteView';
import ConfiguracionView from './components/ConfiguracionView';
import NotFoundView from './components/NotFoundView';
import { Toaster, toast } from 'sonner';

import { ViewType, NetworkAlert, Vulnerability, ChatMessage } from './types';
import { llm, type Message } from './lib/llm';
import { getRecentAlerts, acknowledgeAlert, AlertBackend } from './lib/api';
import { useSSE } from './lib/sse';
import { useSSEStore } from './store/sse-store';
import {
  initialDevices,
} from './data/mockData';
import { getVulnerabilities, mitigateVulnerability, VulnerabilityBackend } from './lib/api';

export default function App() {
  // currentView is kept only for Header active-state styling during the
  // 6-step migration; the URL (via <Routes>) is now the source of truth
  // for which view is rendered. Task 22 switched the Sidebar to React
  // Router <NavLink> components, so the Sidebar no longer reads
  // currentView/setView. Header and the view components still consume
  // them; a future task will remove the state entirely.
  const [currentView, setView] = useState<ViewType>('dashboard');
  const [collapsed, setCollapsed] = useState(false);
  const [isCapturing, setIsCapturing] = useState(true);

  // Core App states
  const [devices, setDevices] = useState(initialDevices);
  const [alerts, setAlerts] = useState<NetworkAlert[]>([]);
  const [vulnerabilities, setVulnerabilities] = useState<Vulnerability[]>([]);
  const [selectedDeviceIP, setSelectedDeviceIP] = useState<string>('');
  const [selectedAlertToExplain, setSelectedAlertToExplain] = useState<NetworkAlert | null>(null);

  // Callback to process an alert from SSE
  const handleAlertEvent = useCallback((payload: unknown) => {
    try {
      const a: AlertBackend = payload as AlertBackend;
      const newAlert: NetworkAlert = {
        id: String(a.id),
        timestamp: a.time,
        severity: (a.severity?.toUpperCase() === 'CRITICAL' || a.severity?.toUpperCase() === 'WARNING') 
          ? (a.severity.toUpperCase() as 'CRITICAL' | 'WARNING') 
          : 'INFO',
        title: a.title,
        description: a.description || '',
        source: a.source,
        destination: 'Desconocido',
        confidence: a.score,
        origin: 'Detección Local',
        timeAgo: 'Hace un momento',
        acknowledged: !!a.acknowledged_at,
      };
      setAlerts((prev) => [newAlert, ...prev]);

      // Trigger toast notification
      if (newAlert.severity === 'CRITICAL') {
        toast.error(newAlert.title, { description: newAlert.source });
      } else if (newAlert.severity === 'WARNING') {
        toast.warning(newAlert.title, { description: newAlert.source });
      } else {
        toast.info(newAlert.title, { description: newAlert.source });
      }
    } catch (err) {
      console.error('Error parsing alert SSE', err);
    }
  }, []);

  // SSE alerts with automatic exponential backoff via useSSE hook
  const { lastEvent: alertEvent } = useSSE<AlertBackend>('/api/v1/stream/alerts', 'alert');
  useEffect(() => {
    if (alertEvent) handleAlertEvent(alertEvent);
  }, [alertEvent, handleAlertEvent]);

  // Listen for capture-status events to update the store
  const { lastEvent: captureStatusEvent } = useSSE<{ status: string }>('/api/v1/stream/capture-status', 'capture-status');
  useEffect(() => {
    if (captureStatusEvent?.status) {
      useSSEStore.getState().setCaptureStatus(captureStatusEvent.status as 'running' | 'paused' | 'unknown');
    }
  }, [captureStatusEvent]);

  // Fetch initial alerts and vulnerabilities
  useEffect(() => {
    getRecentAlerts(100).then(setAlerts).catch(console.error);
    fetchVulnerabilities();
  }, []);

  // Chat conversation logs state
  const [chatHistory, setChatHistory] = useState<ChatMessage[]>([
    {
      id: 1,
      sender: 'bot',
      content:
        'Soporte Inteligente ViewRed de SecOps listo. Puedo responder preguntas sobre el tráfico de red actual, explicar firmas de Suricata, decodificar paquetes Hex Dump o recomendar mititgaciones de CVEs.',
    },
  ]);

  // Actions handlers
  const toggleCapture = () => {
    setIsCapturing(!isCapturing);
  };

  const handleAcknowledgeAlert = async (id: string) => {
    try {
      await acknowledgeAlert(id);
      setAlerts((prev) =>
        prev.map((alert) => (alert.id === id ? { ...alert, acknowledged: true } : alert))
      );
    } catch (err) {
      console.error('Failed to acknowledge alert', err);
    }
  };

  const fetchVulnerabilities = async () => {
    try {
      const backendVulns = await getVulnerabilities();
      const mapped = backendVulns.map((v: VulnerabilityBackend): Vulnerability => ({
        id: String(v.id),
        cve: v.cve_id || 'INFO-PORT',
        title: v.description.split('\n')[0] || 'Vulnerabilidad Detectada',
        description: v.description,
        severity: (v.cvss_score && v.cvss_score >= 9.0) ? 'CRITICAL' : (v.cvss_score && v.cvss_score >= 7.0) ? 'HIGH' : 'MEDIUM',
        cvss: v.cvss_score ? v.cvss_score : 0,
        targetDevice: 'Device',
        targetIP: v.device_ip,
        mitigated: v.mitigated,
        mitigationSteps: 'Aplicar parche o deshabilitar puerto ' + (v.port || ''),
        detectedAt: v.discovered_at,
        scanId: v.id,
      }));
      setVulnerabilities(mapped);
    } catch (err) {
      console.error('Failed to fetch vulnerabilities', err);
    }
  };

  const handleMitigateVuln = async (id: string) => {
    try {
      await mitigateVulnerability(parseInt(id));
      setVulnerabilities((prev) =>
        prev.map((v) => (v.id === id ? { ...v, mitigated: true } : v))
      );
      toast.success('Vulnerabilidad mitigada exitosamente');
    } catch (err) {
      console.error('Failed to mitigate vuln', err);
      toast.error('Error al mitigar vulnerabilidad');
    }
  };

  const handleClearChatHistory = () => {
    setChatHistory([
      {
        id: Date.now(),
        sender: 'bot',
        content: 'Cola de mensajes limpiada. ¿Cómo le puedo asistir a continuación?',
      },
    ]);
  };

  const handleSendMessage = async (content: string) => {
    const userMsg: ChatMessage = {
      id: Date.now(),
      sender: 'admin',
      content,
    };
    const updatedHistory: ChatMessage[] = [...chatHistory, userMsg];
    setChatHistory(updatedHistory);

    const botMsgId = Date.now() + 1;
    setChatHistory((prev) => [...prev, { id: botMsgId, sender: 'bot', content: '' }]);

    const messages: Message[] = updatedHistory.map((msg) => ({
      role: msg.sender === 'admin' ? 'user' : 'assistant',
      content: msg.content,
    }));

    try {
      await llm.stream(messages, (token) => {
        setChatHistory((prev) => prev.map(msg => 
          msg.id === botMsgId ? { ...msg, content: msg.content + token } : msg
        ));
      });
    } catch (err) {
      setChatHistory((prev) => prev.map(msg => 
        msg.id === botMsgId ? { ...msg, content: msg.content + "\n\n[Error de conexión con el Asistente AI]" } : msg
      ));
    }
  };

  const activeAlertCount = alerts.filter((a) => !a.acknowledged).length;

  return (
    <div id="viewred-app-root" className="flex h-screen bg-[#0a0e14] overflow-hidden select-none">
        <Toaster theme="dark" richColors position="top-right" />
        {/* Sidebar Control left */}
        <Sidebar
          collapsed={collapsed}
          setCollapsed={setCollapsed}
          activeAlertCount={activeAlertCount}
        />

        {/* Main Container body right */}
        <div id="main-content-wrapper" className="flex-1 flex flex-col overflow-hidden">
          {/* Header telemetry top */}
          <Header
            currentView={currentView}
            setView={setView}
            isCapturing={isCapturing}
            toggleCapture={toggleCapture}
            openAIAssistant={() => setView('asistente')}
          />

          {/* Dynamic view routes body content */}
          <main id="app-viewport" className="flex-1 overflow-y-auto p-6 scroll-smooth">
            <Routes>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route
                path="/dashboard"
                element={
                  <DashboardView
                    setView={setView}
                    setSelectedAlert={setSelectedAlertToExplain}
                  />
                }
              />
              <Route
                path="/captura"
                element={<CapturaView isCapturing={isCapturing} toggleCapture={toggleCapture} />}
              />
              <Route path="/filtros" element={<FiltrosView />} />
              <Route
                path="/alertas"
                element={
                  <AlertasView
                    alerts={alerts}
                    onAcknowledgeAlert={handleAcknowledgeAlert}
                    setSelectedAlertToExplain={setSelectedAlertToExplain}
                    setView={setView}
                  />
                }
              />
              <Route
                path="/topologia"
                element={
                  <TopologiaView
                    devices={devices}
                    setView={setView}
                    setSelectedDeviceIP={setSelectedDeviceIP}
                  />
                }
              />
              <Route
                path="/vulnerabilidades"
                element={
                  <VulnerabilidadesView
                    vulnerabilities={vulnerabilities}
                    onMitigateVuln={handleMitigateVuln}
                    selectedDeviceIP={selectedDeviceIP}
                    setSelectedDeviceIP={setSelectedDeviceIP}
                    refreshVulnerabilities={fetchVulnerabilities}
                  />
                }
              />
              <Route
                path="/asistente"
                element={
                  <AsistenteView
                    chatHistory={chatHistory}
                    onSubmitMessage={handleSendMessage}
                    onClearHistory={handleClearChatHistory}
                    selectedAlertToExplain={selectedAlertToExplain}
                    onClearExplainedAlert={() => setSelectedAlertToExplain(null)}
                  />
                }
              />
              <Route path="/configuracion" element={<ConfiguracionView />} />
              <Route path="*" element={<NotFoundView />} />
            </Routes>
          </main>
        </div>
      </div>
  );
}
