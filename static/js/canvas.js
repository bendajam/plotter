'use strict';

class PlotCanvas {
  constructor(canvasId, imageUrl, markersData, today) {
    this.canvas = document.getElementById(canvasId);
    this.ctx    = this.canvas.getContext('2d');

    // Tool state
    this.tool    = 'select';
    this.drawing = false;
    this.startX  = 0;
    this.startY  = 0;
    this.curX    = 0;
    this.curY    = 0;

    // Path tool state
    this.pathPoints = []; // array of {x, y} in normalised coords

    // Pending shape waiting for label form
    this.pendingShape = null;

    // Selection
    this.selectedMarkerId = null;
    this.selectedMarkerIds = new Set();

    // Zoom & pan (in canvas-buffer pixel space)
    this.zoom  = 1.0;
    this.panX  = 0;
    this.panY  = 0;
    this._panning   = false;
    this._panStartX = 0;
    this._panStartY = 0;
    this._panOriginX = 0;
    this._panOriginY = 0;

    // Filters
    this.activeCategories = null; // Set of catIds or null (show all)
    this.activeLayers     = null; // Set of layerIds or null (show all)
    this.showExpired      = false;
    this.today            = today || '';

    // Markers
    this.markers = markersData ? markersData.map(m => ({
      id:      m.id,
      shape:   m.shape,
      coords:  typeof m.coords === 'string' ? JSON.parse(m.coords) : m.coords,
      label:   m.label,
      catId:   m.catId   || 0,
      layerId: m.layerId || 0,
      color:   m.color   || '#64748b',
      endDate: m.endDate || '',
    })) : [];

    this.canvas.style.cursor = 'pointer'; // matches default select tool
    this._loadImage(imageUrl);
    this._bindEvents();
  }

  // ── Public API ────────────────────────────────────────────────

  setTool(tool) {
    this.tool = tool;
    this.canvas.style.cursor = tool === 'select' ? 'pointer' : 'crosshair';
  }

  finalizePending(id, shape, coords, label, catId, layerId, color, endDate) {
    if (this.pendingShape) {
      this.markers.push({
        id, shape, coords, label,
        catId:   catId   || 0,
        layerId: layerId || 0,
        color:   color   || '#64748b',
        endDate: endDate || '',
      });
      this.pendingShape = null;
      this._render();
    }
  }

  clearPending() {
    this.pendingShape = null;
    this._render();
  }

  removeMarker(id) {
    this.markers = this.markers.filter(m => m.id !== id);
    if (this.selectedMarkerId === id) this.selectedMarkerId = null;
    this.selectedMarkerIds.delete(id);
    this._render();
  }

  clearGroupSelection() {
    this.selectedMarkerIds.clear();
    this.selectedMarkerId = null;
    this._render();
    this._notifyMultiSelect();
  }

  // Highlight a single marker on canvas without changing the side panel
  selectMarker(id) {
    this.selectedMarkerIds.clear();
    this.selectedMarkerIds.add(id);
    this.selectedMarkerId = id;
    this._render();
  }

  // Highlight a set of markers (e.g. all members of a group)
  selectMarkers(ids) {
    this.selectedMarkerIds = new Set(ids);
    this.selectedMarkerId = ids.length > 0 ? ids[ids.length - 1] : null;
    this._render();
  }

  setCatFilter(categoryIds) {
    this.activeCategories = categoryIds ? new Set(categoryIds) : null;
    this._render();
  }

  setLayerFilter(layerIds) {
    this.activeLayers = layerIds ? new Set(layerIds) : null;
    this._render();
  }

  setShowExpired(val) {
    this.showExpired = val;
    this._render();
  }

  // Path tool methods
  finishPath() {
    if (this.pathPoints.length < 2) {
      this.cancelPath();
      return;
    }
    const coords = { points: [...this.pathPoints] };
    this.pendingShape = { shape: 'path', coords };
    this.pathPoints = [];
    this._render();
    if (typeof window.onShapeDrawn === 'function') {
      window.onShapeDrawn('path', coords);
    }
  }

  cancelPath() {
    this.pathPoints = [];
    this.drawing    = false;
    this._render();
  }

  // Zoom by a factor, optionally centred on a MouseEvent
  zoomBy(factor, mouseEvent) {
    const W = this.canvas.width, H = this.canvas.height;
    let cx = W / 2, cy = H / 2;
    if (mouseEvent) {
      const buf = this._bufferPos(mouseEvent);
      cx = buf.x; cy = buf.y;
    }
    const newZoom = Math.max(0.5, Math.min(12, this.zoom * factor));
    const ratio   = newZoom / this.zoom;
    this.panX  = cx - ratio * (cx - this.panX);
    this.panY  = cy - ratio * (cy - this.panY);
    this.zoom  = newZoom;
    this._render();
    this._notifyZoom();
  }

