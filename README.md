# PlateMate Agentic Dashboard Overview
## Project Knowledge File

---

## 1. Core Concept

PlateMate's agentic dashboard is a restaurant-facing platform that transforms dish-level review data into automated, cross-platform menu actions. It is the B2B side of PlateMate's two-sided marketplace.

**Who it's for:** Restaurant operators (owners, GMs, managers) who manage menus, pricing, and promotions across multiple platforms — typically a POS (Toast), delivery apps (Uber Eats, DoorDash), inventory management (R365), and review platforms (Google).

**The problem:** A restaurant operator who wants to remove an underperforming dish must log into 4-5 different platforms, make the same change manually in each, and hope they don't forget one. A price change is the same. Running a promo requires navigating each platform's separate campaign builder. There is no single system that connects what diners think about a dish to what the restaurant does about it.

**The value prop — "insight to action":** PlateMate aggregates dish-level intelligence from multiple sources (its own structured reviews, Google Reviews, Uber Eats feedback, Toast sales data, R365 food costs), surfaces actionable recommendations, and executes accepted changes across the restaurant's entire tech stack from a single interface. One click, every platform updated.

**What makes this defensible:** PlateMate is the only platform that collects structured, multi-dimensional dish-level reviews (taste, portion, value, overall, reorder intent). This proprietary data doesn't exist anywhere else in this format. Combined with the cross-platform execution layer, PlateMate creates a two-sided moat: a data moat (unique dish intelligence) and an integration moat (deep tech stack connections that increase switching costs).

---

## 2. Platform Integrations Architecture

### Architecture Pattern: Platform Adapters

Every external platform is abstracted behind a uniform adapter interface in the Go backend. This is the core architectural decision — it makes adding platforms trivial and ensures the execution engine, dashboard, and recommendation engine are all platform-agnostic.

```go
type PlatformAdapter interface {
    Name() string
    CanAutomate() bool
    SyncMenu(ctx context.Context, restaurantID string) ([]DishMapping, error)
    RemoveDish(ctx context.Context, dishID string) (*ActionResult, error)
    ChangePrice(ctx context.Context, dishID string, newPrice float64) (*ActionResult, error)
    RunPromo(ctx context.Context, config PromoConfig) (*ActionResult, error)
    GetManualSteps(actionType string, dishID string, params interface{}) *ManualChecklist
    Rollback(ctx context.Context, actionID string, preChangeState interface{}) (*ActionResult, error)
}
```

### 2.1 Uber Eats

**What it provides:**
- Full delivery menu with items, categories, modifiers, pricing, availability
- Item-level suspension (sold out / back in stock)
- Promotional campaign management (6 promo types)
- Order data and reporting (CSV-based, async)
- Item-level customer feedback (thumbs up/down, tags, comments via reporting exports)

**What actions it enables:**
- Remove dish (set `suspension_info.suspension.suspend_until`)
- Restore dish (set `suspension_info.suspension` to null)
- Change price (update `price_info.price` in cents)
- Run promo (create campaign via Promotions API: FLATOFF, PERCENTOFF, BOGO, FREEITEM_MINBASKET, MENU_ITEM_DISCOUNT, FREEDELIVERY)
- Revoke promo

**Key endpoints:**
- Menu read: `GET /v2/eats/stores/{store_id}/menus`
- Item update (sparse): `POST /v2/eats/stores/{store_id}/menus/items/{item_id}`
- Full menu upload: `PUT /v2/eats/stores/{store_id}/menus`
- Create promo: `POST /v1/delivery/stores/{store_id}/promotion`
- Revoke promo: `POST /v1/delivery/promotions/{promotion_id}/revoke`
- Store status: `POST /v1/delivery/store/{store_id}/update-store-status`

**Auth model:** OAuth 2.0. Client credentials flow for day-to-day operations. Authorization code flow for initial store provisioning (onboarding). Separate tokens — cannot mix scopes across grant types. 30-day token expiry.

**Current status:** Sandbox available immediately (test-api.uber.com / sandbox-login.uber.com). No approval needed to start development. Production requires separate application and certification. Sandbox is the first integration built.

**Important constraints:**
- Item update only works if menu was originally uploaded via API (not Menu Maker)
- PUT menu overwrites everything — entities not in the payload are deleted
- Promotions cannot be modified after creation — must revoke and recreate
- Images may take hours to process
- Prices are in cents (1299 = $12.99)
- Promo amounts in smallest currency unit (1000 USD = $10.00)

