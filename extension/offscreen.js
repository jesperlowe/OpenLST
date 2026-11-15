// OpenLST – skjult dokument der afspiller oplæsning og optager diktering.
// Lyd sendes kun til OpenLST på denne computer.
'use strict';

const API = 'http://127.0.0.1:8765';
const STT_RATE = 16000;
const MAX_RECORD_MS = 60_000;
const { concat, resample, encodeWAV } = self.OpenLSTAudio;

// ---------- Oplæsning ----------

const player = new Audio();
const cache = new Map(); // "speed|tekst" -> Promise<objectURL>
let current = null;      // { resolve } for den sætning der afspilles

function key(text, speed) { return speed + '|' + text; }

async function apiError(res) {
  try { const j = await res.json(); if (j.error) return j.error; } catch { /* ikke JSON */ }
  return 'OpenLST svarede med fejl ' + res.status;
}

function fetchTTS(text, speed) {
  const k = key(text, speed);
  if (cache.has(k)) return cache.get(k);
  const p = fetch(API + '/api/tts', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text, speed, language: 'da' }),
  }).then(async (res) => {
    if (!res.ok) throw new Error(await apiError(res));
    return URL.createObjectURL(await res.blob());
  });
  p.catch(() => cache.delete(k));
  cache.set(k, p);
  // Hold cachen lille.
  while (cache.size > 4) {
    const [oldKey, oldP] = cache.entries().next().value;
    cache.delete(oldKey);
    oldP.then((u) => URL.revokeObjectURL(u)).catch(() => {});
  }
  return p;
}

function finish(result) {
  if (current) { const c = current; current = null; c.resolve(result); }
}

player.addEventListener('ended', () => finish({ ok: true }));
player.addEventListener('error', () => finish({ error: 'Lyden kunne ikke afspilles.' }));

async function speak(text, speed) {
  stopPlayback();
  let url;
  try {
    url = await fetchTTS(text, speed);
  } catch (e) {
    return { error: offlineMessage(e) };
  }
  return new Promise((resolve) => {
    current = { resolve };
    player.src = url;
    player.play().catch(() => finish({ error: 'Lyden kunne ikke afspilles.' }));
  });
}

function stopPlayback() {
  player.pause();
  finish({ stopped: true });
}

function offlineMessage(e) {
  if (e instanceof TypeError) return 'OpenLST kører ikke på computeren. Start OpenLST.';
  return e.message || 'Oplæsningen fejlede.';
}

// ---------- Diktering ----------

const rec = { stream: null, ctx: null, node: null, chunks: [], timer: 0 };

async function recStart() {
  if (rec.ctx) return { ok: true };
  try {
    rec.stream = await navigator.mediaDevices.getUserMedia({
      audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
    });
  } catch (e) {
    return { error: 'mic', name: e && e.name };
  }
  rec.ctx = new AudioContext();
  await rec.ctx.audioWorklet.addModule('lib/recorder-worklet.js');
  const src = rec.ctx.createMediaStreamSource(rec.stream);
  rec.node = new AudioWorkletNode(rec.ctx, 'recorder', { numberOfInputs: 1, numberOfOutputs: 1, channelCount: 1 });
  rec.chunks = [];
  rec.node.port.onmessage = (e) => { if (e.data instanceof Float32Array) rec.chunks.push(e.data); };
  const mute = rec.ctx.createGain();
  mute.gain.value = 0;
  src.connect(rec.node).connect(mute).connect(rec.ctx.destination);
  rec.timer = setTimeout(() => recCleanup(), MAX_RECORD_MS + 5000); // nødstop
  return { ok: true };
}

function recCleanup() {
  clearTimeout(rec.timer);
  if (rec.stream) rec.stream.getTracks().forEach((t) => t.stop());
  if (rec.ctx) rec.ctx.close().catch(() => {});
  rec.stream = rec.ctx = rec.node = null;
}

async function recStop() {
  if (!rec.ctx) return { error: 'Ingen optagelse.' };
  await new Promise((resolve) => {
    const t = setTimeout(resolve, 300);
    rec.node.port.addEventListener('message', (e) => { if (e.data === 'flushed') { clearTimeout(t); resolve(); } });
    rec.node.port.postMessage('flush');
  });
  const rate = rec.ctx.sampleRate;
  const chunks = rec.chunks;
  rec.chunks = [];
  recCleanup();
  try {
    const pcm = await resample(concat(chunks), rate, STT_RATE);
    const res = await fetch(API + '/api/stt', {
      method: 'POST',
      headers: { 'Content-Type': 'audio/wav' },
      body: encodeWAV(pcm, STT_RATE),
    });
    if (!res.ok) return { error: await apiError(res) };
    const { text } = await res.json();
    return { text: text || '' };
  } catch (e) {
    return { error: offlineMessage(e) };
  }
}

// ---------- Beskeder ----------

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (msg.target !== 'offscreen') return false;
  const handlers = {
    speak: () => speak(msg.text, msg.speed),
    prefetch: () => { fetchTTS(msg.text, msg.speed).catch(() => {}); return { ok: true }; },
    pause: () => { player.pause(); return { ok: true }; },
    resume: () => { player.play().catch(() => {}); return { ok: true }; },
    stop: () => { stopPlayback(); return { ok: true }; },
    recStart,
    recStop,
    recCancel: () => { recCleanup(); return { ok: true }; },
    status: async () => {
      try {
        const r = await fetch(API + '/api/status');
        return await r.json();
      } catch {
        return { status: 'offline' };
      }
    },
  };
  const h = handlers[msg.type];
  if (!h) return false;
  Promise.resolve(h()).then(sendResponse, (e) => sendResponse({ error: String(e && e.message || e) }));
  return true;
});
