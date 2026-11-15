// Sætningsopdeling til oplæsning. Bruges af både webappen og browser-
// extensionen (extension/lib/sentences.js er en kopi – se scripts/sync-extension.sh).
'use strict';

self.OpenLSTText = (() => {
const MAX_CHUNK = 400; // tegn pr. TTS-forespørgsel

const ABBREVIATIONS = new Set([
  'f.eks.', 'fx.', 'bl.a.', 'kl.', 'ca.', 'dvs.', 'osv.', 'mht.', 'nr.', 'evt.',
  'hhv.', 'jf.', 'pga.', 'ift.', 'iflg.', 'mv.', 'mm.', 'st.', 'tlf.', 'vedr.',
  'inkl.', 'ekskl.', 'max.', 'min.', 'sv.', 'gl.', 'hr.', 'fr.', 'dr.', 'prof.',
]);

function endsWithAbbreviation(s) {
  const last = s.trim().split(/\s+/).pop() || '';
  return ABBREVIATIONS.has(last.toLowerCase());
}

// Deler teksten i sætninger og bevarer positioner, så den kan vises igen
// med fremhævning.
function splitSentences(text) {
  let parts = [];
  if (typeof Intl !== 'undefined' && Intl.Segmenter) {
    const seg = new Intl.Segmenter('da', { granularity: 'sentence' });
    for (const s of seg.segment(text)) parts.push({ start: s.index, end: s.index + s.segment.length });
  } else {
    const re = /[^.!?…\n]*(?:[.!?…]+|\n+|$)\s*/g;
    let m;
    while ((m = re.exec(text)) && m[0].length) parts.push({ start: m.index, end: m.index + m[0].length });
  }

  // Saml igen hvor en forkortelse (f.eks., kl. ...) har delt en sætning.
  const merged = [];
  for (const p of parts) {
    const prev = merged[merged.length - 1];
    if (prev && endsWithAbbreviation(text.slice(prev.start, prev.end)) &&
        !/\n\s*$/.test(text.slice(prev.start, prev.end))) {
      prev.end = p.end;
    } else {
      merged.push({ ...p });
    }
  }

  // Del meget lange sætninger ved komma eller mellemrum.
  const out = [];
  for (const p of merged) {
    let start = p.start;
    while (p.end - start > MAX_CHUNK) {
      const slice = text.slice(start, start + MAX_CHUNK);
      let cut = Math.max(slice.lastIndexOf(', '), slice.lastIndexOf('; '));
      if (cut < MAX_CHUNK / 2) cut = slice.lastIndexOf(' ');
      if (cut <= 0) cut = MAX_CHUNK - 1;
      out.push({ start, end: start + cut + 1 });
      start += cut + 1;
    }
    out.push({ start, end: p.end });
  }

  return out.map((p) => {
    const raw = text.slice(p.start, p.end);
    return { ...p, raw, speak: raw.replace(/\s+/g, ' ').trim() };
  });
}

// Kun sætninger med bogstaver eller tal læses op.
const isSpeakable = (s) => /[\p{L}\p{N}]/u.test(s.speak);

return { splitSentences, isSpeakable };
})();
