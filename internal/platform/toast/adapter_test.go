package toast

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adityapat24/platemate-agentic/internal/platform"
)

// ── Token tests ───────────────────────────────────────────────────────────────

func TestFetchToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/authentication/v1/authentication/login" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req tokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if req.UserAccessType != "TOAST_MACHINE_CLIENT" {
			t.Errorf("unexpected userAccessType: %s", req.UserAccessType)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse{
			Status: "SUCCESS",
			Token: struct {
				TokenType string `json:"tokenType"`
				Token     string `json:"token"`
				ExpiresIn int64  `json:"expiresIn"`
			}{
				TokenType: "Bearer",
				Token:     "test-token-abc",
				ExpiresIn: 86400,
			},
		})
	}))
	defer srv.Close()

	tm := NewTokenManager("client-id", "client-secret", srv.URL)
	tok, err := tm.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok != "test-token-abc" {
		t.Errorf("got token %q, want %q", tok, "test-token-abc")
	}
}

func TestFetchToken_Cached(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse{
			Token: struct {
				TokenType string `json:"tokenType"`
				Token     string `json:"token"`
				ExpiresIn int64  `json:"expiresIn"`
			}{Token: "tok", ExpiresIn: 3600},
		})
	}))
	defer srv.Close()

	tm := NewTokenManager("id", "secret", srv.URL)
	_, _ = tm.GetToken()
	_, _ = tm.GetToken()
	_, _ = tm.GetToken()

	if calls != 1 {
		t.Errorf("expected 1 token fetch, got %d", calls)
	}
}

func TestFetchToken_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer srv.Close()

	tm := NewTokenManager("bad", "creds", srv.URL)
	_, err := tm.GetToken()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── Menu types tests ──────────────────────────────────────────────────────────

func TestMenuItem_PriceCents(t *testing.T) {
	price := 12.99
	item := MenuItem{Price: &price}
	if got := item.PriceCents(); got != 1299 {
		t.Errorf("PriceCents() = %d, want 1299", got)
	}
}

func TestMenuItem_PriceCents_Nil(t *testing.T) {
	item := MenuItem{Price: nil}
	if got := item.PriceCents(); got != 0 {
		t.Errorf("PriceCents() = %d, want 0 for nil price", got)
	}
}

// ── Menu flatten tests ────────────────────────────────────────────────────────

func TestFlattenItems_Simple(t *testing.T) {
	price := 10.0
	menus := &MenusResponse{
		Menus: []Menu{
			{
				GUID: "menu-1",
				Name: "Lunch Menu",
				Groups: []MenuGroup{
					{
						GUID: "group-1",
						Name: "Entrees",
						Items: []MenuItem{
							{GUID: "item-1", Name: "Chicken Parm", Price: &price},
							{GUID: "item-2", Name: "Pasta", Price: &price},
						},
					},
				},
			},
		},
	}

	flat := FlattenItems(menus)
	if len(flat) != 2 {
		t.Fatalf("expected 2 items, got %d", len(flat))
	}
	if flat[0].GroupName != "Entrees" {
		t.Errorf("GroupName = %q, want Entrees", flat[0].GroupName)
	}
	if flat[0].MenuName != "Lunch Menu" {
		t.Errorf("MenuName = %q, want Lunch Menu", flat[0].MenuName)
	}
	if flat[0].Item.Name != "Chicken Parm" {
		t.Errorf("Item.Name = %q, want Chicken Parm", flat[0].Item.Name)
	}
}

func TestFlattenItems_Subgroups(t *testing.T) {
	price := 5.0
	menus := &MenusResponse{
		Menus: []Menu{
			{
				Name: "Dinner",
				Groups: []MenuGroup{
					{
						Name: "Drinks",
						Items: []MenuItem{
							{GUID: "d-1", Name: "Soda", Price: &price},
						},
						Subgroups: []MenuGroup{
							{
								Name: "Cocktails",
								Items: []MenuItem{
									{GUID: "d-2", Name: "Mojito", Price: &price},
								},
							},
						},
					},
				},
			},
		},
	}

	flat := FlattenItems(menus)
	if len(flat) != 2 {
		t.Fatalf("expected 2 items (parent + subgroup), got %d", len(flat))
	}
}

