-- Demo tender with two lots. Safe to run again: nothing is added if DEMO-001 already exists.
-- ASCII only on purpose: Windows PowerShell 5.1 garbles non-Latin text piped into psql.
-- Creates a demo client when the clients table is empty. The creator is the first user (the super admin).

DO $$
DECLARE
    v_client  bigint;
    v_user    bigint;
    v_tender  bigint;
BEGIN
    SELECT id INTO v_user FROM users ORDER BY id LIMIT 1;
    IF v_user IS NULL THEN
        RAISE EXCEPTION 'users table is empty: run createuser first';
    END IF;
    IF EXISTS (SELECT 1 FROM tenders WHERE code = 'DEMO-001') THEN
        RAISE NOTICE 'Tender DEMO-001 already exists, skipping';
        RETURN;
    END IF;

    SELECT id INTO v_client FROM clients ORDER BY id LIMIT 1;
    IF v_client IS NULL THEN
        INSERT INTO clients (legal_name, segment, status)
        VALUES ('Demo Client LLP', 'demo', 'ACTIVE')
        RETURNING id INTO v_client;
    END IF;

    INSERT INTO tenders (
        code, title, description, status, client_id, created_by, published_at, deadline_at,
        planned_start_at, planned_end_at, client_revenue,
        initial_tender_amount, current_tender_amount, visibility_mode,
        works_description, service_schedule, required_capacity, subcontractor_requirements, internal_comments
    ) VALUES (
        'DEMO-001',
        'Demo tender: facility maintenance',
        'Test record for showing the interface. Not a real tender.',
        'PUBLISHED', v_client, v_user, now(), now() + interval '14 days',
        now() + interval '30 days', now() + interval '120 days', 2000000.00,
        1500000.00, 1500000.00, 'ALL_ELIGIBLE',
        'Daily maintenance of facilities, two shifts.',
        'Mon-Sat, 2 shifts of 10 hours',
        8,
        'At least 2 years of experience, own staff, ready to travel.',
        'Internal note: visible to administrators only.'
    ) RETURNING id INTO v_tender;

    INSERT INTO tender_lots (tender_id, name, description, status, initial_tender_amount, current_tender_amount) VALUES
        (v_tender, 'Lot 1. Day shift',   'Maintenance during the day',   'OPEN', 800000.00, 800000.00),
        (v_tender, 'Lot 2. Night shift', 'Maintenance during the night', 'OPEN', 700000.00, 700000.00);

    RAISE NOTICE 'Created tender DEMO-001 with id %', v_tender;
END $$;