  resetZoom() {
    this.zoom = 1.0;
    this.panX = 0;
    this.panY = 0;
    this._render();
    this._notifyZoom();
  }

  // ── Image loading ─────────────────────────────────────────────

  _loadImage(url) {
    this.image = new Image();
    this.image.onload = () => {
      this.canvas.width  = this.image.naturalWidth;
      this.canvas.height = this.image.naturalHeight;
      this.canvas.style.width  = '100%';
      this.canvas.style.height = 'auto';
      this._render();
    };
    this.image.onerror = () => {
      console.error('PlotCanvas: failed to load', url);
      this.canvas.width  = 800;
      this.canvas.height = 500;
      this.canvas.style.width  = '100%';
      this.canvas.style.height = 'auto';
      const ctx = this.ctx;
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(0, 0, 800, 500);
      ctx.fillStyle = '#ef4444';
      ctx.font = 'bold 18px sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText('Could not load image: ' + url, 400, 250);
    };
    this.image.src = url;
  }

  // ── Event binding ─────────────────────────────────────────────

  _bindEvents() {
    this.canvas.addEventListener('mousedown',  e => this._onDown(e));
    this.canvas.addEventListener('mousemove',  e => this._onMove(e));
    this.canvas.addEventListener('mouseup',    e => this._onUp(e));
    this.canvas.addEventListener('mouseleave', () => {
      if (this.drawing && this.tool !== 'path') {
        this.drawing = false;
        this._render();
      }
      if (this._panning) {
        this._panning = false;
        this._render();
      }
    });
    this.canvas.addEventListener('wheel', e => {
      e.preventDefault();
      const factor = e.deltaY < 0 ? 1.15 : 1 / 1.15;
      this.zoomBy(factor, e);
    }, { passive: false });

    // Keyboard: Enter to finish path, Esc to cancel
    document.addEventListener('keydown', e => {
      if (this.tool === 'path') {
        if (e.key === 'Enter') { e.preventDefault(); this.finishPath(); }
        if (e.key === 'Escape') { e.preventDefault(); this.cancelPath(); }
      }
    });
  }

  _bufferPos(e) {
    const rect   = this.canvas.getBoundingClientRect();
    const scaleX = this.canvas.width  / rect.width;
    const scaleY = this.canvas.height / rect.height;
    return {
      x: (e.clientX - rect.left) * scaleX,
      y: (e.clientY - rect.top)  * scaleY,
    };
  }

  _getRelPos(e) {
    const buf = this._bufferPos(e);
    if (this.canvas.width === 0 || this.canvas.height === 0) return { x: 0, y: 0 };
    const worldX = (buf.x - this.panX) / this.zoom;
    const worldY = (buf.y - this.panY) / this.zoom;
    return {
      x: worldX / this.canvas.width,
      y: worldY / this.canvas.height,
    };
  }

  _onDown(e) {
    if (e.button === 1) {
      e.preventDefault();
      this._startPan(e);
      return;
    }
    if (e.button !== 0) return;
    e.preventDefault();

    if (e.ctrlKey || e.metaKey) {
      this._startPan(e);
      return;
    }

    if (this.tool === 'select') {
      const pos = this._getRelPos(e);
      const hit = this._hitTest(pos.x, pos.y);

      if (e.shiftKey) {
        if (hit) {
          if (typeof window.onShiftSelect === 'function') {
            // Group mode: add the marker to the active group and highlight it
            this.selectedMarkerIds.add(hit.id);
            this._render();
            window.onShiftSelect(hit.id);
          } else {
            // Normal multi-select toggle
            if (this.selectedMarkerIds.has(hit.id)) {
              this.selectedMarkerIds.delete(hit.id);
              if (this.selectedMarkerId === hit.id) {
                const remaining = [...this.selectedMarkerIds];
                this.selectedMarkerId = remaining.length > 0 ? remaining[remaining.length - 1] : null;
              }
            } else {
              this.selectedMarkerIds.add(hit.id);
              this.selectedMarkerId = hit.id;
            }
            this._render();
            this._notifyMultiSelect();
          }
        }
        return;
      }

      // Regular click: leave group mode, clear multi-selection, select single
      if (typeof window.clearGroupMode === 'function') window.clearGroupMode();
      this.selectedMarkerIds.clear();
      if (hit) {
        this.selectedMarkerId = hit.id;
        this.selectedMarkerIds.add(hit.id);
        this._render();
        this._notifyMultiSelect();
        htmx.ajax('GET', `/markers/${hit.id}`, { target: '#marker-detail-panel', swap: 'innerHTML' });
      } else {
        this.selectedMarkerId = null;
        this._render();
        this._notifyMultiSelect();
        this._startPan(e);
      }
      return;
    }

    if (this.tool === 'path') {
      const pos = this._getRelPos(e);
      this.pathPoints.push({ x: pos.x, y: pos.y });
      this.drawing = true;
      this.curX = pos.x;
      this.curY = pos.y;
      this._render();
      return;
    }

    this.drawing = true;
    const pos    = this._getRelPos(e);
    this.startX  = pos.x;
    this.startY  = pos.y;
  }

