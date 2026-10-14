-- +goose Up
-- Add run_type to events so the processor can distinguish initial processing
-- from replay. ReplayEvent handler sets this to 'replay'; processor reads it
-- and passes it through to the EventRun. Defaults to 'initial'.
ALTER TABLE events ADD COLUMN run_type VARCHAR(20) NOT NULL DEFAULT 'initial' AFTER status;

-- +goose Down
ALTER TABLE events DROP COLUMN run_type;
