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

const ToolADOGetTicket ToolName = "ado_get_ticket"

var ADOGetTicketArgs = ai.NewSchema().
	AddInt("work_item_id", ai.Required(),
		ai.Description("The ID of the work item to retrieve (e.g., 12345)"))

// ADOGetTicket retrieves a work item from Azure DevOps by ID
func ADOGetTicket(context *core.ConversationContext, args map[string]any) (map[string]any, error) {
	logger.Debug("🔍 ADOGetTicket: Starting")

	// Extract work item ID
	var workItemID int
	switch v := args["work_item_id"].(type) {
	case float64:
		workItemID = int(v)
	case int:
		workItemID = v
	default:
		return nil, fmt.Errorf("work_item_id parameter missing or invalid type: %T", v)
	}

	logger.Debugf("🔍 Fetching work item #%d", workItemID)

	// Make HTTP request
	cfg := config.Current
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/%d?$expand=all&api-version=7.1-preview.3",
		cfg.ADOOrgURL, cfg.ADOProject, workItemID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Errorf("❌ Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// ADO uses HTTP Basic Auth with empty username and PAT as password
	req.SetBasicAuth("", cfg.ADOPAT)

	logger.Debugf("🌐 Making ADO API request to: %s", url)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("❌ HTTP request failed: %v", err)
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("❌ Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Errorf("❌ ADO API error (status %d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("ADO API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Errorf("❌ Failed to parse response: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract fields
	fields := result["fields"].(map[string]any)
	links := result["_links"].(map[string]any)
	htmlLink := links["html"].(map[string]any)
	workItemURL := htmlLink["href"].(string)

	// Extract tags (semicolon-separated string to array)
	tagsStr := ""
	if tagsVal, ok := fields["System.Tags"]; ok && tagsVal != nil {
		tagsStr = tagsVal.(string)
	}
	tags := []string{}
	if tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ";") {
			tags = append(tags, strings.TrimSpace(tag))
		}
	}

	// Helper to safely extract string from field
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

	workItemType := getString("System.WorkItemType")
	title := getString("System.Title")
	state := getString("System.State")
	assignedTo := getUserName("System.AssignedTo", "Unassigned")
	createdBy := getUserName("System.CreatedBy", "Unknown")
	changedBy := getUserName("System.ChangedBy", "Unknown")
	description := getString("System.Description")
	acceptanceCriteria := getString("Microsoft.VSTS.Common.AcceptanceCriteria")
	testRequirements := getString("POR.TestRequirements")
	createdDate := getString("System.CreatedDate")
	changedDate := getString("System.ChangedDate")
	areaPath := getString("System.AreaPath")
	iterationPath := getString("System.IterationPath")

	// Handle QA Person (could be string or object)
	qaPerson := "Unassigned"
	if qaVal, ok := fields["Custom.QAPerson"]; ok && qaVal != nil {
		if qaMap, ok := qaVal.(map[string]any); ok {
			if displayName, ok := qaMap["displayName"].(string); ok {
				qaPerson = displayName
			}
		} else if qaStr, ok := qaVal.(string); ok {
			qaPerson = qaStr
		}
	}

	logger.Infof("✅ Retrieved work item #%d: %s - %s", workItemID, workItemType, title)

	return map[string]any{
		"success":             true,
		"id":                  workItemID,
		"url":                 workItemURL,
		"work_item_type":      workItemType,
		"title":               title,
		"state":               state,
		"assigned_to":         assignedTo,
		"created_by":          createdBy,
		"changed_by":          changedBy,
		"qa_person":           qaPerson,
		"description":         description,
		"acceptance_criteria": acceptanceCriteria,
		"test_requirements":   testRequirements,
		"tags":                tags,
		"created_date":        createdDate,
		"changed_date":        changedDate,
		"area_path":           areaPath,
		"iteration_path":      iterationPath,
		"message":             fmt.Sprintf("Retrieved %s #%d: %s", workItemType, workItemID, title),
	}, nil
}
