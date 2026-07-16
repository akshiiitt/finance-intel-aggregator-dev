// terminal-worker.ts
// Offloads WebSocket JSON parsing from the main thread.

self.onmessage = (e: MessageEvent) => {
  if (e.data.type === "CONNECT") {
    let socket: WebSocket;
    let reconnectTimer: number | null = null;
    let destroyed = false;
    let reconnectAttempts = 0;

    const connect = () => {
      if (destroyed) return;
      socket = new WebSocket(e.data.url);

      socket.onopen = () => {
        reconnectAttempts = 0;
        self.postMessage({ type: "CONNECTED" });
      };

      socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data as string) as { type: string; data: unknown };
          if (msg.type === "ARTICLE_ENGAGED") {
            const article = (msg.data || {}) as any;
            const sanitized = {
              id: Number(article.id || 0),
              title: String(article.title || ""),
              summary: String(article.summary || ""),
              fiScore: Number(article.fiScore || 0),
              category: String(article.category || "general"),
              source: String(article.source || "Unknown"),
              region: String(article.region || "india"),
              sentiment: String(article.sentiment || "neutral"),
              aiModel: String(article.aiModel || "keyword"),
              receivedAt: Date.now()
            };
            self.postMessage({ type: "ARTICLE", data: sanitized });
          }
        } catch {
          // ignore malformed frames
        }
      };

      socket.onclose = () => {
        self.postMessage({ type: "DISCONNECTED" });
        if (!destroyed) {
          reconnectAttempts++;
          const delay = Math.min(30000, 2000 * Math.pow(1.5, reconnectAttempts)) + (Math.random() * 1000);
          reconnectTimer = setTimeout(connect, delay) as any;
        }
      };

      socket.onerror = () => {
        socket.close();
      };
    };

    connect();

    // Store cleanup in global scope for the worker
    (self as any).cleanup = () => {
      destroyed = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (socket) socket.close();
    };
  } else if (e.data.type === "DISCONNECT") {
    if ((self as any).cleanup) {
      (self as any).cleanup();
    }
  }
};
