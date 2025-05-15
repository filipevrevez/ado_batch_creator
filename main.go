package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"filipevrevez.github.com/ado_batch_creator/models"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// Initialize the logger
	logger, err := zap.NewProduction(
	// zap.Option
	)
	if err != nil {
		panic(err)
	}
	defer logger.Sync() // Flushes buffer, if any

	// Initialize Viper
	viper.SetConfigName("config")   // Name of the config file (without extension)
	viper.SetConfigType("yaml")     // Config file format
	viper.AddConfigPath("./config") // Path to look for the config file in the current directory
	viper.AutomaticEnv()            // Automatically read environment variables
	viper.SetDefault("env", "prd")

	// Read the config file
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn("Failed to read config file", zap.Error(err))
		panic("Error reading config file")
	} else {
		logger.Info("Config file loaded successfully")
	}

	var userStories []models.UserStory
	file, err := os.ReadFile(viper.GetString("itemsPath"))
	if err != nil {
		logger.Sugar().Fatalf("Failed to read items file in location %s", viper.GetString("itemsPath"))
	}

	if err := json.Unmarshal(file, &userStories); err != nil {
		logger.Sugar().Panicf("failed to decode file with error: %w", err)
	}

	// Example: Reading a value from the config or environment
	appName := viper.GetString("app.name")
	if appName == "" {
		appName = "FR App"
	}
	logger.Info("Application Name", zap.String("app_name", appName))

	ctx := context.Background()
	// Create user stories in Azure DevOps
	for _, userStory := range userStories {
		err := createUserStory(ctx, userStory, logger)
		if err != nil {
			logger.Error("Failed to create user story", zap.String("name", userStory.Name), zap.Error(err))
		}
	}

	logger.Sugar().Infof("Finish Job. Created: %d US and %d Tasks", len(userStories), 0)
}

// createUserStory creates a user story in Azure DevOps
func createUserStory(ctx context.Context, userStory models.UserStory, logger *zap.Logger) error {
	organization := viper.GetString("devops.organization")
	project := viper.GetString("devops.project")
	pat := viper.GetString("devops.pat")

	// Validate required configuration
	if organization == "" || project == "" || pat == "" {
		return fmt.Errorf("missing Azure DevOps configuration: organization, project, or PAT")
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/wit/workitems/$User%%20Story?api-version=7.0", organization, project)
	logger.Debug("Azure DevOps API URL", zap.String("url", url))

	payload := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/fields/System.Title",
			"value": userStory.Name,
		},
		{
			"op":    "add",
			"path":  "/fields/System.Description",
			"value": userStory.Description,
		},
		{
			"op":    "add",
			"path":  "/fields/System.AssignedTo",
			"value": userStory.Owner,
		},
		{
			"op":    "add",
			"path":  "/fields/Microsoft.VSTS.Common.Priority",
			"value": userStory.Priority,
		},
		{
			"op":    "add",
			"path":  "/fields/System.State",
			"value": userStory.State,
		},
		{
			"op":    "add",
			"path":  "/fields/System.Tags",
			"value": "system_automated", // Add the "system_automated" tag
		},
		{
			"op":    "add",
			"path":  "/fields/System.AreaPath",
			"value": userStory.Area, // Add the "system_automated" tag
		},
		// {
		// 	"op":    "add",
		// 	"path":  "/fields/System.Iteraction",
		// 	"value": userStory.Path, // Add the "system_automated" tag
		// },
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the HTTP request for the user story
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and authentication
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.SetBasicAuth("", pat)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResponseBody map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResponseBody); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		return fmt.Errorf("failed to create user story, status: %s with message: %s", resp.Status, string(errResponseBody["message"].(string)))
	}

	logger.Info("User story created successfully", zap.String("name", userStory.Name))

	// Parse the response to get the user story ID
	var responseBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	userStoryID := int(responseBody["id"].(float64))

	// Create tasks for the user story
	for _, task := range userStory.Tasks {
		if err := createTask(ctx, userStoryID, task, logger, userStory); err != nil {
			logger.Error("Failed to create task", zap.String("task_name", task.Name), zap.Error(err))
		}
	}

	return nil
}

// createTask creates a task in Azure DevOps and links it to a user story
func createTask(ctx context.Context, parentID int, task models.Task, logger *zap.Logger, userStory models.UserStory) error {
	organization := viper.GetString("devops.organization")
	project := viper.GetString("devops.project")
	pat := viper.GetString("devops.pat")

	// Validate required configuration
	if organization == "" || project == "" || pat == "" {
		return fmt.Errorf("missing Azure DevOps configuration: organization, project, or PAT")
	}

	// Azure DevOps REST API URL for creating tasks
	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/wit/workitems/$Task?api-version=7.0", organization, project)

	// Payload for the task
	payload := []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/fields/System.Title",
			"value": task.Name,
		},
		{
			"op":    "add",
			"path":  "/fields/System.Description",
			"value": task.Description,
		},
		{
			"op":    "add",
			"path":  "/fields/System.AssignedTo",
			"value": task.Owner,
		},
		{
			"op":    "add",
			"path":  "/fields/Microsoft.VSTS.Common.Priority",
			"value": task.Priority,
		},
		{
			"op":    "add",
			"path":  "/fields/System.State",
			"value": task.State,
		},
		{
			"op":   "add",
			"path": "/relations/-",
			"value": map[string]interface{}{
				"rel": "System.LinkTypes.Hierarchy-Reverse",
				"url": fmt.Sprintf("https://dev.azure.com/%s/_apis/wit/workItems/%d", organization, parentID),
				"attributes": map[string]string{
					"comment": "Linking task to user story",
				},
			},
		},
		{
			"op":    "add",
			"path":  "/fields/System.AreaPath",
			"value": userStory.Area, // Add the "system_automated" tag
		},
		// {
		// 	"op":    "add",
		// 	"path":  "/fields/System.Iteraction",
		// 	"value": userStory.Path, // Add the "system_automated" tag
		// },
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the HTTP request for the task
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers and authentication
	req.Header.Set("Content-Type", "application/json-patch+json")
	req.SetBasicAuth("", pat)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create task, status: %s", resp.Status)
	}

	logger.Info("Task created successfully", zap.String("name", task.Name))
	return nil
}

// Finds the next iteraction based on dates for that team
func FindNextIteraction(ctx context.Context, team string) *string {

	return nil
}

func FindIteraction(ctx context.Context, iteraction string) *string {

	return nil
}

func GetAdoSettings(logger *zap.Logger) models.AdoSettings {
	adosettings := &models.AdoSettings{}

	organization := viper.GetString("devops.organization")
	project := viper.GetString("devops.project")
	pat := viper.GetString("devops.pat")

	// Validate required configuration
	if organization == "" || project == "" || pat == "" {
		logger.Sugar().Panicf("missing Azure DevOps configuration: organization: %s, project: %s, or PAT: %d", organization, project, len(pat))
		return *adosettings
	}

	adosettings.Organization = organization
	adosettings.Project = project
	adosettings.Pat = pat

	return *adosettings
}
