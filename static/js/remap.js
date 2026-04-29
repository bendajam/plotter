'use strict';

// ── Homography (Direct Linear Transform) ─────────────────────────────────────

function computeH(src4, dst4) {
  // src4, dst4: [{x,y}] × 4  (old image → new image)
  // Returns 3×3 matrix or null if singular.
  const A = [], b = [];
  for (let i = 0; i < 4; i++) {
    const { x: sx, y: sy } = src4[i];
    const { x: dx, y: dy } = dst4[i];
    A.push([sx, sy, 1, 0, 0, 0, -sx * dx, -sy * dx]); b.push(dx);
    A.push([0, 0, 0, sx, sy, 1, -sx * dy, -sy * dy]); b.push(dy);
  }
  const h = gaussElim(A, b);
  if (!h) return null;
  return [
    [h[0], h[1], h[2]],
    [h[3], h[4], h[5]],
    [h[6], h[7], 1],
  ];
}

function gaussElim(A, b) {
  const n = 8;
  A = A.map(r => r.slice());
  b = b.slice();
  for (let col = 0; col < n; col++) {
    let maxRow = col, maxVal = Math.abs(A[col][col]);
    for (let row = col + 1; row < n; row++) {
      if (Math.abs(A[row][col]) > maxVal) { maxVal = Math.abs(A[row][col]); maxRow = row; }
    }
    if (maxVal < 1e-12) return null;
    [A[col], A[maxRow]] = [A[maxRow], A[col]];
    [b[col], b[maxRow]] = [b[maxRow], b[col]];
    for (let row = col + 1; row < n; row++) {
      const f = A[row][col] / A[col][col];
      b[row] -= f * b[col];
      for (let c = col; c < n; c++) A[row][c] -= f * A[col][c];
    }
  }
  const x = new Array(n).fill(0);
  for (let i = n - 1; i >= 0; i--) {
    x[i] = b[i];
    for (let j = i + 1; j < n; j++) x[i] -= A[i][j] * x[j];
    x[i] /= A[i][i];
  }
  return x;
}

function applyH(H, x, y) {
  const w = H[2][0] * x + H[2][1] * y + H[2][2];
  if (Math.abs(w) < 1e-12) return { x, y };
  return {
    x: (H[0][0] * x + H[0][1] * y + H[0][2]) / w,
    y: (H[1][0] * x + H[1][1] * y + H[1][2]) / w,
  };
}

function clamp01(v) { return v < 0 ? 0 : v > 1 ? 1 : v; }

function transformCoords(H, shape, coords) {
  const tf = (x, y) => applyH(H, x, y);
  switch (shape) {
    case 'point': { const p = tf(coords.x, coords.y); return { ...coords, ...p }; }
    case 'circle': { const c = tf(coords.cx, coords.cy); return { ...coords, cx: c.x, cy: c.y }; }
    case 'line': {
      const p1 = tf(coords.x1, coords.y1), p2 = tf(coords.x2, coords.y2);
      return { x1: p1.x, y1: p1.y, x2: p2.x, y2: p2.y };
    }
    case 'rect': {
      const { x, y, w, h } = coords;
      const corners = [[x,y],[x+w,y],[x,y+h],[x+w,y+h]].map(([cx,cy]) => tf(cx, cy));
      const xs = corners.map(p => p.x), ys = corners.map(p => p.y);
      const minX = Math.min(...xs), maxX = Math.max(...xs);
      const minY = Math.min(...ys), maxY = Math.max(...ys);
      return { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
    }
    case 'path':
    case 'area':
      return { points: coords.points.map(p => tf(p.x, p.y)) };
  }
  return coords;
}

// Returns true if all key coordinate points of a transformed shape are within [0,1].
function coordsInBounds(coords, shape) {
  const ok = (x, y) => x >= 0 && x <= 1 && y >= 0 && y <= 1;
  switch (shape) {
    case 'point':  return ok(coords.x,  coords.y);
    case 'circle': return ok(coords.cx, coords.cy);
    case 'line':   return ok(coords.x1, coords.y1) && ok(coords.x2, coords.y2);
    case 'rect':   return ok(coords.x,  coords.y)  && ok(coords.x + coords.w, coords.y + coords.h);
    case 'path':
    case 'area':   return (coords.points || []).every(p => ok(p.x, p.y));
  }
  return true;
}

// ── Shape drawing ─────────────────────────────────────────────────────────────

const PT_COLORS = ['#ef4444', '#f59e0b', '#22c55e', '#3b82f6'];

function hexToRGBA(hex, alpha) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  if (isNaN(r)) return `rgba(100,116,139,${alpha})`;
  return `rgba(${r},${g},${b},${alpha})`;
}

