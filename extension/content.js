// OpenLST – værktøjslinje på hjemmesider.
// Læser markeret tekst (eller det afsnit, der sidst er klikket i) op med
// fremhævning af sætningen, og indsætter dikteret tekst i det aktive felt.
// Sidens indhold ændres ikke: fremhævning sker med CSS Custom Highlight API.
'use strict';

(() => {
  if (window.top !== window || window.__openlst) return;
  window.__openlst = true;

  const { splitSentences, isSpeakable } = self.OpenLSTText;
  const HIGHLIGHT = 'openlst-sentence';
  const SPEED_MIN = 0.6, SPEED_MAX = 1.4, SPEED_DEFAULT = 0.9;
  const BLOCK = 'p,div,li,td,th,h1,h2,h3,h4,h5,h6,section,article,blockquote,pre,dd,dt,' +
    'figcaption,main,header,footer,aside,nav,form,table,tr,ul,ol,label,summary,details';

  let speed = SPEED_DEFAULT;
  let toolbar = null;           // { host, status, buttons... }
  let lastBlock = null;         // afsnit brugeren sidst klikkede i
  const reading = { active: false, paused: false, gen: 0, items: [], index: -1 };
  const rec = { active: false, busy: false, target: null };

  // ---------- Beskeder til baggrunden ----------

  async function send(msg) {
    try {
      return await chrome.runtime.sendMessage(msg);
    } catch {
      return { error: 'OpenLST er blevet opdateret. Genindlæs siden.' };
    }
  }

  // ---------- Tekst og sætninger ----------

  function isOwn(node) {
    return toolbar && (node === toolbar.host || toolbar.host.contains(node));
  }

  function blockOf(node) {
    const el = node.nodeType === Node.ELEMENT_NODE ? node : node.parentElement;
    return el ? el.closest(BLOCK) || el : null;
  }

  // Samler synlig tekst i range og husker, hvor hvert tegn kommer fra.
  function collect(range) {
    let root = range.commonAncestorContainer;
    if (root.nodeType !== Node.ELEMENT_NODE) root = root.parentNode;
    const map = []; // { node, offset, start, len }
    let text = '';
    let prevBlock = null;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
      acceptNode(n) {
        if (!range.intersectsNode(n) || isOwn(n)) return NodeFilter.FILTER_REJECT;
        const p = n.parentElement;
        if (!p || p.closest('script,style,noscript,template,[aria-hidden="true"]')) return NodeFilter.FILTER_REJECT;
        if (p.checkVisibility && !p.checkVisibility()) return NodeFilter.FILTER_REJECT;
        return NodeFilter.FILTER_ACCEPT;
      },
    });
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const s = n === range.startContainer ? range.startOffset : 0;
      const e = n === range.endContainer ? range.endOffset : n.data.length;
      if (e <= s) continue;
      const b = blockOf(n);
      if (prevBlock && b !== prevBlock && !/\s$/.test(text)) text += '\n'; // afsnitsskift
      prevBlock = b;
      map.push({ node: n, offset: s, start: text.length, len: e - s });
      text += n.data.slice(s, e);
    }
    return { text, map };
  }

  // Finder DOM-position for et tegn i den samlede tekst.
  function locate(map, pos, isEnd) {
    for (let i = 0; i < map.length; i++) {
      const m = map[i];
      const inside = isEnd ? pos > m.start && pos <= m.start + m.len : pos >= m.start && pos < m.start + m.len;
      if (inside) return { node: m.node, offset: m.offset + pos - m.start };
      if (!isEnd && pos < m.start) return { node: m.node, offset: m.offset }; // i et indsat linjeskift
      if (isEnd && pos <= m.start) {
        const prev = map[i - 1];
        return prev ? { node: prev.node, offset: prev.offset + prev.len } : null;
      }
    }
    const last = map[map.length - 1];
    return last ? { node: last.node, offset: last.offset + last.len } : null;
  }

  function buildItems(range) {
    const { text, map } = collect(range);
    const items = [];
    for (const s of splitSentences(text)) {
      if (!isSpeakable(s)) continue;
      // Fremhæv uden mellemrum før og efter sætningen.
      const start = s.start + (s.raw.length - s.raw.trimStart().length);
      const end = s.end - (s.raw.length - s.raw.trimEnd().length);
      const a = locate(map, start, false);
      const b = locate(map, end, true);
      let r = null;
      if (a && b) {
        try {
          r = document.createRange();
          r.setStart(a.node, a.offset);
          r.setEnd(b.node, b.offset);
        } catch { r = null; }
      }
      items.push({ text: s.speak, range: r });
    }
    return items;
  }

  // ---------- Fremhævning ----------

  function highlight(range) {
    if (!window.CSS || !CSS.highlights) return;
    if (!range) { CSS.highlights.delete(HIGHLIGHT); return; }
    CSS.highlights.set(HIGHLIGHT, new Highlight(range));
    const rect = range.getBoundingClientRect();
    if (rect.height && (rect.top < 60 || rect.bottom > window.innerHeight - 120)) {
      window.scrollBy({ top: rect.top - window.innerHeight / 3, behavior: 'auto' });
    }
  }

  // ---------- Oplæsning ----------

  function currentRange() {
    const sel = window.getSelection();
    if (sel && sel.rangeCount && !sel.isCollapsed && sel.toString().trim()) {
      const r = sel.getRangeAt(0).cloneRange();
      if (!isOwn(r.commonAncestorContainer)) return r;
    }
    if (lastBlock && document.contains(lastBlock)) {
      const r = document.createRange();
      r.selectNodeContents(lastBlock);
      return r;
    }
    return null;
  }

  async function readCurrent() {
    const range = currentRange();
    if (!range) {
      setStatus('Markér tekst eller klik i et afsnit først.');
      return true;
    }
    const items = buildItems(range);
    if (!items.length) {
      setStatus('Der er ingen tekst at læse.');
      return true;
    }
    window.getSelection()?.removeAllRanges(); // så fremhævningen kan ses
    stopReading(false);
    reading.items = items;
    reading.active = true;
    reading.paused = false;
    playFrom(0);
    return true;
  }

  async function playFrom(i) {
    const gen = ++reading.gen;
    if (i >= reading.items.length) { stopReading(true); return; }
    reading.index = i;
    const item = reading.items[i];
    highlight(item.range);
    setStatus('Læser op …');
    updateButtons();
    const next = reading.items[i + 1];
    if (next) send({ type: 'prefetch', text: next.text, speed });
    const res = await send({ type: 'speak', text: item.text, speed });
    if (gen !== reading.gen) return;
    if (res && res.error) {
      stopReading(false);
      setStatus(res.error, true);
      return;
    }
    if (res && res.stopped) return;
    playFrom(i + 1);
  }

  function stopReading(finished) {
    reading.gen++;
    if (reading.active) send({ type: 'stop' });
    reading.active = false;
    reading.paused = false;
    highlight(null);
    setStatus(finished ? 'Færdig.' : '');
    updateButtons();
  }

  function togglePause() {
    if (!reading.active) return;
    reading.paused = !reading.paused;
    send({ type: reading.paused ? 'pause' : 'resume' });
    setStatus(reading.paused ? 'Pause.' : 'Læser op …');
    updateButtons();
  }

  // ---------- Diktering ----------

  function isEditable(el) {
    if (!el || isOwn(el)) return false;
    if (el.isContentEditable) return true;
    if (el.disabled || el.readOnly) return false;
    if (el.tagName === 'TEXTAREA') return true;
    return el.tagName === 'INPUT' && ['text', 'search', 'email', 'url', 'tel', ''].includes(el.type);
  }

  document.addEventListener('focusin', (e) => {
    const el = e.composedPath()[0];
    if (isEditable(el)) rec.target = el;
  }, true);

  document.addEventListener('mousedown', (e) => {
    const t = e.composedPath()[0];
    if (t instanceof Node && !isOwn(t) && !isEditable(t)) lastBlock = blockOf(t);
  }, true);

  async function toggleDictation() {
    if (rec.busy) return;
    if (rec.active) return stopDictation();
    const el = rec.target;
    if (!el || !document.contains(el)) {
      setStatus('Klik først i et tekstfelt.');
      return;
    }
    if (reading.active) stopReading(false);
    const res = await send({ type: 'recStart' });
    if (res && res.error === 'mic') {
      setStatus('Giv OpenLST adgang til mikrofonen.', true);
      send({ type: 'openSetup' });
      return;
    }
    if (res && res.error) { setStatus(res.error, true); return; }
    rec.active = true;
    el.focus();
    setStatus('Jeg lytter … Tryk igen, når du er færdig.');
    updateButtons();
  }

  async function stopDictation() {
    rec.active = false;
    rec.busy = true;
    setStatus('Skriver din tekst …');
    updateButtons();
    const res = await send({ type: 'recStop' });
    rec.busy = false;
    if (res && res.error) {
      setStatus(res.error, true);
    } else if (!res || !res.text) {
      setStatus('Jeg hørte ikke noget. Prøv igen.');
    } else {
      insertText(rec.target, res.text);
      setStatus('Færdig. Du kan rette teksten.');
    }
    updateButtons();
  }

  function insertText(el, text) {
    if (!el || !document.contains(el)) return;
    el.focus();
    if (el.isContentEditable) {
      const sel = window.getSelection();
      const n = sel && sel.anchorNode;
      if (n && n.nodeType === Node.TEXT_NODE && sel.anchorOffset > 0 && !/\s/.test(n.data[sel.anchorOffset - 1])) {
        text = ' ' + text;
      }
      // execCommand er forældet, men er den eneste måde, der virker med
      // fortryd (Ctrl+Z) og med editorer som Google Docs/Word Online.
      if (!document.execCommand('insertText', false, text)) setStatus('Teksten kunne ikke indsættes her.', true);
      return;
    }
    const s = el.selectionStart ?? el.value.length;
    const e = el.selectionEnd ?? el.value.length;
    const before = el.value.slice(0, s);
    const after = el.value.slice(e);
    let insert = text;
    if (before && !/\s$/.test(before)) insert = ' ' + insert;
    if (after && !/^\s/.test(after)) insert += ' ';
    el.setRangeText(insert, s, e, 'end');
    // Fortæl sidens JavaScript (React, Vue …) at værdien er ændret.
    el.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: insert }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }

  // ---------- Værktøjslinje ----------

  const CSS_TEXT = `
    :host { all: initial; }
    .bar { position: fixed; right: 16px; bottom: 16px; z-index: 2147483647;
      display: flex; flex-wrap: wrap; align-items: center; gap: 8px; max-width: calc(100vw - 32px);
      padding: 10px; border-radius: 16px; background: #fbf8f1; color: #1d1d1f;
      border: 2px solid #c9c3b6; box-shadow: 0 4px 16px rgba(0,0,0,.18);
      font: bold 16px/1.2 Verdana, Tahoma, "Segoe UI", sans-serif; }
    button { font: inherit; min-height: 48px; min-width: 48px; padding: 6px 12px; border-radius: 12px;
      border: 2px solid #c9c3b6; background: #fff; color: inherit; cursor: pointer; }
    button:hover:not(:disabled) { border-color: #1f5fbf; }
    button:focus-visible { outline: 3px solid #ff9e1b; outline-offset: 1px; }
    button:disabled { opacity: .45; cursor: default; }
    .primary { background: #1f5fbf; border-color: #1f5fbf; color: #fff; }
    .mic[aria-pressed="true"] { background: #b3261e; border-color: #b3261e; color: #fff; }
    .speed { display: flex; align-items: center; gap: 4px; font-weight: normal; }
    .speed span { min-width: 3.2em; text-align: center; }
    .status { flex-basis: 100%; font-weight: normal; font-size: 14px; min-height: 1.2em; }
    .status.err { color: #b3261e; font-weight: bold; }
    .close { min-width: 40px; padding: 4px; }
    @media (prefers-color-scheme: dark) {
      .bar { background: #262628; color: #f2f0eb; border-color: #4a4843; }
      button { background: #1b1b1d; border-color: #4a4843; }
    }`;

  function button(label, title, cls) {
    const b = document.createElement('button');
    b.type = 'button';
    b.textContent = label;
    b.title = title;
    if (cls) b.className = cls;
    // Behold fokus i sidens tekstfelt, når der klikkes på værktøjslinjen.
    b.addEventListener('mousedown', (e) => e.preventDefault());
    return b;
  }

  function createToolbar() {
    const host = document.createElement('openlst-toolbar');
    const shadow = host.attachShadow({ mode: 'closed' });
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(CSS_TEXT);
    shadow.adoptedStyleSheets = [sheet];

    const bar = document.createElement('div');
    bar.className = 'bar';
    bar.setAttribute('role', 'toolbar');
    bar.setAttribute('aria-label', 'OpenLST');
    bar.lang = 'da';

    const read = button('🔊 Læs', 'Læs markeret tekst op (Alt+Shift+L)', 'primary');
    const pause = button('⏸', 'Pause');
    const stop = button('⏹', 'Stop');
    const mic = button('🎤 Dikter', 'Diktér i det aktive tekstfelt', 'mic');
    mic.setAttribute('aria-pressed', 'false');
    const slower = button('−', 'Langsommere');
    const faster = button('+', 'Hurtigere');
    const speedLabel = document.createElement('span');
    const speedBox = document.createElement('div');
    speedBox.className = 'speed';
    speedBox.append(slower, speedLabel, faster);
    const close = button('✕', 'Skjul OpenLST', 'close');
    const status = document.createElement('div');
    status.className = 'status';
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');

    read.addEventListener('click', readCurrent);
    pause.addEventListener('click', togglePause);
    stop.addEventListener('click', () => { if (rec.active) stopDictation(); else stopReading(false); });
    mic.addEventListener('click', toggleDictation);
    slower.addEventListener('click', () => setSpeed(speed - 0.1));
    faster.addEventListener('click', () => setSpeed(speed + 0.1));
    close.addEventListener('click', () => setToolbar(false));

    bar.append(read, pause, stop, mic, speedBox, close, status);
    shadow.append(bar);
    document.documentElement.append(host);
    return { host, status, read, pause, stop, mic, speedLabel };
  }

  function setToolbar(visible) {
    chrome.storage.local.set({ toolbar: visible });
    if (visible && !toolbar) {
      toolbar = createToolbar();
      updateButtons();
    } else if (!visible && toolbar) {
      stopReading(false);
      if (rec.active) { send({ type: 'recCancel' }); rec.active = false; }
      toolbar.host.remove();
      toolbar = null;
    }
  }

  function setStatus(msg, isError) {
    if (!toolbar && msg) setToolbar(true);
    if (!toolbar) return;
    toolbar.status.textContent = msg;
    toolbar.status.className = 'status' + (isError ? ' err' : '');
  }

  function setSpeed(v) {
    speed = Math.round(Math.min(SPEED_MAX, Math.max(SPEED_MIN, v)) * 10) / 10;
    chrome.storage.local.set({ speed });
    updateButtons();
  }

  function updateButtons() {
    if (!toolbar) return;
    toolbar.read.disabled = rec.active || rec.busy;
    toolbar.pause.disabled = !reading.active;
    toolbar.pause.textContent = reading.paused ? '▶' : '⏸';
    toolbar.pause.title = reading.paused ? 'Fortsæt' : 'Pause';
    toolbar.stop.disabled = !reading.active && !rec.active;
    toolbar.mic.disabled = rec.busy;
    toolbar.mic.setAttribute('aria-pressed', String(rec.active));
    toolbar.mic.textContent = rec.active ? '⏹ Stop diktat' : '🎤 Dikter';
    toolbar.speedLabel.textContent = speed.toFixed(1) + 'x';
  }

  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    if (rec.active) stopDictation();
    else if (reading.active) stopReading(false);
  });

  chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
    if (msg.type === 'toggleToolbar') { setToolbar(!toolbar); sendResponse(true); }
    if (msg.type === 'readSelection') { readCurrent(); sendResponse(true); }
    return false;
  });

  chrome.storage.local.get(['speed', 'toolbar']).then((s) => {
    if (typeof s.speed === 'number') speed = s.speed;
    if (s.toolbar) setToolbar(true);
  });
})();
