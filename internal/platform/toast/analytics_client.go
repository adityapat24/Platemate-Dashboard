package toast

import (
	"fmt"
	"net/http"
	"time"
)

const (
	analyticsMaxWait     = 30 * time.Second
	analyticsInitialWait = 500 * time.Millisecond
	analyticsMaxBackoff  = 8 * time.Second
)

// AnalyticsClient wraps the Toast Enterprise Reporting API (ERA).
//
// The ERA uses an async pattern:
//  1. POST /era/v1/menu/{timeRange}  → 202 with reportRequestGuid
//  2. GET  /era/v1/menu/{reportRequestGuid} → poll until status != "PENDING"
type AnalyticsClient struct {
	baseClient
}

func NewAnalyticsClient(tm *TokenManager, apiBase string) *AnalyticsClient {
	return &AnalyticsClient{newBaseClient(tm, apiBase)}
}

// GetMenuReport submits a menu analytics report request and polls until complete
// or until the 30-second deadline is exceeded (exponential backoff).
//
// timeRange is a preset key such as "THIS_WEEK" or "LAST_WEEK".
// restaurantGUID is the Toast external GUID for the restaurant.
func (c *AnalyticsClient) GetMenuReport(restaurantGUID, timeRange string) (*MenuAnalyticsReport, error) {
	guid, err := c.submitReport(restaurantGUID, timeRange)
	if err != nil {
		return nil, err
	}
	return c.pollReport(restaurantGUID, guid)
}

// submitReport fires the async POST and returns the reportRequestGuid.
func (c *AnalyticsClient) submitReport(restaurantGUID, timeRange string) (string, error) {
	path := fmt.Sprintf("/era/v1/menu/%s", timeRange)

	body := AnalyticsReportRequest{
		RestaurantGUIDs: []string{restaurantGUID},
		GroupBy:         []string{"MENU_ITEM"},
	}

	// The accepted response body is returned via the out param when do() sees a 202.
	var accepted AnalyticsAcceptedResponse
	err := c.do(http.MethodPost, path, restaurantGUID, body, &accepted)
	if err != nil && !isAccepted(err) {
		return "", fmt.Errorf("submit analytics report: %w", err)
	}
	if accepted.ReportRequestGUID == "" {
		return "", fmt.Errorf("submit analytics report: missing reportRequestGuid in 202 response")
	}
	return accepted.ReportRequestGUID, nil
}

// pollReport polls GET /era/v1/menu/{guid} with exponential backoff until the
// report status is no longer "PENDING" or the 30s deadline is hit.
func (c *AnalyticsClient) pollReport(restaurantGUID, reportGUID string) (*MenuAnalyticsReport, error) {
	path := fmt.Sprintf("/era/v1/menu/%s", reportGUID)
	deadline := time.Now().Add(analyticsMaxWait)
	wait := analyticsInitialWait

	for time.Now().Before(deadline) {
		var report MenuAnalyticsReport
		err := c.do(http.MethodGet, path, restaurantGUID, nil, &report)

		if err == nil {
			// 200 OK — report is ready (status may still say "PENDING" in edge cases).
			if report.Status == "PENDING" {
				// Treat as still processing; fall through to sleep + retry.
			} else {
				return &report, nil
			}
		} else if isAccepted(err) {
			// 202 — still processing; fall through to sleep + retry.
		} else {
			return nil, fmt.Errorf("poll analytics report %s: %w", reportGUID, err)
		}

		time.Sleep(wait)
		wait *= 2
		if wait > analyticsMaxBackoff {
			wait = analyticsMaxBackoff
		}
	}

	return nil, fmt.Errorf("analytics report %s did not complete within %s", reportGUID, analyticsMaxWait)
}