function drawShape(ctx, coords, shape, W, H, color) {
  ctx.save();
  ctx.strokeStyle = color;
  ctx.fillStyle   = hexToRGBA(color, 0.25);
  ctx.lineWidth   = Math.max(2, W * 0.002);

  switch (shape) {
    case 'point': {
      const r = Math.max(6, W * 0.008);
      ctx.beginPath(); ctx.arc(coords.x * W, coords.y * H, r, 0, Math.PI * 2);
      ctx.fill(); ctx.stroke(); break;
    }
    case 'circle':
      ctx.beginPath(); ctx.arc(coords.cx * W, coords.cy * H, coords.r * W, 0, Math.PI * 2);
      ctx.fill(); ctx.stroke(); break;
    case 'line':
      ctx.beginPath();
      ctx.moveTo(coords.x1 * W, coords.y1 * H);
      ctx.lineTo(coords.x2 * W, coords.y2 * H);
      ctx.stroke(); break;
    case 'rect':
      ctx.fillRect(coords.x * W, coords.y * H, coords.w * W, coords.h * H);
      ctx.strokeRect(coords.x * W, coords.y * H, coords.w * W, coords.h * H); break;
    case 'path':
    case 'area': {
      const pts = coords.points;
      if (!pts || pts.length < 2) break;
      ctx.beginPath();
      ctx.moveTo(pts[0].x * W, pts[0].y * H);
      for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
      if (shape === 'area') ctx.closePath();
      ctx.fill(); ctx.stroke(); break;
    }
  }
  ctx.restore();
}

// ── PointPicker ───────────────────────────────────────────────────────────────

class PointPicker {
  constructor(canvasId, imageUrl, onChanged) {
    this.canvas    = document.getElementById(canvasId);
    this.ctx       = this.canvas.getContext('2d');
    this.image     = null;
    this.points    = [];   // [{x,y}] normalized, max 4
    this.dragging  = null; // index or null
    this._H        = null; // homography for preview (set externally)
    this._markers  = null; // markers for preview (set externally)
    this.onChanged = onChanged;

    this._loadImage(imageUrl);
    this._bindEvents();
  }

  _loadImage(url) {
    this.image = new Image();
    this.image.onload = () => {
      this.canvas.width        = this.image.naturalWidth;
      this.canvas.height       = this.image.naturalHeight;
      this.canvas.style.width  = '100%';
      this.canvas.style.height = 'auto';
      this._render();
    };
    this.image.onerror = () => {
      this.canvas.width  = 800;
      this.canvas.height = 500;
      this.ctx.fillStyle = '#1e293b';
      this.ctx.fillRect(0, 0, 800, 500);
      this.ctx.fillStyle = '#ef4444';
      this.ctx.font = '16px sans-serif';
      this.ctx.textAlign = 'center';
      this.ctx.fillText('Could not load image', 400, 250);
    };
    this.image.src = url;
  }

  _getPos(e) {
    const rect   = this.canvas.getBoundingClientRect();
    const scaleX = this.canvas.width  / rect.width;
    const scaleY = this.canvas.height / rect.height;
    return {
      x: clamp01(((e.clientX || e.touches[0].clientX) - rect.left) * scaleX / this.canvas.width),
      y: clamp01(((e.clientY || e.touches[0].clientY) - rect.top)  * scaleY / this.canvas.height),
    };
  }

  _pixelPos(e) {
    // Returns canvas pixel coordinates for hit-testing.
    const rect   = this.canvas.getBoundingClientRect();
    const scaleX = this.canvas.width  / rect.width;
    const scaleY = this.canvas.height / rect.height;
    return {
      x: ((e.clientX || e.touches[0].clientX) - rect.left) * scaleX,
      y: ((e.clientY || e.touches[0].clientY) - rect.top)  * scaleY,
    };
  }

  _hitTest(e) {
    const px = this._pixelPos(e);
    const W = this.canvas.width, H = this.canvas.height;
    const thresh = Math.max(18, W * 0.018);
    for (let i = this.points.length - 1; i >= 0; i--) {
      const dx = this.points[i].x * W - px.x;
      const dy = this.points[i].y * H - px.y;
      if (Math.hypot(dx, dy) < thresh) return i;
    }
    return -1;
  }

