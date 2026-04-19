package ubereats

// ── Store types ──────────────────────────────────────────────────────────────

type StoreListResponse struct {
	Stores     []Store `json:"stores"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Store struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Location struct {
		Address    string  `json:"address"`
		City       string  `json:"city"`
		PostalCode string  `json:"postal_code"`
		Country    string  `json:"country"`
		Lat        float64 `json:"lat"`
		Lng        float64 `json:"lng"`
	} `json:"location"`
	Orderability struct {
		Status         string `json:"status"`
		OfflineReason  string `json:"offline_reason,omitempty"`
		NextOpenTime   string `json:"next_open_time,omitempty"`
		NextCloseTime  string `json:"next_close_time,omitempty"`
	} `json:"orderability,omitempty"`
}

type StoreStatusResponse struct {
	Status          string `json:"status"`
	OfflineReason   string `json:"offline_reason,omitempty"`
	IsOfflineUntil  string `json:"is_offline_until,omitempty"`
}

type UpdateStoreStatusRequest struct {
	Status        string `json:"status"`                   // "ONLINE" | "OFFLINE"
	Reason        string `json:"reason,omitempty"`
	IsOfflineUntil string `json:"is_offline_until,omitempty"` // RFC3339
}

type UpdatePrepTimeRequest struct {
	DefaultPrepTime int              `json:"default_prep_time,omitempty"` // seconds
	DelayConfig     *DelayConfig     `json:"delay_config,omitempty"`
}

type DelayConfig struct {
	DelayUntil    string `json:"delay_until"`    // RFC3339
	DelayDuration int    `json:"delay_duration"` // seconds, max 3600
}

// ── Menu types ───────────────────────────────────────────────────────────────

type MenuConfiguration struct {
	Menus          []Menu          `json:"menus"`
	Categories     []Category      `json:"categories"`
	Items          []MenuItem      `json:"items"`
	ModifierGroups []ModifierGroup `json:"modifier_groups"`
}

type Menu struct {
	ID                  string   `json:"id"`
	Title               MultiLanguageText `json:"title"`
	CategoryIDs         []string `json:"category_ids"`
	ServiceAvailability []ServiceAvailability `json:"service_availability,omitempty"`
}

type Category struct {
	ID       string            `json:"id"`
	Title    MultiLanguageText `json:"title"`
	Entities []CategoryEntity  `json:"entities"`
}

type CategoryEntity struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "ITEM"
}

type MenuItem struct {
	ID               string            `json:"id"`
	Title            MultiLanguageText `json:"title"`
	Description      MultiLanguageText `json:"description,omitempty"`
	ImageURL         string            `json:"image_url,omitempty"`
	PriceInfo        PriceInfo         `json:"price_info"`
	SuspensionInfo   *SuspensionInfo   `json:"suspension_info,omitempty"`
	ModifierGroupIDs struct {
		IDs []string `json:"ids"`
	} `json:"modifier_group_ids,omitempty"`
	TaxInfo      *TaxInfo      `json:"tax_info,omitempty"`
	DishInfo     *DishInfo     `json:"dish_info,omitempty"`
	ExternalData string        `json:"external_data,omitempty"`
}

type MultiLanguageText struct {
	Translations []Translation `json:"translations"`
}

type Translation struct {
	Locale      string `json:"locale"`
	Translation string `json:"translation"`
}

// TitleString extracts the English (or first available) title string.
func (m MultiLanguageText) TitleString() string {
	for _, t := range m.Translations {
		if t.Locale == "en" || t.Locale == "en_US" {
			return t.Translation
		}
	}
	if len(m.Translations) > 0 {
		return m.Translations[0].Translation
	}
	return ""
}

type PriceInfo struct {
	Price     int64          `json:"price"`      // cents
	CorePrice int64          `json:"core_price,omitempty"`
	Overrides []PriceOverride `json:"overrides,omitempty"`
}

type PriceOverride struct {
	ContextType  string `json:"context_type"`
	ContextValue string `json:"context_value"`
	Price        int64  `json:"price"`
}

type SuspensionInfo struct {
	Suspension *Suspension `json:"suspension"`
}

type Suspension struct {
	SuspendUntil int64  `json:"suspend_until,omitempty"` // Unix timestamp
	Reason       string `json:"reason,omitempty"`
}

type TaxInfo struct {
	TaxRate           float64 `json:"tax_rate,omitempty"`
	VatRatePercentage float64 `json:"vat_rate_percentage,omitempty"`
}

type DishInfo struct {
	Classifications struct {
		CanServeAlone bool   `json:"can_serve_alone,omitempty"`
		Alcoholic     int    `json:"alcoholic_items,omitempty"`
		DietaryLabels []string `json:"dietary_labels,omitempty"`
	} `json:"classifications,omitempty"`
}

type ModifierGroup struct {
	ID              string            `json:"id"`
	Title           MultiLanguageText `json:"title"`
	ModifierOptions []ModifierOption  `json:"modifier_options"`
}

type ModifierOption struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "ITEM"
	Quantity struct {
		Min int `json:"min_permitted"`
		Max int `json:"max_permitted"`
	} `json:"quantity_info,omitempty"`
}

type ServiceAvailability struct {
	DayOfWeek string `json:"day_of_week"`
	TimeSlots []struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	} `json:"time_slots"`
}

// ItemUpdateRequest is a sparse update — only set the fields you want to change.
type ItemUpdateRequest struct {
	PriceInfo      *PriceInfo      `json:"price_info,omitempty"`
	SuspensionInfo *SuspensionInfo `json:"suspension_info,omitempty"`
	Title          *MultiLanguageText `json:"title,omitempty"`
	Description    *MultiLanguageText `json:"description,omitempty"`
}

// ── Promotions types ─────────────────────────────────────────────────────────

type CreatePromotionRequest struct {
	StartTime           string              `json:"start_time"`            // RFC3339
	EndTime             string              `json:"end_time"`              // RFC3339
	ExternalPromotionID string              `json:"external_promotion_id,omitempty"`
	UserGroup           string              `json:"user_group"`            // "ALL_CUSTOMERS" | "FIRST_TIME_CUSTOMERS"
	AllowUnlimitedApply bool                `json:"allow_unlimited_apply"`
	CurrencyCode        string              `json:"currency_code"`
	Budget              PromoBudget         `json:"budget"`
	PromoType           string              `json:"promo_type"`
	PromotionDiscount   PromotionDiscount   `json:"promotion_discount"`
}

type PromoBudget struct {
	UnlimitedBudget bool              `json:"unlimited_budget,omitempty"`
	PeriodicBudget  *PeriodicBudget   `json:"periodic_budget,omitempty"`
}

type PeriodicBudget struct {
	BudgetAmount int64  `json:"budget_amount"` // smallest currency unit
	BudgetPeriod string `json:"budget_period"` // "WEEKLY"
}

type PromotionDiscount struct {
	FlatOffDiscount        *FlatOffDiscount        `json:"flat_off_discount,omitempty"`
	PercentOffDiscount     *PercentOffDiscount     `json:"percent_off_discount,omitempty"`
	BogoDiscount           *BogoDiscount           `json:"bogo_discount,omitempty"`
	FreeItemDiscount       *FreeItemDiscount       `json:"free_item_discount,omitempty"`
	MenuItemDiscount       *MenuItemDiscount       `json:"menu_item_discount,omitempty"`
	FreeDeliveryDiscount   *FreeDeliveryDiscount   `json:"free_delivery_discount,omitempty"`
}

type FlatOffDiscount struct {
	MinBasketConstraint *MinBasketConstraint `json:"min_basket_constraint,omitempty"`
	DiscountValue       MoneyAmount          `json:"discount_value"`
}

type PercentOffDiscount struct {
	PercentValue        int                  `json:"percent_value"`
	MaxDiscountValue    *MoneyAmount         `json:"max_discount_value,omitempty"`
	MinBasketConstraint *MinBasketConstraint `json:"min_basket_constraint,omitempty"`
}

type BogoDiscount struct {
	TargetItems []PromoItemRef `json:"target_items"`
}

type FreeItemDiscount struct {
	FreeItems           []PromoItemRef       `json:"free_items"`
	MinBasketConstraint *MinBasketConstraint `json:"min_basket_constraint,omitempty"`
}

type MenuItemDiscount struct {
	ItemDiscounts []ItemDiscount `json:"item_discounts"`
}

type ItemDiscount struct {
	Item           PromoItemRef    `json:"item"`
	DiscountAmount ItemDiscountAmt `json:"discount_amount"`
}

type ItemDiscountAmt struct {
	PercentDiscount *PercentDiscount `json:"percent_discount,omitempty"`
	FlatDiscount    *MoneyAmount     `json:"flat_discount,omitempty"`
}

type PercentDiscount struct {
	PercentValue int `json:"percent_value"`
}

type FreeDeliveryDiscount struct {
	MaxDiscountValue    *MoneyAmount         `json:"max_discount_value,omitempty"`
	MinBasketConstraint *MinBasketConstraint `json:"min_basket_constraint,omitempty"`
}

type PromoItemRef struct {
	ItemExternalID string `json:"item_external_id"`
}

type MinBasketConstraint struct {
	MinSpend MoneyAmount `json:"min_spend"`
}

type MoneyAmount struct {
	Amount       int64  `json:"amount"`       // smallest currency unit
	CurrencyCode string `json:"currency_code"`
}

type PromotionResponse struct {
	PromotionID         string `json:"promotion_id"`
	ExternalPromotionID string `json:"external_promotion_id,omitempty"`
	State               string `json:"state"`
	PromoType           string `json:"promo_type"`
	StartTime           string `json:"start_time"`
	EndTime             string `json:"end_time"`
}

type PromotionListResponse struct {
	Promotions []PromotionResponse `json:"promotions"`
}

// ── Integration / provisioning types ────────────────────────────────────────

type PosProvisionRequest struct {
	WebhookURL    string `json:"webhook_url,omitempty"`
	POSProviderID string `json:"pos_provider_id,omitempty"`
}

type StoresResponse struct {
	Stores []Store `json:"stores"`
}

// ── Webhook event ────────────────────────────────────────────────────────────

type WebhookEvent struct {
	EventType string                 `json:"event_type"`
	EventID   string                 `json:"event_id"`
	Meta      map[string]interface{} `json:"meta"`
}
