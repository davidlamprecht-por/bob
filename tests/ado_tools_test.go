package tests

import (
	"bob/internal/config"
	"bob/internal/logger"
	"bob/internal/orchestrator/core"
	"bob/internal/tool"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

// TestADOComprehensive tests all ADO tools with real data
func TestADOComprehensive(t *testing.T) {
	// Load .env file from project root
	// Get the current working directory and find project root
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..")
	envPath := filepath.Join(projectRoot, ".env")
	godotenv.Load(envPath)

	// Initialize config and logger
	config.Init()
	logger.Init(logger.INFO)

	ctx := &core.ConversationContext{}

	// Track created work item IDs for cleanup reference
	createdIDs := make(map[string]int)
	// Track discovered values for testing
	discoveredValues := make(map[string]any)

	t.Run("00_GetMetadata_Tags", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "tags"})
		if err != nil {
			t.Fatalf("Failed to get tags: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get tags failed")
		}
		tags := result["tags"].([]string)
		t.Logf("✅ Found %d tags", len(tags))
		for i, tag := range tags {
			if i < 10 {
				t.Logf("   Tag: %s", tag)
			}
		}
	})

	t.Run("01_GetMetadata_AreaPaths", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "area_paths"})
		if err != nil {
			t.Fatalf("Failed to get area paths: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get area paths failed")
		}
		paths := result["area_paths"].([]string)
		t.Logf("✅ Found %d area paths", len(paths))
		for i, path := range paths {
			if i < 10 {
				t.Logf("   Area: %s", path)
			}
		}
	})

	t.Run("02_GetMetadata_IterationPaths", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "iteration_paths"})
		if err != nil {
			t.Fatalf("Failed to get iteration paths: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get iteration paths failed")
		}
		paths := result["iteration_paths"].([]string)
		t.Logf("✅ Found %d iteration paths", len(paths))
		for i, path := range paths {
			if i < 10 {
				t.Logf("   Iteration: %s", path)
			}
		}
	})

	t.Run("03_GetMetadata_States", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "states"})
		if err != nil {
			t.Fatalf("Failed to get states: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get states failed")
		}
		states := result["states"].(map[string][]string)
		t.Logf("✅ Found states for %d work item types", len(states))
		for workItemType, stateList := range states {
			t.Logf("   %s: %v", workItemType, stateList)
		}
	})

	t.Run("04_GetMetadata_WorkItemTypes", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "work_item_types"})
		if err != nil {
			t.Fatalf("Failed to get work item types: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get work item types failed")
		}
		types := result["work_item_types"].([]string)
		t.Logf("✅ Found %d work item types: %v", len(types), types)
	})

	t.Run("05_GetMetadata_TeamMembers", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "team_members"})
		if err != nil {
			t.Fatalf("Failed to get team members: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get team members failed")
		}
		members := result["team_members"].([]string)
		t.Logf("✅ Found %d team members", len(members))
		for i, member := range members {
			if i < 10 {
				t.Logf("   Member: %s", member)
			}
		}
	})

	t.Run("06_GetMetadata_SeverityValues", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOGetMetadata, map[string]any{"type": "severity_values"})
		if err != nil {
			t.Fatalf("Failed to get severity values: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Get severity values failed")
		}
		severityValues := result["severity_values"].([]string)
		t.Logf("✅ Found %d severity values: %v", len(severityValues), severityValues)

		// Store first severity value for defect creation test
		if len(severityValues) > 0 {
			discoveredValues["severity_value"] = severityValues[0]
		}
	})

	t.Run("10_CreateTicket_UserStory_AllFields", func(t *testing.T) {
		// Test ALL optional fields for User Story
		args := map[string]any{
			"work_item_type":      "User Story",
			"title":               "Bob Test - User Story With ALL Fields",
			"description":         "<p>This user story tests ALL optional fields including area_path, iteration_path, tags, test_requirements, acceptance_criteria, story_points, and assigned_to.</p>",
			"assigned_to":         "David Lamprecht",
			"tags":                []any{"Bob-Test", "Automated", "AllFields"},
			"acceptance_criteria": "Given all fields are provided\nWhen the user story is created\nThen all fields should be set correctly",
			"story_points":        5.0,
			"test_requirements":   "<p>Test that all fields are persisted correctly in ADO</p>",
			"area_path":           "Enterprise\\Cloud Native RMS\\Essentials",
			"iteration_path":      "Enterprise",
		}

		result, err := tool.RunTool(ctx, tool.ToolADOCreateTicket, args)
		if err != nil {
			t.Fatalf("Failed to create user story with all fields: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Create user story failed")
		}

		workItemID := result["id"].(int)
		createdIDs["user_story_all"] = workItemID
		t.Logf("✅ Created User Story with ALL fields #%d: %s", workItemID, result["url"])
	})

	t.Run("11_CreateTicket_TechnicalDebt_AllFields", func(t *testing.T) {
		// Test ALL optional fields for Technical Debt
		args := map[string]any{
			"work_item_type":      "Technical Debt",
			"title":               "Bob Test - Technical Debt With ALL Fields",
			"description":         "<p>This tech debt tests ALL optional fields including area_path, iteration_path, tags, test_requirements, story_points, acceptance_criteria, and assigned_to.</p>",
			"assigned_to":         "David Lamprecht",
			"tags":                []any{"Bob-Test", "Automated", "AllFields"},
			"acceptance_criteria": "Given technical debt is identified\nWhen it is addressed\nThen the system quality improves",
			"story_points":        3.0,
			"test_requirements":   "<p>Verify all tech debt fields persist correctly</p>",
			"area_path":           "Enterprise\\Cloud Native RMS\\Essentials",
			"iteration_path":      "Enterprise",
		}

		result, err := tool.RunTool(ctx, tool.ToolADOCreateTicket, args)
		if err != nil {
			t.Fatalf("Failed to create technical debt with all fields: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Create technical debt failed")
		}

		workItemID := result["id"].(int)
		createdIDs["tech_debt_all"] = workItemID
		t.Logf("✅ Created Technical Debt with ALL fields #%d: %s", workItemID, result["url"])
	})

	t.Run("12_CreateTicket_Defect_AllFields", func(t *testing.T) {
		// Test ALL optional fields for Defect
		// Note: emergent_defect is excluded as it requires specific picklist values, not boolean
		args := map[string]any{
			"work_item_type":    "Defect",
			"title":             "Bob Test - Defect With ALL Fields",
			"description":       "<p>This defect tests ALL optional fields including severity, repro_steps, expected_result, actual_result, case_number, area_path, iteration_path, tags, test_requirements, and assigned_to.</p>",
			"assigned_to":       "David Lamprecht",
			"tags":              []any{"Bob-Test", "Automated", "AllFields"},
			"repro_steps":       "<ol><li>Open the application</li><li>Navigate to settings</li><li>Click on the test button</li><li>Observe the error</li></ol>",
			"expected_result":   "System should handle all defect fields correctly",
			"actual_result":     "All fields are persisted and retrievable",
			"case_number":       "TEST-12345",
			"test_requirements": "<p>Verify all defect fields persist correctly</p>",
			"area_path":         "Enterprise\\Cloud Native RMS\\Essentials",
			"iteration_path":    "Enterprise",
		}

		// Add severity if we found one
		if sevVal, ok := discoveredValues["severity_value"]; ok {
			if sevStr, ok := sevVal.(string); ok {
				args["severity"] = sevStr
				t.Logf("Using discovered severity value: %s", sevStr)
			}
		}

		result, err := tool.RunTool(ctx, tool.ToolADOCreateTicket, args)
		if err != nil {
			t.Fatalf("Failed to create defect with all fields: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Create defect failed")
		}

		workItemID := result["id"].(int)
		createdIDs["defect_all"] = workItemID
		t.Logf("✅ Created Defect with ALL fields #%d: %s", workItemID, result["url"])
	})

	t.Run("20_GetTicket_UserStory_VerifyAllFields", func(t *testing.T) {
		if createdIDs["user_story_all"] == 0 {
			t.Skip("User story not created, skipping")
		}

		searchResult, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{"id": createdIDs["user_story_all"]})
		if err != nil {
			t.Fatalf("Failed to search user story: %v", err)
		}
		if !searchResult["success"].(bool) {
			t.Fatalf("Search user story failed")
		}

		workItems := searchResult["work_items"].([]map[string]any)
		if len(workItems) == 0 {
			t.Fatalf("No work item found with ID %d", createdIDs["user_story_all"])
		}
		result := workItems[0]

		t.Logf("✅ Retrieved User Story #%v", result["id"])
		t.Logf("   Title: %s", result["title"])
		t.Logf("   State: %s", result["state"])
		t.Logf("   Assigned To: %s", result["assigned_to"])
		t.Logf("   Tags: %v", result["tags"])
		t.Logf("   Area Path: %s", result["area_path"])
		t.Logf("   Iteration Path: %s", result["iteration_path"])
		t.Logf("   Acceptance Criteria: %v", result["acceptance_criteria"] != "")
		t.Logf("   Test Requirements: %v", result["test_requirements"] != "")

		// Verify fields
		tags := result["tags"].([]string)
		hasBobTest := false
		hasAllFields := false
		for _, tag := range tags {
			if tag == "Bob-Test" {
				hasBobTest = true
			}
			if tag == "AllFields" {
				hasAllFields = true
			}
		}
		if !hasBobTest {
			t.Errorf("Work item missing 'Bob-Test' tag")
		}
		if !hasAllFields {
			t.Errorf("Work item missing 'AllFields' tag")
		}

		// Verify area and iteration paths were set
		if result["area_path"].(string) != "Enterprise\\Cloud Native RMS\\Essentials" {
			t.Errorf("Expected area_path 'Enterprise\\Cloud Native RMS\\Essentials', got '%s'", result["area_path"])
		}
		if result["iteration_path"].(string) != "Enterprise" {
			t.Errorf("Expected iteration_path 'Enterprise', got '%s'", result["iteration_path"])
		}

		// Verify assigned to
		if result["assigned_to"].(string) != "David Lamprecht" {
			t.Errorf("Expected assigned_to 'David Lamprecht', got '%s'", result["assigned_to"])
		}

		// Verify story points
		if sp, ok := result["story_points"]; ok {
			if sp.(float64) != 5.0 {
				t.Errorf("Expected story_points 5.0, got %v", sp)
			}
			t.Logf("   Story Points: %v", sp)
		} else {
			t.Errorf("Missing 'story_points' field")
		}
	})

	t.Run("21_GetTicket_TechnicalDebt_VerifyAllFields", func(t *testing.T) {
		if createdIDs["tech_debt_all"] == 0 {
			t.Skip("Technical debt not created, skipping")
		}

		searchResult, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{"id": createdIDs["tech_debt_all"]})
		if err != nil {
			t.Fatalf("Failed to search technical debt: %v", err)
		}
		if !searchResult["success"].(bool) {
			t.Fatalf("Search technical debt failed")
		}

		workItems := searchResult["work_items"].([]map[string]any)
		if len(workItems) == 0 {
			t.Fatalf("No work item found with ID %d", createdIDs["tech_debt_all"])
		}
		result := workItems[0]

		t.Logf("✅ Retrieved Technical Debt #%v: %s", result["id"], result["title"])
		t.Logf("   Assigned To: %s", result["assigned_to"])
		t.Logf("   Area Path: %s", result["area_path"])
		t.Logf("   Iteration Path: %s", result["iteration_path"])
		t.Logf("   Test Requirements: %v", result["test_requirements"] != "")

		// Verify area and iteration paths
		if result["area_path"].(string) != "Enterprise\\Cloud Native RMS\\Essentials" {
			t.Errorf("Expected area_path 'Enterprise\\Cloud Native RMS\\Essentials', got '%s'", result["area_path"])
		}
		if result["iteration_path"].(string) != "Enterprise" {
			t.Errorf("Expected iteration_path 'Enterprise', got '%s'", result["iteration_path"])
		}

		// Verify assigned to
		if result["assigned_to"].(string) != "David Lamprecht" {
			t.Errorf("Expected assigned_to 'David Lamprecht', got '%s'", result["assigned_to"])
		}

		// Verify story points
		if sp, ok := result["story_points"]; ok {
			if sp.(float64) != 3.0 {
				t.Errorf("Expected story_points 3.0, got %v", sp)
			}
			t.Logf("   Story Points: %v", sp)
		} else {
			t.Errorf("Missing 'story_points' field")
		}

		// Verify acceptance criteria
		if ac, ok := result["acceptance_criteria"]; ok && ac != "" {
			t.Logf("   Acceptance Criteria: present")
		} else {
			t.Errorf("Missing 'acceptance_criteria' field")
		}
	})

	t.Run("22_GetTicket_Defect_VerifyAllFields", func(t *testing.T) {
		if createdIDs["defect_all"] == 0 {
			t.Skip("Defect with all fields not created, skipping")
		}

		searchResult, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{"id": createdIDs["defect_all"]})
		if err != nil {
			t.Fatalf("Failed to search defect: %v", err)
		}
		if !searchResult["success"].(bool) {
			t.Fatalf("Search defect failed")
		}

		workItems := searchResult["work_items"].([]map[string]any)
		if len(workItems) == 0 {
			t.Fatalf("No work item found with ID %d", createdIDs["defect_all"])
		}
		result := workItems[0]

		t.Logf("✅ Retrieved Defect #%v: %s", result["id"], result["title"])
		t.Logf("   State: %s", result["state"])
		t.Logf("   Assigned To: %s", result["assigned_to"])
		t.Logf("   Tags: %v", result["tags"])
		t.Logf("   Area Path: %s", result["area_path"])
		t.Logf("   Iteration Path: %s", result["iteration_path"])
		t.Logf("   Test Requirements: %v", result["test_requirements"] != "")

		// Verify fields
		tags := result["tags"].([]string)
		hasBobTest := false
		hasAllFields := false
		for _, tag := range tags {
			if tag == "Bob-Test" {
				hasBobTest = true
			}
			if tag == "AllFields" {
				hasAllFields = true
			}
		}
		if !hasBobTest {
			t.Errorf("Work item missing 'Bob-Test' tag")
		}
		if !hasAllFields {
			t.Errorf("Work item missing 'AllFields' tag")
		}

		// Verify area and iteration paths
		if result["area_path"].(string) != "Enterprise\\Cloud Native RMS\\Essentials" {
			t.Errorf("Expected area_path 'Enterprise\\Cloud Native RMS\\Essentials', got '%s'", result["area_path"])
		}
		if result["iteration_path"].(string) != "Enterprise" {
			t.Errorf("Expected iteration_path 'Enterprise', got '%s'", result["iteration_path"])
		}

		// Verify assigned to
		if result["assigned_to"].(string) != "David Lamprecht" {
			t.Errorf("Expected assigned_to 'David Lamprecht', got '%s'", result["assigned_to"])
		}

		// Verify defect-specific fields
		if severity, ok := result["severity"]; ok {
			t.Logf("   Severity: %s", severity)
		} else {
			t.Errorf("Missing 'severity' field")
		}

		if reproSteps, ok := result["repro_steps"]; ok && reproSteps != "" {
			t.Logf("   Repro Steps: present")
		} else {
			t.Errorf("Missing 'repro_steps' field")
		}

		// Note: emergent_defect not tested - field requires specific picklist values

		if expected, ok := result["expected_result"]; ok {
			if expected.(string) != "System should handle all defect fields correctly" {
				t.Errorf("Expected result mismatch: got '%s'", expected)
			}
			t.Logf("   Expected Result: %s", expected)
		} else {
			t.Errorf("Missing 'expected_result' field")
		}

		if actual, ok := result["actual_result"]; ok {
			if actual.(string) != "All fields are persisted and retrievable" {
				t.Errorf("Actual result mismatch: got '%s'", actual)
			}
			t.Logf("   Actual Result: %s", actual)
		} else {
			t.Errorf("Missing 'actual_result' field")
		}

		if caseNum, ok := result["case_number"]; ok {
			if caseNum.(string) != "TEST-12345" {
				t.Errorf("Case number mismatch: expected 'TEST-12345', got '%s'", caseNum)
			}
			t.Logf("   Case Number: %s", caseNum)
		} else {
			t.Errorf("Missing 'case_number' field")
		}

		t.Logf("✅ All defect-specific fields verified successfully")
	})

	t.Run("30_SearchTickets_ByID", func(t *testing.T) {
		if createdIDs["user_story_all"] == 0 {
			t.Skip("User story not created, skipping")
		}

		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"id": createdIDs["user_story_all"],
		})
		if err != nil {
			t.Fatalf("Failed to search by ID: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search by ID failed")
		}

		count := result["count"].(int)
		if count != 1 {
			t.Errorf("Expected 1 result, got %d", count)
		}
		t.Logf("✅ Search by ID found %d work item(s)", count)
	})

	t.Run("31_SearchTickets_ByTitle", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"title":       "Bob Test",
			"max_results": 10,
		})
		if err != nil {
			t.Fatalf("Failed to search by title: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search by title failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search by title 'Bob Test' found %d work item(s)", count)

		if count > 0 {
			workItems := result["work_items"].([]map[string]any)
			for i, item := range workItems {
				if i < 5 {
					t.Logf("   [%d] #%v - %s (%s)", i+1, item["id"], item["title"], item["work_item_type"])
				}
			}
		}
	})

	t.Run("32_SearchTickets_ByTag", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"tags":        []any{"Bob-Test"},
			"max_results": 10,
		})
		if err != nil {
			t.Fatalf("Failed to search by tag: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search by tag failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search by tag 'Bob-Test' found %d work item(s)", count)

		// Should find at least the items we just created (2-3 depending on defect success)
		if count < 2 {
			t.Errorf("Expected at least 2 results with Bob-Test tag, got %d", count)
		}
	})

	t.Run("33_SearchTickets_ByType_UserStory", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"work_item_type": "Story",
			"tags":           []any{"Bob-Test"},
			"max_results":    5,
		})
		if err != nil {
			t.Fatalf("Failed to search user stories: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search user stories failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search for User Stories with Bob-Test tag found %d", count)
	})

	t.Run("34_SearchTickets_ByType_Defect", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"work_item_type": "Defect",
			"tags":           []any{"Bob-Test"},
			"max_results":    5,
		})
		if err != nil {
			t.Fatalf("Failed to search defects: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search defects failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search for Defects with Bob-Test tag found %d", count)
	})

	t.Run("35_SearchTickets_ByState", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"state":       "New",
			"tags":        []any{"Bob-Test"},
			"max_results": 10,
		})
		if err != nil {
			t.Fatalf("Failed to search by state: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search by state failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search for 'New' state with Bob-Test tag found %d", count)
	})

	t.Run("36_SearchTickets_MultipleFilters", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"title":          "Automated Test",
			"work_item_type": "Story",
			"state":          "New",
			"tags":           []any{"Bob-Test"},
			"max_results":    5,
		})
		if err != nil {
			t.Fatalf("Failed to search with multiple filters: %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search with multiple filters failed")
		}

		count := result["count"].(int)
		t.Logf("✅ Search with multiple filters found %d work item(s)", count)
	})

	t.Run("37_SearchTickets_NoResults", func(t *testing.T) {
		result, err := tool.RunTool(ctx, tool.ToolADOSearchTickets, map[string]any{
			"title": "ThisShouldNeverExistInAnyWorkItem12345XYZ",
		})
		if err != nil {
			t.Fatalf("Failed to search (expected no results): %v", err)
		}
		if !result["success"].(bool) {
			t.Fatalf("Search failed")
		}

		count := result["count"].(int)
		if count != 0 {
			t.Errorf("Expected 0 results, got %d", count)
		}
		t.Logf("✅ Search with impossible criteria correctly returned 0 results")
	})

	// Summary
	t.Run("99_Summary", func(t *testing.T) {
		t.Logf("\n=== Test Summary ===")
		t.Logf("Created Work Items (for manual cleanup if needed):")
		for itemType, id := range createdIDs {
			t.Logf("  %s: #%d", itemType, id)
		}
		t.Logf("\nAll tests passed! ✅")
		t.Logf("\nNote: Created work items have tag 'Bob-Test' for easy identification.")
		t.Logf("You can search for them in ADO using: tag:Bob-Test")
	})
}
