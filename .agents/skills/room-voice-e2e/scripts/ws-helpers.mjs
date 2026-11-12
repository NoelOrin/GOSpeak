// WebSocket 消息抓包 + 协议验证辅助

async function createWSProbe(page, url, token) {
  const probeId = `ws_probe_${Date.now()}_${Math.random().toString(36).slice(2)}`;

  await page.evaluate(({ id, url, token }) => {
    window[id] = {
      ws: null,
      connected: false,
      messages: [],
      eventWaiters: {},
      counter: 0,
    };

    const wsUrl = token ? `${url}?token=${encodeURIComponent(token)}` : url;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => { window[id].connected = true; };

    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        window[id].messages.push(msg);
        const waiters = window[id].eventWaiters[msg.event];
        if (waiters) {
          waiters.forEach((r) => r(msg));
          window[id].eventWaiters[msg.event] = [];
        }
      } catch {
        window[id].messages.push({ raw: ev.data });
      }
    };

    ws.onclose = () => { window[id].connected = false; };
    window[id].ws = ws;
  }, { id: probeId, url, token });

  return {
    probeId,

    async send(event, data) {
      await page.evaluate(({ id, event, data }) => {
        window[id].ws.send(JSON.stringify({ event, data }));
      }, { id: probeId, event, data });
    },

    async waitForEvent(event, timeout = 10000) {
      return page.evaluate(({ id, event, timeout }) => {
        return new Promise((resolve, reject) => {
          const timer = setTimeout(() => reject(new Error(`timeout waiting for ${event}`)), timeout);
          if (!window[id].eventWaiters[event]) window[id].eventWaiters[event] = [];
          window[id].eventWaiters[event].push((msg) => {
            clearTimeout(timer);
            resolve(msg);
          });
        });
      }, { id: probeId, event, timeout });
    },

    async getMessages() {
      return page.evaluate((id) => window[id].messages, probeId);
    },

    async isConnected() {
      return page.evaluate((id) => window[id].connected, probeId);
    },

    async close() {
      await page.evaluate((id) => {
        if (window[id].ws) window[id].ws.close();
        delete window[id];
      }, probeId);
    },
  };
}

function assertMessageFormat(msg) {
  if (!msg || typeof msg !== 'object') throw new Error('message is not an object');
  if (!msg.event || typeof msg.event !== 'string') throw new Error('missing event field');
  if (msg.id !== undefined && typeof msg.id !== 'string') throw new Error('id must be string');
  if (msg.error) {
    if (typeof msg.error.code !== 'number') throw new Error('error.code must be number');
    if (typeof msg.error.message !== 'string') throw new Error('error.message must be string');
  }
}

module.exports = { createWSProbe, assertMessageFormat };
