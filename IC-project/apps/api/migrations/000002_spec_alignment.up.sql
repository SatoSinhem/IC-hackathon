BEGIN;

-- Минимальная доля IC настраивается в app_settings, поэтому жёсткий порог 10% убран из схемы.
ALTER TABLE tenders DROP CONSTRAINT IF EXISTS tenders_initial_ic_share_percent_check;
ALTER TABLE tenders DROP CONSTRAINT IF EXISTS tenders_current_ic_share_percent_check;
ALTER TABLE tenders
 ADD CONSTRAINT tenders_initial_ic_share_range CHECK (initial_ic_share_percent >= 0 AND initial_ic_share_percent <= 100),
 ADD CONSTRAINT tenders_current_ic_share_range CHECK (current_ic_share_percent >= 0 AND current_ic_share_percent <= 100),
 ADD COLUMN works_description TEXT,
 ADD COLUMN service_schedule TEXT,
 ADD COLUMN required_capacity INTEGER CHECK (required_capacity IS NULL OR required_capacity > 0),
 ADD COLUMN subcontractor_requirements TEXT,
 ADD COLUMN internal_comments TEXT;

ALTER TABLE tender_lots ADD COLUMN price_version INTEGER NOT NULL DEFAULT 1 CHECK (price_version >= 1);

ALTER TABLE tender_price_history DROP CONSTRAINT IF EXISTS tender_price_history_previous_ic_share_percent_check;
ALTER TABLE tender_price_history DROP CONSTRAINT IF EXISTS tender_price_history_new_ic_share_percent_check;
ALTER TABLE tender_price_history
 ADD COLUMN tier_key TEXT,
 ADD COLUMN price_version INTEGER CHECK (price_version IS NULL OR price_version >= 1);
-- Ключ ступени есть не у записей, созданных до этой миграции или после отката.
UPDATE tender_price_history SET tier_key = 'LEGACY_' || id WHERE trigger_type = 'DEADLINE_JOB' AND tier_key IS NULL;
ALTER TABLE tender_price_history
 ADD CONSTRAINT price_history_prev_share_range CHECK (previous_ic_share_percent >= 0 AND previous_ic_share_percent <= 100),
 ADD CONSTRAINT price_history_new_share_range CHECK (new_ic_share_percent >= 0 AND new_ic_share_percent <= 100),
 ADD CONSTRAINT price_history_deadline_has_tier CHECK (trigger_type <> 'DEADLINE_JOB' OR tier_key IS NOT NULL);
-- Повторный запуск фоновой задачи не может применить одну и ту же ступень дважды.
CREATE UNIQUE INDEX uq_price_history_deadline_tier
 ON tender_price_history (tender_id, COALESCE(tender_lot_id, 0), tier_key) WHERE trigger_type = 'DEADLINE_JOB';

ALTER TABLE tender_responses DROP CONSTRAINT IF EXISTS tender_responses_tender_lot_id_subcontractor_id_key;
CREATE UNIQUE INDEX uq_responses_active_per_lot
 ON tender_responses (tender_lot_id, subcontractor_id) WHERE status IN ('SUBMITTED','CONFIRMATION_REQUIRED','CONFIRMED','WINNER');
CREATE UNIQUE INDEX uq_responses_one_winner_per_lot ON tender_responses (tender_lot_id) WHERE status = 'WINNER';
CREATE INDEX idx_responses_lot ON tender_responses(tender_lot_id);

CREATE TABLE tender_invited_subcontractors (
 tender_id BIGINT NOT NULL REFERENCES tenders(id) ON DELETE CASCADE,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 invited_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (tender_id, subcontractor_id)
);
CREATE INDEX idx_invited_subcontractor ON tender_invited_subcontractors(subcontractor_id);

CREATE TABLE tender_attachments (
 tender_id BIGINT NOT NULL REFERENCES tenders(id) ON DELETE CASCADE,
 attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE RESTRICT,
 visible_to_subcontractors BOOLEAN NOT NULL DEFAULT FALSE,
 PRIMARY KEY (tender_id, attachment_id)
);

