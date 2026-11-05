-- A running task now has its own status (developing, reviewing, fixing)
-- instead of the running flag on its waiting status.
UPDATE task SET status = 'develop' WHERE status = 'runnable';
UPDATE task SET status = 'review' WHERE status = 'reviewing' AND NOT running;
UPDATE task SET status = 'fixing' WHERE status = 'fix' AND running;
ALTER TABLE task DROP COLUMN running;
