package tool

import (
	"bob/internal/ai"
	"bob/internal/config"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const ToolADOSearchTickets ToolName = "ado_search_tickets"

var ADOSearchTicketsArgs = ai.NewSchema().
	AddInt("id", ai.Description("Work item ID for exact match")).
	AddString("title", ai.Description("Keywords to search in title (uses CONTAINS)")).
	AddString("state", ai.Description("Filter by state (e.g., 'New', 'Active', 'Resolved', 'Closed', 'Removed')")).
	AddString("assigned_to", ai.Description("Filter by assigned user (display name or email)")).
	AddString("work_item_type", ai.Description("Filter by work item type (e.g., 'Story', 'Tech Debt', 'Defect', 'Task', 'Bug')")).
	AddArray("tags", ai.FieldTypeString, ai.Description("Filter by tags (matches any of the provided tags)")).
	AddString("area_path", ai.Description("Filter by area path (e.g., 'Enterprise\\Cloud Native RMS')")).
	AddString("iteration_path", ai.Description("Filter by iteration path (e.g., 'Enterprise\\Sprint 42')")).
	AddString("created_by", ai.Description("Filter by creator (display name or email)")).
	AddString("severity", ai.Description("Filter by severity for defects (e.g., 'Critical', 'Urgent', 'High', 'Medium', 'Low')")).
	AddFloat("story_points", ai.Description("Filter by story points (exact match)")).
	AddString("qa_person", ai.Description("Filter by QA person assigned (display name or email)")).
	AddBool("emergent_defect", ai.Description("Filter emergent defects (true/false)")).
	AddInt("max_results", ai.Description("Maximum number of results to return (default 10, max 200)"))

// ADOSearchTickets searches for work items in Azure DevOps using WIQL
func ADOSearchTickets(context *core.ConversationContext, args map[string]any) (map[string]any, error) {
	logger.Debug("🔍 ADOSearchTickets: Starting comprehensive search")

	cfg := config.Current

	// Get max_results with default
	maxResults := 10
	if mr, ok := args["max_results"]; ok {
		switch v := mr.(type) {
		case float64:
			maxResults = int(v)
		case int:
			maxResults = v
		}
	}
	if maxResults > 200 {
		maxResults = 200
	}

	// Build WIQL query dynamically
	var whereClauses []string

	// Always filter by team project
	whereClauses = append(whereClauses, fmt.Sprintf("[System.TeamProject] = '%s'", cfg.ADOProject))

	// ID - exact match
	if id, ok := args["id"]; ok {
		switch v := id.(type) {
		case float64:
			whereClauses = append(whereClauses, fmt.Sprintf("[System.Id] = %d", int(v)))
		case int:
			whereClauses = append(whereClauses, fmt.Sprintf("[System.Id] = %d", v))
		}
	}

	// Title - contains search
	if title, ok := args["title"].(string); ok && title != "" {
		// Escape single quotes for WIQL
		escapedTitle := strings.ReplaceAll(title, "'", "''")
		whereClauses = append(whereClauses, fmt.Sprintf("[System.Title] CONTAINS '%s'", escapedTitle))
	}

	// State - exact match
	if state, ok := args["state"].(string); ok && state != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.State] = '%s'", state))
	}

	// Assigned To - exact match
	if assignedTo, ok := args["assigned_to"].(string); ok && assignedTo != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.AssignedTo] = '%s'", assignedTo))
	}

	// Work Item Type - exact match
	if workItemType, ok := args["work_item_type"].(string); ok && workItemType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.WorkItemType] = '%s'", workItemType))
	}

	// Tags - contains any
	if tags, ok := args["tags"].([]any); ok && len(tags) > 0 {
		tagClauses := []string{}
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				escapedTag := strings.ReplaceAll(tagStr, "'", "''")
				tagClauses = append(tagClauses, fmt.Sprintf("[System.Tags] CONTAINS '%s'", escapedTag))
			}
		}
		if len(tagClauses) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(tagClauses, " OR ")+")")
		}
	}

	// Area Path - exact match (could use UNDER for hierarchical, but exact for now)
	if areaPath, ok := args["area_path"].(string); ok && areaPath != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.AreaPath] = '%s'", areaPath))
	}

	// Iteration Path - exact match
	if iterationPath, ok := args["iteration_path"].(string); ok && iterationPath != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.IterationPath] = '%s'", iterationPath))
	}

	// Created By - exact match
	if createdBy, ok := args["created_by"].(string); ok && createdBy != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[System.CreatedBy] = '%s'", createdBy))
	}

	// Severity - exact match (defects only)
	if severity, ok := args["severity"].(string); ok && severity != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[Microsoft.VSTS.Common.Severity] = '%s'", severity))
	}

	// Story Points - exact match
	if storyPoints, ok := args["story_points"].(float64); ok {
		whereClauses = append(whereClauses, fmt.Sprintf("[Microsoft.VSTS.Scheduling.StoryPoints] = %f", storyPoints))
	}

	// QA Person - exact match
	if qaPerson, ok := args["qa_person"].(string); ok && qaPerson != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("[Custom.QAPerson] = '%s'", qaPerson))
	}

	// Emergent Defect - boolean
	if emergentDefect, ok := args["emergent_defect"].(bool); ok {
		whereClauses = append(whereClauses, fmt.Sprintf("[Custom.EmergentDefect] = %t", emergentDefect))
	}

	// Build final WIQL query
	wiqlQuery := fmt.Sprintf(`
		SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
		FROM workitems
		WHERE %s
		ORDER BY [System.ChangedDate] DESC
	`, strings.Join(whereClauses, " AND "))

	logger.Debugf("🔍 WIQL Query: %s", wiqlQuery)

	// Execute WIQL query
	wiqlURL := fmt.Sprintf("%s/%s/_apis/wit/wiql?api-version=7.1", cfg.ADOOrgURL, cfg.ADOProject)

	wiqlPayload := map[string]any{"query": wiqlQuery}
	jsonData, err := json.Marshal(wiqlPayload)
	if err != nil {
		logger.Errorf("❌ Failed to marshal WIQL query: %v", err)
		return nil, fmt.Errorf("failed to marshal WIQL query: %w", err)
	}

	req, err := http.NewRequest("POST", wiqlURL, strings.NewReader(string(jsonData)))
	if err != nil {
		logger.Errorf("❌ Failed to create WIQL request: %v", err)
		return nil, fmt.Errorf("failed to create WIQL request: %w", err)
	}

	req.SetBasicAuth("", cfg.ADOPAT)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("❌ WIQL request failed: %v", err)
		return nil, fmt.Errorf("WIQL request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("❌ Failed to read WIQL response: %v", err)
		return nil, fmt.Errorf("failed to read WIQL response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Errorf("❌ WIQL API error (status %d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("WIQL API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse WIQL response to get work item IDs
	var wiqlResult map[string]any
	if err := json.Unmarshal(body, &wiqlResult); err != nil {
		logger.Errorf("❌ Failed to parse WIQL response: %v", err)
		return nil, fmt.Errorf("failed to parse WIQL response: %w", err)
	}

	workItems := wiqlResult["workItems"].([]any)
	if len(workItems) == 0 {
		logger.Info("🔍 No work items found matching search criteria")
		return map[string]any{
			"success":      true,
			"count":        0,
			"work_items":   []any{},
			"message":      "No work items found matching search criteria",
		}, nil
	}

	// Extract work item IDs and limit to max_results
	workItemIDs := []string{}
	for i, item := range workItems {
		if i >= maxResults {
			break
		}
		itemMap := item.(map[string]any)
		id := int(itemMap["id"].(float64))
		workItemIDs = append(workItemIDs, fmt.Sprintf("%d", id))
	}

	logger.Debugf("🔍 Found %d work items, fetching details for first %d", len(workItems), len(workItemIDs))

	// Fetch full details for the work items
	idsParam := strings.Join(workItemIDs, ",")
	detailsURL := fmt.Sprintf("%s/%s/_apis/wit/workitems?ids=%s&api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject, idsParam)

	req2, err := http.NewRequest("GET", detailsURL, nil)
	if err != nil {
		logger.Errorf("❌ Failed to create details request: %v", err)
		return nil, fmt.Errorf("failed to create details request: %w", err)
	}

	req2.SetBasicAuth("", cfg.ADOPAT)

	resp2, err := client.Do(req2)
	if err != nil {
		logger.Errorf("❌ Details request failed: %v", err)
		return nil, fmt.Errorf("details request failed: %w", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		logger.Errorf("❌ Failed to read details response: %v", err)
		return nil, fmt.Errorf("failed to read details response: %w", err)
	}

	if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		logger.Errorf("❌ Details API error (status %d): %s", resp2.StatusCode, string(body2))
		return nil, fmt.Errorf("details API error (status %d): %s", resp2.StatusCode, string(body2))
	}

	// Parse details response
	var detailsResult map[string]any
	if err := json.Unmarshal(body2, &detailsResult); err != nil {
		logger.Errorf("❌ Failed to parse details response: %v", err)
		return nil, fmt.Errorf("failed to parse details response: %w", err)
	}

	// Extract and format results
	results := []map[string]any{}
	for _, item := range detailsResult["value"].([]any) {
		itemMap := item.(map[string]any)
		fields := itemMap["fields"].(map[string]any)
		workItemID := int(itemMap["id"].(float64))

		// Helper to safely extract string
		getString := func(field string) string {
			if val, ok := fields[field]; ok && val != nil {
				if str, ok := val.(string); ok {
					return str
				}
			}
			return ""
		}

		// Helper to safely extract user display name
		getUserName := func(field string, defaultName string) string {
			if val, ok := fields[field]; ok && val != nil {
				if userMap, ok := val.(map[string]any); ok {
					if displayName, ok := userMap["displayName"].(string); ok {
						return displayName
					}
				}
			}
			return defaultName
		}

		workItemURL := fmt.Sprintf("%s/%s/_workitems/edit/%d", cfg.ADOOrgURL, cfg.ADOProject, workItemID)

		// Extract tags (semicolon-separated string to array)
		tagsStr := getString("System.Tags")
		tags := []string{}
		if tagsStr != "" {
			for _, tag := range strings.Split(tagsStr, ";") {
				tags = append(tags, strings.TrimSpace(tag))
			}
		}

		// Story-specific fields
		storyPoints := 0.0
		if spVal, ok := fields["Microsoft.VSTS.Scheduling.StoryPoints"]; ok && spVal != nil {
			if spFloat, ok := spVal.(float64); ok {
				storyPoints = spFloat
			}
		}

		// Defect-specific fields
		severity := getString("Microsoft.VSTS.Common.Severity")
		reproSteps := getString("Microsoft.VSTS.TCM.ReproSteps")
		expectedResult := getString("Custom.ExpectedResult")
		actualResult := getString("Custom.ActualResult")
		caseNumber := getString("Custom.CaseNumber")
		emergentDefect := false
		if edVal, ok := fields["Custom.EmergentDefect"]; ok && edVal != nil {
			if edBool, ok := edVal.(bool); ok {
				emergentDefect = edBool
			}
		}

		workItem := map[string]any{
			"id":                  workItemID,
			"url":                 workItemURL,
			"work_item_type":      getString("System.WorkItemType"),
			"title":               getString("System.Title"),
			"state":               getString("System.State"),
			"assigned_to":         getUserName("System.AssignedTo", "Unassigned"),
			"created_by":          getUserName("System.CreatedBy", "Unknown"),
			"changed_by":          getUserName("System.ChangedBy", "Unknown"),
			"created_date":        getString("System.CreatedDate"),
			"changed_date":        getString("System.ChangedDate"),
			"description":         getString("System.Description"),
			"area_path":           getString("System.AreaPath"),
			"iteration_path":      getString("System.IterationPath"),
			"acceptance_criteria": getString("Microsoft.VSTS.Common.AcceptanceCriteria"),
			"test_requirements":   getString("POR.TestRequirements"),
			"tags":                tags,
		}

		// Add story-specific fields if present
		if storyPoints > 0 {
			workItem["story_points"] = storyPoints
		}

		// Add defect-specific fields if present
		if severity != "" {
			workItem["severity"] = severity
		}
		if reproSteps != "" {
			workItem["repro_steps"] = reproSteps
		}
		if expectedResult != "" {
			workItem["expected_result"] = expectedResult
		}
		if actualResult != "" {
			workItem["actual_result"] = actualResult
		}
		if caseNumber != "" {
			workItem["case_number"] = caseNumber
		}
		if emergentDefect {
			workItem["emergent_defect"] = emergentDefect
		}

		results = append(results, workItem)
	}

	logger.Infof("✅ Found %d work items matching search criteria", len(results))

	return map[string]any{
		"success":    true,
		"count":      len(results),
		"work_items": results,
		"message":    fmt.Sprintf("Found %d work items", len(results)),
	}, nil
}