  _onMove(e) {
    if (this._panning) {
      e.preventDefault();
      const buf  = this._bufferPos(e);
      this.panX  = this._panOriginX + (buf.x - this._panStartX);
      this.panY  = this._panOriginY + (buf.y - this._panStartY);
      this._render();
      return;
    }
    if (!this.drawing) return;
    e.preventDefault();
    const pos = this._getRelPos(e);
    this.curX = pos.x;
    this.curY = pos.y;
    this._render();
  }

  _onUp(e) {
    if (this._panning) {
      this._panning = false;
      this.canvas.style.cursor = this.tool === 'select' ? 'pointer' : 'crosshair';
      return;
    }
    if (!this.drawing) return;
    if (this.tool === 'path') return; // path points added on mousedown; finish via button/Enter
    e.preventDefault();
    this.drawing = false;
    const pos    = this._getRelPos(e);
    this._confirmShape(this.startX, this.startY, pos.x, pos.y);
  }

  _startPan(e) {
    const buf        = this._bufferPos(e);
    this._panning    = true;
    this._panStartX  = buf.x;
    this._panStartY  = buf.y;
    this._panOriginX = this.panX;
    this._panOriginY = this.panY;
    this.canvas.style.cursor = 'grab';
  }

  // ── Shape confirmation ────────────────────────────────────────

  _confirmShape(x1, y1, x2, y2) {
    let coords;
    switch (this.tool) {
      case 'point':
        coords = { x: x1, y: y1 };
        break;
      case 'circle': {
        const r = Math.hypot(x2 - x1, y2 - y1);
        if (r < 0.005) return;
        coords = { cx: x1, cy: y1, r };
        break;
      }
      case 'line': {
        if (Math.hypot(x2 - x1, y2 - y1) < 0.005) return;
        coords = { x1, y1, x2, y2 };
        break;
      }
      case 'rect': {
        const w = Math.abs(x2 - x1), h = Math.abs(y2 - y1);
        if (w < 0.005 || h < 0.005) return;
        coords = { x: Math.min(x1, x2), y: Math.min(y1, y2), w, h };
        break;
      }
      default:
        return;
    }

    this.pendingShape = { shape: this.tool, coords };
    this._render();

    if (typeof window.onShapeDrawn === 'function') {
      window.onShapeDrawn(this.tool, coords);
    }
  }

  // ── Rendering ─────────────────────────────────────────────────

  _render() {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, W, H);
    ctx.setTransform(this.zoom, 0, 0, this.zoom, this.panX, this.panY);

    if (this.image && this.image.complete && this.image.naturalWidth > 0) {
      ctx.drawImage(this.image, 0, 0);
    }

    for (const m of this.markers) {
      if (!this._isVisible(m)) continue;
      this._drawShape(m.coords, m.shape, this.selectedMarkerIds.has(m.id), m.label, false, m.color);
    }

    if (this.pendingShape) {
      this._drawShape(this.pendingShape.coords, this.pendingShape.shape, false, '', true, '#f59e0b');
    }

    // Live preview while dragging a new shape
    if (this.drawing) {
      if (this.tool === 'path') {
        this._drawPathPreview();
      } else {
        this._drawPreview(this.startX, this.startY, this.curX, this.curY);
      }
    }

