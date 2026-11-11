-- Brings a database created from an earlier db/database.sql up to date:
--   psql -U postgres -h localhost golang_gin_db < db/migrations/001_topup_audit_and_roles.sql

BEGIN;

ALTER TABLE public."user" ADD COLUMN IF NOT EXISTS role character varying NOT NULL DEFAULT 'USER';

CREATE TABLE IF NOT EXISTS public.topup_audit (
	id varchar NOT NULL PRIMARY KEY,
	user_id varchar NOT NULL,
	amount numeric(15, 2) NOT NULL,
	source varchar NOT NULL,
	reference varchar NOT NULL,
	actor varchar NOT NULL,
	outcome varchar NOT NULL,
	reason text NULL,
	client_ip varchar NULL,
	transaction_id varchar NULL,
	created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS topup_audit_applied_reference ON public.topup_audit (source, reference) WHERE outcome = 'APPLIED';
CREATE INDEX IF NOT EXISTS topup_audit_user_created_at ON public.topup_audit (user_id, created_at);

CREATE OR REPLACE FUNCTION public.topup_audit_append_only() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'topup_audit is append-only';
END;
$$;

DROP TRIGGER IF EXISTS topup_audit_append_only ON public.topup_audit;
CREATE TRIGGER topup_audit_append_only BEFORE UPDATE OR DELETE ON public.topup_audit FOR EACH ROW EXECUTE PROCEDURE public.topup_audit_append_only();

COMMIT;
