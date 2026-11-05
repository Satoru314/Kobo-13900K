export type Status =
  | 'develop' | 'developing' | 'ci' | 'review' | 'reviewing' | 'fix' | 'fixing'
  | 'done' | 'unresolved' | 'error' | 'cancelled' | 'finished';
export type Mode = 'priority' | 'immediate' | 'off_peak';

export interface Task {
  id: number;
  project_id: number;
  title: string;
  description: string;
  status: Status;
  mode: Mode;
  enqueued_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TaskSummary extends Task {
  project: string;
  pr_url?: string;
  pr_number?: number;
  ci?: 'pending' | 'success' | 'failure';
}

export interface Project {
  id: number;
  dir: string;
}

export interface PullRequest {
  number: number;
  url: string;
  state: 'open' | 'merged' | 'closed';
}

export interface ReviewRound {
  round_no: number;
  result: 'approved' | 'changes_requested' | 'ci_failed';
  comments: string;
}

export interface TaskDetail {
  task: Task;
  project: Project;
  pull_request: PullRequest | null;
  review_rounds: ReviewRound[];
  sessions: Session[];
  last_error?: string;
}

export interface Session {
  role: 'dev' | 'review';
  ai: string;
  ai_label: string;
  provider: string;
  model: string;
  created_at: string;
}

export interface AIModel {
  id: string;
  label: string;
  provider: string;
  model: string;
}

export interface AISetting {
  dev_ai: string;
  review_ai: string;
  separate_review: boolean;
  updated_at?: string;
}

export interface AIView {
  models: AIModel[];
  setting: AISetting;
}

export interface TaskEvent {
  id: number;
  type: string;
  payload: Record<string, unknown> | null;
  created_at: string;
}

export interface Candidate {
  dir: string;
  registered: boolean;
  problem?: 'not_git' | 'not_github';
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? `${method} ${path}: ${res.status}`);
  return data as T;
}

export const api = {
  tasks: () => call<TaskSummary[]>('GET', '/tasks'),
  task: (id: number) => call<TaskDetail>('GET', `/tasks/${id}`),
  events: (id: number) => call<TaskEvent[]>('GET', `/tasks/${id}/events`),
  create: (project: string, title: string, description: string, mode: Mode) =>
    call<Task>('POST', '/tasks', { project, title, description, mode }),
  retry: (id: number) => call<Task>('POST', `/tasks/${id}/retry`),
  cancel: (id: number) => call<Task>('POST', `/tasks/${id}/cancel`),
  queueState: () => call<{ paused: boolean }>('GET', '/queue/state'),
  pause: () => call<{ paused: boolean }>('POST', '/queue/pause'),
  resume: () => call<{ paused: boolean }>('POST', '/queue/resume'),
  projects: () => call<Project[]>('GET', '/projects'),
  candidates: () => call<Candidate[]>('GET', '/projects/candidates'),
  addProject: (dir: string) => call<Project>('POST', '/projects', { dir }),
  ai: () => call<AIView>('GET', '/ai'),
  saveAISetting: (setting: Pick<AISetting, 'dev_ai' | 'review_ai' | 'separate_review'>) =>
    call<AISetting>('PUT', '/ai/setting', setting),
};
