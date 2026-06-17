import { useState } from 'react';
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

import { ViewType, NetworkAlert, Vulnerability, ChatMessage } from './types';
import {
  initialAlerts,
  initialDevices,
  initialVulnerabilities,
} from './data/mockData';

export default function App() {
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

  const handleSendMessage = (content: string) => {
    const userMsg: ChatMessage = {
      id: Date.now(),
      sender: 'admin',
      content,
    };
    setChatHistory((prev) => [...prev, userMsg]);

    // Simulate an incredibly rich technical bot reply with realistic latency
    setTimeout(() => {
      let replyContent = '';
      const query = content.toLowerCase();

      if (query.includes('resumen') || query.includes('24h')) {
        replyContent = `[REPORTE CONTEXTUAL VIEWRED SEC-OPS]\n\nDurante las últimas 24 horas, la sonda de captura pcap ha registrado:\n- Tráfico acumulado: 45.2 GB\n- Paquetes totales: 14.5M\n- Alertas IDS: 3 críticas (vulnerabilidades LDAP y escaneo de puertos).\n\nHost con mayor throughput: web-srv-prod (192.168.1.10) con una tasa promedio de 85 Mbps. Se recomienda auditar los CVEs expuestos en este Endpoint ya que contiene riesgos lógicos no mitigados (CVE-2021-44228).`;
      } else if (query.includes('anómalo') || query.includes('trafico') || query.includes('tráfico')) {
        replyContent = `[ANÁLISIS DE ANOMALÍAS DE RED]\n\nSe han detectado comportamientos fuera de perfil en la interfaz para:\n1. Host: 192.168.1.100 (replica-db-srv)\n   - Tipo: Conexión persistente anómala LDAP a puerto no estándar.\n   - Clasificación catastrófica Suricata: Alta.\n2. Host: 10.0.5.12 (api-gateway)\n   - Tipo: Ráfagas repetitivas indicativas de HTTP/2 Rapid Reset.\n\nAmbas conexiones han sido registradas en la cola de Alertas de Seguridad. ¿Desea que simulemos un bloqueo preventivo?`;
      } else if (query.includes('vulnerabilidades') || query.includes('cve') || query.includes('críticas')) {
        replyContent = `[AUDITORÍA DE RIESGO DE DISPOSITIVIVOS]\n\nEstado de la superficie de ataque:\n- CVE-2021-44228 (Log4Shell, CVSS 10.0): Activo en web-srv-prod. Permite ejecución remota de binarios maliciosos.\n- CVE-2023-44487 (HTTP/2 Rapid Reset, CVSS 7.5): Activo en api-gateway. Riesgo moderado de DoS.\n\nAcción recomendada: Navegue a la pestaña 'Vulnerabilidades', seleccione el host expuesto y presione 'Mitigar Riesgo' para simular el despliegue de parches del kernel.`;
      } else if (query.includes('alerta') || query.includes('analiza')) {
        replyContent = `[ANÁLISIS DE FIRMA SURICATA]\n\nAnalizando la incidencia de seguridad reportada:\n- Identificador: ${
          alerts[0]?.id || 'AL-512'
        }\n- Firma principal: LDAP Remote Command Injection Attempt\n- Nivel de Confianza: 98%\n- Mitigación sugerida: Bloquear el tráfico entrante de la IP remota o actualizar el middleware LDAP vulnerable.`;
      } else {
        replyContent = `[SOPORTE DE RED VIEWRED]\n\nHe recibido su consulta técnica: "${content}".\n\nComo modelo de IA SecOps, he analizado los metadatos pcap activos y no encuentro anomalías severas adicionales asociadas a esos parámetros. Los routers internos reportan estabilidad total y cero paquetes descartados por buffers de memoria. \n\n¿Desea iniciar una sonda de escaneo Nmap en la topologia para encontrar puertos vulnerables abiertos en el sub-segmento?`;
      }

      const botMsg: ChatMessage = {
        id: Date.now() + 1,
        sender: 'bot',
        content: replyContent,
      };
      setChatHistory((prev) => [...prev, botMsg]);
    }, 800);
  };

  const activeAlertCount = alerts.filter((a) => !a.acknowledged).length;

  return (
    <div id="viewred-app-root" className="flex h-screen bg-[#0a0e14] overflow-hidden select-none">
      {/* Sidebar Control left */}
      <Sidebar
        currentView={currentView}
        setView={setView}
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
          {currentView === 'dashboard' && (
            <DashboardView
              setView={setView}
              alerts={alerts}
              setSelectedAlert={setSelectedAlertToExplain}
            />
          )}

          {currentView === 'captura' && (
            <CapturaView isCapturing={isCapturing} toggleCapture={toggleCapture} />
          )}

          {currentView === 'filtros' && <FiltrosView />}

          {currentView === 'alertas' && (
            <AlertasView
              alerts={alerts}
              onAcknowledgeAlert={handleAcknowledgeAlert}
              setSelectedAlertToExplain={setSelectedAlertToExplain}
              setView={setView}
            />
          )}

          {currentView === 'topologia' && (
            <TopologiaView
              devices={devices}
              setView={setView}
              setSelectedDeviceIP={setSelectedDeviceIP}
            />
          )}

          {currentView === 'vulnerabilidades' && (
            <VulnerabilidadesView
              vulnerabilities={vulnerabilities}
              onMitigateVuln={handleMitigateVuln}
              selectedDeviceIP={selectedDeviceIP}
              setSelectedDeviceIP={setSelectedDeviceIP}
            />
          )}

          {currentView === 'asistente' && (
            <AsistenteView
              chatHistory={chatHistory}
              onSubmitMessage={handleSendMessage}
              onClearHistory={handleClearChatHistory}
              selectedAlertToExplain={selectedAlertToExplain}
              onClearExplainedAlert={() => setSelectedAlertToExplain(null)}
            />
          )}

          {currentView === 'configuracion' && <ConfiguracionView />}
        </main>
      </div>
    </div>
  );
}
