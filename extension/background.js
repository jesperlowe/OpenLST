// OpenLST – service worker.
// Formidler mellem sidens indholdsscript og det skjulte offscreen-dokument,
// som afspiller lyd og optager mikrofonen. Alle kald går kun til OpenLST på
// denne computer (127.0.0.1:8765).
'use strict';

const OFFSCREEN = 'offscreen.html';

chrome.runtime.onInstalled.addListener((details) => {
  chrome.contextMenus.create({
    id: 'openlst-read',
    title: '🔊 Læs med OpenLST',
    contexts: ['selection'],
  });
  if (details.reason === 'install') {
    chrome.tabs.create({ url: chrome.runtime.getURL('setup.html') });
  }
});

let creating = null;
async function ensureOffscreen() {
  const contexts = await chrome.runtime.getContexts({ contextTypes: ['OFFSCREEN_DOCUMENT'] });
  if (contexts.length) return;
  if (!creating) {
    creating = chrome.offscreen.createDocument({
      url: OFFSCREEN,
      reasons: ['AUDIO_PLAYBACK', 'USER_MEDIA'],
      justification: 'Afspiller oplæsning og optager diktering lokalt.',
    }).finally(() => { creating = null; });
  }
  await creating;
}

async function toOffscreen(msg) {
  await ensureOffscreen();
  return chrome.runtime.sendMessage({ ...msg, target: 'offscreen' });
}

async function sendToTab(tabId, msg) {
  try {
    return await chrome.tabs.sendMessage(tabId, msg);
  } catch {
    return undefined; // intet indholdsscript (fx edge://-sider eller PDF)
  }
}

// Fra indholdsscript og setup-side.
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.target === 'offscreen') return false; // ikke til os
  if (msg.type === 'openSetup') {
    chrome.tabs.create({ url: chrome.runtime.getURL('setup.html') });
    return false;
  }
  if (['speak', 'prefetch', 'rate', 'pause', 'resume', 'stop', 'recStart', 'recStop', 'recCancel', 'status'].includes(msg.type)) {
    toOffscreen(msg).then(sendResponse, (e) => sendResponse({ error: String(e && e.message || e) }));
    return true; // asynkront svar
  }
  return false;
});

// Højreklik: læs markering. Hvis siden ikke har indholdsscript, læses teksten
// uden fremhævning.
chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (info.menuItemId !== 'openlst-read' || !tab) return;
  const handled = await sendToTab(tab.id, { type: 'readSelection' });
  if (!handled) {
    const { speed = 0.9 } = await chrome.storage.local.get('speed');
    toOffscreen({ type: 'speak', text: info.selectionText, speed }).catch(() => {});
  }
});

chrome.action.onClicked.addListener((tab) => {
  sendToTab(tab.id, { type: 'toggleToolbar' });
});

chrome.commands.onCommand.addListener(async (command, tab) => {
  if (!tab) return;
  if (command === 'read-selection') sendToTab(tab.id, { type: 'readSelection' });
  if (command === 'toggle-toolbar') sendToTab(tab.id, { type: 'toggleToolbar' });
});
