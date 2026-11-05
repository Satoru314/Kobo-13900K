-- AI models are chosen for development and review. The catalog itself (ai.json)
-- is not stored here; these are stable catalog ids.
CREATE TABLE ai_setting (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    dev_ai          TEXT NOT NULL,
    review_ai       TEXT NOT NULL,
    separate_review BOOLEAN NOT NULL DEFAULT false,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO ai_setting (id, dev_ai, review_ai, separate_review)
VALUES (1, 'claude', 'claude', false);

-- The AI used by a session is fixed when the session starts, so resuming a
-- conversation always asks the same provider for the same model.
ALTER TABLE session ADD COLUMN ai TEXT NOT NULL DEFAULT 'claude';
ALTER TABLE session ADD COLUMN model TEXT NOT NULL DEFAULT '';
