-- One row: whether the user paused the AI from the GUI.
CREATE TABLE queue_state (
    id     INTEGER PRIMARY KEY CHECK (id = 1),
    paused BOOLEAN NOT NULL
);
INSERT INTO queue_state (id, paused) VALUES (1, false);
