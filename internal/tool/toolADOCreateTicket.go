package tool

import (
	"bob/internal/ai"
	"bob/internal/config"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const ToolADOCreateTicket ToolName = "ado_create_ticket"

var ADOCreateTicketArgs = ai.NewSchema().
	AddString("work_item_type", ai.Required(),
		ai.Enum("User Story", "Technical Debt", "Defect"),
		ai.Description("Type of work item to create")).
	AddString("title", ai.Required(),
		ai.Description("Title of the work item")).
	AddString("description", ai.Required(),
		ai.Description("Detailed description of the work item")).
	AddString("assigned_to", ai.Description("Display name of person to assign (e.g., 'David Lamprecht')")).
	AddString("area_path", ai.Description("Area path in ADO (e.g., 'Enterprise\\Cloud Native RMS\\Essentials')")).
	AddString("iteration_path", ai.Description("Iteration path in ADO (e.g., 'Enterprise\\Sprint 42')")).
	AddArray("tags", ai.FieldTypeString, ai.Description("Tags to add to the work item")).
	AddString("test_requirements", ai.Description("Test requirements for the work item")).
	AddString("acceptance_criteria", ai.Description("Acceptance criteria (User Story and Technical Debt)")).
	AddFloat("story_points", ai.Description("Story points estimate (User Story and Technical Debt)")).
	AddString("severity",
		ai.Enum("Critical", "Urgent", "High", "Medium", "Low"),
		ai.Description("Severity level (Defect only). Defaults to 'Medium'")).
	AddString("repro_steps", ai.Description("Reproduction steps (Defect only) - detailed steps to reproduce the issue")).
	AddBool("emergent_defect", ai.Description("Whether this is an emergent defect (Defect only). True = urgent/emergent, False = can wait")).
	AddString("expected_result", ai.Description("Expected result (Defect only)")).
	AddString("actual_result", ai.Description("Actual result (Defect only)")).
	AddString("case_number", ai.Description("Case number (Defect only)"))

// ADOCreateTicket creates a new work item in Azure DevOps
func ADOCreateTicket(context *core.ConversationContext, args map[string]any) (map[string]any, error) {
	logger.Debug("🎫 ADOCreateTicket: Starting")

	// Extract required parameters
	workItemType, ok := args["work_item_type"].(string)
	if !ok {
		return nil, fmt.Errorf("work_item_type parameter missing or invalid")
	}

	title, ok := args["title"].(string)
	if !ok {
		return nil, fmt.Errorf("title parameter missing or invalid")
	}

	description, ok := args["description"].(string)
	if !ok {
		return nil, fmt.Errorf("description parameter missing or invalid")
	}

	logger.Debugf("🎫 Creating %s: %s", workItemType, title)

	// Map user-friendly work item type to ADO type
	adoTypeMap := map[string]string{
		"User Story":     "Story",
		"Technical Debt": "Tech Debt",
		"Defect":         "Defect",
	}
	adoWorkItemType := adoTypeMap[workItemType]
	if adoWorkItemType == "" {
		adoWorkItemType = workItemType
	}

	// Build JSON Patch document
	patchDoc := []map[string]any{
		{"op": "add", "path": "/fields/System.Title", "value": title},
		{"op": "add", "path": "/fields/System.Description", "value": description},
	}

	// Add optional fields
	if assignedTo, ok := args["assigned_to"].(string); ok && assignedTo != "" {
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/System.AssignedTo", "value": assignedTo,
		})
	}

	if areaPath, ok := args["area_path"].(string); ok && areaPath != "" {
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/System.AreaPath", "value": areaPath,
		})
	}

	if iterationPath, ok := args["iteration_path"].(string); ok && iterationPath != "" {
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/System.IterationPath", "value": iterationPath,
		})
	}

	// Handle tags - always add "BobBot" tag
	tagList := []string{}
	if tags, ok := args["tags"].([]any); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagList = append(tagList, tagStr)
			}
		}
	}
	// Add BobBot tag if not present
	hasBobBot := false
	for _, tag := range tagList {
		if tag == "BobBot" {
			hasBobBot = true
			break
		}
	}
	if !hasBobBot {
		tagList = append([]string{"BobBot"}, tagList...)
	}
	if len(tagList) > 0 {
		// Tags are semicolon-separated in ADO
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/System.Tags", "value": strings.Join(tagList, "; "),
		})
	}

	if testReq, ok := args["test_requirements"].(string); ok && testReq != "" {
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/POR.TestRequirements", "value": testReq,
		})
	}

	// Type-specific fields
	if workItemType == "User Story" || workItemType == "Technical Debt" {
		if acceptanceCriteria, ok := args["acceptance_criteria"].(string); ok && acceptanceCriteria != "" {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Microsoft.VSTS.Common.AcceptanceCriteria", "value": acceptanceCriteria,
			})
		}

		if storyPoints, ok := args["story_points"].(float64); ok {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Microsoft.VSTS.Scheduling.StoryPoints", "value": storyPoints,
			})
		}
	}

	// Defect fields - Severity is REQUIRED for Defect type
	if workItemType == "Defect" {
		severity := "Medium" // Default
		if sev, ok := args["severity"].(string); ok && sev != "" {
			severity = sev
		}
		patchDoc = append(patchDoc, map[string]any{
			"op": "add", "path": "/fields/Microsoft.VSTS.Common.Severity", "value": severity,
		})

		if reproSteps, ok := args["repro_steps"].(string); ok && reproSteps != "" {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Microsoft.VSTS.TCM.ReproSteps", "value": reproSteps,
			})
		}

		if emergent, ok := args["emergent_defect"].(bool); ok {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Custom.EmergentDefect", "value": emergent,
			})
		}

		if expected, ok := args["expected_result"].(string); ok && expected != "" {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Custom.ExpectedResult", "value": expected,
			})
		}

		if actual, ok := args["actual_result"].(string); ok && actual != "" {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Custom.ActualResult", "value": actual,
			})
		}

		if caseNum, ok := args["case_number"].(string); ok && caseNum != "" {
			patchDoc = append(patchDoc, map[string]any{
				"op": "add", "path": "/fields/Custom.CaseNumber", "value": caseNum,
			})
		}
	}

	// Make HTTP request
	cfg := config.Current
	url := fmt.Sprintf("%s/%s/_apis/wit/workitems/$%s?api-version=7.1-preview.3",
		cfg.ADOOrgURL, cfg.ADOProject, adoWorkItemType)

	jsonData, err := json.Marshal(patchDoc)
	if err != nil {
		logger.Errorf("❌ Failed to marshal patch document: %v", err)
		return nil, fmt.Errorf("failed to marshal patch document: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Errorf("❌ Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// ADO uses HTTP Basic Auth with empty username and PAT as password
	req.SetBasicAuth("", cfg.ADOPAT)
	req.Header.Set("Content-Type", "application/json-patch+json")

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
	workItemID := int(result["id"].(float64))
	fields := result["fields"].(map[string]any)
	links := result["_links"].(map[string]any)
	htmlLink := links["html"].(map[string]any)
	workItemURL := htmlLink["href"].(string)
	createdType := fields["System.WorkItemType"].(string)
	createdTitle := fields["System.Title"].(string)

	logger.Infof("✅ Created work item #%d: %s - %s", workItemID, createdType, createdTitle)

	return map[string]any{
		"success":        true,
		"id":             workItemID,
		"url":            workItemURL,
		"work_item_type": createdType,
		"title":          createdTitle,
		"message":        fmt.Sprintf("Successfully created %s #%d: %s", createdType, workItemID, createdTitle),
	}, nil
}
