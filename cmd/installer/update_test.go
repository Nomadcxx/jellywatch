package main

import "testing"

func TestHandleAIKeys_SpaceVariantsToggleSelectedModel(t *testing.T) {
	base := model{
		step:         stepIntegrationsAI,
		aiEnabled:    true,
		aiState:      aiStateReady,
		aiModels:     []string{"llama3.2", "mistral"},
		aiModelIndex: 1, // highlight "mistral"
	}

	// Legacy space representation
	m1, _ := base.handleAIKeys(" ")
	got1 := m1.(model)
	if got1.aiModel != "mistral" {
		t.Fatalf("expected aiModel selected with \" \", got %q", got1.aiModel)
	}

	// Bubble Tea can emit "space" in some environments/versions.
	m2, _ := base.handleAIKeys("space")
	got2 := m2.(model)
	if got2.aiModel != "mistral" {
		t.Fatalf("expected aiModel selected with \"space\", got %q", got2.aiModel)
	}
}
