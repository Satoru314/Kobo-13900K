import { useCallback, useEffect, useRef, useState, type CSSProperties, type FormEvent } from 'react';
import { api, type Candidate, type Mode, type Project, type Status, type TaskDetail, type TaskEvent, type TaskSummary } from './api';
import { ARROWS, BOARD_H, BOARD_W, BOX_H, CARD, COL, GAP, GUTTER, HDR, LABEL_TOP, SLOTS } from './board';

const MODE_NAMES: Record<Mode, string> = { priority: '優先', immediate: '通常', off_peak: '夜間' };
const MODE_RANK: Record<Mode, number> = { priority: 0, immediate: 1, off_peak: 2 };
const VISOR: Partial<Record<Status, string>> = { developing: '開発中', fixing: '修正中', reviewing: 'レビュー' };
const BORDER = '1px solid #d6d5d1';

// Columns list their statuses in take order: the running one first, then the waiting ones.
const LAYOUT: { name: string; statuses: Status[] }[][] = [
  [{ name: 'develop', statuses: ['developing', 'develop'] }, { name: 'fix', statuses: ['fixing', 'fix'] }],
  [{ name: 'ci', statuses: ['ci'] }, { name: 'review', statuses: ['reviewing', 'review'] }],
  [{ name: 'done', statuses: ['done', 'unresolved'] }, { name: 'error', statuses: ['error'] }],
];

const time = (t: string | null) => (t ? Date.parse(t) : 0);

// Waiting tasks in the order the queue takes them; finished ones newest first.
function order(tasks: TaskSummary[], status: Status): TaskSummary[] {
  const of = tasks.filter((t) => t.status === status);
  if (status === 'develop') {
    return of.sort((a, b) => MODE_RANK[a.mode] - MODE_RANK[b.mode] || time(a.enqueued_at) - time(b.enqueued_at) || a.id - b.id);
  }
  if (status === 'review' || status === 'fix') return of.sort((a, b) => time(a.enqueued_at) - time(b.enqueued_at) || a.id - b.id);
  if (status === 'ci') return of.sort((a, b) => a.id - b.id);
  return of.sort((a, b) => time(b.updated_at) - time(a.updated_at));
}

const CI_MARKS = {
  pending: { label: 'CI 実行中', color: '#bf8700', path: 'M8 4a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z' },
  failure: { label: 'CI 失敗', color: '#cf222e', path: 'M3.72 3.72a.75.75 0 0 1 1.06 0L8 6.94l3.22-3.22a.749.749 0 0 1 1.275.326.749.749 0 0 1-.215.734L9.06 8l3.22 3.22a.749.749 0 0 1-.326 1.275.749.749 0 0 1-.734-.215L8 9.06l-3.22 3.22a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042L6.94 8 3.72 4.78a.75.75 0 0 1 0-1.06Z' },
  success: { label: 'CI 成功', color: '#1a7f37', path: 'M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z' },
};
const PR_PATH = 'M1.5 3.25a2.25 2.25 0 1 1 3 2.122v5.256a2.251 2.251 0 1 1-1.5 0V5.372A2.25 2.25 0 0 1 1.5 3.25Zm5.677-.177L9.573.677A.25.25 0 0 1 10 .854V2.5h1A2.5 2.5 0 0 1 13.5 5v5.628a2.251 2.251 0 1 1-1.5 0V5a1 1 0 0 0-1-1h-1v1.646a.25.25 0 0 1-.427.177L7.177 3.427a.25.25 0 0 1 0-.354ZM3.75 2.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm0 9.5a.75.75 0 1 0 0 1.5.75.75 0 0 0 0-1.5Zm8.25.75a.75.75 0 1 0 1.5 0 .75.75 0 0 0-1.5 0Z';

function readStored(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function store(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // per-viewer convenience only
  }
}

