# API Specification

## Authentication
- POST /api/login
  - Request: { username, password }
  - Response: { token, expiresIn }
- POST /api/logout

## Poems
- GET /api/poems?query=...&author=...&dynasty=...&tag=...
- GET /api/poems/:id
- POST /api/poems (admin/editor only)
- PUT /api/poems/:id (admin/editor only)
- DELETE /api/poems/:id (admin only, approval required if workflow enabled)

## Authors
- GET /api/authors
- GET /api/authors/:id

## Reviews
- POST /api/reviews
  - Request: { itemId, rating_accuracy, rating_readability, rating_value, text }
  - Response: { reviewId }

## Complaints
- POST /api/complaints
- GET /api/complaints/:id (staff only)
- PATCH /api/complaints/:id/status (arbitrator only)

## Pricing & Campaigns
- GET /api/pricing
- POST /api/coupons (marketing manager only)
- POST /api/campaigns (marketing manager only)

## Content Packs
- GET /api/content-packs
- POST /api/content-packs/download (triggers resumable download)

## Crawler
- POST /api/crawler/jobs
- GET /api/crawler/jobs/:id/status

## Admin & Audit
- GET /api/audit-logs
- GET /api/approvals/pending
- POST /api/approvals/:id/approve

## Error Codes
- 401 Unauthorized
- 403 Forbidden
- 404 Not Found
- 409 Conflict (idempotency violation)
- 422 Validation Error
- 500 Internal Server Error

## Security
- All endpoints require session token.
- Passwords are salted/hashed.
- Sensitive complaint notes are encrypted at rest.