  _bindEvents() {
    const down = e => {
      e.preventDefault();
      const hit = this._hitTest(e);
      if (hit >= 0) {
        this.dragging = hit;
        this.canvas.style.cursor = 'grabbing';
      } else if (this.points.length < 4) {
        this.points.push(this._getPos(e));
        this.dragging = this.points.length - 1;
        this._render();
        this.onChanged();
      }
    };
    const move = e => {
      if (this.dragging === null) {
        const hit = this._hitTest(e);
        this.canvas.style.cursor = hit >= 0 ? 'grab' : this.points.length < 4 ? 'crosshair' : 'default';
        return;
      }
      e.preventDefault();
      this.points[this.dragging] = this._getPos(e);
      this._render();
      this.onChanged();
    };
    const up = () => {
      this.dragging = null;
      this.canvas.style.cursor = this.points.length < 4 ? 'crosshair' : 'default';
    };

    this.canvas.addEventListener('mousedown',  down);
    this.canvas.addEventListener('mousemove',  move);
    this.canvas.addEventListener('mouseup',    up);
    this.canvas.addEventListener('mouseleave', up);
    this.canvas.addEventListener('touchstart', down, { passive: false });
    this.canvas.addEventListener('touchmove',  move, { passive: false });
    this.canvas.addEventListener('touchend',   up);

    this.canvas.style.cursor = 'crosshair';
  }

  clear() {
    this.points   = [];
    this.dragging = null;
    this._H       = null;
    this._markers = null;
    this.canvas.style.cursor = 'crosshair';
    this._render();
    this.onChanged();
  }

  drawPreview(H, markers) {
    this._H       = H;
    this._markers = markers;
    this._render();
  }

  _render() {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;
    ctx.clearRect(0, 0, W, H);

    if (this.image && this.image.complete && this.image.naturalWidth > 0) {
      ctx.drawImage(this.image, 0, 0);
    }

    // Transformed marker preview
    if (this._H && this._markers) {
      ctx.save();
      ctx.globalAlpha = 0.85;
      for (const m of this._markers) {
        const coords = typeof m.coords === 'string' ? JSON.parse(m.coords) : m.coords;
        const tc     = transformCoords(this._H, m.shape, coords);
        const inBounds = coordsInBounds(tc, m.shape);
        drawShape(ctx, tc, m.shape, W, H, inBounds ? '#f59e0b' : '#ef4444');
      }
      ctx.restore();
    }

    // Reference points
    const r = Math.max(12, W * 0.014);
    const fontSize = Math.max(12, W * 0.013);

    for (let i = 0; i < this.points.length; i++) {
      const p   = this.points[i];
      const px  = p.x * W, py = p.y * H;
      const col = PT_COLORS[i];

      ctx.save();
      // Outer glow ring
      ctx.beginPath();
      ctx.arc(px, py, r + 4, 0, Math.PI * 2);
      ctx.strokeStyle = 'rgba(255,255,255,0.7)';
      ctx.lineWidth   = 3;
      ctx.stroke();

      // Filled circle
      ctx.beginPath();
      ctx.arc(px, py, r, 0, Math.PI * 2);
      ctx.fillStyle   = col;
      ctx.fill();
      ctx.strokeStyle = '#fff';
      ctx.lineWidth   = 2;
      ctx.stroke();

      // Number
      ctx.fillStyle    = '#fff';
      ctx.font         = `bold ${fontSize}px sans-serif`;
      ctx.textAlign    = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(i + 1, px, py);
      ctx.restore();
    }
  }
}

// ── RemapController ───────────────────────────────────────────────────────────

class RemapController {
  constructor() {
    this.newPicker    = null;
    this.oldPicker    = null;
    this.newImagePath = null;

    document.getElementById('upload-form').addEventListener('submit', e => {
      e.preventDefault();
      this._doUpload();
    });
    document.getElementById('clear-btn').addEventListener('click', () => {
      this.newPicker && this.newPicker.clear();
      this.oldPicker && this.oldPicker.clear();
    });
    document.getElementById('restart-btn').addEventListener('click', () => {
      document.getElementById('stage-upload').style.display = '';
      document.getElementById('stage-pick').style.display   = 'none';
      this.newPicker = null;
      this.oldPicker = null;
    });
    document.getElementById('apply-btn').addEventListener('click', () => this._applyRemap());
  }

