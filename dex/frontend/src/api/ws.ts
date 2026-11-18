type Handler = (msg: any) => void;

export class DexSocket {
  private ws: WebSocket | null = null;
  private handlers = new Set<Handler>();
  private pending: string[] = [];
  private timer: number | null = null;
  private closed = false;
  private attempt = 0;
  private token = "";

  connect(token: string) {
    this.token = token;
    this.closed = false;
    this.open();
  }

  private url() {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const q = this.token ? `?token=${this.token}` : "";
    return `${proto}//${location.host}/ws${q}`;
  }

  private open() {
    this.cleanupSocket();
    const ws = new WebSocket(this.url());
    this.ws = ws;
    ws.onopen = () => {
      this.attempt = 0;
      this.pending.splice(0).forEach((m) => ws.send(m));
      this.heartbeat();
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        this.handlers.forEach((h) => h(msg));
      } catch { /* ignore */ }
    };
    ws.onclose = () => {
      this.ws = null;
      if (!this.closed) this.reconnect();
    };
    ws.onerror = () => ws.close();
  }

  private heartbeat() {
    this.clearBeat();
    this.timer = window.setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ op: "ping" }));
    }, 15000);
  }

  private reconnect() {
    this.clearBeat();
    const wait = Math.min(8000, 400 * 2 ** this.attempt++);
    window.setTimeout(() => { if (!this.closed) this.open(); }, wait);
  }

  subscribe(channel: string, symbol: string) {
    const payload = JSON.stringify({ op: "subscribe", channel, symbol });
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(payload);
    else this.pending.push(payload);
  }

  unsubscribe(channel: string, symbol: string) {
    const payload = JSON.stringify({ op: "unsubscribe", channel, symbol });
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(payload);
  }

  on(h: Handler) {
    this.handlers.add(h);
    return () => this.handlers.delete(h);
  }

  close() {
    this.closed = true;
    this.clearBeat();
    this.cleanupSocket();
    this.handlers.clear();
  }

  private cleanupSocket() {
    if (!this.ws) return;
    this.ws.onopen = this.ws.onclose = this.ws.onerror = this.ws.onmessage = null;
    try { this.ws.close(); } catch { /* ignore */ }
    this.ws = null;
  }

  private clearBeat() {
    if (this.timer) window.clearInterval(this.timer);
    this.timer = null;
  }
}

export const dexSocket = new DexSocket();
