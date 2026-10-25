// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 SukramJ.
//
// A small QR code encoder (ISO/IEC 18004): byte mode, error correction
// level M, versions 1 to 6 — enough for a Matter pairing payload
// ("MT:" and 19 base-38 characters, version 2). It renders the pairing
// QR code in the browser from the payload text, so the daemon carries no
// image dependency. qrMatrix(text, mask) returns the module matrix
// (true = dark); mask is 0..7 or undefined for the lowest penalty.
"use strict";

(function (root) {
  // Level M: [total codewords, EC codewords per block, blocks] per version
  // (ISO/IEC 18004 Table 9); every block of v1..v6 M has the same size.
  const M = [null, [26, 10, 1], [44, 16, 1], [70, 26, 1], [100, 18, 2], [134, 24, 2], [172, 16, 4]];
  // Alignment pattern centres (Annex E).
  const ALIGN = [null, [], [6, 18], [6, 22], [6, 26], [6, 30], [6, 34]];

  // GF(256) with the polynomial 0x11D.
  const EXP = new Array(512), LOG = new Array(256);
  for (let i = 0, x = 1; i < 255; i++) {
    EXP[i] = x;
    LOG[x] = i;
    x <<= 1;
    if (x & 0x100) x ^= 0x11d;
  }
  for (let i = 255; i < 512; i++) EXP[i] = EXP[i - 255];
  const mul = (a, b) => (a === 0 || b === 0 ? 0 : EXP[LOG[a] + LOG[b]]);

  function generator(degree) {
    let g = [1];
    for (let i = 0; i < degree; i++) {
      const next = new Array(g.length + 1).fill(0);
      for (let j = 0; j < g.length; j++) {
        next[j] ^= g[j];
        next[j + 1] ^= mul(g[j], EXP[i]);
      }
      g = next;
    }
    return g;
  }

  function ecc(data, degree) {
    const g = generator(degree);
    const r = data.concat(new Array(degree).fill(0));
    for (let i = 0; i < data.length; i++) {
      const c = r[i];
      if (c !== 0) for (let j = 0; j < g.length; j++) r[i + j] ^= mul(g[j], c);
    }
    return r.slice(data.length);
  }

  function codewords(bytes, version) {
    const [total, ecPer, blocks] = M[version];
    const dataCount = total - ecPer * blocks;
    const bits = [];
    const put = (v, n) => { for (let i = n - 1; i >= 0; i--) bits.push((v >>> i) & 1); };
    put(0b0100, 4);
    put(bytes.length, 8);
    for (const b of bytes) put(b, 8);
    put(0, Math.min(4, dataCount * 8 - bits.length));
    while (bits.length % 8) bits.push(0);
    const data = [];
    for (let i = 0; i < bits.length; i += 8) {
      let v = 0;
      for (let j = 0; j < 8; j++) v = (v << 1) | bits[i + j];
      data.push(v);
    }
    for (let pad = 0xec; data.length < dataCount; pad ^= 0xec ^ 0x11) data.push(pad);
    const per = dataCount / blocks;
    const dataBlocks = [], ecBlocks = [];
    for (let b = 0; b < blocks; b++) {
      const block = data.slice(b * per, (b + 1) * per);
      dataBlocks.push(block);
      ecBlocks.push(ecc(block, ecPer));
    }
    const out = [];
    for (let i = 0; i < per; i++) for (const bl of dataBlocks) out.push(bl[i]);
    for (let i = 0; i < ecPer; i++) for (const bl of ecBlocks) out.push(bl[i]);
    return out;
  }

  function base(version) {
    const n = 17 + 4 * version;
    const m = [], fn = [];
    for (let i = 0; i < n; i++) { m.push(new Array(n).fill(false)); fn.push(new Array(n).fill(false)); }
    const set = (r, c, v) => { m[r][c] = v; fn[r][c] = true; };
    const finder = (r0, c0) => {
      for (let r = -1; r <= 7; r++) for (let c = -1; c <= 7; c++) {
        const rr = r0 + r, cc = c0 + c;
        if (rr < 0 || cc < 0 || rr >= n || cc >= n) continue;
        const ring = Math.max(Math.abs(r - 3), Math.abs(c - 3));
        set(rr, cc, ring !== 2 && ring !== 4);
      }
    };
    finder(0, 0); finder(0, n - 7); finder(n - 7, 0);
    for (let i = 8; i < n - 8; i++) { set(6, i, i % 2 === 0); set(i, 6, i % 2 === 0); }
    const al = ALIGN[version];
    for (const r of al) for (const c of al) {
      if ((r === 6 && c === 6) || (r === 6 && c === al[al.length - 1]) || (r === al[al.length - 1] && c === 6)) continue;
      for (let dr = -2; dr <= 2; dr++) for (let dc = -2; dc <= 2; dc++) set(r + dr, c + dc, Math.max(Math.abs(dr), Math.abs(dc)) !== 1);
    }
    // Format areas (filled later) and the dark module.
    for (let i = 0; i < 9; i++) { fn[8][i] = true; fn[i][8] = true; }
    for (let i = 0; i < 8; i++) { fn[8][n - 1 - i] = true; fn[n - 1 - i][8] = true; }
    set(n - 8, 8, true);
    return { m, fn, n };
  }

  const MASKS = [
    (r, c) => (r + c) % 2 === 0,
    (r) => r % 2 === 0,
    (r, c) => c % 3 === 0,
    (r, c) => (r + c) % 3 === 0,
    (r, c) => (Math.floor(r / 2) + Math.floor(c / 3)) % 2 === 0,
    (r, c) => ((r * c) % 2) + ((r * c) % 3) === 0,
    (r, c) => (((r * c) % 2) + ((r * c) % 3)) % 2 === 0,
    (r, c) => (((r + c) % 2) + ((r * c) % 3)) % 2 === 0,
  ];

  function place(b, cw) {
    const { m, fn, n } = b;
    let i = 0;
    for (let right = n - 1; right >= 1; right -= 2) {
      if (right === 6) right = 5;
      for (let vert = 0; vert < n; vert++) {
        for (let j = 0; j < 2; j++) {
          const c = right - j;
          const upward = ((right + 1) & 2) === 0;
          const r = upward ? n - 1 - vert : vert;
          if (fn[r][c]) continue;
          m[r][c] = i < cw.length * 8 && ((cw[i >>> 3] >>> (7 - (i & 7))) & 1) === 1;
          i++;
        }
      }
    }
  }

  function format(b, mask) {
    const { m, n } = b;
    const data = (0b00 << 3) | mask; // level M is 00
    let rem = data;
    for (let i = 0; i < 10; i++) rem = (rem << 1) ^ ((rem >>> 9) * 0x537);
    const bits = ((data << 10) | rem) ^ 0x5412;
    const bit = (i) => ((bits >>> i) & 1) === 1;
    // m[row][column]; the first copy around the top-left finder, the
    // second split between the other two.
    for (let i = 0; i <= 5; i++) m[i][8] = bit(i);
    m[7][8] = bit(6); m[8][8] = bit(7); m[8][7] = bit(8);
    for (let i = 9; i < 15; i++) m[8][14 - i] = bit(i);
    for (let i = 0; i < 8; i++) m[8][n - 1 - i] = bit(i);
    for (let i = 8; i < 15; i++) m[n - 15 + i][8] = bit(i);
    m[n - 8][8] = true;
  }

  function penalty(m) {
    const n = m.length;
    let p = 0;
    const line = (get) => {
      for (let a = 0; a < n; a++) {
        let run = 1;
        for (let k = 1; k < n; k++) {
          if (get(a, k) === get(a, k - 1)) { run++; if (run === 5) p += 3; else if (run > 5) p++; }
          else run = 1;
        }
        for (let k = 0; k + 10 < n + 1; k++) {
          const s = [];
          for (let t = 0; t < 11 && k + t < n; t++) s.push(get(a, k + t) ? 1 : 0);
          if (s.length < 11) break;
          const v = s.join("");
          if (v === "10111010000" || v === "00001011101") p += 40;
        }
      }
    };
    line((r, c) => m[r][c]);
    line((c, r) => m[r][c]);
    for (let r = 0; r < n - 1; r++) for (let c = 0; c < n - 1; c++) {
      const v = m[r][c];
      if (v === m[r][c + 1] && v === m[r + 1][c] && v === m[r + 1][c + 1]) p += 3;
    }
    let dark = 0;
    for (const row of m) for (const v of row) if (v) dark++;
    p += Math.floor(Math.abs(dark * 20 - n * n * 10) / (n * n)) * 10;
    return p;
  }

  function qrMatrix(text, forceMask) {
    const bytes = Array.from(new TextEncoder().encode(text));
    let version = 1;
    while (version <= 6) {
      const [total, ecPer, blocks] = M[version];
      if (4 + 8 + bytes.length * 8 <= (total - ecPer * blocks) * 8) break;
      version++;
    }
    if (version > 6) throw new Error("qr: payload too long");
    const cw = codewords(bytes, version);
    let best = null, bestScore = Infinity;
    for (let mask = 0; mask < 8; mask++) {
      if (forceMask !== undefined && mask !== forceMask) continue;
      const b = base(version);
      place(b, cw);
      for (let r = 0; r < b.n; r++) for (let c = 0; c < b.n; c++) if (!b.fn[r][c] && MASKS[mask](r, c)) b.m[r][c] = !b.m[r][c];
      format(b, mask);
      const score = penalty(b.m);
      if (score < bestScore) { best = b.m; bestScore = score; }
    }
    return best;
  }

  // qrSVG renders the matrix as an SVG string with a quiet zone of 4.
  function qrSVG(text, scale) {
    const m = qrMatrix(text);
    const n = m.length, q = 4, size = (n + 2 * q) * scale;
    let d = "";
    for (let r = 0; r < n; r++) for (let c = 0; c < n; c++) if (m[r][c]) d += `M${(c + q) * scale},${(r + q) * scale}h${scale}v${scale}h-${scale}z`;
    return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}" width="${size}" height="${size}" role="img" aria-label="Pairing QR code"><rect width="100%" height="100%" fill="#fff"/><path d="${d}" fill="#000"/></svg>`;
  }

  root.qrMatrix = qrMatrix;
  root.qrSVG = qrSVG;
  if (typeof module !== "undefined") module.exports = { qrMatrix, qrSVG };
})(typeof window !== "undefined" ? window : globalThis);
