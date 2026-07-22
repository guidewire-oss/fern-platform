// Pure geometry for the visual field-mapping editor. A connector runs
// from a JIRA field's right edge (left column is Fern, right is JIRA in
// v1; we mirror v1's "JIRA right-edge → Fern left-edge" line) to a Fern
// field's left edge, expressed in container-relative coordinates so the
// SVG overlay (positioned at the container origin) lines up regardless of
// page scroll.

export interface Rect {
  left: number;
  right: number;
  top: number;
  height: number;
}

export interface LineCoords {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

// portCoords converts absolute bounding rects into the container-relative
// endpoints of a connector: start at the JIRA field's right-edge midpoint,
// end at the Fern field's left-edge midpoint.
export function portCoords(jira: Rect, fern: Rect, container: Rect): LineCoords {
  return {
    x1: jira.right - container.left,
    y1: jira.top + jira.height / 2 - container.top,
    x2: fern.left - container.left,
    y2: fern.top + fern.height / 2 - container.top,
  };
}

// bezierPath renders a smooth horizontal S-curve between two endpoints —
// nicer than a straight line for crossing connectors. The control points
// pull horizontally by half the gap so lines leave/enter the ports level.
export function bezierPath({ x1, y1, x2, y2 }: LineCoords): string {
  const dx = Math.max(24, Math.abs(x2 - x1) / 2);
  return `M ${x1} ${y1} C ${x1 - dx} ${y1}, ${x2 + dx} ${y2}, ${x2} ${y2}`;
}
