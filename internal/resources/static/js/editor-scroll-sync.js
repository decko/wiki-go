/**
 * Editor Scroll Sync Module
 * Provides utilities for bidirectional scroll synchronisation between the
 * CodeMirror editor pane and the rendered preview pane.
 *
 * Depends on `data-source-line` attributes being present on block-level
 * elements in the preview HTML (emitted by the server renderer when
 * ?source_lines=1 is passed to /api/render-markdown).
 */

/**
 * Build a sorted array mapping source-line numbers to their rendered DOM
 * elements.  Only elements that carry a `data-source-line` attribute are
 * included, so the map is sparse – there will be no entry for blank lines or
 * continuation lines inside a paragraph.
 *
 * @param {Element} previewEl  The `.editor-preview` container element.
 * @returns {{ line: number, el: Element }[]}  Ascending by `line`.
 */
function buildLineMap(previewEl) {
    if (!previewEl) return [];

    const nodes = previewEl.querySelectorAll(
        'h1[data-source-line], h2[data-source-line], h3[data-source-line], ' +
        'h4[data-source-line], h5[data-source-line], h6[data-source-line], ' +
        'p[data-source-line], pre[data-source-line], blockquote[data-source-line], ' +
        'hr[data-source-line], table[data-source-line], li[data-source-line]'
    );

    const map = [];
    nodes.forEach(el => {
        const line = parseInt(el.getAttribute('data-source-line'), 10);
        if (!isNaN(line)) {
            map.push({ line, el });
        }
    });

    // Sort ascending by line number (should already be in DOM order, but be safe)
    map.sort((a, b) => a.line - b.line);
    return map;
}

/**
 * Given the top-visible line number in the editor, return the preview element
 * whose source line is closest at or before `editorLine`.
 *
 * Uses binary search for O(log n) lookup.
 *
 * @param {{ line: number, el: Element }[]} lineMap
 * @param {number} editorLine  0-based line index from CodeMirror.
 * @returns {Element|null}
 */
function getPreviewElForLine(lineMap, editorLine) {
    if (!lineMap || lineMap.length === 0) return null;

    let lo = 0;
    let hi = lineMap.length - 1;
    let best = 0;

    while (lo <= hi) {
        const mid = (lo + hi) >>> 1;
        if (lineMap[mid].line <= editorLine) {
            best = mid;
            lo = mid + 1;
        } else {
            hi = mid - 1;
        }
    }

    return lineMap[best].el;
}

/**
 * Given the current `scrollTop` of the preview pane, return the source line
 * number of the topmost visible block element.
 *
 * Walks the map in reverse (bottom to top) and returns the line of the first
 * entry whose element's `offsetTop` is ≤ `previewScrollTop`.
 *
 * @param {{ line: number, el: Element }[]} lineMap
 * @param {number} previewScrollTop  `previewEl.scrollTop`.
 * @param {Element} previewEl        The preview container (used as offset parent).
 * @returns {number}  0-based line index, or 0 if nothing matches.
 */
function getEditorLineForScrollTop(lineMap, previewScrollTop, previewEl) {
    if (!lineMap || lineMap.length === 0) return 0;

    const containerTop = previewEl.getBoundingClientRect().top;

    for (let i = lineMap.length - 1; i >= 0; i--) {
        const elTop = lineMap[i].el.getBoundingClientRect().top - containerTop + previewScrollTop;
        if (elTop <= previewScrollTop + 4) { // +4px tolerance
            return lineMap[i].line;
        }
    }

    return lineMap[0].line;
}

window.EditorScrollSync = {
    buildLineMap,
    getPreviewElForLine,
    getEditorLineForScrollTop,
};

// ─── Feedback-loop guards ──────────────────────────────────────────────────────
let _isPreviewScrolling = false;
let _isEditorScrolling  = false;

/**
 * Sync the preview pane to match the editor's current scroll position.
 * Call this from a CodeMirror 'scroll' event handler.
 *
 * @param {CodeMirror} editor
 * @param {{ line: number, el: Element }[]} lineMap
 * @param {Element} previewEl
 */
