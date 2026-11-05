// The board fills the space it is given: boxes share the height and columns the width, so the
// page never scrolls. A box holding more cards than fit scrolls inside itself, and the arrows
// between boxes are worked out from the same sizes.
export const CARD = 57, HDR = 32;
const GAP = 48, GUTTER = 64, MAX_COL = 400, MIN_COL = 160, MIN_BOX = 2 + HDR + CARD;

export interface BoardLayout { width: number; height: number; col: number; gutter: number; gap: number; boxH: number; labelTop: number; arrows: string[] }

interface Box { x: number; y: number; w: number; h: number }
interface Pt { x: number; y: number }

// Arrows run along the line between two box centers. Both ends stay outside every box
// by MARGIN px; the whole shape (4px shaft and heads) is checked, not just the line.
const MARGIN = 8, SHAFT = 2, HEAD_LEN = 12, HEAD_HALF = 7;
const center = (b: Box): Pt => ({ x: b.x + b.w / 2, y: b.y + b.h / 2 });
const inside = (p: Pt, b: Box) =>
  p.x > b.x - MARGIN && p.x < b.x + b.w + MARGIN && p.y > b.y - MARGIN && p.y < b.y + b.h + MARGIN;
const exit = (b: Box, d: Pt) => Math.min((b.w / 2 + MARGIN) / Math.abs(d.x || 1e-9), (b.h / 2 + MARGIN) / Math.abs(d.y || 1e-9));

function shape(s: Pt, t: Pt, both: boolean): Pt[] {
  const len = Math.hypot(t.x - s.x, t.y - s.y);
  const u = { x: (t.x - s.x) / len, y: (t.y - s.y) / len }, n = { x: -u.y, y: u.x };
  const at = (p: Pt, along: number, side: number) => ({ x: p.x + u.x * along + n.x * side, y: p.y + u.y * along + n.y * side });
  const tail = both
    ? [at(s, HEAD_LEN, -SHAFT), at(s, HEAD_LEN, -HEAD_HALF), s, at(s, HEAD_LEN, HEAD_HALF), at(s, HEAD_LEN, SHAFT)]
    : [at(s, 0, -SHAFT), at(s, 0, SHAFT)];
  return [...tail, at(t, -HEAD_LEN, SHAFT), at(t, -HEAD_LEN, HEAD_HALF), t, at(t, -HEAD_LEN, -HEAD_HALF), at(t, -HEAD_LEN, -SHAFT)];
}

function arrowPath(boxes: Box[], from: Box, to: Box, both = false): string {
  const p = center(from), q = center(to), d = { x: q.x - p.x, y: q.y - p.y };
  let ts = exit(from, d), te = 1 - exit(to, d);
  const pt = (k: number) => ({ x: p.x + d.x * k, y: p.y + d.y * k });
  const step = 0.5 / Math.hypot(d.x, d.y);
  for (let guard = 0; guard < 400; guard++) {
    const pts = shape(pt(ts), pt(te), both);
    const half = pts.length / 2;
    const badTail = pts.slice(0, half).some((v) => boxes.some((b) => inside(v, b)));
    const badHead = pts.slice(half).some((v) => boxes.some((b) => inside(v, b)));
    if (!badTail && !badHead) return 'M' + pts.map((v) => `${v.x.toFixed(1)} ${v.y.toFixed(1)}`).join('L') + 'Z';
    if (badTail) ts += step;
    if (badHead) te -= step;
  }
  throw new Error('arrow does not fit between boxes');
}

// width and height are the space available; past the minimum sizes the board overflows it.
export function boardLayout(width: number, height: number): BoardLayout {
  const col = Math.max(MIN_COL, Math.min(MAX_COL, Math.floor((width - 2 * GUTTER) / 3)));
  const boxH = Math.max(MIN_BOX, Math.floor((height - GAP) / 2));
  const boxes = [0, 1, 2].flatMap((c) => [0, 1].map((r) => ({ x: c * (col + GUTTER), y: r * (boxH + GAP), w: col, h: boxH })));
  const [develop, fix, ci, review, done] = boxes;
  return {
    width: 3 * col + 2 * GUTTER,
    height: 2 * boxH + GAP,
    col,
    gutter: GUTTER,
    gap: GAP,
    boxH,
    labelTop: 1 + HDR + CARD / 2,
    arrows: [
      arrowPath(boxes, develop, ci), arrowPath(boxes, ci, review), arrowPath(boxes, review, fix),
      arrowPath(boxes, fix, ci, true), arrowPath(boxes, review, done),
    ],
  };
}
