CREATE TABLE project (
    id             BIGSERIAL PRIMARY KEY,
    dir            TEXT NOT NULL UNIQUE,
    owner          TEXT NOT NULL,
    name           TEXT NOT NULL,
    default_branch TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task (
    id          BIGSERIAL PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES project(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    status      TEXT NOT NULL,
    mode        TEXT NOT NULL,
    running     BOOLEAN NOT NULL DEFAULT false,
    enqueued_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX task_status_idx ON task (status);

CREATE TABLE pull_request (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT NOT NULL UNIQUE REFERENCES task(id),
    number     INTEGER NOT NULL,
    url        TEXT NOT NULL,
    branch     TEXT NOT NULL,
    state      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_at  TIMESTAMPTZ
);

CREATE TABLE session (
    id                  BIGSERIAL PRIMARY KEY,
    task_id             BIGINT NOT NULL REFERENCES task(id),
    role                TEXT NOT NULL,
    provider            TEXT NOT NULL,
    external_session_id TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, role)
);

CREATE TABLE review_round (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT NOT NULL REFERENCES task(id),
    round_no   INTEGER NOT NULL,
    result     TEXT NOT NULL,
    comments   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, round_no)
);

CREATE TABLE event (
    id         BIGSERIAL PRIMARY KEY,
    task_id    BIGINT NOT NULL REFERENCES task(id),
    session_id BIGINT REFERENCES session(id),
    type       TEXT NOT NULL,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX event_task_idx ON event (task_id, id);
