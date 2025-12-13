# AI Goal Wizard Implementation Plan

## Overview

**Goal:** Implement an interactive "Wizard" workflow that generates personalized Bingo cards using Generative AI.
**Core Value:** Solves the "blank canvas" problem by converting user-selected category + interest into specific, achievable, and safe goals.

## Implemented Scope (This Branch)

- **Wizard UX**: Modal wizard to generate 24 goals, review/edit inline, then either create a new card or append goals to an existing card.
- **Auth**: Session cookie required (browser session only; API tokens are not accepted).
- **Email verification gating**: Users get **5 free AI generations** before email verification is required; remaining count is shown in the UI verification banner and the wizard.
- **Rate limiting**: Redis-backed request limiter per-user (default 10/hour; higher in development; configurable via `AI_RATE_LIMIT`).
- **Provider**: Google Gemini `gemini-2.5-flash-lite` via backend proxy (API key never exposed to the client).
- **Output**: Exactly 24 strings in a JSON array, returned to the client as `{ "goals": [...] }`.
- **Audit logs**: Persists model/tokens/duration/status to `ai_generation_logs` for cost monitoring (no storage of user prompt fields).
- **Content style**: Goals are framed as "micro-adventure quests" in impersonal imperative phrasing (no "you/your/you're"), with a realism constraint toward modern US day-trip/road-trip locations.

## User Stories

1.  **As a new user**, I want to describe my interests (e.g., "cooking," "hiking") so that I don't have to think of 24 unique goals from scratch.
2.  **As a user**, I want to choose the "difficulty" and "budget level" of my card so that the goals fit my lifestyle.
3.  **As a user**, I want to review the AI-generated goals and swap out ones I don't like before creating the card.
4.  **As a system owner**, I want to ensure generated content is safe (no NSFW/hate speech) and that the system cannot be abused to bypass LLM costs.

## Design Decisions & Evaluation

### 1. AI Provider Selection

We evaluated three primary contenders for cost-effective, high-speed generation:

| Provider | Model | Cost (Input/Output per 1M tokens) | Speed | Safety Filters | Verdict |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Google** | **Gemini 2.5 Flash-Lite** | **$0.10 / $0.40** (Paid) or **FREE** (Tier) | **Fastest** | **Excellent (Native)** | **WINNER** |
| Google | Gemini 2.5 Flash | $0.30 / $2.50 | Fast | Excellent | Too expensive |
| OpenAI | GPT-4o-mini | $0.15 / $0.60 | Fast | Good | Runner Up |

**Why Gemini 2.5 Flash-Lite?**
-   **Cost:** Lowest cost per request ($0.10/$0.40 vs $0.30/$2.50 for standard Flash).
-   **Structured Output:** Excellent support for JSON schema enforcement.
-   **Safety:** Built-in configurable safety settings.

### 2. Architecture: Hosted Proxy vs. BYOK (Bring Your Own Key)

**Decision: Hosted Proxy (Backend-driven)**

*   **User Experience:** "Bring Your Own Key" adds massive friction. Non-technical users do not have API keys. A wizard should be magical, not administrative.
*   **Security:** API keys must never be exposed to the client. A backend proxy is required regardless.
*   **Safety:** We must enforce the System Prompt to ensure safety. Allowing users to supply their own key might imply they can supply their own prompts, which breaks the "Bingo" constraint.
*   **Cost Control:** With Gemini Flash, the cost is so low that subsidizing it is feasible. We will mitigate abuse via Rate Limiting.

### 3. Data Persistence

*   **User Input Data:** **Transient**. We do *not* store wizard inputs (category, focus, difficulty, budget, context) in the database to minimize privacy risks. This data exists only in memory during the request lifecycle.
*   **Generated Goals:** Stored only when the user clicks "Create Card" (saved as standard `CardItem` rows).
*   **Wizard State:** In-memory only (closing the modal resets the wizard; no `localStorage` persistence).

## Gemini API Integration Details

### Endpoint
`POST https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent`

### Request Payload
We use `generationConfig` to enforce a JSON array of strings as the output, and send the API key via the `x-goog-api-key` request header (not a query param).

