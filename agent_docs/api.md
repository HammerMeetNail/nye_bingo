# API & Authentication

## API Routes

Auth: `POST /api/auth/{register,login,logout}`, `GET /api/auth/me`, `POST /api/auth/password`, `PUT /api/auth/searchable`
Email Auth: `POST /api/auth/{verify-email,resend-verification,magic-link,forgot-password,reset-password}`, `GET /api/auth/magic-link/verify`

Cards: `POST /api/cards`, `GET /api/cards`, `GET /api/cards/archive`, `GET /api/cards/export`, `GET /api/cards/{id}`, `GET /api/cards/{id}/stats`, `POST /api/cards/{id}/{items,shuffle,finalize}`, `PUT /api/cards/{id}/visibility`, `PUT /api/cards/visibility/bulk`, `PUT /api/cards/archive/bulk`, `DELETE /api/cards/bulk`

Items: `PUT/DELETE /api/cards/{id}/items/{pos}`, `POST /api/cards/{id}/swap`, `PUT /api/cards/{id}/items/{pos}/{complete,uncomplete,notes}`

Suggestions: `GET /api/suggestions`, `GET /api/suggestions/categories`

Friends: `GET /api/friends`, `GET /api/friends/search`, `POST /api/friends/requests`, `PUT /api/friends/requests/{id}/{accept,reject}`, `DELETE /api/friends/requests/{id}/cancel`, `DELETE /api/friends/{id}`, `GET /api/friends/{id}/card`, `GET /api/friends/{id}/cards`
Friend Invites: `GET/POST /api/friends/invites`, `POST /api/friends/invites/accept`, `DELETE /api/friends/invites/{id}/revoke`
Blocks: `GET/POST /api/blocks`, `DELETE /api/blocks/{id}`

Reactions: `POST/DELETE /api/items/{id}/react`, `GET /api/items/{id}/reactions`, `GET /api/reactions/emojis`

Support: `POST /api/support`

## API Documentation & Tokens

The API is documented using OpenAPI 3.0 and available at `/api/docs` (Swagger UI).

**Access**:
- API access requires a Bearer token in the Authorization header.
- Users generate tokens in their profile settings (`#profile`).
- Tokens have scopes (`read`, `write`, `read_write`) and optional expiration.

**Adding New Endpoints**:
1. Implement the handler and register the route in `cmd/server/main.go`.
2. Apply appropriate middleware: `requireRead`, `requireWrite`, or `requireSession` (for non-API routes).
3. Update `web/static/openapi.yaml` to document the new endpoint, including request/response schemas and security requirements.
4. Verify the documentation appears correctly in Swagger UI at `/api/docs`.

**Swagger UI**:
- Hosted at `/api/docs`
- Assets (JS/CSS) vendorized in `web/static/swagger/` (no external CDN dependency)
- Definition file: `web/static/openapi.yaml`

## Email Service

**Provider**: [Resend](https://resend.com) - Domain verified for yearofbingo.com
**Local Development**: Use Mailpit (SMTP capture) or console logging
**See**: `plans/auth.md` for full email authentication implementation plan
