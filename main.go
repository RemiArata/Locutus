package main


import (
	"fmt"
	"os"
	"log"
	"context"
	"errors"
	"strings"

	"Locutus/helpers"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
    "github.com/slack-go/slack/socketmode"

	// "github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

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
        return err
    }

	fmt.Println("Request from user", user)
 
    text := strings.ToLower(event.Text)
	fmt.Println("****************************************")
	location := strings.IndexRune(text, '<')
	fmt.Println(text[:location])
	formattedText := text[:location]
 
    attachment := slack.Attachment{}

	ctx := context.Background()

	str, err := llm.Call(ctx, formattedText)
	if err != nil {
		log.Fatal(err)
	}
	attachment.Text = str
	attachment.Color = "#4af030"


	fmt.Println(event.ThreadTimeStamp)


	if event.ThreadTimeStamp != "" {
		_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment), slack.MsgOptionTS(event.TimeStamp))
		if err != nil {
			return fmt.Errorf("failed to post message: %w", err)
		}
	} else {
		_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
		if err != nil {
			return fmt.Errorf("failed to post message: %w", err)
		}
	}	

    return nil
}


func main() {
	fmt.Println("You will be assimilated!")

	var token string
	// var channelId string
	var appToken string

	if helpers.CheckEnvExists() {
		// token, channelID, appToken = helpers.LoadEnvVariables()
		token, appToken = helpers.LoadEnvVariables()
	} else {
		fmt.Println(".ENV does not exist. Please add and try again :).")
		os.Exit(1)
	}

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