### 2.2 Toast POS

**What it provides:**
- Full in-house menu with resolved pricing, modifiers, tags, calories, prep stations, visibility flags
- Real-time stock/inventory status per menu item
- Historical order data (dish-level: items per order, revenue, time patterns, dine-in vs delivery)
- Native dish-level sales analytics (net sales, quantity sold, average price, waste count per menu item)
- Labor data (hours, wages, tips)
- Restaurant profile, hours, delivery zones

**What actions it enables:**
- Mark item out of stock / set quantity / restore (Stock API — primary agentic write action)
- Apply discounts to live orders (Orders API)
- Read dish-level sales reports (Analytics API)

**Key endpoints:**
- Menu read: `GET /menus/v2/menus`
- Menu freshness check: `GET /menus/v2/metadata`
- Stock update: `PUT /stock/v1/inventory/update`
- Stock read: `GET /stock/v1/inventory`
- Menu analytics: `POST /era/v1/menu/{timeRange}` → `GET /era/v1/menu/{reportRequestGuid}`
- Sales analytics: `POST /era/v1/metrics/{timeRange}` → `GET /era/v1/metrics/{reportRequestGuid}`
- Restaurant info: `GET /restaurants/v1/restaurants/{guid}`

**Auth model:** OAuth 2.0 client credentials flow. All requests require `Toast-Restaurant-External-ID` header. Access via custom integration credentials created by the restaurant operator in Toast Web (Integrations → Toast API Access). Required scopes: `menus:read`, `stock:read`, `stock:write`, `orders:read`, `enterprise-metrics:read`.

**Current status:** Restaurant partner confirmed on Toast and willing to share credentials. Custom integration path — no partner program application needed. Credentials expected in Week 1 via customer interview.

**Important constraints:**
- Menus API is read-only — cannot delete or add menu items via API. Removal is achieved by setting stock status to OUT_OF_STOCK via Stock API.
- Price changes via API depend on configuration API write access, which may not be available on custom integrations. Fallback: manual step with link to Toast Web.
- Analytics API is async (POST to request report → GET to retrieve, may return 202 while processing).
- Standard API access is read-only. Write access requires custom integration or partner integration credentials.
- `multiLocationId` is the consistent identifier across locations in a group.

### 2.3 Restaurant365 (R365)

**What it provides:**
- Actual food cost per dish (recipe-level ingredient costing from vendor invoices)
- Vendor and purchasing data (supplier pricing, invoice history, cost changes over time)
- Actual vs. theoretical food cost variance
- Dish-level sales data (menu item name, quantity, amount per ticket)
- Labor cost data (hours, pay rate, job title)
- Full chart of accounts and financial categorization

**What actions it enables:**
- Read: dish-level sales data, ingredient costs, vendor pricing, inventory levels
- Write: push AP invoices and journal entries (lower priority for MVP)
- Derived: adjust par levels and purchasing recommendations when a dish is removed

**Key endpoints (OData — read):**
- Sales detail (dish-level): `GET /api/v2/views/SalesDetail` — fields: `menuitem`, `amount`, `quantity`, `salesID`
- Sales tickets: `GET /api/v2/views/SalesEmployee` — fields: `netSales`, `dayPart`, `serviceType`, `numberOfGuests`
- Financial transactions: `GET /api/v2/views/Transaction` + `TransactionDetail`
- Inventory items: `GET /api/v2/views/Item`
- Vendors: `GET /api/v2/views/Company`
- Labor: `GET /api/v2/views/LaborDetail`
- Locations: `GET /api/v2/views/Location`

**Key endpoints (REST API — write):**
- Authentication: `POST /APIv1/Authenticate/JWT`
- AP Invoices: `POST /APIv1/APInvoices`

**Auth model:** Two mechanisms. REST API uses JWT bearer token via `POST /APIv1/Authenticate/JWT`. OData connector uses Basic auth with `Domain\Username` format. Per-customer database — each restaurant has their own R365 subdomain.

**Current status:** Restaurant partner needs to email R365 Support requesting API access for PlateMate. R365 Support confirms with the customer and provides credentials. Expected timeline: 1-2 weeks from request.

