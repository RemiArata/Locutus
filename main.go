package main


import (
	"fmt"
	"os"
	// "time"
	"log"
	"context"
	"errors"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
  "github.com/slack-go/slack/socketmode"

	// "github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/prompts"
)

var conversationMemory = make(map[string][]string)

func HandleEventMessage(event slackevents.EventsAPIEvent, client *slack.Client) error {
	switch event.Type {
	// First we check if this is an CallbackEvent
	case slackevents.CallbackEvent:


		innerEvent := event.InnerEvent
		// Yet Another Type switch on the actual Data to see if its an AppMentionEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			// The application has been mentioned since this Event is a Mention event
			err := HandleAppMentionEventToBot(ev, client)
			if err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func HandleAppMentionEventToBot(event *slackevents.AppMentionEvent, client *slack.Client) error {

	llm, err := ollama.New(ollama.WithModel("granite3-dense"))
	if err != nil {
		log.Fatal(err)
	}

	user, err := client.GetUserInfo(event.User)
	if err != nil {
			return fmt.Errorf("failed to get user info: %w", err)
	}

	fmt.Println("Request from user", user)

  text := strings.ToLower(event.Text)
	fmt.Println("****************************************")

	// If the input starts with <, extract the string after the first >
	// Otherwise, extract the string before the first <
	location := strings.IndexRune(text, '<')
	formattedText := text

	fmt.Println(text[:location])

	if location == 0 {
    // Text starts with '<', so get everything after the first '>'
    closeIndex := strings.IndexRune(text, '>')
    if closeIndex != -1 && closeIndex+1 < len(text) {
        formattedText = strings.TrimSpace(text[closeIndex+1:])
    }
		else {
        formattedText = ""
    }
	} else if location > 0 {
			// Get everything before the first '<'
			formattedText = strings.TrimSpace(text[:location])
	} else {
			// Fallback if no '<'
			formattedText = strings.TrimSpace(text)
	}

	fmt.Println("formattedText is: ", formattedText)

	// Start saving context for user
	userID := event.User
	conversationMemory[userID] = append(conversationMemory[userID], fmt.Sprintf("User: %s", formattedText))

	// Set limit to last 10 messages
	if len(conversationMemory[userID]) > 10 {
		conversationMemory[userID] = conversationMemory[userID][len(conversationMemory[userID])-10:]
	}

	// Create the prompt template
	promptTemplate := prompts.NewPromptTemplate(
		"Here is a conversation between a helpful assistant and a user:\n\n{{.history}}\nUser: {{.input}}\nBot:",
		[]string{"history", "input"},
	)

	// Call the chain with history and input
	history := strings.Join(conversationMemory[userID][:len(conversationMemory[userID])-1], "\n") // exclude current message

	input := map[string]interface{}{
		"history": history,
		"input":   formattedText,
	}

	// Create a new chain
	chain := chains.NewLLMChain(llm, promptTemplate)

	ctx := context.Background()

	response, err := chain.Call(ctx, input)
	if err != nil {
		return fmt.Errorf("chain execution failed: %w", err)
	}

	// The response should be a map. Assuming it contains the result under the key "text"
	textResponse, ok := response["text"].(string)
	if !ok {
		return fmt.Errorf("unexpected response format, expected a string in 'text' field")
	}

	// Add bot response to memory
	conversationMemory[userID] = append(conversationMemory[userID], "Bot: "+textResponse)

	// Post to Slack
	attachment := slack.Attachment{
		Text:  textResponse,
		Color: "#4af030",
	}

	_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	// _, err = llm.Call(ctx, formattedText,
	// 	llms.WithTemperature(0.8),
	// 	llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			// attachment.Text = string(chunk)
			// attachment.Color = "#4af030"

			// _, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
			// if err != nil {
			// 	return fmt.Errorf("failed to post message: %w", err)
			// }
	// 		return
	// 	}),
	// )
	// if err != nil {
	// 	log.Fatal(err)
	// }

    // if strings.Contains(text, "hello") || strings.Contains(text, "hi") {
    //     attachment.Text = fmt.Sprintf("Hello %s", user.Name)
    //     attachment.Color = "#4af030"
    // } else if strings.Contains(text, "weather") {
    //     attachment.Text = fmt.Sprintf("Weather is sunny today. %s", user.Name)
    //     attachment.Color = "#4af030"
    // } else {
    //     attachment.Text = fmt.Sprintf("I am good. How are you %s?", user.Name)
    //     attachment.Color = "#4af030"
    // }

  return nil
}

func main() {
	fmt.Println("You will be assimilated!")

	godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
  channelID := os.Getenv("SLACK_CHANNEL_ID")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	fmt.Println("Token:", token)
	fmt.Println("Channel ID:", channelID)
	client := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))

	socketClient := socketmode.New(
        client,
        socketmode.OptionDebug(true),
        socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
    )

    ctx, cancel := context.WithCancel(context.Background())

    defer cancel()

	go func(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
        for {
            select {
            case <-ctx.Done():
                log.Println("Shutting down socketmode listener")
                return
            case event := <-socketClient.Events:

                switch event.Type {

                case socketmode.EventTypeEventsAPI:

                    eventsAPI, ok := event.Data.(slackevents.EventsAPIEvent)
                    if !ok {
                        log.Printf("Could not type cast the event to the EventsAPI: %v\n", event)
                        continue
                    }

                    socketClient.Ack(*event.Request)
                    err := HandleEventMessage(eventsAPI, client)
					if err != nil {
						log.Fatal(err)
					}
                }
            }
        }
    }(ctx, client, socketClient)

    socketClient.Run()
}