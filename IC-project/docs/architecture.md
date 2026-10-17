# IC Tender — architecture

```text
Browser -> Next.js frontend (apps/web) -> HTTP/JSON -> Go REST API (apps/api) -> PostgreSQL
```

The Go API is the authority for roles, workflow transitions, price calculation and audit records.

## Roles
- `SUPER_ADMIN`: approvals, manual winner selection, partner directory, internal finance, settings.
- `JUNIOR_ADMIN`: prepares tenders and draft amendments; cannot approve own tender or change global rules.
- `SUBCONTRACTOR`: sees eligible published tenders, own responses, assigned objects and completed objects only.

There are no client accounts.

## Invariants
1. IC Group sets the tender price.
2. Subcontractors do not submit counter-prices.
3. A human with appropriate permissions selects the winner.
4. Deadline proximity is the only automatic trigger for changing an open tender amount.
5. No automatic price adjustment after a winner is selected.
6. Internal ratings, penalties, comments and IC margin are admin-only.
7. A contract is completed only after explicit administrator confirmation.
8. Additional services and penalty adjustments are separate contract amendments.
9. Unapproved financial amendments do not affect final calculations.
10. Financial data uses exact decimal arithmetic.

## Scaffold status
The starter contains a minimal Go API, a Next.js shell and an initial schema. Authentication, role middleware, business endpoints, deadline worker, imports and full operational workflows remain to be implemented.
