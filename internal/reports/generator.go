package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/viewred/viewred/internal/storage"
)

// GenerateReport creates a PDF report for the given time window using data
// extracted from the database. It saves the PDF to the specified output directory.
// Returns the absolute path of the generated file.
func GenerateReport(ctx context.Context, querier storage.Querier, since time.Time, until time.Time, testContext *storage.ReportHistory, outputDir string) (string, error) {
	if querier == nil {
		return "", fmt.Errorf("reports: querier is nil")
	}

	// 1. Fetch Dashboard Summary (Rates and Counts)
	summary, err := querier.QueryDashboardSummary(ctx, since)
	if err != nil {
		return "", fmt.Errorf("reports: failed to fetch dashboard summary: %w", err)
	}

	// 2. Fetch Top Talkers
	topTalkers, err := querier.QueryTopTalkers(ctx, since, 10)
	if err != nil {
		return "", fmt.Errorf("reports: failed to fetch top talkers: %w", err)
	}

	// 3. Fetch Protocol Distribution
	protocols, err := querier.QueryProtocolDistribution(ctx, since)
	if err != nil {
		return "", fmt.Errorf("reports: failed to fetch protocols: %w", err)
	}

	// 4. Fetch Alerts within the time window
	alerts, _, err := querier.QueryAlerts(ctx, storage.AlertFilter{Limit: 50, Offset: 0})
	if err != nil {
		alerts = []storage.Alert{}
	}

	var windowAlerts []storage.Alert
	for _, a := range alerts {
		if a.Time.After(since) && (a.Time.Before(until) || a.Time.Equal(until)) {
			windowAlerts = append(windowAlerts, a)
		}
	}
	if len(windowAlerts) > 10 {
		windowAlerts = windowAlerts[:10] // limit to 10 in the report
	}

	// 5. Fetch Traffic Timeline
	timeline, err := querier.QueryTrafficTimeline(ctx, since, until)
	if err != nil {
		timeline = []storage.TrafficPoint{} // fallback
	}

	topConvs, err := querier.QueryTopConversations(ctx, since, until, 10)
	if err != nil {
		topConvs = []storage.TopConversation{}
	}

	// 6.b Fetch Top Ports
	topPorts, err := querier.QueryTopPorts(ctx, since, 10)
	if err != nil {
		topPorts = []storage.TopPort{}
	}

	// 7. Fetch Devices
	devices, _, err := querier.QueryDevices(ctx, storage.DeviceFilter{Limit: 20})
	if err != nil {
		devices = []storage.Device{}
	}

	// 8. Fetch Vulnerabilities
	vulns, err := querier.GetVulnerabilities(ctx, false)
	if err != nil {
		vulns = []storage.Vulnerability{}
	}

	// 9. Generate PDF
	reportName := fmt.Sprintf("ViewRed_Report_%s_%s.pdf", 
		since.Format("20060102-150405"), 
		until.Format("20060102-150405"))
	
	filePath := fmt.Sprintf("%s/%s", outputDir, reportName)
	
	err = generatePDF(filePath, since, until, testContext, summary, topTalkers, protocols, topPorts, windowAlerts, timeline, topConvs, devices, vulns)
	if err != nil {
		return "", fmt.Errorf("reports: failed to generate PDF: %w", err)
	}

	return filePath, nil
}