```json
{
  "systemInstruction": {
    "parts": [
      { "text": "..." }
    ]
  },
  "contents": [
    {
      "parts": [
        {
          "text": "USER_MESSAGE"
        }
      ]
    }
  ],
  "generationConfig": {
    "responseMimeType": "application/json",
    "responseSchema": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "temperature": 1.0
  },
  "safetySettings": [
    { "category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE" },
    { "category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_MEDIUM_AND_ABOVE" },
    { "category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_MEDIUM_AND_ABOVE" },
    { "category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_MEDIUM_AND_ABOVE" }
  ]
}
```

### Response Format

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "[\"Goal 1\", \"Goal 2\", ...]"
          }
        ]
      },
      "finishReason": "STOP",
      "safetyRatings": [...]
    }
  ]
}
```

## Authentication & Data Privacy

### Authentication Strategy
We will use the **Gemini API Key** (via Google AI Studio) for authentication.
*   **Mechanism**: The key is sent as the `x-goog-api-key` HTTP request header.
*   **Security**: The key is stored as a backend environment variable (`GEMINI_API_KEY`) and is **never** exposed to the client.

### Data Privacy & Training
*   **Terms of Service**: We will use a **Paid Tier** (Blaze plan) or an account with **Cloud Billing enabled**. Under these terms, Google **does not** use input data (prompts) or output data (responses) to train their models.
*   **Vertex AI Comparison**: While Vertex AI offers "Zero Data Retention" guarantees, the complexity of setting up GCP IAM, Service Accounts, and OAuth flows is excessive for this feature's current scope. The standard Gemini API's privacy terms for paid accounts are sufficient for consumer-grade goals (non-PII, non-HIPAA).
*   **Long-Term**: If the app pivots to enterprise or healthcare use cases, we will migrate to Vertex AI to leverage its enhanced compliance controls.

## Database Schema

### New Table: `ai_generation_logs` (Audit & Analytics)

While we don't store user PII, we need to track usage for rate limiting and cost monitoring.

```sql
CREATE TABLE ai_generation_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model VARCHAR(50) NOT NULL,                           -- e.g. "gemini-2.5-flash-lite"
    tokens_input INT NOT NULL,
    tokens_output INT NOT NULL,
    duration_ms INT NOT NULL,
    status VARCHAR(20) NOT NULL,                          -- 'success', 'error', 'safety_block'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_logs_user_date ON ai_generation_logs(user_id, created_at);
