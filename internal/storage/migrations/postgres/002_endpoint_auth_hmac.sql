-- Add hmac (2) as a valid notygo_endpoints.auth_type code.
-- The inline CHECK in 001_init.sql was auto-named
-- notygo_endpoints_auth_type_check by PostgreSQL.
-- +goose Up

ALTER TABLE notygo_endpoints DROP CONSTRAINT notygo_endpoints_auth_type_check;
ALTER TABLE notygo_endpoints ADD CONSTRAINT notygo_endpoints_auth_type_check CHECK (auth_type IN (0, 1, 2));

-- +goose Down

ALTER TABLE notygo_endpoints DROP CONSTRAINT notygo_endpoints_auth_type_check;
ALTER TABLE notygo_endpoints ADD CONSTRAINT notygo_endpoints_auth_type_check CHECK (auth_type IN (0, 1));