  _doUpload() {
    const file = document.getElementById('image-file').files[0];
    if (!file) return;

    const errEl  = document.getElementById('upload-error');
    const progEl = document.getElementById('upload-progress');
    errEl.style.display  = 'none';
    progEl.style.display = '';

    document.querySelector('#upload-form button[type=submit]').disabled = true;

    const fd = new FormData();
    fd.append('image', file);

    fetch(`/plots/${PLOT_ID}/image-upload`, { method: 'POST', body: fd })
      .then(r => r.ok ? r.json() : r.text().then(t => Promise.reject(t)))
      .then(data => {
        progEl.style.display = 'none';
        this.newImagePath = data.image_path;
        this._startPickStage(data.image_url);
      })
      .catch(err => {
        progEl.style.display = 'none';
        document.querySelector('#upload-form button[type=submit]').disabled = false;
        errEl.textContent    = 'Upload failed: ' + err;
        errEl.style.display  = '';
      });
  }

  _startPickStage(newImageUrl) {
    document.getElementById('stage-upload').style.display = 'none';
    document.getElementById('stage-pick').style.display   = '';

    const onChange = () => this._onChanged();
    this.newPicker = new PointPicker('new-canvas', newImageUrl, onChange);
    this.oldPicker = new PointPicker('old-canvas', OLD_IMAGE,   onChange);
    this._onChanged();
  }

  _onChanged() {
    const np = this.newPicker ? this.newPicker.points.length : 0;
    const op = this.oldPicker ? this.oldPicker.points.length : 0;

    document.getElementById('new-count').textContent = `${np} / 4`;
    document.getElementById('old-count').textContent = `${op} / 4`;

    const ready = np === 4 && op === 4;
    document.getElementById('apply-btn').disabled = !ready;

    if (ready) {
      this._updatePreview();
    } else {
      // Clear any stale preview on the new image
      if (this.newPicker) this.newPicker.drawPreview(null, null);
      this._setStatus(this._progressMsg(np, op));
    }
  }

  _progressMsg(np, op) {
    if (np < 4 && op < 4) return `Place ${4-np} more on the new photo and ${4-op} more on the current plot.`;
    if (np < 4)            return `Place ${4-np} more point${4-np===1?'':'s'} on the new photo.`;
                           return `Place ${4-op} more point${4-op===1?'':'s'} on the current plot.`;
  }

  _updatePreview() {
    const srcPts = this.oldPicker.points;
    const dstPts = this.newPicker.points;
    const H = computeH(srcPts, dstPts);
    if (!H) {
      this._setStatus('⚠ Points may be collinear — try spreading them further apart.');
      this.newPicker.drawPreview(null, null);
      return;
    }

    const markers = (MARKERS || []).map(m => ({
      ...m,
      coords: typeof m.coords === 'string' ? JSON.parse(m.coords) : m.coords,
    }));

    // Count how many markers fall outside the new image bounds.
    let outOfBounds = 0;
    for (const m of markers) {
      const tc = transformCoords(H, m.shape, m.coords);
      if (!coordsInBounds(tc, m.shape)) outOfBounds++;
    }

    this.newPicker.drawPreview(H, markers);

    if (outOfBounds > 0) {
      this._setStatus(
        `Preview ready. ${outOfBounds} marker${outOfBounds === 1 ? '' : 's'} (shown in red) fall outside the new photo's bounds — they will be clamped to the edge. Consider choosing control points that cover those areas, or accept that those markers won't be visible.`
      );
    } else {
      this._setStatus('Preview ready — all markers fit within the new photo. Adjust points or click Apply Remap.');
    }
    document.getElementById('remap-error').style.display = 'none';
  }

  _setStatus(msg) {
    document.getElementById('remap-status').textContent = msg;
  }

  _applyRemap() {
    const srcPts = this.oldPicker.points;
    const dstPts = this.newPicker.points;

    const btn = document.getElementById('apply-btn');
    btn.disabled    = true;
    btn.textContent = 'Applying…';

    const capturedDate = (document.getElementById('captured-date') || {}).value || '';
    fetch(`/plots/${PLOT_ID}/remap`, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify({
        new_image_path: this.newImagePath,
        src_points:     srcPts,
        dst_points:     dstPts,
        captured_date:  capturedDate,
      }),
    })
      .then(r => r.ok ? r.json() : r.text().then(t => Promise.reject(t)))
      .then(data => { window.location.href = data.redirect; })
      .catch(err => {
        btn.disabled    = false;
        btn.textContent = 'Apply Remap';
        const errEl = document.getElementById('remap-error');
        errEl.textContent   = 'Error: ' + err;
        errEl.style.display = '';
      });
  }
}

new RemapController();