```

### New User Column: `users.ai_free_generations_used` (Verification Gate)

To allow a limited trial of AI for unverified accounts while keeping costs bounded, we track a per-user counter:

```sql
ALTER TABLE users
ADD COLUMN ai_free_generations_used INT NOT NULL DEFAULT 0;
```

- Unverified users can successfully generate goals up to **5 times**, after which the API returns `403` until the user verifies their email.
- The UI displays the remaining free generations in the existing email verification banner and in the wizard itself.

## API Endpoints

### 1. Generate Goals

`POST /api/ai/generate`

**Auth:** Session required.
**Verification:** Email verification required after **5 free generations** per user (unverified users can generate up to 5 times, then must verify).
**Rate Limit:** Redis-backed per-user limit (default **10 requests / hour** in non-development; higher in development; override via `AI_RATE_LIMIT`).

**Request Payload:**
```json
{
  "category": "hobbies",       // "health", "career", "social", "travel", "mix"
  "focus": "cooking",          // Optional free-text (max 100 chars)
  "difficulty": "medium",      // "easy", "medium", "hard"
  "budget": "free",            // "free", "low", "medium", "high"
  "context": "I want to learn italian cuisine" // Optional extra context (max 500 chars)
}
```

**Response:**
```json
{
  "goals": [
    "Cook a pasta dish from scratch",
    "Make homemade pizza dough",
    "Identify 3 different fresh herbs",
    ... (24 items)
  ],
  "free_remaining": 3
}
```

## Technical Architecture

### Backend (Go)

**Package:** `internal/services/ai`

**Interfaces:**
```go
type AIProvider interface {
    GenerateGoals(ctx context.Context, userID uuid.UUID, prompt GoalPrompt) ([]string, UsageStats, error)
}
```

**Implementation Details:**
-   **Request shape:** Uses `systemInstruction` + `contents` and the `x-goog-api-key` header.
-   **Prompt engineering:** Uses an "adventure curator / micro-adventure" persona with strict formatting rules (JSON array; 24 items; each item is `Short Title: Description`; <15 words).
-   **Injection mitigation:** Focus/context are sanitized, angle brackets are escaped, and the prompt explicitly instructs the model to treat tagged content as background only.
-   **Parsing & validation:** Strips markdown code fences if present, trims whitespace, truncates to 24 if needed, and errors if the result is not exactly 24.
-   **Usage logging:** Writes `model`, `tokens_input`, `tokens_output`, `duration_ms`, `status` to `ai_generation_logs` (best-effort; does not log user-provided prompt fields).

### Frontend (JS)

**State Management (`window.AIWizard.state`):**
-   `step`: `'input' | 'loading' | 'review'`
-   `inputs`: `{ category, focus, difficulty, budget, context }`
-   `results`: `string[]` (24)
-   `mode`: `'create' | 'append'`
-   `targetCardId`: `string | null`

## UI / UX Flow

### Step 1: "The Interview" (Input)
```
┌─────────────────────────────────────────────────────────────┐
│ 🧙 Bingo Wizard                                             │
├─────────────────────────────────────────────────────────────┤
│ What area of life is this for?                              │
│ [ ▼ Hobbies ]                                               │
│                                                             │
│ What specifically? (e.g. "Hiking", "Coding")                │
│ [_______________________]                                   │
│                                                             │
│ How intense?                                                │
│ ( ) Chill   (●) Balanced   ( ) Hardcore                     │
│                                                             │
│ Budget level                                                │
│ (●) Free   ( ) Low   ( ) Medium   ( ) High                  │
│                                                             │
│ Any other context?                                          │
│ [ I live in a city, I don't drive...    ]                   │
│                                                             │
│                   [ Generate Goals ✨ ]                     │
└─────────────────────────────────────────────────────────────┘
```

### Step 2: "The Review" (Refinement)
```
┌─────────────────────────────────────────────────────────────┐
│ 🔮 Your Generated Card                                      │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
│ │ Walk 5k │ │ New Rec │ │ ...     │ │ ...     │ │ ...     │ │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ │
│ ... (Grid of 24 items) ...                                  │
│                                                             │
│ [ Start Over ]   [ Edit items inline ]                      │
│                                                             │
│          [ Create Card → ]  or  [ Add to Card → ]           │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Phases (Adjusted to Match Implementation)

### Phase 0: Configuration & Secrets
1.  [x] **Environment**: `.env.example` includes `GEMINI_API_KEY`.
2.  [x] **Compose Files**: `compose.yaml`, `compose.prod.yaml`, and `compose.server.yaml` pass `GEMINI_API_KEY` through to the server container.
3.  [x] **Config Loader**: `internal/config/config.go` loads `GEMINI_API_KEY` into `cfg.AI.GeminiAPIKey`.
4.  [x] **CI/CD**: GitHub Actions deployment writes `GEMINI_API_KEY` into the deployed `.env` on the server (expects the `GEMINI_API_KEY` secret to exist in GitHub).

### Phase 1: Infrastructure & Service
1.  [x] **Migration**: `ai_generation_logs` table created (user_id required).
2.  [x] **Service**: `internal/services/ai/gemini.go` implements Gemini calls, JSON schema enforcement, response parsing, safety handling, and usage logging.

### Phase 2: API Layer
1.  [x] **Handler**: `internal/handlers/ai.go` validates input and maps service errors to HTTP responses.
2.  [x] **Rate Limiting**: Redis-backed fixed-window limiter (`INCR` + `EXPIRE`) per user ID; fail-closed for the AI endpoint.
3.  [x] **Route**: `POST /api/ai/generate` registered in `cmd/server/main.go` and documented in `web/static/openapi.yaml`.
4.  [x] **Tests**: Unit tests for handler validation/error mapping, provider request/response parsing, and injection escaping.

### Phase 3: Frontend Implementation
1.  [x] **API Client**: `API.ai.generate(...)` calls `POST /api/ai/generate`.
2.  [x] **Wizard Logic**: `web/static/js/ai-wizard.js` implements input → loading → review, inline editing, create-card, and append-to-card flows.
3.  [x] **Integration**: Wizard entry points added to the create-card modal and the card view.
4.  [x] **Assets**: `ai-wizard.js` is included in the SPA template via the hashed asset manifest.

## Testing Strategy

Given the external dependency (AI API) and the non-deterministic nature of LLMs, testing requires a robust strategy that balances reliability with cost.

### 1. Unit Tests (`internal/services/ai/`)
*   **Prompt Construction**: Verify that user inputs (category, focus, difficulty) are correctly formatted into the final prompt string.
*   **Response Parsing**: Test the JSON parsing logic against various mock AI responses (valid JSON, malformed JSON, empty arrays).
*   **Error Handling**: Ensure the service correctly maps API errors (429, decode failures, safety finish reasons) to internal application errors.

### 2. QA & Safety Verification
*   **Safety Probe**: A specific manual test set where we attempt to feed "unsafe" inputs to the local dev environment to verify the safety filters trigger the correct error message.

## Error Handling Specification

We will strictly adhere to the project's existing error handling patterns (Service-defined errors -> Handler mapping).

### 1. Service Layer Errors (`internal/services/ai/errors.go`)
We will define specific sentinel errors to handle the nuances of AI interaction:

```go
var (
    ErrAIProviderUnavailable = errors.New("AI provider is currently unavailable") // 503
    ErrSafetyViolation       = errors.New("generated content violated safety policies") // 400
    ErrRateLimitExceeded     = errors.New("rate limit exceeded") // 429
    ErrInvalidInput          = errors.New("invalid input parameters") // 400
)
```

### 2. HTTP Handler Mapping (`internal/handlers/ai.go`)
The handler will catch these errors and map them to standard HTTP status codes using the existing `writeError` helper:

| Service Error | HTTP Status | User Message |
| :--- | :--- | :--- |
| `ErrAIProviderUnavailable` | `503 Service Unavailable` | "The AI service is currently down. Please try again later." |
| `ErrSafetyViolation` | `400 Bad Request` | "We couldn't generate safe goals for that topic. Please try rephrasing." |
| `ErrRateLimitExceeded` | `429 Too Many Requests` | "AI provider rate limit exceeded." |
| `ErrAINotConfigured` | `503 Service Unavailable` | "AI is not configured on this server. Please try again later." |
| `ErrInvalidInput` | `400 Bad Request` | "Invalid input provided." |
| *(Unknown Error)* | `500 Internal Server Error` | "An unexpected error occurred." |

**Note:** In addition to the provider-side 429 above, the server also enforces a Redis-backed per-user request limit on this endpoint (returns `429` with `"Rate limit exceeded"` when exceeded).

### 3. JSON Response Format
All errors will return the standard JSON structure:
```json
{
  "error": "We couldn't generate safe goals for that topic. Please try rephrasing."
}
```

## Security & Safety Considerations

1.  **Input Injection**:
    -   User free-text input must be sanitized before insertion into the prompt to prevent "Ignore previous instructions" attacks.
    -   *Mitigation*: We will use the Gemini API's "structure" fields (System Instructions vs User Content) effectively to segregate instructions from data.
2.  **Output Safety**:
    -   We rely on Gemini's "Hate speech", "Harassment", and "Dangerous content" filters set to `BLOCK_MEDIUM_AND_ABOVE`.
    -   If the API returns a safety violation, we show a generic error: "We couldn't generate safe goals for that topic. Please try rephrasing."
3.  **Cost Denial of Service**:
    -   Strict Rate Limits (10/hr).
    -   Request size limits on the handler (8KB JSON body; focus/context max lengths).

## Not Implemented (Planned Enhancements)

- **Wizard resume via `localStorage`**: The wizard state is currently in-memory only (closing the modal resets it).
- **Regenerate-all / per-item regeneration**: The wizard currently supports “Start Over” (new run) and manual editing, but not server-side regeneration controls.
- **Recorded “live” provider tests (VCR/cassette)**: Tests use mocked HTTP round-trippers instead of recorded live responses.
- **Frontend unit tests for the wizard**: No JS tests were added for `AIWizard`.

## Cost Analysis

**Scenario: 1,000 Cards Generated per Month**

*   **Prompt Size**: ~300 tokens (System + User context).
*   **Response Size**: ~400 tokens (24 short sentences).
*   **Total Tokens**: 700 * 1,000 = 700,000 tokens.
*   **Gemini Flash Price**: ~$0.30 / 1M tokens (blended).
*   **Total Cost**: **$0.21 / month**.

**Conclusion**: The feature is effectively free to operate at our current scale.