func TestFlattenItems_Empty(t *testing.T) {
	flat := FlattenItems(&MenusResponse{})
	if len(flat) != 0 {
		t.Errorf("expected 0 items, got %d", len(flat))
	}
}

// ── Stock client tests ────────────────────────────────────────────────────────

func TestGetInventory_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stock/v1/inventory" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Toast-Restaurant-External-ID") != "rest-guid-123" {
			t.Errorf("missing or wrong restaurant header: %q", r.Header.Get("Toast-Restaurant-External-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StockInventoryResponse{
			StockDataList: []StockItem{
				{MenuItemGUID: "item-1", StockStatus: "IN_STOCK"},
				{MenuItemGUID: "item-2", StockStatus: "OUT_OF_STOCK"},
			},
		})
	}))
	defer srv.Close()

	client := NewStockClient(newFakeTokenManager(srv.URL), srv.URL)
	inventory, err := client.GetInventory("rest-guid-123")
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if len(inventory) != 2 {
		t.Errorf("expected 2 items, got %d", len(inventory))
	}
	if inventory["item-1"].StockStatus != "IN_STOCK" {
		t.Errorf("expected IN_STOCK, got %s", inventory["item-1"].StockStatus)
	}
	if inventory["item-2"].StockStatus != "OUT_OF_STOCK" {
		t.Errorf("expected OUT_OF_STOCK, got %s", inventory["item-2"].StockStatus)
	}
}

// ── Menu client tests ─────────────────────────────────────────────────────────

func TestGetMenus_ParsesHierarchy(t *testing.T) {
	price := 15.99
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(MenusResponse{
			Menus: []Menu{
				{
					GUID: "menu-guid",
					Name: "All Day Menu",
					Groups: []MenuGroup{
						{
							GUID: "group-guid",
							Name: "Sandwiches",
							Items: []MenuItem{
								{
									GUID:                "item-guid",
									Name:                "BLT",
									MultiLocationID:     "mlid-001",
									Price:               &price,
									PricingStrategyType: "BASE",
								},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewMenuClient(newFakeTokenManager(srv.URL), srv.URL)
	resp, err := client.GetMenus("rest-guid")
	if err != nil {
		t.Fatalf("GetMenus: %v", err)
	}
	if len(resp.Menus) != 1 {
		t.Fatalf("expected 1 menu, got %d", len(resp.Menus))
	}
	item := resp.Menus[0].Groups[0].Items[0]
	if item.Name != "BLT" {
		t.Errorf("item name = %q, want BLT", item.Name)
	}
	if item.MultiLocationID != "mlid-001" {
		t.Errorf("multiLocationId = %q, want mlid-001", item.MultiLocationID)
	}
	if item.PriceCents() != 1599 {
		t.Errorf("PriceCents() = %d, want 1599", item.PriceCents())
	}
}

// ── Analytics client tests ────────────────────────────────────────────────────

func TestAnalyticsPolling_CompletesAfterRetry(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(AnalyticsAcceptedResponse{
				ReportRequestGUID: "report-guid-xyz",
			})
		case http.MethodGet:
			polls++
			w.Header().Set("Content-Type", "application/json")
			if polls < 2 {
				_ = json.NewEncoder(w).Encode(MenuAnalyticsReport{Status: "PENDING"})
				return
			}
			_ = json.NewEncoder(w).Encode(MenuAnalyticsReport{
				Status: "COMPLETED",
				Data: []MenuItemAnalytics{
					{MenuItemGUID: "item-1", QuantitySold: 42, NetSalesAmount: 629.58},
				},
			})
		}
	}))
	defer srv.Close()

	client := NewAnalyticsClient(newFakeTokenManager(srv.URL), srv.URL)
	report, err := client.GetMenuReport("rest-guid", "THIS_WEEK")
	if err != nil {
		t.Fatalf("GetMenuReport: %v", err)
	}
	if len(report.Data) != 1 {
		t.Fatalf("expected 1 data row, got %d", len(report.Data))
	}
	if report.Data[0].QuantitySold != 42 {
		t.Errorf("QuantitySold = %d, want 42", report.Data[0].QuantitySold)
	}
}

// ── Adapter write-gating tests ────────────────────────────────────────────────

func TestAdapter_WriteOperationsReturnErrWriteNotSupported(t *testing.T) {
	a := &Adapter{}
	ctx := context.Background()

	if _, err := a.RemoveDish(ctx, "dm-id"); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("RemoveDish: want ErrWriteNotSupported, got %v", err)
	}
	if _, err := a.RestoreDish(ctx, "dm-id"); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("RestoreDish: want ErrWriteNotSupported, got %v", err)
	}
	if _, err := a.ChangePrice(ctx, "dm-id", 1000); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("ChangePrice: want ErrWriteNotSupported, got %v", err)
	}
	if _, err := a.RunPromo(ctx, "dm-id", platform.PromoConfig{}); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("RunPromo: want ErrWriteNotSupported, got %v", err)
	}
	if _, err := a.RevokePromo(ctx, "promo-id"); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("RevokePromo: want ErrWriteNotSupported, got %v", err)
	}
	if _, err := a.Rollback(ctx, "action-id", nil); !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("Rollback: want ErrWriteNotSupported, got %v", err)
	}
}

