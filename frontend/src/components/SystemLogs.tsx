import { useEffect, useRef, useState } from 'react';
import { useAuth } from '../context/AuthContext';

// Simple ANSI to HTML parser for our specific colors
const formatLogLine = (line: string) => {
  // Regex to match color codes
  const parts = line.split(/(\x1b\[\d+m)/g);
  
  let currentColor = 'text-gray-300';
  
  return (
    <span>
      {parts.map((part, i) => {
        if (part === '\x1b[32m') { currentColor = 'text-green-400'; return null; }
        if (part === '\x1b[33m') { currentColor = 'text-yellow-400'; return null; }
        if (part === '\x1b[34m') { currentColor = 'text-blue-400'; return null; }
        if (part === '\x1b[31m') { currentColor = 'text-red-400'; return null; }
        if (part === '\x1b[36m') { currentColor = 'text-cyan-400'; return null; }
        if (part === '\x1b[0m') { currentColor = 'text-gray-300'; return null; }
        if (part.startsWith('\x1b')) return null; // Ignore other codes

        return <span key={i} className={currentColor}>{part}</span>;
      })}
    </span>
  );
};

export default function SystemLogs() {
  const [logs, setLogs] = useState<string[]>([]);
  const { token } = useAuth();
  const wsRef = useRef<WebSocket | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');

  useEffect(() => {
    if (!token) return;

    const connect = () => {
      const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api';
      // Replace http/https with ws/wss
      const wsProtocol = apiUrl.startsWith('https') ? 'wss' : 'ws';
      const wsBase = apiUrl.replace(/^https?:\/\//, '');
      const wsUrl = `${wsProtocol}://${wsBase}/monitor/logs?token=${token}`;

      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setStatus('connected');
        setLogs(prev => [...prev, '\x1b[32m[System] Connected to log stream...\x1b[0m']);
      };

      ws.onmessage = (event) => {
        setLogs((prev) => {
          const newLogs = [...prev, event.data];
          if (newLogs.length > 2000) return newLogs.slice(-2000); // Keep buffer manageable
          return newLogs;
        });
      };

      ws.onclose = () => {
        setStatus('disconnected');
        // Optional: Reconnect logic could go here
      };

      ws.onerror = () => {
        setStatus('disconnected');
        setLogs(prev => [...prev, '\x1b[31m[System] Connection error.\x1b[0m']);
      };
    };

    connect();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [token]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg shadow-lg overflow-hidden flex flex-col h-[500px]">
      <div className="bg-gray-800 px-4 py-2 flex justify-between items-center border-b border-gray-700">
        <div className="flex items-center gap-2">
            <span className="text-gray-400 font-mono text-sm">root@hunt-engine:~# tail -f docker-compose.logs</span>
        </div>
        <div className="flex items-center gap-2 text-xs">
            <span className={`w-2 h-2 rounded-full ${status === 'connected' ? 'bg-green-500 animate-pulse' : 'bg-red-500'}`}></span>
            <span className="text-gray-400 uppercase">{status}</span>
        </div>
      </div>
      
      <div 
        ref={scrollRef}
        className="flex-1 p-4 overflow-y-auto font-mono text-xs md:text-sm leading-5 bg-black text-gray-300 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent"
      >
        {logs.length === 0 && <div className="text-gray-600 italic">Waiting for logs...</div>}
        {logs.map((line, idx) => (
          <div key={idx} className="whitespace-pre-wrap break-all">
            {formatLogLine(line)}
          </div>
        ))}
      </div>
    </div>
  );
}
