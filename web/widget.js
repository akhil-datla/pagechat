/**
 * PageChat Embeddable Widget
 *
 * Drop this on any page:
 *   <script src="https://your-pagechat-server.com/widget.js"></script>
 *
 * Optional attributes on the script tag:
 *   data-server="https://your-server.com"   — override server URL
 *   data-position="bottom-left"             — bottom-right (default) or bottom-left
 *   data-theme="dark"                       — light (default) or dark
 */
;(() => {
  // ── Detect server URL from script src ──
  const scriptEl = document.currentScript;
  const scriptSrc = scriptEl ? scriptEl.src : '';
  const serverOverride = scriptEl ? scriptEl.getAttribute('data-server') : null;
  const position = (scriptEl ? scriptEl.getAttribute('data-position') : null) || 'bottom-right';
  const theme = (scriptEl ? scriptEl.getAttribute('data-theme') : null) || 'light';

  let serverURL;
  if (serverOverride) {
    serverURL = serverOverride.replace(/\/+$/, '');
  } else if (scriptSrc) {
    const url = new URL(scriptSrc);
    serverURL = url.origin;
  } else {
    serverURL = window.location.origin;
  }

  const wsProto = serverURL.startsWith('https') ? 'wss:' : 'ws:';
  const wsHost = serverURL.replace(/^https?:\/\//, '');
  const website = window.location.href;

  // ── State ──
  let ws = null;
  let username = localStorage.getItem('pagechat_username') || '';
  let isOpen = false;
  let unreadCount = 0;
  let reconnectDelay = 1000;
  let lastAuthor = null;

  // ── Theme ──
  const t = theme === 'dark' ? {
    bg: '#1e293b', surface: '#334155', border: '#475569',
    text: '#f1f5f9', muted: '#94a3b8', primary: '#3b82f6',
    primaryHover: '#2563eb', ownBg: '#3b82f6', ownText: '#fff',
    otherBg: '#475569', otherText: '#f1f5f9', inputBg: '#1e293b',
  } : {
    bg: '#f8fafc', surface: '#ffffff', border: '#e2e8f0',
    text: '#0f172a', muted: '#64748b', primary: '#2563eb',
    primaryHover: '#1d4ed8', ownBg: '#2563eb', ownText: '#fff',
    otherBg: '#f1f5f9', otherText: '#0f172a', inputBg: '#f1f5f9',
  };

  // ── Styles ──
  const css = document.createElement('style');
  css.textContent = `
    #pc-widget * { margin:0; padding:0; box-sizing:border-box; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif; }
    #pc-widget { position:fixed; ${position === 'bottom-left' ? 'left:20px' : 'right:20px'}; bottom:20px; z-index:999999; }

    #pc-fab {
      width:56px; height:56px; border-radius:50%; border:none;
      background:${t.primary}; color:#fff; cursor:pointer;
      box-shadow:0 4px 12px rgba(0,0,0,0.15);
      display:flex; align-items:center; justify-content:center;
      transition:transform 0.2s,background 0.2s;
      position:relative;
    }
    #pc-fab:hover { background:${t.primaryHover}; transform:scale(1.05); }
    #pc-fab svg { width:26px; height:26px; }
    #pc-badge {
      position:absolute; top:-4px; right:-4px;
      background:#ef4444; color:#fff; font-size:11px; font-weight:700;
      min-width:20px; height:20px; border-radius:10px;
      display:none; align-items:center; justify-content:center; padding:0 5px;
    }
    #pc-badge.visible { display:flex; }

    #pc-panel {
      position:absolute; bottom:70px; ${position === 'bottom-left' ? 'left:0' : 'right:0'};
      width:370px; max-width:calc(100vw - 40px); height:500px; max-height:70vh;
      background:${t.surface}; border-radius:16px;
      box-shadow:0 20px 60px rgba(0,0,0,0.15); border:1px solid ${t.border};
      display:none; flex-direction:column; overflow:hidden;
    }
    #pc-panel.open { display:flex; }

    #pc-header {
      padding:14px 16px; border-bottom:1px solid ${t.border};
      display:flex; align-items:center; justify-content:space-between;
      background:${t.surface};
    }
    #pc-header-left { display:flex; align-items:center; gap:8px; }
    #pc-header-title { font-size:15px; font-weight:700; color:${t.primary}; }
    #pc-status {
      width:8px; height:8px; border-radius:50%; background:#ef4444;
      transition:background 0.3s;
    }
    #pc-status.on { background:#22c55e; }
    #pc-close {
      background:none; border:none; cursor:pointer; color:${t.muted};
      font-size:20px; line-height:1; padding:4px;
    }
    #pc-close:hover { color:${t.text}; }

    #pc-messages {
      flex:1; overflow-y:auto; padding:12px; display:flex;
      flex-direction:column; gap:2px; background:${t.bg};
    }
    #pc-messages::-webkit-scrollbar { width:5px; }
    #pc-messages::-webkit-scrollbar-thumb { background:${t.border}; border-radius:3px; }

    .pc-empty {
      flex:1; display:flex; align-items:center; justify-content:center;
      color:${t.muted}; font-size:13px;
    }

    .pc-group { display:flex; flex-direction:column; gap:1px; margin-bottom:6px; }
    .pc-group.own { align-items:flex-end; }
    .pc-group.other { align-items:flex-start; }

    .pc-author { font-size:11px; font-weight:600; color:${t.muted}; padding:0 6px; margin-bottom:1px; }

    .pc-bubble {
      max-width:260px; padding:8px 12px; font-size:13px; line-height:1.45;
      word-wrap:break-word;
      box-shadow:0 1px 2px rgba(0,0,0,0.06);
    }
    .pc-group.own .pc-bubble { background:${t.ownBg}; color:${t.ownText}; border-radius:12px 12px 3px 12px; }
    .pc-group.other .pc-bubble { background:${t.otherBg}; color:${t.otherText}; border-radius:12px 12px 12px 3px; }

    .pc-time { font-size:10px; color:${t.muted}; padding:0 6px; margin-top:1px; }
    .pc-group.own .pc-time { text-align:right; }

    #pc-input-area {
      padding:10px 12px; border-top:1px solid ${t.border};
      display:flex; gap:8px; background:${t.surface};
    }
    #pc-input {
      flex:1; padding:10px 12px; border:1px solid ${t.border};
      border-radius:10px; font-size:13px; outline:none;
      background:${t.inputBg}; color:${t.text};
      transition:border-color 0.2s;
    }
    #pc-input:focus { border-color:${t.primary}; }
    #pc-input::placeholder { color:${t.muted}; }
    #pc-send {
      padding:10px 16px; background:${t.primary}; color:#fff;
      border:none; border-radius:10px; font-size:13px;
      font-weight:600; cursor:pointer; transition:background 0.2s;
      white-space:nowrap;
    }
    #pc-send:hover { background:${t.primaryHover}; }
    #pc-send:disabled { opacity:0.5; cursor:not-allowed; }

    /* Username prompt inside panel */
    #pc-username-prompt {
      padding:24px 16px; text-align:center;
      display:flex; flex-direction:column; gap:12px; align-items:center;
      flex:1; justify-content:center; background:${t.bg};
    }
    #pc-username-prompt h3 { font-size:16px; font-weight:700; color:${t.text}; }
    #pc-username-prompt p { font-size:13px; color:${t.muted}; }
    #pc-username-input {
      width:80%; padding:10px 12px; border:1px solid ${t.border};
      border-radius:10px; font-size:14px; text-align:center;
      outline:none; background:${t.surface}; color:${t.text};
    }
    #pc-username-input:focus { border-color:${t.primary}; }
    #pc-username-submit {
      padding:10px 24px; background:${t.primary}; color:#fff;
      border:none; border-radius:10px; font-size:14px;
      font-weight:600; cursor:pointer;
    }
    #pc-username-submit:hover { background:${t.primaryHover}; }

    @media (max-width:420px) {
      #pc-panel { width:calc(100vw - 24px); ${position === 'bottom-left' ? 'left:-8px' : 'right:-8px'}; }
    }
  `;
  document.head.appendChild(css);

  // ── DOM ──
  const widget = document.createElement('div');
  widget.id = 'pc-widget';
  widget.innerHTML = `
    <div id="pc-panel">
      <div id="pc-header">
        <div id="pc-header-left">
          <span id="pc-header-title">PageChat</span>
          <div id="pc-status"></div>
        </div>
        <button id="pc-close">&times;</button>
      </div>
      <div id="pc-chat-body">
        ${username ? `
          <div id="pc-messages"><div class="pc-empty">No messages yet</div></div>
        ` : `
          <div id="pc-username-prompt">
            <h3>Join the conversation</h3>
            <p>Pick a name to start chatting</p>
            <input type="text" id="pc-username-input" placeholder="Your name" maxlength="30" autocomplete="off">
            <button id="pc-username-submit">Join</button>
          </div>
        `}
      </div>
      <div id="pc-input-area" style="${username ? '' : 'display:none'}">
        <input type="text" id="pc-input" placeholder="Type a message..." maxlength="2000" autocomplete="off">
        <button id="pc-send" disabled>Send</button>
      </div>
    </div>
    <button id="pc-fab">
      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round"
          d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
      </svg>
      <div id="pc-badge"></div>
    </button>
  `;
  document.body.appendChild(widget);

  // ── Refs ──
  const fab     = document.getElementById('pc-fab');
  const panel   = document.getElementById('pc-panel');
  const close   = document.getElementById('pc-close');
  const badge   = document.getElementById('pc-badge');
  const status  = document.getElementById('pc-status');
  const body    = document.getElementById('pc-chat-body');
  const inputArea = document.getElementById('pc-input-area');

  // ── FAB Toggle ──
  fab.addEventListener('click', () => {
    isOpen = !isOpen;
    panel.classList.toggle('open', isOpen);
    if (isOpen) {
      unreadCount = 0;
      badge.classList.remove('visible');
      const input = document.getElementById('pc-input');
      if (input) input.focus();
    }
  });

  close.addEventListener('click', () => {
    isOpen = false;
    panel.classList.remove('open');
  });

  // ── Username (if not set) ──
  if (!username) {
    setTimeout(() => {
      const uInput = document.getElementById('pc-username-input');
      const uBtn   = document.getElementById('pc-username-submit');
      if (!uInput || !uBtn) return;

      function submitName() {
        const name = uInput.value.trim();
        if (!name) return;
        username = name;
        localStorage.setItem('pagechat_username', username);

        body.innerHTML = '<div id="pc-messages"><div class="pc-empty">No messages yet</div></div>';
        inputArea.style.display = '';
        initChat();
      }

      uBtn.addEventListener('click', submitName);
      uInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') submitName(); });
    }, 0);
  } else {
    initChat();
  }

  function initChat() {
    loadHistory();
    connectWS();
    bindInput();
  }

  // ── History ──
  async function loadHistory() {
    try {
      const res = await fetch(`${serverURL}/api/messages?website=${encodeURIComponent(website)}`);
      const msgs = await res.json();
      if (msgs && msgs.length) {
        const container = document.getElementById('pc-messages');
        container.innerHTML = '';
        msgs.forEach(m => renderMsg(m));
        scrollBottom();
      }
    } catch (e) {
      console.warn('[PageChat] History load failed:', e);
    }
  }

  // ── WebSocket ──
  function connectWS() {
    ws = new WebSocket(`${wsProto}//${wsHost}/ws?website=${encodeURIComponent(website)}`);

    ws.onopen = () => {
      status.classList.add('on');
      const send = document.getElementById('pc-send');
      if (send) send.disabled = false;
      reconnectDelay = 1000;
    };

    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data);
      renderMsg(msg);
      scrollBottom();
      if (!isOpen) {
        unreadCount++;
        badge.textContent = unreadCount > 99 ? '99+' : unreadCount;
        badge.classList.add('visible');
      }
    };

    ws.onclose = () => {
      status.classList.remove('on');
      const send = document.getElementById('pc-send');
      if (send) send.disabled = true;
      setTimeout(() => {
        reconnectDelay = Math.min(reconnectDelay * 2, 30000);
        connectWS();
      }, reconnectDelay);
    };

    ws.onerror = () => ws.close();
  }

  // ── Input ──
  function bindInput() {
    const input = document.getElementById('pc-input');
    const send  = document.getElementById('pc-send');
    if (!input || !send) return;

    function doSend() {
      const content = input.value.trim();
      if (!content || !ws || ws.readyState !== WebSocket.OPEN) return;
      ws.send(JSON.stringify({ username, content }));
      input.value = '';
      input.focus();
    }

    send.addEventListener('click', doSend);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        doSend();
      }
    });
  }

  // ── Render ──
  function renderMsg(msg) {
    const container = document.getElementById('pc-messages');
    if (!container) return;

    const empty = container.querySelector('.pc-empty');
    if (empty) empty.remove();

    const isOwn = msg.username === username;
    const isSameAuthor = lastAuthor === msg.username;

    const group = document.createElement('div');
    group.className = `pc-group ${isOwn ? 'own' : 'other'}`;

    if (!isSameAuthor) {
      const author = document.createElement('div');
      author.className = 'pc-author';
      author.textContent = msg.username;
      group.appendChild(author);
    }

    const bubble = document.createElement('div');
    bubble.className = 'pc-bubble';
    bubble.textContent = msg.content;
    group.appendChild(bubble);

    if (msg.timestamp) {
      const time = document.createElement('div');
      time.className = 'pc-time';
      const d = new Date(msg.timestamp);
      time.textContent = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      group.appendChild(time);
    }

    container.appendChild(group);
    lastAuthor = msg.username;
  }

  function scrollBottom() {
    const container = document.getElementById('pc-messages');
    if (container) {
      requestAnimationFrame(() => { container.scrollTop = container.scrollHeight; });
    }
  }
})();
