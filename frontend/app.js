// OpenLST – brugerflade.
// Al tekst og lyd sendes kun til OpenLST på denne computer (samme adresse som
// siden). Intet sendes til internettet.
'use strict';

(() => {
  const $ = (id) => document.getElementById(id);
  const textEl = $('text');
  const readerEl = $('reader');
  const statusEl = $('status');
  const readBtn = $('read');
  const dictateBtn = $('dictate');
  const pauseBtn = $('pause');
  const stopBtn = $('stop');
  const repeatBtn = $('repeat');
  const speedEl = $('speed');

  const MAX_RECORD_MS = 60_000;   // diktering stopper automatisk efter 60 s
  const STT_RATE = 16000;

  const { splitSentences, isSpeakable, speechParts } = self.OpenLSTText;
  const { concat, resample, encodeWAV } = self.OpenLSTAudio;
  const engines = { tts: false, stt: false };

  // ---------- Hjælpere ----------

  function setStatus(msg, kind = '') {
    statusEl.textContent = msg;
    statusEl.className = 'status' + (kind ? ' ' + kind : '');
  }

  function store(key, value) {
    try { localStorage.setItem('openlst.' + key, value); } catch { /* privat vindue */ }
  }
  function load(key) {
    try { return localStorage.getItem('openlst.' + key); } catch { return null; }
  }

  async function apiError(res) {
    try {
      const j = await res.json();
      if (j && j.error) return j.error;
    } catch { /* ikke JSON */ }
    return 'Noget gik galt. Prøv igen.';
  }

  // ---------- Oplæsning ----------
  // Teksten læses som en kø af bidder (speechParts): den første er kort, så
  // eleven hører lyd med det samme. Lyd hentes op til PREFETCH bidder foran og
  // caches pr. tekst, så Gentag og spring er øjeblikkelige. Hastigheden styres
  // af afspilleren (playbackRate med bevaret tonehøjde): den ændres med det
  // samme og kræver aldrig ny lyd fra Piper.

  const PREFETCH = 2;
  const CACHE_MAX = 16;

  const player = new Audio();
  player.preservesPitch = true;
  const reading = {
    active: false,
    paused: false,
    sentences: [],
    spans: [],
    items: [],   // { sentence, text }
    pos: -1,     // aktuel bid
    index: -1,   // aktuel sætning
    gen: 0,
  };
  const audioCache = new Map(); // tekst -> { promise, ctrl, done }

  function speed() {
    return Math.min(1.4, Math.max(0.6, parseFloat(speedEl.value) || 0.9));
  }

  function applySpeed() {
    player.defaultPlaybackRate = speed();
    player.playbackRate = speed();
  }

  function fetchAudio(text) {
    const hit = audioCache.get(text);
    if (hit) { // senest brugt bagerst
      audioCache.delete(text);
      audioCache.set(text, hit);
      return hit.promise;
    }
    const ctrl = new AbortController();
    const entry = { ctrl, done: false };
    entry.promise = fetch('/api/tts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text, speed: 1, language: 'da' }),
      signal: ctrl.signal,
    }).then(async (res) => {
      if (!res.ok) throw new Error(await apiError(res));
      return URL.createObjectURL(await res.blob());
    });
    entry.promise.then(() => { entry.done = true; },
      () => { if (audioCache.get(text) === entry) audioCache.delete(text); });
    audioCache.set(text, entry);
    while (audioCache.size > CACHE_MAX) {
      const [k, old] = audioCache.entries().next().value;
      audioCache.delete(k);
      old.ctrl.abort();
      old.promise.then((u) => URL.revokeObjectURL(u)).catch(() => {});
    }
    return entry.promise;
  }

  // Afbryd lyd, der stadig er ved at blive lavet (færdig lyd beholdes).
  function abortPending() {
    for (const [k, e] of audioCache) {
      if (!e.done) { e.ctrl.abort(); audioCache.delete(k); }
    }
  }

  function prefetch(pos, depth) {
    const item = reading.items[pos];
    if (!item || depth <= 0) return;
    const gen = reading.gen;
    fetchAudio(item.text).then(() => { if (gen === reading.gen) prefetch(pos + 1, depth - 1); }, () => {});
  }

  function buildItems(from) {
    const items = [];
    let first = true;
    for (let i = from; i < reading.sentences.length; i++) {
      const s = reading.sentences[i];
      if (!isSpeakable(s)) continue;
      for (const text of speechParts(s.speak, first)) items.push({ sentence: i, text });
      first = false;
    }
    return items;
  }

  function highlight(i) {
    reading.spans.forEach((s, j) => s.classList.toggle('current', j === i));
    const el = reading.spans[i];
    if (el) el.scrollIntoView({ block: 'nearest' });
  }

  // Start oplæsning fra sætning i (første bid bliver kort).
  function playSentence(i) {
    reading.paused = false;
    reading.items = buildItems(i);
    playAt(0);
  }

  async function playAt(pos) {
    const gen = ++reading.gen;
    player.pause();
    const item = reading.items[pos];
    if (!item) { finishReading(); return; }
    reading.pos = pos;
    reading.index = item.sentence;
    highlight(item.sentence);
    updateButtons();
    if (!reading.paused) setStatus('Læser op …');

    let url;
    try {
      url = await fetchAudio(item.text);
    } catch (e) {
      if (gen !== reading.gen || e.name === 'AbortError') return;
      stopReading();
      setStatus(e.message || 'Oplæsningen fejlede.', 'error');
      return;
    }
    if (gen !== reading.gen) return;
    prefetch(pos + 1, PREFETCH);

    player.src = url;
    applySpeed();
    if (!reading.paused) {
      try { await player.play(); } catch { /* afbrudt */ }
    }
  }

  player.addEventListener('ended', () => {
    if (reading.active) playAt(reading.pos + 1);
  });

  function startReading(fromIndex = 0) {
    const text = textEl.value;
    const sentences = splitSentences(text);
    if (!sentences.some(isSpeakable)) {
      setStatus('Skriv eller indsæt en tekst først.');
      textEl.focus();
      return;
    }
    stopDictationIfActive();
    clearReading();
    reading.sentences = sentences;
    renderReader(text, sentences);
    reading.active = true;
    textEl.hidden = true;
    readerEl.hidden = false;
    playSentence(fromIndex);
  }

  function renderReader(text, sentences) {
    readerEl.textContent = '';
    reading.spans = [];
    let pos = 0;
    sentences.forEach((s, i) => {
      if (s.start > pos) readerEl.append(text.slice(pos, s.start));
      const span = document.createElement('span');
      span.className = 'sentence';
      span.textContent = s.raw;
      span.dataset.index = String(i);
      readerEl.append(span);
      reading.spans.push(span);
      pos = s.end;
    });
    if (pos < text.length) readerEl.append(text.slice(pos));
  }

  function clearReading() {
    const wasActive = reading.active;
    reading.gen++;
    player.pause();
    player.removeAttribute('src');
    // Under oplæsning afbrydes forudhentning; ellers kan der være en
    // forberedelse af teksten i gang (se prewarm), som skal bruges nu.
    if (wasActive) abortPending();
  }

  function stopReading() {
    clearReading();
    reading.active = false;
    reading.paused = false;
    readerEl.hidden = true;
    textEl.hidden = false;
    updateButtons();
    setStatus('');
  }

  function finishReading() {
    const last = reading.index;
    stopReading();
    reading.index = last;
    setStatus('Færdig.');
    updateButtons();
  }

  function togglePause() {
    if (!reading.active) return;
    reading.paused = !reading.paused;
    if (reading.paused) {
      player.pause();
      setStatus('Pause.');
    } else {
      setStatus('Læser op …');
      if (player.src) player.play().catch(() => {});
    }
    updateButtons();
  }

  // Gentag: under oplæsning gentages den aktuelle sætning; ellers hele teksten.
  function repeat() {
    if (reading.active) playSentence(reading.index);
    else startReading(0);
  }

  readerEl.addEventListener('click', (e) => {
    const span = e.target.closest('.sentence');
    if (!span || !reading.active) return;
    playSentence(parseInt(span.dataset.index, 10));
  });

  speedEl.addEventListener('input', () => {
    const v = speed();
    speedEl.setAttribute('aria-valuetext', v < 0.85 ? 'Langsom' : v > 1.15 ? 'Hurtig' : 'Normal');
    store('speed', String(v));
    applySpeed(); // virker med det samme – også midt i en sætning
  });

  // Forbered første bid, mens eleven skriver eller indsætter tekst, så Læs
  // starter med det samme. Kun én forberedelse ad gangen.
  let prewarmTimer = 0;
  let prewarmBusy = false;
  function prewarm() {
    if (!engines.tts || reading.active || prewarmBusy) return;
    const s = splitSentences(textEl.value).find(isSpeakable);
    const part = s && speechParts(s.speak, true)[0];
    if (!part || audioCache.has(part)) return;
    prewarmBusy = true;
    fetchAudio(part).catch(() => {}).finally(() => { prewarmBusy = false; });
  }
  textEl.addEventListener('input', () => {
    clearTimeout(prewarmTimer);
    prewarmTimer = setTimeout(prewarm, 700);
  });

  // ---------- Diktering ----------

  const rec = { active: false, busy: false, stream: null, ctx: null, node: null, chunks: [], sel: [0, 0], timer: 0 };

  async function startDictation() {
    if (rec.busy) return;
    if (reading.active) stopReading();
    if (!navigator.mediaDevices || !window.AudioWorkletNode) {
      setStatus('Din browser kan ikke optage. Brug Edge eller Chrome.', 'error');
      return;
    }
    rec.sel = [textEl.selectionStart ?? textEl.value.length, textEl.selectionEnd ?? textEl.value.length];
    rec.chunks = [];
    try {
      rec.stream = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
      rec.ctx = new AudioContext();
      await rec.ctx.audioWorklet.addModule('recorder-worklet.js');
      const src = rec.ctx.createMediaStreamSource(rec.stream);
      rec.node = new AudioWorkletNode(rec.ctx, 'recorder', { numberOfInputs: 1, numberOfOutputs: 1, channelCount: 1 });
      rec.node.port.onmessage = (e) => { if (e.data instanceof Float32Array) rec.chunks.push(e.data); };
      // Forbind via stum gain, så browseren holder processoren kørende.
      const mute = rec.ctx.createGain();
      mute.gain.value = 0;
      src.connect(rec.node).connect(mute).connect(rec.ctx.destination);
    } catch (e) {
      cleanupRecording();
      if (e && e.name === 'NotAllowedError') setStatus('Mikrofonen er blokeret. Tillad mikrofonen i browseren.', 'error');
      else if (e && e.name === 'NotFoundError') setStatus('Der er ingen mikrofon.', 'error');
      else setStatus('Mikrofonen kunne ikke startes.', 'error');
      return;
    }
    rec.active = true;
    rec.timer = setTimeout(() => stopDictation(), MAX_RECORD_MS);
    setStatus('Jeg lytter … Tryk på Stop, når du er færdig.', 'recording');
    updateButtons();
  }

  function cleanupRecording() {
    clearTimeout(rec.timer);
    if (rec.stream) rec.stream.getTracks().forEach((t) => t.stop());
    if (rec.ctx) rec.ctx.close().catch(() => {});
    rec.stream = rec.ctx = rec.node = null;
    rec.active = false;
  }

  async function stopDictation() {
    if (!rec.active) return;
    // Hent den sidste bid lyd fra workletten, før vi lukker.
    await new Promise((resolve) => {
      const t = setTimeout(resolve, 300);
      rec.node.port.addEventListener('message', (e) => { if (e.data === 'flushed') { clearTimeout(t); resolve(); } });
      rec.node.port.postMessage('flush');
    });
    const rate = rec.ctx.sampleRate;
    const chunks = rec.chunks;
    cleanupRecording();
    rec.chunks = [];

    rec.busy = true;
    updateButtons();
    setStatus('Skriver din tekst …');
    try {
      const pcm = await resample(concat(chunks), rate, STT_RATE);
      const res = await fetch('/api/stt', {
        method: 'POST',
        headers: { 'Content-Type': 'audio/wav' },
        body: encodeWAV(pcm, STT_RATE),
      });
      if (!res.ok) throw new Error(await apiError(res));
      const { text } = await res.json();
      if (!text) {
        setStatus('Jeg hørte ikke noget. Prøv igen.');
      } else {
        insertAtCursor(text);
        setStatus('Færdig. Du kan rette teksten.');
      }
    } catch (e) {
      setStatus(e.message || 'Dikteringen fejlede.', 'error');
    } finally {
      rec.busy = false;
      updateButtons();
    }
  }

  function stopDictationIfActive() {
    if (rec.active) cleanupRecording();
  }

  function insertAtCursor(text) {
    const [s, e] = rec.sel;
    const before = textEl.value.slice(0, s);
    const after = textEl.value.slice(e);
    let insert = text;
    if (before && !/\s$/.test(before)) insert = ' ' + insert;
    if (after && !/^\s/.test(after)) insert += ' ';
    textEl.setRangeText(insert, s, e, 'end');
    textEl.focus();
  }

  // ---------- Knapper ----------

  function setLabel(btn, icon, label) {
    btn.querySelector('.icon').textContent = icon;
    btn.querySelector('.label').textContent = label;
  }

  function updateButtons() {
    readBtn.disabled = !engines.tts || rec.active || rec.busy;
    dictateBtn.disabled = !engines.stt || rec.busy;
    dictateBtn.setAttribute('aria-pressed', String(rec.active));
    setLabel(dictateBtn, rec.active ? '⏹' : '🎤', rec.active ? 'Stop diktat' : 'Dikter');
    pauseBtn.disabled = !reading.active;
    setLabel(pauseBtn, reading.paused ? '▶' : '⏸', reading.paused ? 'Fortsæt' : 'Pause');
    stopBtn.disabled = !reading.active && !rec.active;
    repeatBtn.disabled = !engines.tts || rec.active || rec.busy || (!reading.active && reading.index < 0);
  }

  readBtn.addEventListener('click', () => startReading(0));
  dictateBtn.addEventListener('click', () => (rec.active ? stopDictation() : startDictation()));
  pauseBtn.addEventListener('click', togglePause);
  stopBtn.addEventListener('click', () => {
    if (rec.active) stopDictation();
    else stopReading();
  });
  repeatBtn.addEventListener('click', repeat);

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (rec.active) stopDictation();
      else if (reading.active) stopReading();
    }
  });

  // ---------- Opstart ----------

  async function init() {
    const saved = parseFloat(load('speed'));
    if (saved >= 0.6 && saved <= 1.4) speedEl.value = String(saved);
    speedEl.dispatchEvent(new Event('input'));

    try {
      const res = await fetch('/api/status');
      const st = await res.json();
      engines.tts = !!st.tts;
      engines.stt = !!st.stt;
      if (!st.tts && !st.stt) setStatus('Oplæsning og diktering virker ikke på denne computer.', 'error');
      else if (!st.tts) setStatus('Oplæsning virker ikke på denne computer.', 'error');
      else if (!st.stt) setStatus('Diktering virker ikke på denne computer.', 'error');
    } catch {
      setStatus('OpenLST kører ikke. Start OpenLST igen.', 'error');
    }
    updateButtons();
    prewarm(); // fx tekst, som browseren har gendannet
    textEl.focus();
  }

  init();
})();