**Important constraints:**
- Sales endpoints throttled to 31-day windows per request
- Sales endpoints (`SalesEmployee`, `SalesDetail`, `SalesPayment`) do NOT support `$select` or `$count`
- Use `rowVersion` property for incremental sync
- Intraday polling data is NOT available via OData — only after Daily Sales Summary
- No self-service OAuth flow — R365 Support involvement required for each new restaurant

### 2.4 Google Reviews / Places

**What it provides:**
- Full review corpus: all Google reviews for the restaurant (via Business Profile API with Manager access)
- Per review: star rating, full text, reviewer name, timestamp, owner reply
- AI-powered review summary synthesized from all reviews (via Places API — `reviewSummary`)
- Restaurant metadata: overall rating, review count, hours, location, attributes
- Competitor data: nearby restaurant reviews and summaries (via Places API Nearby Search)

**What actions it enables:**
- Read: all reviews (Business Profile API, paginated 50/page)
- Read: AI review summary (Places API `reviewSummary` field)
- Write: reply to reviews (Business Profile API)
- Derived: NLP extraction of dish mentions from review text → dish-level sentiment

**Key endpoints (Business Profile API — full review access):**
- List all reviews: `GET /v4/accounts/{accountId}/locations/{locationId}/reviews` (paginated, 50/page)
- Single review: `GET /v4/accounts/{accountId}/locations/{locationId}/reviews/{reviewId}`
- Reply to review: `PUT /v4/accounts/{accountId}/locations/{locationId}/reviews/{reviewId}/reply`
- Multi-location batch: `POST /v4/accounts/{accountId}/locations:batchGetReviews`

**Key endpoints (Places API — metadata + summaries):**
- Place details: `GET /v1/places/{PLACE_ID}` — includes `reviewSummary`, `generativeSummary`, `rating`, `userRatingCount`
- Text search: `POST /v1/places:searchText`
- Nearby search: `POST /v1/places:searchNearby`

**Auth model:** Business Profile API uses OAuth 2.0 — restaurant grants PlateMate Manager access to their Google Business Profile. Places API uses API key via `X-Goog-Api-Key` header.

**Current status:** PlateMate already completed a Places API POC on the consumer side. Business Profile API requires restaurant to add PlateMate as Manager on their profile — to be requested during customer interview.

