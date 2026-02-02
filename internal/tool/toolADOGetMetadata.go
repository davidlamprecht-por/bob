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

const ToolADOGetMetadata ToolName = "ado_get_metadata"

var ADOGetMetadataArgs = ai.NewSchema().
	AddString("type", ai.Required(),
		ai.Enum("tags", "area_paths", "iteration_paths", "states", "work_item_types", "team_members", "severity_values"),
		ai.Description("Type of metadata to retrieve"))

// ADOGetMetadata retrieves metadata from Azure DevOps to help with filtering and queries
func ADOGetMetadata(context *core.ConversationContext, args map[string]any) (map[string]any, error) {
	logger.Debug("📋 ADOGetMetadata: Starting")

	metadataType, ok := args["type"].(string)
	if !ok {
		return nil, fmt.Errorf("type parameter missing or invalid")
	}

	logger.Debugf("📋 Fetching metadata type: %s", metadataType)

	cfg := config.Current
	client := &http.Client{}

	switch metadataType {
	case "tags":
		return getTagsMetadata(cfg, client)
	case "area_paths":
		return getAreaPathsMetadata(cfg, client)
	case "iteration_paths":
		return getIterationPathsMetadata(cfg, client)
	case "states":
		return getStatesMetadata(cfg, client)
	case "work_item_types":
		return getWorkItemTypesMetadata(cfg, client)
	case "team_members":
		return getTeamMembersMetadata(cfg, client)
	case "severity_values":
		return getSeverityValuesMetadata(cfg, client)
	default:
		return nil, fmt.Errorf("unknown metadata type: %s", metadataType)
	}
}

// getTagsMetadata retrieves all distinct tags from work items using Tags API
func getTagsMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	// Use the dedicated Tags List API endpoint
	url := fmt.Sprintf("%s/%s/_apis/wit/tags?api-version=7.1", cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tags API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tags API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	json.Unmarshal(body, &result)

	tags := []string{}
	if value, ok := result["value"].([]any); ok {
		for _, item := range value {
			itemMap := item.(map[string]any)
			if tagName, ok := itemMap["name"].(string); ok {
				tags = append(tags, tagName)
			}
		}
	}

	logger.Infof("✅ Found %d distinct tags", len(tags))
	return map[string]any{"success": true, "tags": tags, "message": fmt.Sprintf("Found %d distinct tags", len(tags))}, nil
}

// getAreaPathsMetadata retrieves area paths using classification nodes API
func getAreaPathsMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s/_apis/wit/classificationnodes/areas?$depth=10&api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("classification nodes request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("classification API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	json.Unmarshal(body, &result)

	// Extract paths recursively
	paths := []string{}
	var extractPaths func(node map[string]any, basePath string)
	extractPaths = func(node map[string]any, basePath string) {
		name := node["name"].(string)
		currentPath := name
		if basePath != "" {
			currentPath = basePath + "\\" + name
		}
		paths = append(paths, currentPath)

		if children, ok := node["children"].([]any); ok {
			for _, child := range children {
				extractPaths(child.(map[string]any), currentPath)
			}
		}
	}

	extractPaths(result, "")

	logger.Infof("✅ Found %d area paths", len(paths))
	return map[string]any{"success": true, "area_paths": paths, "message": fmt.Sprintf("Found %d area paths", len(paths))}, nil
}

// getIterationPathsMetadata retrieves iteration paths using classification nodes API
func getIterationPathsMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s/_apis/wit/classificationnodes/iterations?$depth=10&api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("classification nodes request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("classification API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	json.Unmarshal(body, &result)

	// Extract paths recursively
	paths := []string{}
	var extractPaths func(node map[string]any, basePath string)
	extractPaths = func(node map[string]any, basePath string) {
		name := node["name"].(string)
		currentPath := name
		if basePath != "" {
			currentPath = basePath + "\\" + name
		}
		paths = append(paths, currentPath)

		if children, ok := node["children"].([]any); ok {
			for _, child := range children {
				extractPaths(child.(map[string]any), currentPath)
			}
		}
	}

	extractPaths(result, "")

	logger.Infof("✅ Found %d iteration paths", len(paths))
	return map[string]any{"success": true, "iteration_paths": paths, "message": fmt.Sprintf("Found %d iteration paths", len(paths))}, nil
}

// getStatesMetadata retrieves valid states for each work item type
func getStatesMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s/_apis/wit/workitemtypes?api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("work item types request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("work item types API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	json.Unmarshal(body, &result)

	statesByType := make(map[string][]string)
	for _, item := range result["value"].([]any) {
		itemMap := item.(map[string]any)
		typeName := itemMap["name"].(string)

		if states, ok := itemMap["states"].([]any); ok {
			stateNames := []string{}
			for _, state := range states {
				// States can be either strings or objects with "name" field
				switch v := state.(type) {
				case string:
					stateNames = append(stateNames, v)
				case map[string]any:
					if name, ok := v["name"].(string); ok {
						stateNames = append(stateNames, name)
					}
				}
			}
			statesByType[typeName] = stateNames
		}
	}

	logger.Infof("✅ Found states for %d work item types", len(statesByType))
	return map[string]any{"success": true, "states": statesByType, "message": fmt.Sprintf("Found states for %d work item types", len(statesByType))}, nil
}

