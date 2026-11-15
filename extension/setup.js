'use strict';
// Opsætning: tjek at OpenLST kører, og giv extensionen adgang til mikrofonen.
// Tilladelsen gives til extensionen (ikke til hjemmesider) og bruges af det
// skjulte offscreen-dokument under diktering.

const serverEl = document.getElementById('server');
const micEl = document.getElementById('mic');
const grantBtn = document.getElementById('grant');

async function checkServer() {
  try {
    const r = await fetch('http://127.0.0.1:8765/api/status');
    const s = await r.json();
    if (s.tts && s.stt) {
      serverEl.textContent = '✓ OpenLST kører.';
      serverEl.className = 'ok';
    } else {
      serverEl.textContent = 'OpenLST kører, men oplæsning eller diktering mangler. Geninstallér OpenLST.';
      serverEl.className = 'err';
    }
  } catch {
    serverEl.textContent = 'OpenLST kører ikke. Start OpenLST fra Start-menuen.';
    serverEl.className = 'err';
    setTimeout(checkServer, 3000);
  }
}

async function checkMic() {
  try {
    const p = await navigator.permissions.query({ name: 'microphone' });
    if (p.state === 'granted') {
      micEl.textContent = '✓ Mikrofonen er klar.';
      micEl.className = 'ok';
      grantBtn.hidden = true;
    }
  } catch { /* ukendt – vis knappen */ }
}

grantBtn.addEventListener('click', async () => {
  try {
    const s = await navigator.mediaDevices.getUserMedia({ audio: true });
    s.getTracks().forEach((t) => t.stop());
    checkMic();
  } catch {
    micEl.textContent = 'Mikrofonen blev ikke tilladt. Tjek også Windows-indstillinger → Beskyttelse af personlige oplysninger → Mikrofon.';
    micEl.className = 'err';
  }
});

checkServer();
checkMic();
