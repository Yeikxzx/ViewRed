import React, { useState, useRef, useEffect } from 'react';
import { Sparkles, Send, Bot, User, Trash2, ArrowRight } from 'lucide-react';
import { ChatMessage, NetworkAlert } from '../types';
import { llm } from '../lib/llm';

interface AsistenteViewProps {
  chatHistory: ChatMessage[];
  onSubmitMessage: (content: string) => void;
  onClearHistory: () => void;
  selectedAlertToExplain: NetworkAlert | null;
  onClearExplainedAlert: () => void;
}

export default function AsistenteView({
  chatHistory,
  onSubmitMessage,
  onClearHistory,
  selectedAlertToExplain,
  onClearExplainedAlert,
}: AsistenteViewProps) {
  const [inputValue, setInputValue] = useState('');
  const [llmAvailable, setLlmAvailable] = useState(false);

  useEffect(() => {
    llm.isAvailable().then(setLlmAvailable);
  }, []);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatHistory]);

  // Handle preset suggestions
  const presetQueries = [
    'Resumen de las últimas 24h',
    'Tráfico anómalo detectado',
    'Vulnerabilidades críticas expuestas',
    'Dispositivos nuevos en VLAN 20',
  ];

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputValue.trim()) return;
    onSubmitMessage(inputValue);
    setInputValue('');
  };

  const handlePresetClick = (query: string) => {
    onSubmitMessage(query);
  };

  // Auto trigger query if alert was specified to analyze
  useEffect(() => {
    if (selectedAlertToExplain) {
      onSubmitMessage(`Analiza la alerta crítica activa: "${selectedAlertToExplain.title}" del host origen: ${selectedAlertToExplain.source}`);
      onClearExplainedAlert();
    }
  }, [selectedAlertToExplain]);

  return (
    <div id="ai-assistant-wrapper" className="flex flex-col h-[520px] bg-[#131820] border border-[#2a3242] rounded-lg overflow-hidden text-left font-sans">
      {/* Header telemetry info */}
      <div className="bg-[#0a0e14] border-b border-[#2a3242] p-4 flex justify-between items-center select-none">
        <div className="flex items-center gap-2.5">
          <div className="bg-[#3b82f6]/10 text-[#3b82f6] p-1.5 rounded flex items-center justify-center">
            <Sparkles className="h-4 w-4 animate-pulse" />
          </div>
          <div className="flex flex-col">
            <span className="font-sans font-bold text-xs text-[#e4e7eb]">
              Asistente de Inteligencia ViewRed
            </span>
            <span className={`font-mono text-[9px] uppercase tracking-wider ${llmAvailable ? 'text-[#10b981]' : 'text-[#f59e0b]'}`}>
              Modelo Activo: Qwen 2.5 (3B Local) • {llmAvailable ? 'CONECTADO' : 'MOCK'}
            </span>
          </div>
        </div>
        <button
          onClick={onClearHistory}
          className="text-[#5c6473] hover:text-red-400 p-1.5 rounded hover:bg-[#1c2230] transition-colors"
          title="Borrar historial"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>

      {/* Messages layout */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 bg-[#10141a]/40 select-text">
        {chatHistory.map((msg) => {
          const isAdmin = msg.sender === 'admin';
          return (
            <div
              key={msg.id}
              className={`flex items-start gap-3 max-w-[85%] ${isAdmin ? 'ml-auto flex-row-reverse' : ''}`}
            >
              {/* Profile icon */}
              <div
                className={`w-7 h-7 rounded-sm flex items-center justify-center border shrink-0 ${
                  isAdmin ? 'bg-[#1c2230] border-[#2a3242]' : 'bg-[#3b82f6]/10 border-[#3b82f6]/30'
                }`}
              >
                {isAdmin ? (
                  <User className="h-3.5 w-3.5 text-[#e4e7eb]" />
                ) : (
                  <Bot className="h-3.5 w-3.5 text-[#3b82f6]" />
                )}
              </div>

              {/* Bubbles content */}
              <div
                className={`p-3 rounded-lg text-xs leading-relaxed ${
                  isAdmin
                    ? 'bg-[#3b82f6] text-white font-medium rounded-tr-none'
                    : 'bg-[#131820] border border-[#2a3242]/70 text-[#9ba3b4] rounded-tl-none whitespace-pre-wrap font-mono select-text prose-invert select-text'
                }`}
              >
                {msg.content}
              </div>
            </div>
          );
        })}
        <div ref={messagesEndRef} />
      </div>

      {/* Preset quick buttons */}
      <div className="bg-[#0a0e14]/50 border-t border-[#1c2230]/70 p-2.5 flex flex-wrap gap-2 items-center select-none">
        <span className="text-[9px] font-mono font-bold text-[#5c6473] uppercase mr-1">Sugerencias:</span>
        {presetQueries.map((query, idx) => (
          <button
            key={idx}
            onClick={() => handlePresetClick(query)}
            className="text-[10px] bg-[#131820] hover:bg-[#1c2230] text-[#9ba3b4] hover:text-white px-2.5 py-1 rounded border border-[#2a3242]/60 font-mono transition-all cursor-pointer"
          >
            {query}
          </button>
        ))}
      </div>

      {/* Input controls form */}
      <form
        onSubmit={handleSend}
        className="bg-[#0a0e14] border-t border-[#2a3242] p-3 flex gap-2 items-center"
      >
        <input
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          placeholder="Pregunte sobre anomalías, puertos, logs de Nmap o firmas de IDS..."
          className="flex-1 bg-[#131820] border border-[#2a3242] focus:border-[#3b82f6] rounded px-3 py-2 text-xs text-[#e4e7eb] outline-none font-sans"
        />
        <button
          type="submit"
          className="bg-[#3b82f6] hover:bg-[#3b82f6]/95 text-white p-2 rounded flex items-center justify-center transition-all cursor-pointer shadow-lg shadow-[#3b82f6]/10"
        >
          <Send className="h-3.5 w-3.5" />
        </button>
      </form>
    </div>
  );
}