**Important constraints:**
- Business Profile API requires one-time approval from Google (apply for GBP API access). PlateMate must register as a GBP Organization.
- Places API returns only 5 "most relevant" reviews (hard cap) — but the `reviewSummary` is synthesized from all reviews.
- Business Profile API is free. Places API costs $25 per 1,000 requests ($200/month free credit).
- Must display attribution per Google's requirements ("Summarized with Gemini" for review summaries).
- Explicit written/digital consent required from restaurant owner (Google's third-party policy).

### 2.5 PlateMate (Own Data)

**What it provides:**
- Structured dish-level reviews: taste (1-5), portion (1-5), value (1-5), overall (1-5)
- Reorder intent: would you order again (yes/no)
- Optional free-text review comments
- Optional dish photos
- User profiles and social graph (friends feed, collections)

**What actions it enables:**
- Primary data source for dish health scoring and recommendations
- The only source of structured, multi-dimensional dish ratings in the market
- Reorder intent is a direct proxy for customer loyalty per dish

**Where it lives:** MongoDB via existing Go backend API.

**Current status:** Consumer app on TestFlight, review flow fully built, pending App Store launch.

---

## 3. Dashboard MVP Feature Map

### 3.1 Unified Menu View

The central menu table showing every dish with cross-platform status and performance data.

**Columns:** Dish name, category, price (per platform), platform badges (which platforms the dish is active on), stock status, health score, review count, reorder rate.

**Powered by:**
- Toast Menus V2 API (`GET /menus/v2/menus`) — canonical menu structure, resolved pricing, item GUIDs
- Uber Eats Menu API (`GET /v2/eats/stores/{store_id}/menus`) — delivery menu, item IDs, suspension status
- R365 SalesDetail OData (`GET /api/v2/views/SalesDetail`) — menu item names matched to sales data
- PlateMate MongoDB — dish-level review aggregates

**Entity resolution:** Dishes are fuzzy-matched across platforms by name and mapped into a unified `dish_mapping` document (see Section 6).

### 3.2 Dish Deep Dive

Drill-down view for any individual dish showing all available intelligence.

**Sections:**
- Rating breakdown: bar chart of taste, portion, value, overall averages (PlateMate reviews)
- Reorder rate: headline percentage with trend (PlateMate reviews)
- Trend chart: rolling weekly average of overall rating (PlateMate reviews)
- External signals: Google Review mentions with extracted sentiment tags
- Uber Eats feedback: thumbs up/down rate, customer tags (from reporting exports)
- Sales performance: units sold, revenue, daypart breakdown (Toast Analytics API: `POST /era/v1/menu/{timeRange}`)
- Cost & margin: food cost per dish, margin calculation (R365 TransactionDetail + SalesDetail)
- Review feed: individual PlateMate reviews with text and photos
- Photo gallery: user-submitted dish photos

### 3.3 Action Center

Three primary actions available from the dashboard:

**Remove a Dish:**
- Operator selects dish from searchable dropdown
- Platform impact preview shows which systems will be updated and how
- Execute triggers the execution engine
- Automated: Toast (`PUT /stock/v1/inventory/update` → OUT_OF_STOCK), Uber Eats (`POST /v2/eats/stores/{sid}/menus/items/{iid}` → suspension), R365 (inventory par level adjustment)
- Manual fallback: any platform without API write access gets a step-by-step checklist with deep links

**Change a Price:**
- Operator selects dish, enters new price
- Shows current price on each platform
- Automated: Uber Eats (`POST /v2/eats/stores/{sid}/menus/items/{iid}` → price_info update), Toast (config API if available, otherwise manual step)
- Manual fallback for platforms without price write access

**Run a Promo:**
- Uber Eats only for MVP
- Operator selects promo type: percentage off, BOGO, free item, spend threshold
- Configures: which items, audience (new vs all customers), duration, budget cap
- Execute: `POST /v1/delivery/stores/{store_id}/promotion`
- Promo types map to Uber Eats API: PERCENTOFF, BOGO, FREEITEM_MINBASKET, MENU_ITEM_DISCOUNT, FLATOFF, FREEDELIVERY

### 3.4 Execution Status Tracker

Real-time view shown after operator clicks Execute:

- Each platform displayed as a row: platform name/logo, action description, status icon (✅ ⏳ ❌ 👤), duration
- Status updates streamed via Server-Sent Events from `GET /api/actions/:id/stream`
- For automated platforms: spinner → success/failure with timing
- For manual platforms: expandable checklist with steps and deep links, "Mark Complete" button
- Summary bar: "3 of 4 actions completed automatically"
- Undo button (triggers rollback within 24-hour window)

### 3.5 Action History

Audit trail of all past actions:

- Table: date, action type, dish, initiator, status
- Status: ✅ All Complete, ⚠️ Partial, ❌ Failed
- Click to expand per-platform breakdown with timestamps and details
- Filter by action type, date range
- Rollback status shown where applicable

### 3.6 Recommendations / Suggestions

Surfaced by the recommendation engine (Python service):

- Recommendation cards with: type (Remove, Promote, Investigate, Adjust Price), dish name, confidence level (High/Medium/Low), evidence summary (2-3 data points), platform impact preview
- Accept → triggers execution engine with the recommended action
- Decline → prompts for optional reason (dropdown), sets 14-day cooldown
- Tell Me More → navigates to dish deep dive

**Recommendation rules (initial, to be tuned):**
- REMOVE: overall_avg < 2.5 AND reorder_rate < 0.25 AND review_count >= 10
- PROMOTE: overall_avg > 4.2 AND reorder_rate > 0.75 AND review_count >= 10
- INVESTIGATE: 3+ weeks declining trend OR rating category variance > 2.0
- ADJUST_PRICE: value_avg < 2.5 AND taste_avg > 3.5 AND portion_avg > 3.5

**Safeguards:** Minimum review threshold, recency weighting (30-day reviews count 2x), cooldown after decline (14 days), operator always has final say, rollback available for 24 hours.

### 3.7 Settings / Platform Connections

- Platform connection status: 🟢 Connected / 🟡 Pending / 🔴 Error per platform
- Last sync timestamps per platform
- Manual re-sync trigger
- Demo mode toggle (uses sandbox/mock data for risk-free demonstrations)
- Credential management (secure update of API keys)

---

## 4. Agentic Workflows

### Layer 1: Direct Execution

Operator explicitly triggers an action. PlateMate executes it across platforms.

**Example — Remove a dish:**
1. Operator selects "Chicken Parmesan" → clicks "Remove Dish" → clicks "Execute"
2. Execution engine creates action record in MongoDB
3. Dispatches to all adapters in parallel (Go goroutines):
   - Toast adapter: `PUT /stock/v1/inventory/update` with `{ "guid": "<toast_guid>", "status": "OUT_OF_STOCK" }`
   - Uber Eats adapter: `POST /v2/eats/stores/{sid}/menus/items/{iid}` with `{ "suspension_info": { "suspension": { "suspend_until": <unix_ts> } } }`
   - R365 adapter: query ingredient mapping → check shared ingredients → adjust par levels
   - DoorDash (if applicable): generate manual checklist
4. SSE streams status to dashboard in real-time
5. Pre-change state stored for each platform (enables rollback)
6. Action logged with full audit trail

**Example — Run a promo:**
1. Operator configures: 20% off Chicken Parm, new customers, 7 days, $200 budget
2. Execution engine dispatches to Uber Eats adapter:
   - `POST /v1/delivery/stores/{sid}/promotion` with `{ "promo_type": "MENU_ITEM_DISCOUNT", "promotion_discount": { "menu_item_discount": { "item_discounts": [{ "item": { "item_external_id": "CHICKEN_PARM" }, "discount_amount": { "percent_discount": { "percent_value": 20 } } }] } }, "start_time": "...", "end_time": "...", "user_group": "FIRST_TIME_CUSTOMERS", "budget": { "periodic_budget": { "budget_amount": 20000, "budget_period": "WEEKLY" } } }`
3. Campaign ID stored, tracked in action history

### Layer 2: Orchestrated Recommendations

Recommendation engine analyzes data and suggests actions. Operator reviews and approves. PlateMate executes.

**Example — Underperforming dish detected:**
1. Recommendation engine (Python service) runs nightly:
   - Queries PlateMate reviews: Chicken Parm has 2.1 avg overall across 24 reviews, 19% reorder rate
   - Queries Google Reviews: NLP extracts 8 mentions of "chicken parm" — 6 negative ("dry," "overpriced," "disappointing")
   - Queries Toast Analytics: sales down 22% over 4 weeks
   - Queries R365: food cost is $5.40, margin is thin at current price
2. Algorithm generates: REMOVE recommendation, HIGH confidence
3. Evidence payload: "Overall rating 2.1/5 (24 reviews). Reorder rate 19%. Sales declining 22% over 4 weeks. Negative Google Review mentions: 'dry', 'overpriced'. Food cost $5.40 with declining margin."
4. Dashboard displays recommendation card
5. Operator clicks Accept → execution engine fires Layer 1 flow

**Example — Price adjustment opportunity:**
1. Recommendation engine detects: Lobster Roll scores 4.6 taste, 4.3 portion, but 2.1 value across 18 reviews
2. Cross-references R365: food cost is $8.20, current price $18, strong margin
3. Cross-references Toast: only 12 units/week despite high taste scores
4. Generates: ADJUST_PRICE recommendation, MEDIUM confidence
5. Evidence: "Customers love the taste (4.6) and portion (4.3) but rate value poorly (2.1). Consider reducing price to increase volume — current margin supports it."
6. Operator accepts → enters new price → execution engine updates across platforms

### Layer 3: Autonomous Monitoring

PlateMate continuously monitors data streams and proactively alerts operators. No action taken without approval, but the system surfaces issues the operator might miss.

**Example — Cross-channel sentiment divergence:**
1. Background job compares dish performance across channels:
   - Fish & Chips: 4.5 stars from dine-in reviews (Google) but 3.1 on delivery (Uber Eats feedback)
2. Alert surfaces on dashboard: "Fish & Chips performs well dine-in but poorly on delivery. Possible packaging or transit issue."
3. Operator investigates, decides what to do

**Example — Competitor intelligence:**
1. Periodic Places API Nearby Search (`POST /v1/places:searchNearby`) pulls review summaries for competing restaurants within 0.5 miles
2. NLP identifies dishes competitors are praised for
3. Alert: "3 nearby competitors have highly-rated burgers. Your burger has mixed sentiment (3.6 avg). Consider recipe improvement or promotional pricing."

**Example — Ingredient cost spike:**
1. R365 Transaction/TransactionDetail data shows chicken price spiked 20%
2. System identifies all dishes using chicken (via ingredient mapping)
3. Alert: "Chicken cost up 20%. Affected dishes: Chicken Parm ($1.08/unit impact), Chicken Caesar ($0.72 impact). Current margins and recommended price adjustments shown."

---

## 5. Restaurant Onboarding Flow

### Step 1: PlateMate Account Setup
- Restaurant signs up for PlateMate dashboard
- PlateMate scrapes their website to load initial menu data
- Restaurant reviews and confirms menu accuracy

### Step 2: Toast POS Connection
- Restaurant operator goes to Toast Web → Integrations → Toast API Access
- Creates custom integration credentials with scopes: `menus:read`, `stock:read`, `stock:write`, `orders:read`, `enterprise-metrics:read`
- Securely shares `client_id`, `client_secret`, and `restaurant_external_id` with PlateMate
- PlateMate pulls full menu via Menus V2 API, stores mappings
- Verified when menu appears correctly in dashboard's Unified Menu View

### Step 3: Uber Eats Connection
- PlateMate provides OAuth redirect: `https://auth.uber.com/oauth/v2/authorize?client_id=...&scope=eats.pos_provisioning`
- Restaurant logs in, authorizes PlateMate
- PlateMate discovers stores via `GET /v1/eats/stores`, provisions selected locations via `POST /v1/eats/stores/{store_id}/pos_data`
- Once provisioned, PlateMate's client_credentials token has perpetual access
- PlateMate pulls delivery menu, creates item ID mappings

### Step 4: Google Business Profile Connection
- Restaurant adds PlateMate's GBP Organization as a Manager on their Google Business Profile
- PlateMate discovers locations via `accounts.locations.list`
- Begins pulling all reviews via `accounts.locations.reviews.list`
- NLP pipeline processes review text, extracts dish mentions, tags sentiment

### Step 5: Restaurant365 Connection
- Restaurant contacts R365 Support to request API access for PlateMate (include PlateMate's technical contact on the email)
- R365 Support confirms with the restaurant and provides API credentials (username, password, customer subdomain)
- PlateMate authenticates via `POST /APIv1/Authenticate/JWT`
- Begins pulling sales data, inventory items, and transaction details via OData connector
- Maps R365 menu item names to PlateMate dish IDs

### Step 6: Verification
- Dashboard shows Unified Menu View with data from all connected platforms
- Operator confirms dish mappings are correct (especially cross-platform name matching)
- PlateMate performs a test action on a designated test dish to verify write access
- Platform connection status all shows 🟢 Connected

---

## 6. Data Model / Entity Resolution

### The Core Problem

The same dish exists differently on each platform:
- Toast: "Classic Chicken Parmesan" (GUID: `abc-123`)
- Uber Eats: "Chicken Parm" (item_id: `CHKN_PRM`)
- Google Reviews: "the chicken parm was dry" (unstructured text mention)
- R365: "Chicken Parmesan" (menuitem field in SalesDetail)
- PlateMate: "Chicken Parmesan" (scraped from restaurant website)

### Unified Dish Entity

```javascript
{
  _id: ObjectId,
  restaurant_id: ObjectId,
  canonical_name: "Chicken Parmesan",   // PlateMate's normalized name
  category: "Entrees",
  
  platforms: {
    toast: {
      guid: "abc-123",
      multi_location_id: "100000000171238879",
      menu_group: "Dinner Entrees",
      current_price: 16.00,
      stock_status: "IN_STOCK",
      last_synced: ISODate
    },
    uber_eats: {
      item_id: "CHKN_PRM",
      store_id: "store-uuid",
      current_price: 1999,        // cents
      active: true,
      last_synced: ISODate
    },
    r365: {
      menu_item_name: "Chicken Parmesan",  // exact string from SalesDetail
      ingredient_ids: ["item-001", "item-002", "item-003"],
      last_synced: ISODate
    },
    google: {
      mention_count: 47,
      avg_sentiment: 0.62,        // -1 to 1
      common_themes: ["generous portions", "great sauce", "too greasy"],
      last_analyzed: ISODate
    }
  },
  
  analytics: {
    platemate_review_count: 24,
    avg_overall: 4.2,
    avg_taste: 4.5,
    avg_portion: 4.0,
    avg_value: 3.8,
    reorder_rate: 0.78,
    trend_direction: "stable",     // "improving", "declining", "stable"
    health_score: 7.8,             // composite score
    last_calculated: ISODate
  },
  
  financials: {                     // from R365
    food_cost: 5.40,
    margin_percent: 0.66,
    weekly_units: 142,              // from Toast Analytics
    weekly_revenue: 2272.00,
    last_calculated: ISODate
  }
}
```

### Entity Resolution Pipeline

1. **Seed from scraper:** PlateMate's website scraper provides the canonical dish list and names.
2. **Toast matching:** Pull Toast menu, fuzzy-match item names to canonical names. Store Toast GUIDs. High-confidence matches (>90% similarity) are auto-mapped. Low-confidence matches flagged for operator review during onboarding.
3. **Uber Eats matching:** Same fuzzy-match process. Map Uber Eats item IDs. Name normalization handles common patterns ("Classic Caesar Salad" vs "Caesar Salad" vs "Caesar").
4. **R365 matching:** Match `menuitem` field from SalesDetail to canonical names. R365 names come from the POS, so they typically match Toast names closely.
5. **Google mentions:** NLP processes review text, extracts dish name references using the canonical name list as a dictionary. Fuzzy matching handles variations ("the caesar," "their caesar salad," "CS").
6. **Operator verification:** During onboarding, operator reviews the mapping and corrects any mismatches. This is a one-time step per restaurant, with incremental updates when menus change.

### Matching Algorithm

Fuzzy string matching with normalization:
- Lowercase, strip common prefixes ("Classic", "House", "Our Famous")
- Remove common suffixes ("Plate", "Platter", "Entrée")
- Levenshtein distance with threshold (>0.85 similarity = auto-match)
- Token-set matching for reordered words ("Grilled Chicken Caesar" ↔ "Caesar Salad, Grilled Chicken")
- Operator override for any match

---

## 7. Cross-Platform Capabilities

These are insights and actions uniquely possible because PlateMate sits across multiple data sources simultaneously.

### Cross-Channel Sentiment Comparison
Compare dine-in sentiment (Google Reviews) with delivery sentiment (Uber Eats feedback) and PlateMate's own structured ratings. Surfaces divergences — a dish that's great dine-in but terrible on delivery likely has a packaging or transit issue, not a recipe issue.

**Data sources:** Google Reviews (Business Profile API) + Uber Eats feedback (reporting export) + PlateMate reviews

### Margin-Aware Recommendations
Combine customer sentiment (PlateMate reviews) with financial data (R365 food costs, Toast sales revenue) to recommend actions that consider profitability, not just popularity. A dish with great reviews but terrible margins needs a different action than a dish with bad reviews and good margins.

**Data sources:** PlateMate reviews + R365 TransactionDetail + Toast Analytics API

### Sentiment-Sales Correlation
Identify disconnects between what customers think and what actually sells. A dish with strong positive sentiment but low sales volume may need better menu placement or marketing. A dish with high sales but declining sentiment may need recipe attention before it starts losing volume.

**Data sources:** PlateMate reviews + Google Reviews + Toast Analytics API (`POST /era/v1/menu/{timeRange}`)

### Competitive Intelligence
Use Places API Nearby Search to pull review summaries and ratings for competing restaurants. Identify dishes competitors are praised for that the connected restaurant doesn't offer, or dishes where the connected restaurant has a sentiment advantage.

**Data sources:** Google Places API (`POST /v1/places:searchNearby`, Place Details `reviewSummary`)

### Cross-Platform Price Intelligence
Show the same dish's price on every platform side-by-side. Combined with commission data (Uber Eats ~25%) and food cost (R365), calculate actual margin per platform per dish.

**Data sources:** Toast Menus V2 (dine-in price) + Uber Eats Menu API (delivery price) + R365 TransactionDetail (food cost)

### Review Response Intelligence
Monitor incoming Google reviews, flag negative reviews mentioning specific dishes, and draft responses for the restaurant manager. Connected to dish performance data — if a dish is already flagged for removal by the recommendation engine, the review response can acknowledge the feedback and note that changes are being made.

**Data sources:** Google Business Profile API (reviews + reply endpoint) + PlateMate recommendation engine

---

## 8. Key Design Principles

### Complementary, Not Competitive
PlateMate is positioned as a tool that helps restaurants sell more on their existing platforms (Toast, Uber Eats, etc.), not as a replacement for any of them. This avoids competitive tension with the platforms whose APIs we depend on. Uber Eats' licensing agreement explicitly permits showing Uber data in a merchant dashboard aggregated with other sales data.

### Merchant-Authorized Access
Every platform integration is authorized directly by the restaurant operator. PlateMate never scrapes, reverse-engineers, or bypasses platform access controls. Toast credentials are created by the operator. Uber Eats access goes through OAuth consent. Google Business Profile access requires the operator to add PlateMate as a Manager. R365 access requires the operator to request it from R365 Support.

### Data Separation
Platform data is used only for the purposes permitted by each platform's terms. Uber Eats data cannot be fed into the consumer recommendation engine (per Uber's licensing agreement Section 3.2.2). Customer personal data (names, phone numbers) is not exposed to other users or third parties. Each platform's data is stored with clear provenance and access controls.

### Platform-Agnostic Architecture
The adapter pattern ensures PlateMate is not dependent on any single platform. Adding a new POS (Square, Clover), delivery platform (Grubhub, DoorDash when API opens), or inventory system (MarketMan) is a new adapter implementing the same interface. The execution engine, dashboard, and recommendation engine never reference platform-specific logic.

### Action-Oriented, Not Just Analytical
Every piece of data surfaced in the dashboard is connected to an action the operator can take. Review scores aren't just numbers — they're evidence for a recommendation. Sales trends aren't just charts — they're triggers for pricing changes. The dashboard exists to drive decisions, not just display information.

### Operator Has Final Authority
All recommendations are suggestions. All automated actions require explicit operator approval (Accept). All automated changes can be rolled back within 24 hours. PlateMate provides intelligence and execution, not autonomy. This is critical for building trust during the early adoption phase.

---

## 9. Phasing / Roadmap

### Phase 1: Uber Eats + Google Reviews + Dashboard Polish

**Uber Eats:** Full adapter built against sandbox. Menu sync, remove dish, change price, run promo. All operations tested and demo-ready. Swap to production when approved.

**Google Reviews/Places:** Review ingestion pipeline live. All reviews pulled via Business Profile API. NLP dish-mention extraction running. External review data displayed in dashboard alongside PlateMate's own reviews.

**Dashboard:** UI overhauled to investor-demo quality. Unified Menu View, Action Center with all 3 action types, dish deep dive with multi-source data. Execution status tracker with SSE. Action history page.

### Phase 2: Toast

**Toast:** Full adapter built against restaurant's custom integration credentials. Menu sync, stock update (remove/restore), sales analytics ingestion. End-to-end tested with real restaurant data. First real agentic action executed on the restaurant's actual Toast POS.

**Dashboard enhancement:** Toast sales data (units sold, revenue, daypart analysis) integrated into dish deep dive. Cross-platform price comparison view. Stock status synced in real-time.

### Phase 3: Restaurant365

**R365:** Adapter built against restaurant's R365 credentials. Sales data, ingredient cost data, and vendor data flowing into PlateMate. Ingredient mapping to dishes established. Par level adjustment on dish removal.

**Dashboard enhancement:** Food cost per dish displayed alongside review ratings. Margin calculation (revenue from Toast - cost from R365). Margin-aware recommendations. Sentiment-sales-cost correlation views.

### Phase 4: Recommendation Engine Live

**CTO's Python service** integrated via API contract with Go backend. Restaurant-facing recommendations generated from combined data sources (PlateMate reviews + Google Reviews + Toast sales + R365 costs). Recommendation cards surfaced in dashboard. Accept → execute flow wired end-to-end. Shadow mode for validation before surfacing to operators.

### Beyond

- Additional restaurants (same adapters, new credentials)
- Additional platforms (DoorDash when API opens, Grubhub, Square, Clover — new adapters)
- Review response agent (draft Google review replies based on dish data)
- Autonomous monitoring alerts (sentiment shifts, cost spikes, competitor moves)
- Multi-location support for restaurant groups
- Middleware integration (Deliverect/KitchenHub) when restaurant count exceeds direct integration efficiency
