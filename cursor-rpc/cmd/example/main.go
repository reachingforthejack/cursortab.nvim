package main

import (
	"context"
	"fmt"
	"log"

	"github.com/everestmz/cursor-rpc"
	aiserverv1 "github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1"
)

func main() {
	// Get default credentials from cursor
	creds, err := cursor.GetDefaultCredentials()
	if err != nil {
		log.Fatal(err)
	}

	// Set up the service
	aiService := cursor.NewAiServiceClient()

	// Use cursor.NewRequest to inject credentials & create the request object before sending
	models, err := aiService.AvailableModels(context.TODO(), cursor.NewRequest(creds, &aiserverv1.AvailableModelsRequest{
		IsNightly:                true,
		IncludeLongContextModels: true,
	}))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Available models:")
	for _, model := range models.Msg.ModelNames {
		fmt.Println(" -", model)
	}

	model := models.Msg.ModelNames[len(models.Msg.ModelNames)-1]

	fmt.Println("Selected model", model)

	resp, err := aiService.StreamChat(context.TODO(), cursor.NewRequest(creds, &aiserverv1.GetChatRequest{
		ModelDetails: &aiserverv1.ModelDetails{
			ModelName: &model,
		},
		Conversation: []*aiserverv1.ConversationMessage{
			{
				Text: "Hello, who are you?",
				Type: aiserverv1.ConversationMessage_MESSAGE_TYPE_HUMAN,
			},
		},
	}))
	if err != nil {
		log.Fatal(err)
	}

	for resp.Receive() {
		next := resp.Msg()
		fmt.Printf(next.Text)
	}
	fmt.Println()
}
