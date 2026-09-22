package toast

// ── Restaurant types ─────────────────────────────────────────────────────────

type RestaurantResponse struct {
	GUID        string             `json:"guid"`
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Timezone    string             `json:"timezone"`
	Location    RestaurantLocation `json:"location"`
	Hours       []SchedulePeriod   `json:"hours,omitempty"`
	Status      string             `json:"status,omitempty"` // "ACTIVE", "INACTIVE"
}

type RestaurantLocation struct {
	Address string  `json:"address"`
	City    string  `json:"city"`
	State   string  `json:"state"`
	Zip     string  `json:"zip"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Long    float64 `json:"long"`
}

type SchedulePeriod struct {
	DayOfWeek string     `json:"dayOfWeek"`
	TimePeriods []HourRange `json:"timePeriods"`
}

type HourRange struct {
	Start string `json:"start"` // "HH:MM:SS"
	End   string `json:"end"`
}

// ── Menu V2 types ────────────────────────────────────────────────────────────
//
// Toast Menus V2 returns a hierarchy: Menu → MenuGroup → MenuItem.
// Prices are resolved per pricingStrategyType; null is valid for SIZE_PRICE
// and OPEN_PRICE items. multiLocationId is the stable cross-location ID.

type MenusResponse struct {
	Menus []Menu `json:"menus"`
}

type MenuMetadataResponse struct {
	LastUpdated string `json:"lastUpdated"` // ISO 8601
	MenuCount   int    `json:"menuCount"`
}

type Menu struct {
	GUID            string      `json:"guid"`
	Name            string      `json:"name"`
	Description     string      `json:"description,omitempty"`
	MultiLocationID string      `json:"multiLocationId,omitempty"`
	Groups          []MenuGroup `json:"groups"`
}

type MenuGroup struct {
	GUID            string      `json:"guid"`
	Name            string      `json:"name"`
	Description     string      `json:"description,omitempty"`
	MultiLocationID string      `json:"multiLocationId,omitempty"`
	Items           []MenuItem  `json:"items"`
	Subgroups       []MenuGroup `json:"subgroups,omitempty"`
}

type MenuItem struct {
	GUID                string             `json:"guid"`
	Name                string             `json:"name"`
	Description         string             `json:"description,omitempty"`
	MultiLocationID     string             `json:"multiLocationId,omitempty"`
	SKU                 string             `json:"sku,omitempty"`
	Price               *float64           `json:"price"` // nullable for SIZE_PRICE / OPEN_PRICE
	PricingStrategyType string             `json:"pricingStrategyType"` // BASE | MENU_SPECIFIC | TIME_SPECIFIC | SIZE_PRICE | OPEN_PRICE
	Calories            *int               `json:"calories,omitempty"`
	UnitOfMeasure       string             `json:"unitOfMeasure,omitempty"` // NONE | LB | OZ
	ItemTags            []ItemTag          `json:"itemTags,omitempty"`
	ModifierGroups      []ModifierGroupRef `json:"modifierGroups,omitempty"`
	PrepStations        []PrepStationRef   `json:"prepStations,omitempty"`
	ContentAdvisories   []string           `json:"contentAdvisories,omitempty"`
	IsDiscounted        bool               `json:"isDiscounted,omitempty"`
}

// PriceCents converts a Toast float price (dollars) to integer cents.
// Returns 0 for null-price items (SIZE_PRICE, OPEN_PRICE).
func (m *MenuItem) PriceCents() int64 {
	if m.Price == nil {
		return 0
	}
	return int64(*m.Price * 100)
}

type ItemTag struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

type ModifierGroupRef struct {
	GUID string `json:"guid"`
}

type PrepStationRef struct {
	GUID string `json:"guid"`
}

// ── Stock types ──────────────────────────────────────────────────────────────

type StockInventoryResponse struct {
	StockDataList []StockItem `json:"stockDataList"`
}

type StockItem struct {
	MenuItemGUID string   `json:"menuItemGuid"`
	Quantity     *float64 `json:"quantity,omitempty"`
	StockStatus  string   `json:"stockStatus"` // "IN_STOCK" | "OUT_OF_STOCK"
}

// ── Analytics types ──────────────────────────────────────────────────────────
//
// Toast Analytics API uses an async pattern:
//   POST /era/v1/menu/{timeRange}  → 202 with reportRequestGuid
//   GET  /era/v1/menu/{reportRequestGuid} → 200 with data (or 202 still pending)

type AnalyticsReportRequest struct {
	RestaurantGUIDs []string      `json:"restaurantGuids"`
	DateRange       *DateRange    `json:"dateRange,omitempty"`
	GroupBy         []string      `json:"groupBy"` // ["MENU_ITEM"]
}

type DateRange struct {
	Start string `json:"start"` // ISO 8601 date
	End   string `json:"end"`
}

// AnalyticsAcceptedResponse is the 202 body from POST /era/v1/menu/{timeRange}.
type AnalyticsAcceptedResponse struct {
	ReportRequestGUID string `json:"reportRequestGuid"`
}

// MenuAnalyticsReport is the final result from GET /era/v1/menu/{guid}.
// The status field is "COMPLETED" when data is ready.
type MenuAnalyticsReport struct {
	Status string               `json:"status"` // "PENDING" | "COMPLETED"
	Data   []MenuItemAnalytics  `json:"data"`
}

type MenuItemAnalytics struct {
	MenuItemGUID     string  `json:"menuItemGuid"`
	NetSalesAmount   float64 `json:"netSalesAmount"`
	GrossSalesAmount float64 `json:"grossSalesAmount"`
	QuantitySold     int     `json:"quantitySold"`
	AveragePrice     float64 `json:"averagePrice"`
	WasteCount       int     `json:"wasteCount"`
}