ALTER TABLE contracts
 ADD COLUMN works_description TEXT,
 ADD COLUMN initial_terms TEXT,
 ADD COLUMN signed_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT;
CREATE TABLE contract_attachments (
 contract_id BIGINT NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
 attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE RESTRICT,
 kind TEXT NOT NULL DEFAULT 'OTHER' CHECK (kind IN ('CONTRACT','ADDENDUM','SIGNED','COMPLETION','OTHER')),
 visible_to_subcontractor BOOLEAN NOT NULL DEFAULT FALSE,
 PRIMARY KEY (contract_id, attachment_id)
);

ALTER TABLE contract_amendments
 ADD COLUMN violation_fact TEXT,
 ADD COLUMN violation_date DATE,
 ADD COLUMN review_status TEXT NOT NULL DEFAULT 'NOT_REQUIRED' CHECK (review_status IN ('NOT_REQUIRED','PENDING','CONFIRMED','REJECTED')),
 ADD COLUMN decision_comment TEXT,
 ADD CONSTRAINT amendments_penalty_requires_facts CHECK (type <> 'PENALTY' OR status = 'DRAFT' OR (violation_fact IS NOT NULL AND violation_date IS NOT NULL)),
 ADD CONSTRAINT amendments_service_requires_amount CHECK (type <> 'ADDITIONAL_SERVICE' OR status = 'DRAFT' OR amount IS NOT NULL),
 ADD CONSTRAINT amendments_approved_has_approver CHECK (status NOT IN ('APPROVED','APPLIED') OR (approved_by IS NOT NULL AND approved_at IS NOT NULL)),
 ADD CONSTRAINT amendments_applied_has_date CHECK (status <> 'APPLIED' OR applied_at IS NOT NULL);
CREATE TABLE amendment_attachments (
 amendment_id BIGINT NOT NULL REFERENCES contract_amendments(id) ON DELETE CASCADE,
 attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE RESTRICT,
 PRIMARY KEY (amendment_id, attachment_id)
);
INSERT INTO amendment_attachments (amendment_id, attachment_id)
 SELECT id, evidence_attachment_id FROM contract_amendments WHERE evidence_attachment_id IS NOT NULL;
ALTER TABLE contract_amendments DROP COLUMN evidence_attachment_id;

ALTER TABLE subcontractors DROP CONSTRAINT IF EXISTS subcontractors_check;
ALTER TABLE subcontractors
 ADD CONSTRAINT subcontractors_available_le_total CHECK (available_capacity <= total_capacity),
 ADD COLUMN last_verified_at TIMESTAMPTZ;
CREATE TABLE regions (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE specializations (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE subcontractor_regions (
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE CASCADE,
 region_id BIGINT NOT NULL REFERENCES regions(id) ON DELETE RESTRICT,
 PRIMARY KEY (subcontractor_id, region_id)
);
CREATE TABLE subcontractor_specializations (
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE CASCADE,
 specialization_id BIGINT NOT NULL REFERENCES specializations(id) ON DELETE RESTRICT,
 PRIMARY KEY (subcontractor_id, specialization_id)
);
CREATE INDEX idx_subcontractor_regions_region ON subcontractor_regions(region_id);
CREATE INDEX idx_subcontractor_specializations_spec ON subcontractor_specializations(specialization_id);
INSERT INTO regions (name)
 SELECT DISTINCT btrim(v) FROM subcontractors s,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(s.regions) = 'array' THEN s.regions ELSE '[]'::jsonb END) AS v
 WHERE btrim(v) <> '' ON CONFLICT (name) DO NOTHING;
INSERT INTO subcontractor_regions (subcontractor_id, region_id)
 SELECT DISTINCT s.id, r.id FROM subcontractors s,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(s.regions) = 'array' THEN s.regions ELSE '[]'::jsonb END) AS v
 JOIN regions r ON r.name = btrim(v);
INSERT INTO specializations (name)
 SELECT DISTINCT btrim(v) FROM subcontractors s,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(s.specializations) = 'array' THEN s.specializations ELSE '[]'::jsonb END) AS v
 WHERE btrim(v) <> '' ON CONFLICT (name) DO NOTHING;
