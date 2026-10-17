BEGIN;
CREATE TABLE subcontractors (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 legal_name TEXT NOT NULL, bin TEXT UNIQUE, contact_name TEXT, phone TEXT, email TEXT,
 specializations JSONB NOT NULL DEFAULT '[]'::jsonb, regions JSONB NOT NULL DEFAULT '[]'::jsonb,
 total_capacity INTEGER NOT NULL DEFAULT 0 CHECK (total_capacity >= 0),
 available_capacity INTEGER NOT NULL DEFAULT 0 CHECK (available_capacity >= 0),
 can_train_operators BOOLEAN NOT NULL DEFAULT FALSE,
 training_capacity INTEGER NOT NULL DEFAULT 0 CHECK (training_capacity >= 0),
 training_lead_days INTEGER CHECK (training_lead_days IS NULL OR training_lead_days >= 0),
 internal_rating NUMERIC(3,2) CHECK (internal_rating IS NULL OR (internal_rating >= 0 AND internal_rating <= 5)),
 internal_notes TEXT, status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','INACTIVE')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (available_capacity <= total_capacity OR total_capacity = 0)
);
CREATE TABLE clients (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, legal_name TEXT NOT NULL, bin TEXT, segment TEXT,
 internal_notes TEXT, status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE users (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL,
 role TEXT NOT NULL CHECK (role IN ('SUPER_ADMIN','JUNIOR_ADMIN','SUBCONTRACTOR')),
 status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','SUSPENDED','INVITED')),
 subcontractor_id BIGINT REFERENCES subcontractors(id) ON DELETE RESTRICT,
 must_change_password BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_login_at TIMESTAMPTZ,
 CHECK ((role = 'SUBCONTRACTOR' AND subcontractor_id IS NOT NULL) OR (role <> 'SUBCONTRACTOR' AND subcontractor_id IS NULL))
);
CREATE TABLE objects (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, client_id BIGINT NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
 name TEXT NOT NULL, city TEXT NOT NULL, address TEXT, segment TEXT, area_sqm NUMERIC(12,2) CHECK (area_sqm IS NULL OR area_sqm >= 0),
 status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('DRAFT','ACTIVE','INACTIVE')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE tenders (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, code TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT,
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','PENDING_APPROVAL','CHANGES_REQUESTED','APPROVED','PUBLISHED','CLOSED','AWARDED','IN_PROGRESS','COMPLETED','CANCELLED')),
 client_id BIGINT NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,
 created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 approved_by BIGINT REFERENCES users(id) ON DELETE RESTRICT, published_at TIMESTAMPTZ, deadline_at TIMESTAMPTZ NOT NULL,
 planned_start_at TIMESTAMPTZ, planned_end_at TIMESTAMPTZ,
 client_revenue NUMERIC(14,2) CHECK (client_revenue IS NULL OR client_revenue >= 0),
 initial_ic_share_percent NUMERIC(5,2) NOT NULL DEFAULT 15.00 CHECK (initial_ic_share_percent >= 10 AND initial_ic_share_percent <= 100),
 current_ic_share_percent NUMERIC(5,2) NOT NULL DEFAULT 15.00 CHECK (current_ic_share_percent >= 10 AND current_ic_share_percent <= 100),
 initial_tender_amount NUMERIC(14,2) NOT NULL CHECK (initial_tender_amount >= 0),
 current_tender_amount NUMERIC(14,2) NOT NULL CHECK (current_tender_amount >= 0),
 visibility_mode TEXT NOT NULL DEFAULT 'ALL_ELIGIBLE' CHECK (visibility_mode IN ('ALL_ELIGIBLE','INVITED_ONLY')),
 awarded_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (planned_end_at IS NULL OR planned_start_at IS NULL OR planned_end_at > planned_start_at)
);
CREATE TABLE tender_lots (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tender_id BIGINT NOT NULL REFERENCES tenders(id) ON DELETE RESTRICT,
 name TEXT NOT NULL, description TEXT, status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','CLOSED','AWARDED','CANCELLED')),
 initial_tender_amount NUMERIC(14,2) NOT NULL CHECK (initial_tender_amount >= 0),
 current_tender_amount NUMERIC(14,2) NOT NULL CHECK (current_tender_amount >= 0),
 awarded_subcontractor_id BIGINT REFERENCES subcontractors(id) ON DELETE RESTRICT, awarded_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE tender_lot_objects (
 tender_lot_id BIGINT NOT NULL REFERENCES tender_lots(id) ON DELETE RESTRICT,
 object_id BIGINT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, PRIMARY KEY (tender_lot_id, object_id)
);
CREATE TABLE tender_responses (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tender_lot_id BIGINT NOT NULL REFERENCES tender_lots(id) ON DELETE RESTRICT,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 status TEXT NOT NULL DEFAULT 'SUBMITTED' CHECK (status IN ('SUBMITTED','CONFIRMATION_REQUIRED','CONFIRMED','WITHDRAWN','NOT_SELECTED','WINNER')),
 capacity_confirmed BOOLEAN NOT NULL DEFAULT FALSE, available_from DATE,
 accepted_amount NUMERIC(14,2) NOT NULL CHECK (accepted_amount >= 0), accepted_price_version INTEGER NOT NULL DEFAULT 1 CHECK (accepted_price_version >= 1),
 comment TEXT, submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE (tender_lot_id, subcontractor_id)
);
CREATE TABLE contracts (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, contract_number TEXT NOT NULL UNIQUE,
 tender_lot_id BIGINT NOT NULL UNIQUE REFERENCES tender_lots(id) ON DELETE RESTRICT,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','ACTIVE','PENDING_CLOSURE','COMPLETED','TERMINATED')),
 agreed_amount NUMERIC(14,2) NOT NULL CHECK (agreed_amount >= 0), start_date DATE, end_date DATE,
 completed_at TIMESTAMPTZ, completed_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date),
 CHECK ((status = 'COMPLETED' AND completed_at IS NOT NULL AND completed_by IS NOT NULL) OR status <> 'COMPLETED')
);
CREATE TABLE contract_objects (
 contract_id BIGINT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
 object_id BIGINT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT, PRIMARY KEY (contract_id, object_id)
);
CREATE TABLE attachments (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, storage_key TEXT NOT NULL UNIQUE,
 original_filename TEXT NOT NULL, mime_type TEXT NOT NULL, size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
 uploaded_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE contract_amendments (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, contract_id BIGINT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
 object_id BIGINT REFERENCES objects(id) ON DELETE RESTRICT,
 type TEXT NOT NULL CHECK (type IN ('ADDITIONAL_SERVICE','PENALTY','CONTRACT_CHANGE')),
 description TEXT NOT NULL, amount NUMERIC(14,2), reason TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','PENDING_APPROVAL','APPROVED','REJECTED','APPLIED')),
 external_approval_reference TEXT, evidence_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT,
 created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 approved_by BIGINT REFERENCES users(id) ON DELETE RESTRICT, approved_at TIMESTAMPTZ, applied_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE tender_price_history (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tender_id BIGINT NOT NULL REFERENCES tenders(id) ON DELETE RESTRICT,
 tender_lot_id BIGINT REFERENCES tender_lots(id) ON DELETE RESTRICT,
 previous_amount NUMERIC(14,2) NOT NULL CHECK (previous_amount >= 0), new_amount NUMERIC(14,2) NOT NULL CHECK (new_amount >= 0),
 previous_ic_share_percent NUMERIC(5,2) NOT NULL CHECK (previous_ic_share_percent >= 10),
 new_ic_share_percent NUMERIC(5,2) NOT NULL CHECK (new_ic_share_percent >= 10), reason TEXT NOT NULL,
 trigger_type TEXT NOT NULL CHECK (trigger_type IN ('MANUAL','DEADLINE_JOB')),
 changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), changed_by BIGINT REFERENCES users(id) ON DELETE RESTRICT
);
CREATE TABLE notifications (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 type TEXT NOT NULL, title TEXT NOT NULL, message TEXT NOT NULL, related_entity_type TEXT, related_entity_id BIGINT,
 read_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE audit_logs (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, actor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 action TEXT NOT NULL, entity_type TEXT NOT NULL, entity_id BIGINT, before_data JSONB, after_data JSONB, reason TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE app_settings (
 key TEXT PRIMARY KEY, value JSONB NOT NULL, updated_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE operators (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, external_platform_id TEXT UNIQUE, full_name TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','INACTIVE','SUSPENDED')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE planned_shifts (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, object_id BIGINT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT,
 operator_id BIGINT NOT NULL REFERENCES operators(id) ON DELETE RESTRICT,
 subcontractor_id BIGINT NOT NULL REFERENCES subcontractors(id) ON DELETE RESTRICT,
 starts_at TIMESTAMPTZ NOT NULL, ends_at TIMESTAMPTZ NOT NULL,
 status TEXT NOT NULL DEFAULT 'PLANNED' CHECK (status IN ('PLANNED','CANCELLED','COMPLETED')),
 source_reference TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), CHECK (ends_at > starts_at)
);
CREATE TABLE attendance_records (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, planned_shift_id BIGINT NOT NULL UNIQUE REFERENCES planned_shifts(id) ON DELETE RESTRICT,
 actual_start_at TIMESTAMPTZ, actual_end_at TIMESTAMPTZ,
 checklist_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT, notes TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (actual_end_at IS NULL OR actual_start_at IS NULL OR actual_end_at >= actual_start_at)
);
CREATE TABLE quality_issues (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, object_id BIGINT NOT NULL REFERENCES objects(id) ON DELETE RESTRICT,
 contract_id BIGINT REFERENCES contracts(id) ON DELETE RESTRICT, planned_shift_id BIGINT REFERENCES planned_shifts(id) ON DELETE RESTRICT,
 description TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','IN_PROGRESS','RESOLVED','OVERDUE','DISPUTED')),
 due_at TIMESTAMPTZ, resolved_at TIMESTAMPTZ, evidence_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE subcontractor_acts (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, contract_id BIGINT NOT NULL REFERENCES contracts(id) ON DELETE RESTRICT,
 period_start DATE NOT NULL, period_end DATE NOT NULL, amount NUMERIC(14,2) NOT NULL CHECK (amount >= 0),
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','PENDING_REVIEW','APPROVED','REJECTED','PAID')),
 document_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (period_end >= period_start)
);
CREATE TABLE monthly_closings (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, period_start DATE NOT NULL, period_end DATE NOT NULL,
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','REVIEW','APPROVED','CLOSED')),
 summary JSONB NOT NULL DEFAULT '{}'::jsonb, created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 approved_by BIGINT REFERENCES users(id) ON DELETE RESTRICT, closed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (period_end >= period_start)
);
CREATE INDEX idx_objects_client_id ON objects(client_id);
CREATE INDEX idx_tenders_status_deadline ON tenders(status, deadline_at);
CREATE INDEX idx_tenders_client_id ON tenders(client_id);
CREATE INDEX idx_lots_tender_id ON tender_lots(tender_id);
CREATE INDEX idx_responses_subcontractor_status ON tender_responses(subcontractor_id, status);
CREATE INDEX idx_contracts_subcontractor_status ON contracts(subcontractor_id, status);
CREATE INDEX idx_amendments_contract_status ON contract_amendments(contract_id, status);
CREATE INDEX idx_shifts_object_time ON planned_shifts(object_id, starts_at, ends_at);
CREATE INDEX idx_shifts_operator_time ON planned_shifts(operator_id, starts_at, ends_at);
CREATE INDEX idx_quality_issues_status_due ON quality_issues(status, due_at);
CREATE INDEX idx_audit_logs_entity ON audit_logs(entity_type, entity_id, created_at DESC);
COMMIT;