export function App() {
  const [tasks, setTasks] = useState<TaskSummary[]>([]);
  const [error, setError] = useState('');
  const [selected, setSelected] = useState<number | null>(null);
  const selectedRef = useRef<number | null>(null);
  selectedRef.current = selected;
  const [detailVersion, setDetailVersion] = useState(0);

  const reload = useCallback(() => {
    api.tasks().then((t) => { setTasks(t); setError(''); }, (e: Error) => setError(e.message));
  }, []);

  useEffect(() => {
    reload();
    const source = new EventSource('/events');
    // A (re)connect may have missed changes, so the whole list is fetched again.
    source.onopen = reload;
    source.onmessage = (m) => {
      const { task_id } = JSON.parse(m.data) as { task_id: number };
      reload();
      if (task_id === selectedRef.current) setDetailVersion((v) => v + 1);
    };
    return () => source.close();
  }, [reload]);

  return (
    <div style={{ minHeight: '100vh', boxSizing: 'border-box', fontFamily: "'Noto Sans JP', 'Hiragino Sans', system-ui, sans-serif", color: '#24231f', background: '#f5f5f3', display: 'flex', flexDirection: 'column' }}>
      <header style={{ padding: '18px max(16px, 2.5vw) 14px', borderBottom: '1px solid #e3e2df', background: '#f5f5f3', display: 'flex', alignItems: 'center', gap: 16 }}>
        <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1 }}>
          <span style={{ fontFamily: 'Inter, sans-serif', fontSize: 24, fontWeight: 800, letterSpacing: '-0.02em', color: '#1f1e1b' }}>Kobo</span>
          <span style={{ fontFamily: 'Inter, sans-serif', fontSize: 10, fontWeight: 700, letterSpacing: '0.1em', color: '#55534e', marginTop: 3 }}>13900K</span>
        </div>
        {error && <span style={{ fontSize: 13, fontWeight: 600, color: '#cf222e' }}>{error}</span>}
      </header>
      <main style={{ flexGrow: 1, overflowX: 'auto', padding: '20px max(16px, 2.5vw) 200px calc(max(16px, 2.5vw) + 20px)' }}>
        <Board tasks={tasks} open={setSelected} />
      </main>
      <Composer />
      {selected !== null && <Detail id={selected} version={detailVersion} close={() => setSelected(null)} />}
    </div>
  );
}

function Board({ tasks, open }: { tasks: TaskSummary[]; open: (id: number) => void }) {
  return (
    <div style={{ position: 'relative', display: 'flex', alignItems: 'flex-start', width: BOARD_W }}>
      {LAYOUT.map((cols, i) => (
        <div key={i} style={{ display: 'contents' }}>
          {i > 0 && <div style={{ flex: `0 0 ${GUTTER}px` }} />}
          <div style={{ flex: `0 0 ${COL}px`, display: 'flex', flexDirection: 'column' }}>
            {cols.map((col) => (
              <div key={col.name}>
                <Column name={col.name} tasks={col.statuses.flatMap((s) => order(tasks, s))} open={open} />
                <div style={{ height: GAP }} />
              </div>
            ))}
          </div>
        </div>
      ))}
      <svg aria-hidden="true" width={BOARD_W} height={BOARD_H} viewBox={`0 0 ${BOARD_W} ${BOARD_H}`} style={{ position: 'absolute', top: 0, left: 0, pointerEvents: 'none' }} fill="#cfcdc8">
        {ARROWS.map((d) => <path key={d} d={d} />)}
      </svg>
    </div>
  );
}

