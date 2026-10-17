# Implementation plan

## Phase 1 — foundation
- [x] Monorepo structure
- [x] Go API bootstrap
- [x] Next.js bootstrap
- [x] PostgreSQL Compose service
- [x] Initial SQL schema
- [x] Verify migration on a clean database
- [ ] CI and test setup

## Phase 2 — security
- [x] Authentication and session management
- [ ] Role-based and object-level authorization (role guards exist; object-level checks come with each endpoint)
- [x] Admin account provisioning and first-login password change
- [ ] Audit logging (authentication events are journaled; business events are not yet)

## Phase 3 — tender lifecycle
- [ ] Drafts and approvals
- [ ] Publish/close tender
- [ ] Subcontractor responses
- [ ] Manual winner selection in transaction
- [ ] Contract creation

## Phase 4 — pricing
- [ ] Decimal pricing service
- [ ] Configurable deadline tiers
- [ ] Idempotent background worker
- [ ] Price history and notifications

## Phase 5 — operations
- [ ] Admin-only subcontractor directory
- [ ] Contracts and objects
- [ ] Contract amendments
- [ ] Additional-service approval
- [ ] Penalty-adjustment workflow
- [ ] Explicit contract completion
- [ ] Completed objects for subcontractors

## Phase 6 — hackathon data
- [ ] Import contracts and amendments
- [ ] Import client portfolio and standards
- [ ] Import subcontractors, operators, shifts and attendance
- [ ] Validate conflicts and quality issues
- [ ] Monthly closing and financial report
- [ ] Integrate employment platform only after reviewing its API documentation

## Phase 7 — hardening
- [ ] Unit/integration/E2E tests
- [ ] Permission and IDOR tests
- [ ] Production build
- [ ] Deployment and backup instructions