    ctx.setTransform(1, 0, 0, 1, 0, 0);
  }

  _isVisible(m) {
    // Category filter
    if (this.activeCategories && !this.activeCategories.has(m.catId)) return false;
    // Layer filter
    if (this.activeLayers && !this.activeLayers.has(m.layerId)) return false;
    // Expiry filter
    if (!this.showExpired && m.endDate && this.today && m.endDate < this.today) return false;
    return true;
  }

  _drawShape(coords, shape, selected, label, pending, baseColor) {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;

    ctx.save();
    const lw = Math.max(2, W * 0.002);
    ctx.lineWidth = lw;

    const stroke = pending ? '#f59e0b' : selected ? '#fff' : (baseColor || '#ef4444');
    const fill   = pending ? 'rgba(245,158,11,0.25)' : selected
      ? hexToRGBA(stroke, 0.35)
      : hexToRGBA(stroke, 0.2);

    ctx.strokeStyle = stroke;
    ctx.fillStyle   = fill;

    const dotR  = Math.max(6, W * 0.008);
    const fSize = Math.max(12, W * 0.013);

    switch (shape) {
      case 'point': {
        const px = coords.x * W, py = coords.y * H;
        if (selected || pending) {
          ctx.save();
          ctx.strokeStyle = 'rgba(255,255,255,0.7)';
          ctx.lineWidth   = lw * 2.5;
          ctx.beginPath();
          ctx.arc(px, py, dotR + 2, 0, Math.PI * 2);
          ctx.stroke();
          ctx.restore();
        }
        ctx.beginPath();
        ctx.arc(px, py, dotR, 0, Math.PI * 2);
        ctx.fill();
        ctx.stroke();
        if (label) this._label(ctx, label, px + dotR + 3, py + fSize * 0.4, fSize, stroke);
        break;
      }
      case 'circle': {
        const cx = coords.cx * W, cy = coords.cy * H, r = coords.r * W;
        ctx.beginPath();
        ctx.arc(cx, cy, r, 0, Math.PI * 2);
        ctx.fill();
        ctx.stroke();
        if (label) this._label(ctx, label, cx + r + 3, cy, fSize, stroke);
        break;
      }
      case 'line': {
        ctx.save();
        ctx.strokeStyle = 'rgba(0,0,0,0.3)';
        ctx.lineWidth   = lw * 3;
        ctx.beginPath();
        ctx.moveTo(coords.x1 * W, coords.y1 * H);
        ctx.lineTo(coords.x2 * W, coords.y2 * H);
        ctx.stroke();
        ctx.restore();
        ctx.beginPath();
        ctx.moveTo(coords.x1 * W, coords.y1 * H);
        ctx.lineTo(coords.x2 * W, coords.y2 * H);
        ctx.stroke();
        break;
      }
      case 'rect': {
        const rx = coords.x * W, ry = coords.y * H;
        const rw = coords.w * W, rh = coords.h * H;
        ctx.fillRect(rx, ry, rw, rh);
        ctx.strokeRect(rx, ry, rw, rh);
        if (label) this._label(ctx, label, rx + 4, ry + fSize, fSize, stroke);
        break;
      }
      case 'path': {
        const pts = coords.points;
        if (!pts || pts.length < 2) break;
        // Shadow for visibility
        ctx.save();
        ctx.strokeStyle = 'rgba(0,0,0,0.3)';
        ctx.lineWidth   = lw * 3;
        ctx.beginPath();
        ctx.moveTo(pts[0].x * W, pts[0].y * H);
        for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
        ctx.stroke();
        ctx.restore();
        ctx.beginPath();
        ctx.moveTo(pts[0].x * W, pts[0].y * H);
        for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
        ctx.stroke();
        // Vertex dots
        for (const p of pts) {
          ctx.beginPath();
          ctx.arc(p.x * W, p.y * H, Math.max(3, W * 0.004), 0, Math.PI * 2);
          ctx.fill();
          ctx.stroke();
        }
        if (label) this._label(ctx, label, pts[0].x * W + 4, pts[0].y * H - 4, fSize, stroke);
        break;
      }
    }
    ctx.restore();
  }

  _drawPathPreview() {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;
    const pts = this.pathPoints;
    if (pts.length === 0) return;

    ctx.save();
    ctx.strokeStyle = '#f59e0b';
    ctx.fillStyle   = 'rgba(245,158,11,0.5)';
    ctx.lineWidth   = Math.max(2, W * 0.002);
    ctx.setLineDash([Math.max(4, W * 0.005), Math.max(2, W * 0.003)]);

    // Draw committed segments
    ctx.beginPath();
    ctx.moveTo(pts[0].x * W, pts[0].y * H);
    for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
    // Rubber-band to cursor
    ctx.lineTo(this.curX * W, this.curY * H);
    ctx.stroke();

    // Vertex dots
    const dotR = Math.max(4, W * 0.005);
    ctx.setLineDash([]);
    for (const p of pts) {
      ctx.beginPath();
      ctx.arc(p.x * W, p.y * H, dotR, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.restore();
  }

  _label(ctx, text, x, y, fontSize, color) {
    ctx.save();
    ctx.font         = `bold ${fontSize}px sans-serif`;
    ctx.shadowColor  = 'rgba(0,0,0,0.8)';
    ctx.shadowBlur   = 4;
    ctx.fillStyle    = '#fff';
    ctx.fillText(text, x, y);
    ctx.restore();
  }

  _drawPreview(x1, y1, x2, y2) {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;

    ctx.save();
    ctx.strokeStyle = '#f59e0b';
    ctx.fillStyle   = 'rgba(245,158,11,0.15)';
    ctx.lineWidth   = Math.max(2, W * 0.002);
    ctx.setLineDash([Math.max(4, W * 0.005), Math.max(2, W * 0.003)]);

    const dotR = Math.max(6, W * 0.008);

    switch (this.tool) {
      case 'point': {
        ctx.beginPath();
        ctx.arc(x1 * W, y1 * H, dotR, 0, Math.PI * 2);
        ctx.fill(); ctx.stroke();
        break;
      }
      case 'circle': {
        const dx = (x2 - x1) * W, dy = (y2 - y1) * H;
        ctx.beginPath();
        ctx.arc(x1 * W, y1 * H, Math.hypot(dx, dy), 0, Math.PI * 2);
        ctx.fill(); ctx.stroke();
        break;
      }
      case 'line': {
        ctx.beginPath();
        ctx.moveTo(x1 * W, y1 * H);
        ctx.lineTo(x2 * W, y2 * H);
        ctx.stroke();
        break;
      }
      case 'rect': {
        const rx = Math.min(x1, x2) * W, ry = Math.min(y1, y2) * H;
        const rw = Math.abs(x2 - x1) * W, rh = Math.abs(y2 - y1) * H;
        ctx.fillRect(rx, ry, rw, rh);
        ctx.strokeRect(rx, ry, rw, rh);
        break;
      }
    }
    ctx.restore();
  }

  // ── Hit testing ───────────────────────────────────────────────

  _hitTest(x, y) {
    for (let i = this.markers.length - 1; i >= 0; i--) {
      const m = this.markers[i];
      if (!this._isVisible(m)) continue;
      if (this._hitMarker(m, x, y)) return m;
    }
    return null;
  }

  _hitMarker(m, x, y) {
    const c = m.coords, t = 0.02;
    switch (m.shape) {
      case 'point':  return Math.hypot(x - c.x, y - c.y) < t;
      case 'circle': return Math.hypot(x - c.cx, y - c.cy) <= c.r + t;
      case 'line':   return this._distToSeg(x, y, c.x1, c.y1, c.x2, c.y2) < t;
      case 'rect':   return x >= c.x - t && x <= c.x + c.w + t &&
                            y >= c.y - t && y <= c.y + c.h + t;
      case 'path': {
        const pts = c.points;
        if (!pts) return false;
        for (let i = 0; i < pts.length - 1; i++) {
          if (this._distToSeg(x, y, pts[i].x, pts[i].y, pts[i+1].x, pts[i+1].y) < t) return true;
        }
        return false;
      }
    }
    return false;
  }

  _distToSeg(px, py, x1, y1, x2, y2) {
    const A = px-x1, B = py-y1, C = x2-x1, D = y2-y1;
    const lenSq = C*C + D*D;
    const t = lenSq ? Math.max(0, Math.min(1, (A*C+B*D)/lenSq)) : 0;
    return Math.hypot(px-(x1+t*C), py-(y1+t*D));
  }

  // ── Helpers ───────────────────────────────────────────────────

  _notifyMultiSelect() {
    if (typeof window.onMultiSelect === 'function') {
      const ids = [...this.selectedMarkerIds];
      const items = ids.map(id => {
        const m = this.markers.find(m => m.id === id);
        return m ? { id, label: m.label || '(unlabeled)' } : null;
      }).filter(Boolean);
      window.onMultiSelect(ids, items);
    }
  }

  _notifyZoom() {
    if (typeof window.onZoomChange === 'function') {
      window.onZoomChange(Math.round(this.zoom * 100));
    }
  }
}

function hexToRGBA(hex, alpha) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  if (isNaN(r)) return `rgba(100,116,139,${alpha})`;
  return `rgba(${r},${g},${b},${alpha})`;
}
