// Fixed layout: every block is a fixed-size box holding task cards; past SLOTS cards it
// scrolls, so the blocks never move and the arrows between them sit at fixed coordinates.
export const CARD = 57, HDR = 32, GAP = 48, SLOTS = 4, COL = 400, GUTTER = 64;
export const BOX_H = 2 + HDR + SLOTS * CARD;
export const BOARD_W = 3 * COL + 2 * GUTTER;
export const BOARD_H = 2 * BOX_H + GAP;
export const LABEL_TOP = 1 + HDR + CARD / 2;

interface Box { x: number; y: number; w: number; h: number }
interface Pt { x: number; y: number }

// Arrows run along the line between two box centers. Both ends stay outside every box
// by MARGIN px; the whole shape (4px shaft and heads) is checked, not just the line.
const MARGIN = 8, SHAFT = 2, HEAD_LEN = 12, HEAD_HALF = 7;
const box = (c: number, r: number): Box => ({ x: c * (COL + GUTTER), y: r * (BOX_H + GAP), w: COL, h: BOX_H });
const boxes = [0, 1, 2].flatMap((c) => [box(c, 0), box(c, 1)]);
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

function arrowPath(from: Box, to: Box, both = false): string {
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

const [bDevelop, bFix, bCi, bReview, bDone] = boxes;
export const ARROWS = [
  arrowPath(bDevelop, bCi), arrowPath(bCi, bReview), arrowPath(bReview, bFix),
  arrowPath(bFix, bCi, true), arrowPath(bReview, bDone),
];