function Column({ name, tasks, open }: { name: string; tasks: TaskSummary[]; open: (id: number) => void }) {
  // the last card draws its bottom edge unless the box is full and its own border closes it
  const bottom = (i: number) => (i === tasks.length - 1 && tasks.length >= SLOTS ? 'none' : BORDER);
  const visor = tasks.length > 0 ? VISOR[tasks[0].status] : undefined;
  const rest = visor ? tasks.slice(1) : tasks;
  return (
    <section style={{ position: 'relative', boxSizing: 'border-box', height: BOX_H, border: BORDER, background: '#ffffff', display: 'flex', flexDirection: 'column' }}>
      <div style={{ flex: '0 0 auto', boxSizing: 'border-box', height: HDR, display: 'flex', alignItems: 'center', gap: 8, padding: '0 12px', borderBottom: BORDER }}>
        <span style={{ fontSize: 16, fontWeight: 600, color: '#1f1e1b' }}>{name}</span>
        <span style={{ fontSize: 14, fontWeight: 500, color: '#6b6964' }}>{tasks.length}</span>
      </div>
      {visor && (
        <>
          <Card task={tasks[0]} bottom={bottom(0)} open={open} visor />
          <span style={{ position: 'absolute', right: 'calc(100% + 6px)', top: LABEL_TOP, transform: 'translateY(-50%)', writingMode: 'vertical-rl', fontSize: 12, fontWeight: 700, letterSpacing: '0.06em', lineHeight: 1, color: '#0969da', whiteSpace: 'nowrap' }}>{visor}</span>
        </>
      )}
      <div className="noscroll" style={{ flex: '1 1 0', minHeight: 0, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>
        {rest.map((t, i) => <Card key={t.id} task={t} bottom={bottom(i + (visor ? 1 : 0))} open={open} />)}
      </div>
    </section>
  );
}

function Card({ task, bottom, open, visor }: { task: TaskSummary; bottom: string; open: (id: number) => void; visor?: boolean }) {
  const ci = task.ci ? CI_MARKS[task.ci] : null;
  return (
    <div style={{ position: 'relative', flex: '0 0 auto', height: CARD, boxSizing: 'border-box', borderBottom: bottom }}>
      <button className="row" onClick={() => open(task.id)} style={{ display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 2, width: '100%', height: '100%', textAlign: 'left', padding: `0 ${task.pr_url ? 62 : 12}px 0 12px`, border: 'none', background: '#ffffff', fontFamily: 'inherit', cursor: 'pointer' }}>
        <span style={{ fontSize: 14, fontWeight: 600, lineHeight: 1.5, color: '#1f1e1b', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '100%' }}>{task.title}</span>
        <span style={{ display: 'flex', gap: 8, fontSize: 12, fontWeight: 500, color: '#4a4843' }}>
          {task.project}
          {task.status === 'unresolved' && <span style={{ fontWeight: 600, color: '#b4441c' }}>unresolved</span>}
        </span>
      </button>
      {task.pr_url && (
        <span style={{ position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)', display: 'flex', alignItems: 'center', gap: 2 }}>
          {ci && (
            <span aria-label={ci.label} title={ci.label} style={{ width: 16, height: 16, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true" fill={ci.color}><path d={ci.path} /></svg>
            </span>
          )}
          <a className="pr-icon" href={task.pr_url} target="_blank" rel="noopener" aria-label="PR を開く" style={{ width: 28, height: 28, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true" fill="#1f883d"><path d={PR_PATH} /></svg>
          </a>
        </span>
      )}
      {visor && <div aria-hidden="true" style={{ position: 'absolute', inset: -1, border: '3px solid #54aeff', pointerEvents: 'none' }} />}
    </div>
  );
}

const PROBLEMS: Record<NonNullable<Candidate['problem']>, string> = { not_git: 'git ではない', not_github: 'GitHub ではない' };

function Composer() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [project, setProject] = useState(readStored('kobo.project') ?? '');
  const [mode, setMode] = useState<Mode>(() => {
    const m = readStored('kobo.mode');
    return m === 'priority' || m === 'immediate' || m === 'off_peak' ? m : 'immediate';
  });
  const [modeOpen, setModeOpen] = useState(false);
  const [candidates, setCandidates] = useState<Candidate[] | null>(null);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [error, setError] = useState('');

  const loadProjects = useCallback(() => api.projects().then(setProjects, (e: Error) => setError(e.message)), []);
  useEffect(() => { loadProjects(); }, [loadProjects]);
  const current = projects.some((p) => p.dir === project) ? project : (projects[0]?.dir ?? '');

  const choose = (dir: string) => { setProject(dir); store('kobo.project', dir); };
  const openPicker = () => api.candidates().then(setCandidates, (e: Error) => setError(e.message));
  const add = (dir: string) =>
    api.addProject(dir).then(() => { setCandidates(null); choose(dir); return loadProjects(); }, (e: Error) => setError(e.message));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    api.create(current, title, description, mode).then(
      () => { setTitle(''); setDescription(''); setError(''); },
      (err: Error) => setError(err.message),
    );
  };

  const sorted = (candidates ?? []).map((c) => ({ ...c, rank: c.registered ? 2 : c.problem ? 1 : 0 }))
    .sort((a, b) => a.rank - b.rank || a.dir.localeCompare(b.dir, 'en', { sensitivity: 'base' }));

  return (
    <div style={{ position: 'fixed', left: 0, right: 0, bottom: 0, padding: '0 16px 20px', display: 'flex', justifyContent: 'center', pointerEvents: 'none' }}>
      <div style={{ width: 'min(760px, 100%)', display: 'flex', flexDirection: 'column', gap: 8, pointerEvents: 'auto' }}>
        {candidates && (
          <div style={{ display: 'flex', flexDirection: 'column', border: BORDER, background: '#ffffff', overflow: 'hidden', boxShadow: '0 4px 16px rgba(15,15,15,0.08)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '7px 10px', borderBottom: BORDER }}>
              <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, fontWeight: 600, color: '#3a3935' }}>~/programF/</span>
              <button type="button" className="ghost" onClick={() => setCandidates(null)} style={{ marginLeft: 'auto', height: 24, padding: '0 8px', border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 12, fontWeight: 600, color: '#4a4843', cursor: 'pointer' }}>閉じる</button>
            </div>
            <ul style={{ margin: 0, padding: 4, listStyle: 'none', maxHeight: 240, overflowY: 'auto', display: 'flex', flexDirection: 'column' }}>
              {sorted.map((c) => {
                const disabled = c.registered || !!c.problem;
                return (
                  <li key={c.dir}>
                    <button type="button" className="ghost" disabled={disabled} onClick={() => add(c.dir)} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 8, padding: '6px 8px', border: 'none', background: 'transparent', fontFamily: 'inherit', textAlign: 'left', cursor: disabled ? 'default' : 'pointer' }}>
                      <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13, fontWeight: 500, color: disabled ? '#8a8780' : '#1f1e1b' }}>{c.dir}/</span>
                      <span style={{ marginLeft: 'auto', fontSize: 11, fontWeight: 500, color: c.problem ? '#b4441c' : '#6b6964' }}>{c.registered ? '登録済み' : c.problem ? PROBLEMS[c.problem] : ''}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        )}
        <form onSubmit={submit} style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '12px 12px 10px 16px', border: BORDER, background: '#ffffff', boxShadow: '0 4px 16px rgba(15,15,15,0.06)' }}>
          <input className="bare" aria-label="タイトル" placeholder="タイトル" value={title} onChange={(e) => setTitle(e.target.value)} style={{ height: 28, padding: 0, border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 15, fontWeight: 600, color: '#1f1e1b' }} />
          <div aria-hidden="true" style={{ height: 1, margin: '4px -12px 6px -16px', background: '#e3e2df' }} />
          <textarea className="bare" aria-label="内容" placeholder="内容" rows={2} value={description} onChange={(e) => setDescription(e.target.value)} style={{ padding: 0, border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 14, lineHeight: 1.6, color: '#24231f', resize: 'none' }} />
          {error && <span style={{ fontSize: 12, fontWeight: 600, color: '#cf222e' }}>{error}</span>}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 2 }}>
            <select className="ghost" aria-label="プロジェクト" value={current} onChange={(e) => (e.target.value ? choose(e.target.value) : openPicker())} style={{ height: 32, marginLeft: -6, padding: '0 6px', border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 13, fontWeight: 600, color: '#3a3935', cursor: 'pointer' }}>
              {projects.map((p) => <option key={p.id} value={p.dir}>{p.dir}</option>)}
              <option value="">追加…</option>
            </select>
            <div style={{ position: 'relative', marginLeft: 'auto' }}>
              <button type="button" className="ghost" aria-label="実行の優先度" aria-expanded={modeOpen} onClick={() => setModeOpen(!modeOpen)} style={{ height: 32, padding: '0 6px 0 10px', border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 13, fontWeight: 600, color: '#3a3935', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 4 }}>
                {MODE_NAMES[mode]}
                <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M3 4.5L6 7.5l3-3" stroke="#3a3935" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" fill="none" /></svg>
              </button>
              {modeOpen && (
                <ul role="menu" style={{ position: 'absolute', right: 0, bottom: 'calc(100% + 6px)', margin: 0, padding: 4, listStyle: 'none', minWidth: 168, border: BORDER, background: '#ffffff', boxShadow: '0 4px 16px rgba(15,15,15,0.08)' }}>
                  {(Object.keys(MODE_NAMES) as Mode[]).map((m) => (
                    <li key={m}>
                      <button type="button" role="menuitemradio" aria-checked={m === mode} className="ghost" onClick={() => { setMode(m); store('kobo.mode', m); setModeOpen(false); }} style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 8, padding: '7px 8px', border: 'none', background: 'transparent', fontFamily: 'inherit', fontSize: 13, fontWeight: 600, color: '#1f1e1b', textAlign: 'left', cursor: 'pointer' }}>
                        {MODE_NAMES[m]}
                        <span style={{ marginLeft: 'auto', width: 14, display: 'flex', justifyContent: 'center' }}>
                          {m === mode && <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true" fill="#1f1e1b"><path d={CI_MARKS.success.path} /></svg>}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <button type="submit" className="btn btn-primary" aria-label="発行" style={{ width: 34, height: 34, border: 'none', background: '#1f1e1b', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true"><path d="M8 13V3M3.5 7.5L8 3l4.5 4.5" stroke="#ffffff" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" fill="none" /></svg>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

const ROUND_RESULTS = {
  approved: { label: '承認', color: '#1f883d' },
  changes_requested: { label: '指摘あり', color: '#1f1e1b' },
  ci_failed: { label: 'CI 失敗', color: '#cf222e' },
};

const STATUS_TEXT: Partial<Record<Status, string>> = {
  developing: '開発を開始', reviewing: 'レビューを開始', fixing: '修正を開始', done: 'done', unresolved: 'unresolved', cancelled: '取り消し',
};

const RED = '#cf222e', GRAY = '#9b9993';

// flow turns the events into the lines of 実行フロー. Events that only repeat another line are left out.
function flow(events: TaskEvent[]): { id: number; time: string; text: string; dot: string }[] {
  const today = new Date().toDateString();
  const out = [];
  for (const e of events) {
    const p = e.payload ?? {};
    let text = '', dot = GRAY;
    switch (e.type) {
      case 'task_created': text = 'タスク発行'; break;
      case 'status_changed': {
        const to = p.to as Status;
        if (to === 'finished') text = `終了（${String(p.reason ?? '')}）`;
        else if (to === 'error') { text = 'error'; dot = RED; }
        else text = STATUS_TEXT[to] ?? '';
        break;
      }
      case 'pr_created': text = `PR #${p.number} を作成`; break;
      case 'ci_result':
        if (p.state === 'success') text = 'CI 成功';
        if (p.state === 'failure') { text = 'CI 失敗'; dot = RED; }
        break;
      case 'review_result': {
        const r = ROUND_RESULTS[p.result as keyof typeof ROUND_RESULTS];
        text = `${p.round_no}周目：${r.label}`;
        if (p.result === 'ci_failed') dot = RED;
        break;
      }
      case 'system_error': text = `システムエラー：${String(p.message ?? '')}`; dot = RED; break;
      case 'blocked': text = 'AI が「進められない」と報告'; dot = RED; break;
      case 'retried': text = '再実行'; break;
    }
    if (!text) continue;
    const d = new Date(e.created_at);
    const hm = d.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' });
    out.push({ id: e.id, time: d.toDateString() === today ? hm : `${d.getMonth() + 1}/${d.getDate()}`, text, dot });
  }
  return out;
}

const H3: CSSProperties = { margin: 0, paddingBottom: 6, borderBottom: '1px solid #e3e2df', fontSize: 13, fontWeight: 700, color: '#3a3935' };

function Detail({ id, version, close }: { id: number; version: number; close: () => void }) {
  const [detail, setDetail] = useState<TaskDetail | null>(null);
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    Promise.all([api.task(id), api.events(id)]).then(
      ([d, e]) => { setDetail(d); setEvents(e); setError(''); },
      (e: Error) => setError(e.message),
    );
  }, [id, version]);

  const act = (call: (id: number) => Promise<unknown>) => call(id).then(close, (e: Error) => setError(e.message));
  const t = detail?.task;

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(15,15,15,0.18)', display: 'flex', justifyContent: 'flex-end' }}>
      <button aria-label="閉じる" onClick={close} style={{ flexGrow: 1, border: 'none', background: 'transparent', cursor: 'default' }} />
      <aside style={{ width: 'min(560px, 100%)', height: '100%', overflowY: 'auto', boxSizing: 'border-box', background: '#ffffff', boxShadow: '-8px 0 24px rgba(15,15,15,0.10)', padding: '18px 28px 40px', display: 'flex', flexDirection: 'column', gap: 18 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, fontWeight: 600, color: '#55534e' }}>#{id}</span>
          <button className="ghost" aria-label="閉じる" onClick={close} style={{ width: 32, height: 32, border: 'none', background: 'transparent', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true"><path d="M3 3l8 8M11 3l-8 8" stroke="#3a3935" strokeWidth="1.6" strokeLinecap="round" fill="none" /></svg>
          </button>
        </div>
        {error && <span style={{ fontSize: 13, fontWeight: 600, color: RED }}>{error}</span>}
        {detail && t && (
          <>
            <h2 style={{ margin: 0, fontSize: 24, fontWeight: 700, lineHeight: 1.4, color: '#1f1e1b' }}>{t.title}</h2>
            <dl style={{ margin: 0, display: 'grid', gridTemplateColumns: '96px minmax(0, 1fr)', rowGap: 10, fontSize: 14, borderTop: '1px solid #e3e2df', borderBottom: '1px solid #e3e2df', padding: '14px 0' }}>
              <dt style={{ color: '#55534e' }}>status</dt>
              <dd style={{ margin: 0, fontFamily: "'JetBrains Mono', monospace", fontSize: 13, fontWeight: 600, color: t.status === 'error' ? '#d44c47' : t.status === 'unresolved' ? '#b4441c' : '#1f1e1b' }}>{t.status}</dd>
              <dt style={{ color: '#55534e' }}>プロジェクト</dt>
              <dd style={{ margin: 0, fontWeight: 500 }}>{detail.project.dir}</dd>
              <dt style={{ color: '#55534e' }}>実行</dt>
              <dd style={{ margin: 0, fontWeight: 500 }}>{MODE_NAMES[t.mode]}</dd>
              <dt style={{ color: '#55534e' }}>PR</dt>
              <dd style={{ margin: 0, fontWeight: 500 }}>
                {detail.pull_request
                  ? <a href={detail.pull_request.url} target="_blank" rel="noopener">#{detail.pull_request.number}</a>
                  : <span style={{ color: '#6b6964' }}>まだありません</span>}
              </dd>
            </dl>
            {(t.status === 'error' || t.status === 'develop') && (
              <div style={{ display: 'flex', gap: 8 }}>
                {t.status === 'error' && (
                  <button type="button" className="btn btn-primary" onClick={() => act(api.retry)} style={{ height: 32, padding: '0 14px', border: 'none', background: '#1f1e1b', fontFamily: 'inherit', fontSize: 13, fontWeight: 600, color: '#ffffff', cursor: 'pointer' }}>再実行</button>
                )}
                {t.status === 'develop' && (
                  <button type="button" className="ghost" onClick={() => act(api.cancel)} style={{ height: 32, padding: '0 14px', border: BORDER, background: '#ffffff', fontFamily: 'inherit', fontSize: 13, fontWeight: 600, color: '#1f1e1b', cursor: 'pointer' }}>取り消し</button>
                )}
              </div>
            )}
            {detail.last_error && (
              <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <h3 style={H3}>停止理由</h3>
                <p style={{ margin: 0, fontSize: 14, lineHeight: 1.7, color: '#24231f', whiteSpace: 'pre-line' }}>{detail.last_error}</p>
              </section>
            )}
            <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <h3 style={H3}>内容</h3>
              <p style={{ margin: 0, fontSize: 14, lineHeight: 1.75, color: '#24231f', whiteSpace: 'pre-line' }}>{t.description}</p>
            </section>
            {detail.review_rounds.length > 0 && (
              <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <h3 style={H3}>レビュー</h3>
                {detail.review_rounds.map((r) => (
                  <div key={r.round_no} style={{ display: 'flex', gap: 10, fontSize: 14, lineHeight: 1.6 }}>
                    <span style={{ flexShrink: 0, width: 44, color: '#55534e', fontWeight: 500 }}>{r.round_no}周目</span>
                    <span style={{ flexShrink: 0, fontWeight: 700, color: ROUND_RESULTS[r.result].color }}>{ROUND_RESULTS[r.result].label}</span>
                    <span style={{ color: '#24231f', whiteSpace: 'pre-line' }}>{r.comments}</span>
                  </div>
                ))}
              </section>
            )}
            <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <h3 style={H3}>実行フロー</h3>
              <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column' }}>
                {flow(events).map((e) => (
                  <li key={e.id} style={{ display: 'grid', gridTemplateColumns: '48px 14px minmax(0, 1fr)', gap: 8, fontSize: 13, lineHeight: 1.6, padding: '3px 0' }}>
                    <span style={{ color: '#55534e', fontVariantNumeric: 'tabular-nums' }}>{e.time}</span>
                    <span style={{ display: 'flex', justifyContent: 'center', paddingTop: 7 }}><span style={{ width: 6, height: 6, borderRadius: '50%', background: e.dot }} /></span>
                    <span style={{ color: '#24231f' }}>{e.text}</span>
                  </li>
                ))}
              </ol>
            </section>
          </>
        )}
      </aside>
    </div>
  );
}