func TestAdapter_CanAutomate_AlwaysFalse(t *testing.T) {
	a := &Adapter{}
	for _, act := range []platform.ActionType{
		platform.ActionRemoveDish,
		platform.ActionRestoreDish,
		platform.ActionChangePrice,
		platform.ActionRunPromo,
		platform.ActionRevokePromo,
	} {
		if a.CanAutomate(act) {
			t.Errorf("CanAutomate(%s) = true, want false", act)
		}
	}
}

func TestAdapter_Name(t *testing.T) {
	a := &Adapter{}
	if a.Name() != "toast" {
		t.Errorf("Name() = %q, want toast", a.Name())
	}
}

func TestAdapter_GetManualSteps_NotNil(t *testing.T) {
	a := &Adapter{}
	steps := a.GetManualSteps(platform.ActionRemoveDish, "dm-id", nil)
	if steps == nil {
		t.Fatal("GetManualSteps returned nil, want checklist")
	}
	if steps.Platform != "toast" {
		t.Errorf("Platform = %q, want toast", steps.Platform)
	}
	if len(steps.Steps) == 0 {
		t.Error("expected at least one manual step")
	}
}

// ── NormalizeItemName tests ───────────────────────────────────────────────────

func TestNormalizeItemName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Classic Chicken Parm", "chicken parm"},
		{"The BLT", "blt"},
		{"House Salad", "salad"},
		{"Steak Plate", "steak"},
	}
	for _, c := range cases {
		if got := NormalizeItemName(c.in); got != c.want {
			t.Errorf("NormalizeItemName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── Restaurant client tests ───────────────────────────────────────────────────

func TestGetRestaurant_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/restaurants/v1/restaurants/rest-guid-abc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RestaurantResponse{
			GUID:     "rest-guid-abc",
			Name:     "Test Bistro",
			Timezone: "America/New_York",
		})
	}))
	defer srv.Close()

	client := NewRestaurantClient(newFakeTokenManager(srv.URL), srv.URL)
	rest, err := client.GetRestaurant("rest-guid-abc")
	if err != nil {
		t.Fatalf("GetRestaurant: %v", err)
	}
	if rest.Name != "Test Bistro" {
		t.Errorf("Name = %q, want Test Bistro", rest.Name)
	}
	if rest.Timezone != "America/New_York" {
		t.Errorf("Timezone = %q, want America/New_York", rest.Timezone)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newFakeTokenManager returns a TokenManager pre-seeded with a valid cached token.
// Tests that don't care about the auth flow use this to bypass token fetching.
func newFakeTokenManager(serverURL string) *TokenManager {
	tm := NewTokenManager("test-id", "test-secret", serverURL)
	tm.cc = &cachedToken{
		value:     "fake-token",
		expiresAt: time.Now().Add(1 * time.Hour),
	}
	return tm
}