// getWorkItemTypesMetadata retrieves available work item types
func getWorkItemTypesMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s/_apis/wit/workitemtypes?api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("work item types request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("work item types API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	json.Unmarshal(body, &result)

	types := []string{}
	for _, item := range result["value"].([]any) {
		itemMap := item.(map[string]any)
		types = append(types, itemMap["name"].(string))
	}

	logger.Infof("✅ Found %d work item types", len(types))
	return map[string]any{"success": true, "work_item_types": types, "message": fmt.Sprintf("Found %d work item types", len(types))}, nil
}

// getTeamMembersMetadata retrieves team members who can be assigned work items
func getTeamMembersMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	// Get default team for the project
	url := fmt.Sprintf("%s/_apis/projects/%s/teams?api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("", cfg.ADOPAT)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("teams request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("teams API error (status %d): %s", resp.StatusCode, string(body))
	}

	var teamsResult map[string]any
	json.Unmarshal(body, &teamsResult)

	teams := teamsResult["value"].([]any)
	if len(teams) == 0 {
		return map[string]any{"success": true, "team_members": []string{}, "message": "No teams found"}, nil
	}

	// Get first team's members
	firstTeam := teams[0].(map[string]any)
	teamName := firstTeam["name"].(string)

	membersURL := fmt.Sprintf("%s/_apis/projects/%s/teams/%s/members?api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject, teamName)

	req2, _ := http.NewRequest("GET", membersURL, nil)
	req2.SetBasicAuth("", cfg.ADOPAT)

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("team members request failed: %w", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var membersResult map[string]any
	json.Unmarshal(body2, &membersResult)

	members := []string{}
	for _, member := range membersResult["value"].([]any) {
		memberMap := member.(map[string]any)
		identity := memberMap["identity"].(map[string]any)
		displayName := identity["displayName"].(string)
		members = append(members, displayName)
	}

	logger.Infof("✅ Found %d team members", len(members))
	return map[string]any{"success": true, "team_members": members, "message": fmt.Sprintf("Found %d team members in team '%s'", len(members), teamName)}, nil
}

// getSeverityValuesMetadata retrieves valid severity values by looking at existing defects
func getSeverityValuesMetadata(cfg config.Config, client *http.Client) (map[string]any, error) {
	// Search for recent defects to extract severity values
	wiqlQuery := "SELECT [System.Id] FROM workitems WHERE [System.TeamProject] = '" + cfg.ADOProject + "' AND [System.WorkItemType] = 'Defect' ORDER BY [System.ChangedDate] DESC"

	wiqlURL := fmt.Sprintf("%s/%s/_apis/wit/wiql?api-version=7.1", cfg.ADOOrgURL, cfg.ADOProject)
	wiqlPayload := map[string]any{"query": wiqlQuery}
	jsonData, _ := json.Marshal(wiqlPayload)

	req, _ := http.NewRequest("POST", wiqlURL, strings.NewReader(string(jsonData)))
	req.SetBasicAuth("", cfg.ADOPAT)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WIQL request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("WIQL API error (status %d): %s", resp.StatusCode, string(body))
	}

	var wiqlResult map[string]any
	json.Unmarshal(body, &wiqlResult)

	workItems := wiqlResult["workItems"].([]any)
	if len(workItems) == 0 {
		return map[string]any{"success": true, "severity_values": []string{}, "message": "No defects found to extract severity values"}, nil
	}

	// Get up to 100 defects to find all severity values
	ids := []string{}
	for i, item := range workItems {
		if i >= 100 {
			break
		}
		itemMap := item.(map[string]any)
		ids = append(ids, fmt.Sprintf("%d", int(itemMap["id"].(float64))))
	}

	// Fetch defects with severity field
	detailsURL := fmt.Sprintf("%s/%s/_apis/wit/workitems?ids=%s&fields=Microsoft.VSTS.Common.Severity&api-version=7.1",
		cfg.ADOOrgURL, cfg.ADOProject, strings.Join(ids, ","))

	req2, _ := http.NewRequest("GET", detailsURL, nil)
	req2.SetBasicAuth("", cfg.ADOPAT)

	resp2, err := client.Do(req2)
	if err != nil {
		return nil, fmt.Errorf("details request failed: %w", err)
	}
	defer resp2.Body.Close()

	body2, _ := io.ReadAll(resp2.Body)
	var detailsResult map[string]any
	json.Unmarshal(body2, &detailsResult)

	// Extract unique severity values
	severityMap := make(map[string]bool)
	for _, item := range detailsResult["value"].([]any) {
		itemMap := item.(map[string]any)
		fields := itemMap["fields"].(map[string]any)
		if sevVal, ok := fields["Microsoft.VSTS.Common.Severity"]; ok && sevVal != nil {
			if sevStr, ok := sevVal.(string); ok {
				severityMap[sevStr] = true
			}
		}
	}

	severityValues := []string{}
	for sev := range severityMap {
		severityValues = append(severityValues, sev)
	}

	logger.Infof("✅ Found %d severity values", len(severityValues))
	return map[string]any{"success": true, "severity_values": severityValues, "message": fmt.Sprintf("Found %d severity values from existing defects", len(severityValues))}, nil
}
