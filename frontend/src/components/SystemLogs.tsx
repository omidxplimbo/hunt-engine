import { useEffect, useRef, useState } from 'react';
import { useAuth } from '../context/AuthContext';

type LogStatus = 'connecting' | 'connected' | 'disconnected';

const formatLogLine = (line: string) => {
  const parts = line.split(/(\x1b\[\d+m)/g);
  let currentColor = 'text-gray-300';

  return (
    <>
      {parts.map((part, index) => {
        if (part === '\x1b[32m') {
          currentColor = 'text-green-400';
          return null;
        }

        if (part === '\x1b[33m') {
          currentColor = 'text-yellow-400';
          return null;
        }

        if (part === '\x1b[34m') {
          currentColor = 'text-blue-400';
          return null;
        }

        if (part === '\x1b[31m') {
          currentColor = 'text-red-400';
          return null;
        }

        if (part === '\x1b[36m') {
          currentColor = 'text-cyan-400';
          return null;
        }

        if (part === '\x1b[0m') {
          currentColor = 'text-gray-300';
          return null;
        }

        if (part.startsWith('\x1b')) {
          return null;
        }

        return (
          <span key={index} className={currentColor}>
            {part}
          </span>
        );
      })}
    </>
  );
};

const buildSystemLogsWebSocketUrl = (token: string) => {
  const rawApiUrl = import.meta.env.VITE_API_URL || '/api';

  const apiUrl = new URL(rawApiUrl, window.location.origin);
  apiUrl.protocol = apiUrl.protocol === 'https:' ? 'wss:' : 'ws:';

  const apiPath = apiUrl.pathname.replace(/\/$/, '');
  apiUrl.pathname = `${apiPath}/monitor/logs`;
  apiUrl.search = '';
  apiUrl.searchParams.set('token', token);

  return apiUrl.toString();
};

export default function SystemLogs() {
  const [logs, setLogs] = useState<string[]>([]);
  const [status, setStatus] = useState<LogStatus>('connecting');

  const { token } = useAuth();

  const wsRef = useRef<WebSocket | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!token) {
      setStatus('disconnected');
      return;
    }

    let isUnmounted = false;

    const wsUrl = buildSystemLogsWebSocketUrl(token);
    const ws = new WebSocket(wsUrl);

    wsRef.current = ws;
    setStatus('connecting');

    ws.onopen = () => {
      if (isUnmounted) return;

      setStatus('connected');
      setLogs((prev) => [
        ...prev,
        '\x1b[32m[System] Connected to log stream...\x1b[0m',
      ]);
    };

    ws.onmessage = (event) => {
      if (isUnmounted) return;

      setLogs((prev) => {
        const next = [...prev, String(event.data)];
        return next.length > 2000 ? next.slice(-2000) : next;
      });
    };

    ws.onerror = () => {
      if (isUnmounted) return;

      setStatus('disconnected');
      setLogs((prev) => [
        ...prev,
        '\x1b[31m[System] Connection error.\x1b[0m',
      ]);
    };

    ws.onclose = () => {
      if (isUnmounted) return;
      setStatus('disconnected');
    };

    return () => {
      isUnmounted = true;

      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [token]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  return (
    <div className="border border-green-500/70 bg-black/80 shadow-[0_0_20px_rgba(0,255,65,0.15)]">
      <div className="flex items-center justify-between border-b border-green-500/30 bg-gray-900/80 px-4 py-3">
        <div className="font-mono text-sm text-gray-400">
          root@hunt-engine:~# tail -f docker-compose.logs
        </div>

        <div className="flex items-center gap-2 font-mono text-xs uppercase tracking-widest text-gray-400">
          <span
            className={`h-2 w-2 rounded-full ${
              status === 'connected'
                ? 'bg-green-500'
                : status === 'connecting'
                  ? 'bg-yellow-500'
                  : 'bg-red-500'
            }`}
          />
          {status}
        </div>
      </div>

      <div
        ref={scrollRef}
        className="h-[520px] overflow-y-auto p-4 font-mono text-sm leading-6"
      >
        {logs.length === 0 ? (
          <div className="text-gray-600">Waiting for logs...</div>
        ) : (
          logs.map((line, index) => (
            <div key={`${index}-${line.slice(0, 20)}`} className="whitespace-pre-wrap">
              {formatLogLine(line)}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
