package toast

import (
	"fmt"
	"net/http"
)

// MenuClient wraps the Toast Menus V2 API.
// Always use V2 for analytics-compatible data; V3 is for ordering integrations only.
type MenuClient struct {
	baseClient
}

func NewMenuClient(tm *TokenManager, apiBase string) *MenuClient {
	return &MenuClient{newBaseClient(tm, apiBase)}
}

// GetMenus fetches the full menu hierarchy for a restaurant.
// Returns Menus → MenuGroups → MenuItems with pricing, modifiers, item tags, prep stations.
func (c *MenuClient) GetMenus(restaurantGUID string) (*MenusResponse, error) {
	var resp MenusResponse
	if err := c.do(http.MethodGet, "/menus/v2/menus", restaurantGUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("get menus for %s: %w", restaurantGUID, err)
	}
	return &resp, nil
}

// GetMetadata fetches menu freshness information (last update time, menu count).
// Use this for lightweight freshness checks before deciding whether to do a full sync.
func (c *MenuClient) GetMetadata(restaurantGUID string) (*MenuMetadataResponse, error) {
	var resp MenuMetadataResponse
	if err := c.do(http.MethodGet, "/menus/v2/metadata", restaurantGUID, nil, &resp); err != nil {
		return nil, fmt.Errorf("get menu metadata for %s: %w", restaurantGUID, err)
	}
	return &resp, nil
}

// FlattenItems walks the full menu hierarchy and returns every MenuItem with its
// menu name, group name, and the item itself. Useful for building the flat list
// that SyncMenu needs without duplicating traversal logic.
func FlattenItems(menus *MenusResponse) []FlatMenuItem {
	var out []FlatMenuItem
	for _, menu := range menus.Menus {
		flattenGroup(menu.Name, menu.Groups, &out)
	}
	return out
}

type FlatMenuItem struct {
	MenuName  string
	GroupName string
	Item      MenuItem
}

func flattenGroup(menuName string, groups []MenuGroup, out *[]FlatMenuItem) {
	for _, g := range groups {
		for _, item := range g.Items {
			*out = append(*out, FlatMenuItem{
				MenuName:  menuName,
				GroupName: g.Name,
				Item:      item,
			})
		}
		// Recurse into subgroups.
		flattenGroup(menuName, g.Subgroups, out)
	}
}
