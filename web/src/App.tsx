import { useState } from 'react';
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

import { ViewType, NetworkAlert, Vulnerability, ChatMessage } from './types';
import { llm, type Message } from './lib/llm';
import {
  initialAlerts,
  initialDevices,
  initialVulnerabilities,
} from './data/mockData';

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
  const [alerts, setAlerts] = useState<NetworkAlert[]>(initialAlerts);
  const [vulnerabilities, setVulnerabilities] = useState<Vulnerability[]>(initialVulnerabilities);
  const [selectedDeviceIP, setSelectedDeviceIP] = useState<string>('');
  const [selectedAlertToExplain, setSelectedAlertToExplain] = useState<NetworkAlert | null>(null);

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

  const handleAcknowledgeAlert = (id: string) => {
    setAlerts((prev) =>
      prev.map((alert) => (alert.id === id ? { ...alert, acknowledged: true } : alert))
    );
  };

  const handleMitigateVuln = (id: string) => {
    setVulnerabilities((prev) =>
      prev.map((v) => (v.id === id ? { ...v, mitigated: true } : v))
    );
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

    const messages: Message[] = updatedHistory.map((msg) => ({
      role: msg.sender === 'admin' ? 'user' : 'assistant',
      content: msg.content,
    }));

    try {
      const replyContent = await llm.chat(messages);
      const botMsg: ChatMessage = {
        id: Date.now() + 1,
        sender: 'bot',
        content: replyContent,
      };
      setChatHistory((prev) => [...prev, botMsg]);
    } catch (err) {
      const botMsg: ChatMessage = {
        id: Date.now() + 1,
        sender: 'bot',
        content: `[Error] No se pudo contactar al modelo: ${err}`,
      };
      setChatHistory((prev) => [...prev, botMsg]);
    }
  };

  const activeAlertCount = alerts.filter((a) => !a.acknowledged).length;

  return (
    <div id="viewred-app-root" className="flex h-screen bg-[#0a0e14] overflow-hidden select-none">
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
                    alerts={alerts}
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
