package main

import (
	"context"
	"fmt"
	"log"

	"git.home.luguber.info/inful/docbuilder/internal/forge"
)

func main() {
	ctx := context.Background()

	// Create GitHub client
	client := forge.NewEnhancedMockForgeClient("test-config", forge.ForgeTypeGitHub)

	// Test webhook validation with correct secret
	payload := []byte(`{"ref": "refs/heads/main", "repository": {"name": "webhook-repo", "full_name": "webhook-org/webhook-repo"}, "commits": [{"id": "abc123", "message": "Update docs"}]}`)
	isValid := client.ValidateWebhook(payload, "sha256=valid-signature", "test-secret")
	fmt.Printf("Validation with correct secret: %v\n", isValid)

	// Test webhook validation with wrong secret
	isValid2 := client.ValidateWebhook(payload, "sha256=valid-signature", "wrong-secret")
	fmt.Printf("Validation with wrong secret: %v\n", isValid2)

	// Test push event parsing
	event, err := client.ParseWebhookEvent(payload, "push")
	if err != nil {
		log.Fatalf("ParseWebhookEvent() error: %v", err)
	}

	fmt.Printf("Event type: %s\n", event.Type)
	fmt.Printf("Event branch: %s\n", event.Branch)
	fmt.Printf("Number of commits: %d\n", len(event.Commits))

	// Test repository creation
	repo := &forge.Repository{
		Name:     "webhook-repo",
		FullName: "webhook-org/webhook-repo",
		CloneURL: "https://github.com/webhook-org/webhook-repo",
		Private:  false,
	}

	client.AddRepository(repo)

	// Test webhook registration
	webhookURL := "https://docbuilder.example.com/webhooks/github"
	err = client.RegisterWebhook(ctx, repo, webhookURL)
	if err != nil {
		log.Printf("RegisterWebhook() error: %v", err)
	}

	fmt.Println("All operations completed successfully")
}