function syncEditorToPreview(editor, lineMap, previewEl) {
    if (_isEditorScrolling || !lineMap || lineMap.length === 0) return;

    const scrollInfo = editor.getScrollInfo();
    const scrollTop    = scrollInfo.top;
    const clientHeight = scrollInfo.clientHeight;
    const totalHeight  = scrollInfo.height;

    _isPreviewScrolling = true;

    // Snap to bottom when editor is at the bottom
    if (scrollTop + clientHeight >= totalHeight - 4) {
        previewEl.scrollTop = previewEl.scrollHeight;
        requestAnimationFrame(() => { _isPreviewScrolling = false; });
        return;
    }

    const topLine = editor.lineAtHeight(scrollTop, 'local');

    // Find the two map entries that surround topLine for interpolation
    let beforeIdx = 0;
    for (let i = 0; i < lineMap.length - 1; i++) {
        if (lineMap[i].line <= topLine) {
            beforeIdx = i;
        } else {
            break;
        }
    }
    const afterIdx = beforeIdx < lineMap.length - 1 ? beforeIdx + 1 : beforeIdx;

    const before = lineMap[beforeIdx];
    const after  = lineMap[afterIdx];

    // Fraction of how far we are between the two surrounding source lines
    const fraction = (before !== after && after.line > before.line)
        ? (topLine - before.line) / (after.line - before.line)
        : 0;

    // Measure pixel positions of both elements relative to the preview container
    const previewCurrentScroll = previewEl.scrollTop;
    const containerTop = previewEl.getBoundingClientRect().top;
    const beforePx = before.el.getBoundingClientRect().top - containerTop + previewCurrentScroll;
    const afterPx  = (before !== after)
        ? after.el.getBoundingClientRect().top - containerTop + previewCurrentScroll
        : beforePx + before.el.offsetHeight;

    // Interpolate and apply
    previewEl.scrollTop = beforePx + fraction * (afterPx - beforePx);
    requestAnimationFrame(() => { _isPreviewScrolling = false; });
}

window.EditorScrollSync.syncEditorToPreview = syncEditorToPreview;
window.EditorScrollSync._isPreviewScrolling = () => _isPreviewScrolling;
window.EditorScrollSync._isEditorScrolling  = () => _isEditorScrolling;
window.EditorScrollSync.setEditorScrolling  = (v) => { _isEditorScrolling = v; };
window.EditorScrollSync.setPreviewScrolling = (v) => { _isPreviewScrolling = v; };

/**
 * Sync the editor pane to match the preview's current scroll position.
 * Call this from a 'scroll' event listener on the preview container.
 *
 * @param {Element} previewEl
 * @param {{ line: number, el: Element }[]} lineMap
 * @param {CodeMirror} editor
 */
function syncPreviewToEditor(previewEl, lineMap, editor) {
    if (_isPreviewScrolling || !lineMap || lineMap.length === 0) return;

    const scrollTop = previewEl.scrollTop;
    const maxScroll = previewEl.scrollHeight - previewEl.clientHeight;

    _isEditorScrolling = true;

    // Snap to bottom when preview is at the bottom
    if (maxScroll > 0 && scrollTop >= maxScroll - 4) {
        editor.scrollTo(null, editor.getScrollInfo().height);
        requestAnimationFrame(() => { _isEditorScrolling = false; });
        return;
    }

    // Find the two surrounding map entries for interpolation
    const containerTop = previewEl.getBoundingClientRect().top;
    let beforeIdx = 0;
    for (let i = 0; i < lineMap.length - 1; i++) {
        const elTop = lineMap[i].el.getBoundingClientRect().top - containerTop + scrollTop;
        if (elTop <= scrollTop) {
            beforeIdx = i;
        } else {
            break;
        }
    }
    const afterIdx = beforeIdx < lineMap.length - 1 ? beforeIdx + 1 : beforeIdx;

    const before = lineMap[beforeIdx];
    const after  = lineMap[afterIdx];

    // Pixel positions of both surrounding elements in the scrollable content
    const beforePx = before.el.getBoundingClientRect().top - containerTop + scrollTop;
    const afterPx  = (before !== after)
        ? after.el.getBoundingClientRect().top - containerTop + scrollTop
        : beforePx + before.el.offsetHeight;

    // Interpolate fraction between the two elements
    const fraction = (afterPx > beforePx)
        ? Math.min(1, Math.max(0, (scrollTop - beforePx) / (afterPx - beforePx)))
        : 0;

    // Interpolate line number and scroll editor
    const targetLine = before.line + fraction * (after.line - before.line);
    const coords = editor.charCoords({ line: Math.round(targetLine), ch: 0 }, 'local');
    editor.scrollTo(null, coords.top);
    requestAnimationFrame(() => { _isEditorScrolling = false; });
}

window.EditorScrollSync.syncPreviewToEditor = syncPreviewToEditor;