INSERT INTO subcontractor_specializations (subcontractor_id, specialization_id)
 SELECT DISTINCT s.id, p.id FROM subcontractors s,
 LATERAL jsonb_array_elements_text(CASE WHEN jsonb_typeof(s.specializations) = 'array' THEN s.specializations ELSE '[]'::jsonb END) AS v
 JOIN specializations p ON p.name = btrim(v);
ALTER TABLE subcontractors DROP COLUMN regions, DROP COLUMN specializations;

CREATE TABLE subcontractor_reviews (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 contract_id BIGINT REFERENCES contracts(id) ON DELETE RESTRICT,
 rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
 comment TEXT,
 author_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_reviews_subcontractor ON subcontractor_reviews(subcontractor_id, created_at DESC);
CREATE TABLE subcontractor_documents (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 attachment_id BIGINT NOT NULL REFERENCES attachments(id) ON DELETE RESTRICT,
 doc_type TEXT NOT NULL,
 verified_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subcontractor_documents_subcontractor ON subcontractor_documents(subcontractor_id);

CREATE TABLE user_permissions (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 permission TEXT NOT NULL CHECK (permission ~ '^[A-Z][A-Z0-9_]*$'),
 granted_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (user_id, permission)
);
CREATE TABLE user_sessions (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash BYTEA NOT NULL UNIQUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 expires_at TIMESTAMPTZ NOT NULL,
 last_seen_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ,
 ip_address INET,
 user_agent TEXT,
 CHECK (expires_at > created_at)
);
CREATE INDEX idx_sessions_active_user ON user_sessions(user_id) WHERE revoked_at IS NULL;

ALTER TABLE app_settings ADD CONSTRAINT app_settings_share_values CHECK (
 key NOT IN ('pricing.base_ic_share_percent', 'pricing.min_ic_share_percent')
 OR CASE WHEN jsonb_typeof(value) = 'number' THEN (value #>> '{}')::numeric BETWEEN 0 AND 100 ELSE FALSE END
);
INSERT INTO app_settings (key, value) VALUES
 ('pricing.base_ic_share_percent', '15'::jsonb),
 ('pricing.min_ic_share_percent', '10'::jsonb)
 ON CONFLICT (key) DO NOTHING;

CREATE INDEX idx_contract_objects_object ON contract_objects(object_id);
CREATE INDEX idx_tender_lot_objects_object ON tender_lot_objects(object_id);
CREATE INDEX idx_notifications_unread ON notifications(recipient_user_id, created_at DESC) WHERE read_at IS NULL;

-- Самоутверждение допустимо только для SUPER_ADMIN.
CREATE FUNCTION enforce_no_self_approval() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.approved_by IS NOT NULL AND NEW.approved_by = NEW.created_by
    AND COALESCE((SELECT role FROM users WHERE id = NEW.approved_by), '') <> 'SUPER_ADMIN' THEN
  RAISE EXCEPTION 'self-approval is allowed only for SUPER_ADMIN' USING ERRCODE = 'check_violation';
 END IF;
 RETURN NEW;
END
$$;
CREATE TRIGGER trg_tenders_no_self_approval BEFORE INSERT OR UPDATE OF approved_by ON tenders
 FOR EACH ROW EXECUTE FUNCTION enforce_no_self_approval();
CREATE TRIGGER trg_amendments_no_self_approval BEFORE INSERT OR UPDATE OF approved_by ON contract_amendments
 FOR EACH ROW EXECUTE FUNCTION enforce_no_self_approval();

CREATE FUNCTION forbid_row_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END
$$;
CREATE TRIGGER trg_audit_logs_append_only BEFORE UPDATE OR DELETE ON audit_logs
 FOR EACH ROW EXECUTE FUNCTION forbid_row_change();
CREATE TRIGGER trg_price_history_append_only BEFORE UPDATE OR DELETE ON tender_price_history
 FOR EACH ROW EXECUTE FUNCTION forbid_row_change();

COMMIT;
