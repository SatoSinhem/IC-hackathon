# API contract conventions

Base path: `/api/v1`. Use JSON. Standard error shape: `{ "error": { "code": "...", "message": "..." } }`.

- Validate all request bodies on the server.
- Check role and resource ownership for every request.
- Never return internal-only fields to subcontractor endpoints.
- Use transactions for award, price adjustment, amendment approval and completion.
- Never trust browser-supplied role, status, winner ID or final price.

Planned route groups: `/auth`, `/admin/tenders`, `/admin/subcontractors`, `/admin/contracts`, `/admin/amendments`, `/subcontractor/tenders`, `/subcontractor/responses`, `/subcontractor/objects`, `/subcontractor/completed-objects`, `/internal/jobs`.

Implemented routes:
- `GET /api/v1/health`, `GET /api/v1/ready`
- `POST /api/v1/auth/login` (`{email,password}` -> `{user}` + session cookie), `POST /api/v1/auth/logout` (204), `GET /api/v1/auth/me`, `POST /api/v1/auth/change-password` (`{currentPassword,newPassword}`, 204).

Auth errors: 401 `unauthenticated` / `invalid_credentials`, 403 `forbidden` / `password_change_required` / `forbidden_origin`, 422 `weak_password`, 429 `rate_limited` (with `Retry-After`).
Unsafe methods with a foreign `Origin` (or `Sec-Fetch-Site: cross-site`) are rejected. Until the first password change only `/auth/me`, `/auth/logout` and `/auth/change-password` are reachable.
