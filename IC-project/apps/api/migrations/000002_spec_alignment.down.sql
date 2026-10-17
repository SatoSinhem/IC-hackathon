BEGIN;

-- Откат не удаляет данные молча: если старая схема не может их вместить, откат прерывается.
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM tender_responses GROUP BY tender_lot_id, subcontractor_id HAVING COUNT(*) > 1) THEN
  RAISE EXCEPTION 'cannot roll back 000002: a subcontractor has several responses for one lot; resolve them manually';
 END IF;
 IF EXISTS (SELECT 1 FROM amendment_attachments GROUP BY amendment_id HAVING COUNT(*) > 1) THEN
  RAISE EXCEPTION 'cannot roll back 000002: an amendment has several attachments; resolve them manually';
 END IF;
 IF EXISTS (SELECT 1 FROM tenders WHERE initial_ic_share_percent < 10 OR current_ic_share_percent < 10)
    OR EXISTS (SELECT 1 FROM tender_price_history WHERE previous_ic_share_percent < 10 OR new_ic_share_percent < 10) THEN
  RAISE EXCEPTION 'cannot roll back 000002: IC share below 10 percent exists; resolve it manually';
 END IF;
END $$;

DROP TRIGGER IF EXISTS trg_price_history_append_only ON tender_price_history;
DROP TRIGGER IF EXISTS trg_audit_logs_append_only ON audit_logs;
DROP FUNCTION IF EXISTS forbid_row_change();
DROP TRIGGER IF EXISTS trg_amendments_no_self_approval ON contract_amendments;
DROP TRIGGER IF EXISTS trg_tenders_no_self_approval ON tenders;
DROP FUNCTION IF EXISTS enforce_no_self_approval();

DROP INDEX IF EXISTS idx_notifications_unread;
DROP INDEX IF EXISTS idx_tender_lot_objects_object;
DROP INDEX IF EXISTS idx_contract_objects_object;

ALTER TABLE app_settings DROP CONSTRAINT IF EXISTS app_settings_share_values;
DELETE FROM app_settings WHERE key IN ('pricing.base_ic_share_percent', 'pricing.min_ic_share_percent');

DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS user_permissions;
DROP TABLE IF EXISTS subcontractor_documents;
DROP TABLE IF EXISTS subcontractor_reviews;

ALTER TABLE subcontractors
 ADD COLUMN specializations JSONB NOT NULL DEFAULT '[]'::jsonb,
 ADD COLUMN regions JSONB NOT NULL DEFAULT '[]'::jsonb;
UPDATE subcontractors s SET
 regions = COALESCE((SELECT jsonb_agg(r.name ORDER BY r.name) FROM subcontractor_regions sr JOIN regions r ON r.id = sr.region_id WHERE sr.subcontractor_id = s.id), '[]'::jsonb),
 specializations = COALESCE((SELECT jsonb_agg(p.name ORDER BY p.name) FROM subcontractor_specializations ss JOIN specializations p ON p.id = ss.specialization_id WHERE ss.subcontractor_id = s.id), '[]'::jsonb);
DROP TABLE subcontractor_specializations;
DROP TABLE subcontractor_regions;
DROP TABLE specializations;
DROP TABLE regions;
ALTER TABLE subcontractors DROP CONSTRAINT IF EXISTS subcontractors_available_le_total;
ALTER TABLE subcontractors DROP COLUMN last_verified_at;
ALTER TABLE subcontractors ADD CONSTRAINT subcontractors_check CHECK (available_capacity <= total_capacity OR total_capacity = 0);

ALTER TABLE contract_amendments ADD COLUMN evidence_attachment_id BIGINT REFERENCES attachments(id) ON DELETE RESTRICT;
UPDATE contract_amendments a SET evidence_attachment_id = (SELECT MIN(aa.attachment_id) FROM amendment_attachments aa WHERE aa.amendment_id = a.id);
DROP TABLE amendment_attachments;
ALTER TABLE contract_amendments
 DROP CONSTRAINT IF EXISTS amendments_applied_has_date,
 DROP CONSTRAINT IF EXISTS amendments_approved_has_approver,
 DROP CONSTRAINT IF EXISTS amendments_service_requires_amount,
 DROP CONSTRAINT IF EXISTS amendments_penalty_requires_facts,
 DROP COLUMN decision_comment,
 DROP COLUMN review_status,
 DROP COLUMN violation_date,
 DROP COLUMN violation_fact;

DROP TABLE IF EXISTS contract_attachments;
ALTER TABLE contracts
 DROP COLUMN signed_attachment_id,
 DROP COLUMN initial_terms,
 DROP COLUMN works_description;

DROP TABLE IF EXISTS tender_attachments;
DROP TABLE IF EXISTS tender_invited_subcontractors;

DROP INDEX IF EXISTS idx_responses_lot;
DROP INDEX IF EXISTS uq_responses_one_winner_per_lot;
DROP INDEX IF EXISTS uq_responses_active_per_lot;
ALTER TABLE tender_responses ADD CONSTRAINT tender_responses_tender_lot_id_subcontractor_id_key UNIQUE (tender_lot_id, subcontractor_id);

DROP INDEX IF EXISTS uq_price_history_deadline_tier;
ALTER TABLE tender_price_history
 DROP CONSTRAINT IF EXISTS price_history_deadline_has_tier,
 DROP CONSTRAINT IF EXISTS price_history_new_share_range,
 DROP CONSTRAINT IF EXISTS price_history_prev_share_range,
 DROP COLUMN price_version,
 DROP COLUMN tier_key,
 ADD CONSTRAINT tender_price_history_previous_ic_share_percent_check CHECK (previous_ic_share_percent >= 10),
 ADD CONSTRAINT tender_price_history_new_ic_share_percent_check CHECK (new_ic_share_percent >= 10);

ALTER TABLE tender_lots DROP COLUMN price_version;

ALTER TABLE tenders
 DROP COLUMN internal_comments,
 DROP COLUMN subcontractor_requirements,
 DROP COLUMN required_capacity,
 DROP COLUMN service_schedule,
 DROP COLUMN works_description,
 DROP CONSTRAINT IF EXISTS tenders_current_ic_share_range,
 DROP CONSTRAINT IF EXISTS tenders_initial_ic_share_range,
 ADD CONSTRAINT tenders_initial_ic_share_percent_check CHECK (initial_ic_share_percent >= 10 AND initial_ic_share_percent <= 100),
 ADD CONSTRAINT tenders_current_ic_share_percent_check CHECK (current_ic_share_percent >= 10 AND current_ic_share_percent <= 100);

COMMIT;
