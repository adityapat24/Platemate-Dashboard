package toast

import (
	"fmt"
	"net/http"
)

// RestaurantClient wraps the Toast Restaurants API.
type RestaurantClient struct {
	baseClient
}

func NewRestaurantClient(tm *TokenManager, apiBase string) *RestaurantClient {
	return &RestaurantClient{newBaseClient(tm, apiBase)}
}

// GetRestaurant fetches the profile, hours, and timezone for a restaurant.
// restaurantGUID is the Toast external GUID (Toast-Restaurant-External-ID).
func (c *RestaurantClient) GetRestaurant(restaurantGUID string) (*RestaurantResponse, error) {
	path := fmt.Sprintf("/restaurants/v1/restaurants/%s", restaurantGUID)
	var resp RestaurantResponse
	if err := c.do(http.MethodGet, path, restaurantGUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("get restaurant %s: %w", restaurantGUID, err)
	}
	return &resp, nil
}
