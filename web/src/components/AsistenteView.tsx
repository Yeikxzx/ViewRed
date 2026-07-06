import React, { useEffect, useRef, useState } from 'react';
import {
  Bot,
  BrainCircuit,
  ChevronRight,
  Cpu,
  MessageSquareText,
  Send,
  Sparkles,
  Terminal,
  Trash2,
  User,
} from 'lucide-react';
import { ChatMessage, NetworkAlert } from '../types';
import { llm } from '../lib/llm';

interface AsistenteViewProps {
  chatHistory: ChatMessage[];
  onSubmitMessage: (content: string) => void;
  onClearHistory: () => void;
  selectedAlertToExplain: NetworkAlert | null;
  onClearExplainedAlert: () => void;
}

const presetQueries = [
  'Resumen de las ultimas 24h',
  'Trafico anomalo detectado',
  'Vulnerabilidades criticas expuestas',
  'Dispositivos nuevos detectados',
];

function messageSender(message: ChatMessage) {
  return message.sender === 'admin' ? 'admin' : 'assistant';
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
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    llm.isAvailable().then(setLlmAvailable);
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatHistory]);

  useEffect(() => {
    if (selectedAlertToExplain) {
      onSubmitMessage(
        `Analiza la alerta activa: "${selectedAlertToExplain.title}" del host origen ${selectedAlertToExplain.source}`,
      );
      onClearExplainedAlert();
    }
  }, [selectedAlertToExplain, onSubmitMessage, onClearExplainedAlert]);

  const handleSend = (event: React.FormEvent) => {
    event.preventDefault();
    const content = inputValue.trim();
    if (!content) return;
    onSubmitMessage(content);
    setInputValue('');
  };

  return (
    <div id="ai-assistant-wrapper" className="flex h-[560px] flex-col overflow-hidden rounded-lg border border-[#263041] bg-[#111720] text-left font-sans">
      <div className="border-b border-[#263041] bg-[#0a0e14] p-4 select-none">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-blue-400/25 bg-blue-500/10 text-blue-300">
              <BrainCircuit className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-sm font-bold text-[#f8fafc]">Asistente de Inteligencia ViewRed</h2>
                <span
                  className={`rounded-md border px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider ${
                    llmAvailable
                      ? 'border-emerald-400/25 bg-emerald-500/10 text-emerald-300'
                      : 'border-amber-400/25 bg-amber-500/10 text-amber-300'
                  }`}
                >
                  {llmAvailable ? 'Conectado' : 'Mock'}
                </span>
              </div>
              <p className="mt-1 font-mono text-[11px] text-[#8792a8]">
                Qwen 2.5 3B Local · analisis de alertas, puertos, firmas IDS y hallazgos Nmap
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <div className="hidden items-center gap-2 rounded-lg border border-[#263041] bg-[#111720] px-3 py-2 font-mono text-[10px] text-[#8792a8] sm:flex">
              <Cpu className="h-3.5 w-3.5 text-blue-300" />
              <span>Contexto local</span>
            </div>
            <button
              type="button"
              onClick={onClearHistory}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-red-500/25 px-3 font-mono text-xs text-red-300 transition-all hover:bg-red-500/10"
              title="Borrar historial"
            >
              <Trash2 className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Limpiar</span>
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto bg-[#0d121a] p-4 select-text">
        {chatHistory.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <div className="max-w-md rounded-lg border border-[#263041] bg-[#111720] p-5 text-center">
              <div className="mx-auto mb-3 flex h-11 w-11 items-center justify-center rounded-lg border border-blue-400/25 bg-blue-500/10 text-blue-300">
                <MessageSquareText className="h-5 w-5" />
              </div>
              <h3 className="text-sm font-bold text-[#f8fafc]">Listo para investigar</h3>
              <p className="mt-2 text-xs leading-relaxed text-[#8792a8]">
                Pregunta por una alerta, una IP, un puerto o un escaneo reciente para obtener contexto operativo.
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {chatHistory.map((message) => {
              const sender = messageSender(message);
              const isAdmin = sender === 'admin';
              return (
                <div key={message.id} className={`flex items-start gap-3 ${isAdmin ? 'justify-end' : 'justify-start'}`}>
                  {!isAdmin && (
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-blue-400/25 bg-blue-500/10 text-blue-300">
                      <Bot className="h-4 w-4" />
                    </div>
                  )}

                  <div className={`max-w-[82%] space-y-1 ${isAdmin ? 'items-end text-right' : 'items-start text-left'}`}>
                    <div className={`font-mono text-[10px] uppercase tracking-wider ${isAdmin ? 'text-blue-300' : 'text-[#657089]'}`}>
                      {isAdmin ? 'Operador' : 'ViewRed AI'}
                    </div>
                    <div
                      className={`rounded-lg border px-4 py-3 text-xs leading-relaxed shadow-sm ${
                        isAdmin
                          ? 'border-blue-400/30 bg-blue-500 text-white'
                          : 'border-[#263041] bg-[#111720] font-mono text-[#b8c3d7] whitespace-pre-wrap'
                      }`}
                    >
                      {message.content}
                    </div>
                  </div>

                  {isAdmin && (
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-[#334155] bg-[#182131] text-[#f8fafc]">
                      <User className="h-4 w-4" />
                    </div>
                  )}
                </div>
              );
            })}
            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      <div className="border-t border-[#263041] bg-[#0a0e14] p-3 select-none">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 font-mono text-[10px] font-bold uppercase tracking-wider text-[#657089]">
            <Sparkles className="h-3 w-3 text-blue-300" />
            Sugerencias
          </span>
          {presetQueries.map((query) => (
            <button
              key={query}
              type="button"
              onClick={() => onSubmitMessage(query)}
              className="group inline-flex items-center gap-1.5 rounded-md border border-[#263041] bg-[#111720] px-2.5 py-1.5 font-mono text-[10px] text-[#aab6c9] transition-all hover:border-blue-400/35 hover:bg-[#182131] hover:text-white"
            >
              {query}
              <ChevronRight className="h-3 w-3 text-[#657089] transition-transform group-hover:translate-x-0.5" />
            </button>
          ))}
        </div>

        <form onSubmit={handleSend} className="rounded-lg border border-[#263041] bg-[#111720] p-2">
          <div className="flex items-center gap-2">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-[#263041] bg-[#0a0e14] text-blue-300">
              <Terminal className="h-4 w-4" />
            </div>
            <input
              type="text"
              value={inputValue}
              onChange={(event) => setInputValue(event.target.value)}
              placeholder="Pregunta sobre anomalias, puertos, logs de Nmap o firmas IDS..."
              className="h-9 min-w-0 flex-1 bg-transparent px-1 text-xs text-[#e5edf8] outline-none placeholder:text-[#566176]"
            />
            <button
              type="submit"
              className="inline-flex h-9 items-center justify-center gap-2 rounded-md bg-blue-500 px-3 font-mono text-xs font-bold text-white transition-all hover:bg-blue-400 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={!inputValue.trim()}
            >
              <Send className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Enviar</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
