'use strict';

const RASTER_SIZE = 1024; // offscreen canvas resolution for area tool

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

    // Area (freehand brush) tool state
    this.brushPoints    = []; // preview path points
    this.brushRadius    = 0.015; // in normalised coords (fraction of image width)
    this._lastBrushPos  = null; // previous raster paint position

    // Offscreen raster canvas for marching-squares contour extraction
    this._rasterCanvas = document.createElement('canvas');
    this._rasterCanvas.width  = RASTER_SIZE;
    this._rasterCanvas.height = RASTER_SIZE;
    this._rasterCtx = this._rasterCanvas.getContext('2d');
    this._rasterCtx.fillStyle = '#000';

    // Pending shape waiting for label form
    this.pendingShape = null;

    // Transplant mode
    this.transplantMode     = false;
    this.transplantMarkerId = null;

    // Hover tracking (for label-on-hover)
    this.hoveredMarkerId = null;

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
    this.dateFilter       = '';   // 'YYYY-MM-DD' or '' for no filter

    // Markers
    this.markers = markersData ? markersData.map(m => ({
      id:          m.id,
      shape:       m.shape,
      coords:      typeof m.coords === 'string' ? JSON.parse(m.coords) : m.coords,
      label:       m.label,
      catId:       m.catId       || 0,
      layerId:     m.layerId     || 0,
      color:       m.color       || '#64748b',
      endDate:     m.endDate     || '',
      plantedDate: m.plantedDate || '',
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

  setDateFilter(date) {
    this.dateFilter = date || '';
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

  cancelArea() {
    this.brushPoints   = [];
    this._lastBrushPos = null;
    this.drawing       = false;
    this._render();
  }

  _paintRasterDot(nx, ny) {
    const rW = this._rasterCanvas.width, rH = this._rasterCanvas.height;
    const r  = this.brushRadius * rW; // radius relative to width, matching preview
    this._rasterCtx.beginPath();
    this._rasterCtx.arc(nx * rW, ny * rH, r, 0, Math.PI * 2);
    this._rasterCtx.fill();
  }

  _paintRasterStroke(x1, y1, x2, y2) {
    const rW = this._rasterCanvas.width, rH = this._rasterCanvas.height;
    this._rasterCtx.beginPath();
    this._rasterCtx.moveTo(x1 * rW, y1 * rH);
    this._rasterCtx.lineTo(x2 * rW, y2 * rH);
    this._rasterCtx.stroke();
  }

  _confirmArea() {
    this.brushPoints   = [];
    this._lastBrushPos = null;
    // Extract contour from the offscreen raster via marching squares
    const raw = marchingSquares(this._rasterCtx, this._rasterCanvas.width, this._rasterCanvas.height);
    if (raw.length < 3) { this._render(); return; }
    // Simplify the many small marching-squares segments with Douglas-Peucker
    const simplified = douglasPeucker(raw, 0.003);
    if (simplified.length < 3) { this._render(); return; }

    const coords = { points: simplified };
    this.pendingShape = { shape: 'area', coords };
    this._render();
    if (typeof window.onShapeDrawn === 'function') {
      window.onShapeDrawn('area', coords);
    }
  }

  startTransplant(markerId) {
    this.transplantMode     = true;
    this.transplantMarkerId = markerId;
    this.canvas.style.cursor = 'crosshair';
  }

  cancelTransplant() {
    this.transplantMode     = false;
    this.transplantMarkerId = null;
    this.canvas.style.cursor = this.tool === 'select' ? 'pointer' : 'crosshair';
  }

  updateMarkerCoords(id, coords) {
    const m = this.markers.find(m => m.id === id);
    if (m) {
      m.coords = coords;
      this._render();
    }
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
      // Size the raster canvas to match the image aspect ratio so that
      // normalized coords map identically in both the raster and the main canvas.
      this._rasterCanvas.width  = RASTER_SIZE;
      this._rasterCanvas.height = Math.max(1, Math.round(RASTER_SIZE * this.image.naturalHeight / this.image.naturalWidth));
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
      let dirty = false;
      if (this.drawing && this.tool !== 'path') {
        if (this.tool === 'area') { this._confirmArea(); return; }
        this.drawing = false; dirty = true;
      }
      if (this._panning)                         { this._panning = false; dirty = true; }
      if (this.hoveredMarkerId !== null)          { this.hoveredMarkerId = null; dirty = true; }
      if (dirty) this._render();
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
      if (this.tool === 'area' && e.key === 'Escape') {
        e.preventDefault();
        this.cancelArea();
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

    // Transplant mode: next left-click sets the new position
    if (this.transplantMode) {
      const pos    = this._getRelPos(e);
      const marker = this.markers.find(m => m.id === this.transplantMarkerId);
      if (marker) {
        const newCoords = this._shiftedCoords(marker.coords, marker.shape, pos.x, pos.y);
        this.transplantMode     = false;
        this.transplantMarkerId = null;
        this.canvas.style.cursor = 'pointer';
        if (typeof window.onTransplantClick === 'function') {
          window.onTransplantClick(marker.id, newCoords);
        }
      }
      return;
    }

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

    if (this.tool === 'area') {
      const pos = this._getRelPos(e);
      this._rasterCtx.clearRect(0, 0, RASTER_SIZE, RASTER_SIZE);
      this._rasterCtx.fillStyle   = '#000';
      this._rasterCtx.strokeStyle = '#000';
      this._rasterCtx.lineCap     = 'round';
      this._rasterCtx.lineJoin    = 'round';
      this._rasterCtx.lineWidth   = this.brushRadius * this._rasterCanvas.width * 2;
      this.brushPoints   = [{ x: pos.x, y: pos.y }];
      this._lastBrushPos = pos;
      this._paintRasterDot(pos.x, pos.y); // initial dot for single clicks
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

    // Update hovered marker for label-on-hover (only in select mode when idle)
    if (this.tool === 'select' && !this.drawing && !this.transplantMode) {
      const pos    = this._getRelPos(e);
      const hit    = this._hitTest(pos.x, pos.y);
      const newId  = hit ? hit.id : null;
      if (newId !== this.hoveredMarkerId) {
        this.hoveredMarkerId = newId;
        this._render();
      }
    }

    // Area tool: always show brush cursor, collect points while dragging
    if (this.tool === 'area') {
      e.preventDefault();
      const pos = this._getRelPos(e);
      this.curX = pos.x;
      this.curY = pos.y;
      if (this.drawing && this._lastBrushPos) {
        // Paint a continuous stroke segment on the raster — no threshold, no gaps
        this._paintRasterStroke(this._lastBrushPos.x, this._lastBrushPos.y, pos.x, pos.y);
        this._lastBrushPos = pos;
        // Collect preview points with a light threshold (only for drawing the path preview)
        const last = this.brushPoints[this.brushPoints.length - 1];
        if (Math.hypot(pos.x - last.x, pos.y - last.y) > 0.003) {
          this.brushPoints.push({ x: pos.x, y: pos.y });
        }
      }
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
    if (this.tool === 'area') {
      e.preventDefault();
      this.drawing = false;
      this._confirmArea();
      return;
    }
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
      const selected  = this.selectedMarkerIds.has(m.id);
      const showLabel = selected || m.id === this.hoveredMarkerId;
      this._drawShape(m.coords, m.shape, selected, showLabel ? m.label : '', false, m.color);
    }

    if (this.pendingShape) {
      this._drawShape(this.pendingShape.coords, this.pendingShape.shape, false, '', true, '#f59e0b');
    }

    // Area brush cursor — always shown when tool is active
    if (this.tool === 'area') {
      this._drawAreaPreview();
    }

    // Live preview while dragging a new shape
    if (this.drawing && this.tool !== 'area') {
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
    // Expiry filter (against real today when no date filter, against filter date when set)
    const refDate = this.dateFilter || this.today;
    if (!this.showExpired && m.endDate && refDate && m.endDate < refDate) return false;
    // Date filter: hide markers not yet planted by the selected date
    if (this.dateFilter && m.plantedDate && m.plantedDate > this.dateFilter) return false;
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
        // Points keep a constant screen size regardless of zoom level.
        const pr = dotR / this.zoom;
        if (selected || pending) {
          ctx.save();
          ctx.strokeStyle = 'rgba(255,255,255,0.7)';
          ctx.lineWidth   = lw * 2.5;
          ctx.beginPath();
          ctx.arc(px, py, pr + 2 / this.zoom, 0, Math.PI * 2);
          ctx.stroke();
          ctx.restore();
        }
        ctx.beginPath();
        ctx.arc(px, py, pr, 0, Math.PI * 2);
        ctx.fill();
        ctx.stroke();
        if (label) this._label(ctx, label, px + pr + 3 / this.zoom, py + fSize * 0.4, fSize, stroke);
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
      case 'area': {
        const pts = coords.points;
        if (!pts || pts.length < 3) break;
        ctx.beginPath();
        ctx.moveTo(pts[0].x * W, pts[0].y * H);
        for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
        // Centroid label
        if (label) {
          let cx = 0, cy = 0;
          for (const p of pts) { cx += p.x; cy += p.y; }
          this._label(ctx, label, (cx / pts.length) * W, (cy / pts.length) * H, fSize, stroke);
        }
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

  _drawAreaPreview() {
    const ctx = this.ctx;
    const W = this.canvas.width, H = this.canvas.height;
    const pts = this.brushPoints;
    const r   = this.brushRadius * W; // uniform pixel radius → circle on screen

    ctx.save();
    ctx.fillStyle   = 'rgba(245,158,11,0.25)';
    ctx.strokeStyle = 'rgba(245,158,11,0.6)';
    ctx.lineCap     = 'round';
    ctx.lineJoin    = 'round';
    ctx.lineWidth   = r * 2;

    if (pts.length >= 2) {
      // Draw the stroke path — matches what's painted on the raster
      ctx.beginPath();
      ctx.moveTo(pts[0].x * W, pts[0].y * H);
      for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x * W, pts[i].y * H);
      ctx.stroke();
    }

    // Always show the brush cursor circle at the current mouse position
    ctx.lineWidth = Math.max(1, W * 0.001);
    ctx.beginPath();
    ctx.arc(this.curX * W, this.curY * H, r, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();

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
        ctx.arc(x1 * W, y1 * H, dotR / this.zoom, 0, Math.PI * 2);
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

  // ── Transplant helpers ────────────────────────────────────────

  // Return new coords with the shape's anchor moved to (newX, newY).
  // For multi-point shapes (line, path) the whole shape is shifted by the same delta.
  _shiftedCoords(coords, shape, newX, newY) {
    switch (shape) {
      case 'point':
        return { x: newX, y: newY };
      case 'circle':
        return { cx: newX, cy: newY, r: coords.r };
      case 'rect':
        return { x: newX, y: newY, w: coords.w, h: coords.h };
      case 'line': {
        const dx = newX - coords.x1, dy = newY - coords.y1;
        return { x1: newX, y1: newY, x2: coords.x2 + dx, y2: coords.y2 + dy };
      }
      case 'path':
      case 'area': {
        const dx = newX - coords.points[0].x, dy = newY - coords.points[0].y;
        return { points: coords.points.map(p => ({ x: p.x + dx, y: p.y + dy })) };
      }
      default:
        return Object.assign({}, coords);
    }
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
      case 'area': {
        const pts = c.points;
        if (!pts || pts.length < 3) return false;
        return pointInPolygon(x, y, pts);
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

// ── Marching squares contour extraction ──────────────────────────────────────
// Returns an ordered array of {x, y} points (normalised 0-1) tracing the
// outer boundary of painted pixels on the offscreen raster canvas.

function marchingSquares(ctx, w, h) {
  const { data } = ctx.getImageData(0, 0, w, h);
  const inside = (x, y) => x >= 0 && x < w && y >= 0 && y < h &&
                            data[(y * w + x) * 4 + 3] > 128;

  // Collect undirected segments between edge midpoints
  const segments = [];
  for (let y = 0; y < h - 1; y++) {
    for (let x = 0; x < w - 1; x++) {
      const tl = inside(x,   y)   ? 8 : 0;
      const tr = inside(x+1, y)   ? 4 : 0;
      const br = inside(x+1, y+1) ? 2 : 0;
      const bl = inside(x,   y+1) ? 1 : 0;
      const c  = tl | tr | br | bl;
      if (c === 0 || c === 15) continue;

      // Edge midpoints normalised independently per axis
      const T = { x: (x + 0.5) / w, y:  y      / h };
      const R = { x: (x + 1)   / w, y: (y+0.5) / h };
      const B = { x: (x + 0.5) / w, y: (y+1)   / h };
      const L = { x:  x        / w, y: (y+0.5) / h };

      switch (c) {
        case  1: segments.push([L, B]); break;
        case  2: segments.push([B, R]); break;
        case  3: segments.push([L, R]); break;
        case  4: segments.push([T, R]); break;
        case  5: segments.push([T, R]); segments.push([L, B]); break;
        case  6: segments.push([T, B]); break;
        case  7: segments.push([T, L]); break;
        case  8: segments.push([T, L]); break;
        case  9: segments.push([T, B]); break;
        case 10: segments.push([T, R]); segments.push([L, B]); break;
        case 11: segments.push([T, R]); break;
        case 12: segments.push([L, R]); break;
        case 13: segments.push([B, R]); break;
        case 14: segments.push([L, B]); break;
      }
    }
  }

  return connectSegments(segments);
}

// Stitch unordered segments into a single ordered polygon by matching endpoints.
function connectSegments(segments) {
  if (segments.length === 0) return [];
  const prec = 2000; // grid snap precision
  const key  = p => `${Math.round(p.x * prec)},${Math.round(p.y * prec)}`;

  // Build: point-key → list of segment indices touching it
  const adj = new Map();
  for (let i = 0; i < segments.length; i++) {
    for (const p of segments[i]) {
      const k = key(p);
      if (!adj.has(k)) adj.set(k, []);
      adj.get(k).push(i);
    }
  }

  const visited = new Array(segments.length).fill(false);
  const polygon = [];
  let si = 0;                      // start segment index
  let cur = segments[0][0];        // current point

  for (;;) {
    visited[si] = true;
    polygon.push(cur);

    const [a, b] = segments[si];
    const next = key(cur) === key(a) ? b : a;
    const neighbors = adj.get(key(next)) || [];
    const nextSi = neighbors.find(i => !visited[i]);
    if (nextSi === undefined) break;
    si  = nextSi;
    cur = next;
  }

  return polygon;
}

// ── Douglas-Peucker polyline simplification ───────────────────────────────────

function perpendicularDistance(p, a, b) {
  const dx = b.x - a.x, dy = b.y - a.y;
  if (dx === 0 && dy === 0) return Math.hypot(p.x - a.x, p.y - a.y);
  const t = ((p.x - a.x) * dx + (p.y - a.y) * dy) / (dx * dx + dy * dy);
  return Math.hypot(p.x - (a.x + t * dx), p.y - (a.y + t * dy));
}

function douglasPeucker(pts, eps) {
  if (pts.length < 3) return pts.slice();
  let maxD = 0, idx = 0;
  for (let i = 1; i < pts.length - 1; i++) {
    const d = perpendicularDistance(pts[i], pts[0], pts[pts.length - 1]);
    if (d > maxD) { maxD = d; idx = i; }
  }
  if (maxD > eps) {
    const L = douglasPeucker(pts.slice(0, idx + 1), eps);
    const R = douglasPeucker(pts.slice(idx), eps);
    return L.slice(0, -1).concat(R);
  }
  return [pts[0], pts[pts.length - 1]];
}

// ── Ray-casting point-in-polygon test ─────────────────────────────────────────

function pointInPolygon(x, y, polygon) {
  let inside = false;
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const xi = polygon[i].x, yi = polygon[i].y;
    const xj = polygon[j].x, yj = polygon[j].y;
    if (((yi > y) !== (yj > y)) && (x < (xj - xi) * (y - yi) / (yj - yi) + xi)) {
      inside = !inside;
    }
  }
  return inside;
}
