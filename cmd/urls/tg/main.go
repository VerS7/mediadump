package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"github.com/VerS7/mediadump/parse"
)

type AuthCodeHandler struct{}

func (a *AuthCodeHandler) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter auth code: ")
	code, err := reader.ReadString('\n')
	return code, err
}

func VerifyArgs() error {
	// App
	if *AppID == 0 {
		return fmt.Errorf("app ID not provided")
	}
	if *AppHash == "" {
		return fmt.Errorf("app hash not provided")
	}

	// User account credentials
	if *PhoneNumber == "" {
		return fmt.Errorf("phone number not provided")
	}
	if !strings.Contains(*PhoneNumber, "+") {
		return fmt.Errorf("provided incorrect phone number")
	}
	if *Password == "" {
		return fmt.Errorf("password not provided")
	}
	if *UserID == 0 {
		return fmt.Errorf("user ID not provided")
	}

	return nil
}

var (
	AppID   = flag.Int("app-id", 0, "Telegram API App ID")
	AppHash = flag.String("app-hash", "", "Telegram API App hash")

	PhoneNumber = flag.String("phone", "", "Your Telegram phone number")
	Password    = flag.String("password", "", "Your Telegram password")

	UserID = flag.Int("uid", 0, "User ID with target chat (usually your own uid)")

	StoreSession    = flag.Bool("session", false, "Store session to file (prevent re-auth)")
	SessionFilePath = flag.String("session-file", "session.json", "File to store session")

	OutputFilePath   = flag.String("output", "links.txt", "Output file with URLs from Telegram messages")
	OutputFileAppend = flag.Bool("append", false, "Append parsed URLs to end of output file")

	BatchDelay = flag.Int("delay", 1, "Wait time between requesting messages history in seconds")
	BatchLimit = flag.Int("limit", 100, "Messages limit per one history request. Recommended default")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Telegram URLs dump usage\n")
		flag.PrintDefaults()
	}

	flag.Parse()
}

func main() {
	if err := VerifyArgs(); err != nil {
		panic(err)
	}

	appID := *AppID
	appHash := *AppHash
	targetUserID := *UserID

	opts := telegram.Options{}

	if *StoreSession {
		fmt.Println("Initializing session from file...")

		if _, err := os.Stat(*SessionFilePath); errors.Is(err, os.ErrNotExist) {
			fmt.Println("Session file not found. Creating...")
		}

		opts.SessionStorage = &session.FileStorage{
			Path: *SessionFilePath,
		}
	}

	client := telegram.NewClient(appID, appHash, opts)

	var fileFlag int
	if *OutputFileAppend {
		fileFlag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	} else {
		fileFlag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	file, _ := os.OpenFile(*OutputFilePath, fileFlag, 0666)
	defer file.Close()

	sb := strings.Builder{}

	err := client.Run(context.Background(), func(ctx context.Context) error {
		flow := auth.NewFlow(
			auth.Constant(
				*PhoneNumber,
				*Password,
				&AuthCodeHandler{},
			),
			auth.SendCodeOptions{},
		)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		api := client.API()
		peer := &tg.InputPeerUser{
			UserID:     int64(targetUserID),
			AccessHash: 0,
		}

		limit := *BatchLimit
		req := tg.MessagesGetHistoryRequest{
			Peer:  peer,
			Limit: limit,
		}

		fmt.Println("Begin parsing message history...")
		for {
			msgs, err := api.MessagesGetHistory(ctx, &req)
			if err != nil {
				return fmt.Errorf("failed to get history: %w", err)
			}

			messages, ok := msgs.(*tg.MessagesMessagesSlice)
			if !ok {
				log.Println("Unexpected response type. Skip...")
				continue
			}

			if len(messages.Messages) == 0 {
				return nil
			}

			fmt.Printf("Found batch of %d message(s). OffsetID: %d\n", len(messages.Messages), req.OffsetID)

			minID := 0

			for _, msg := range messages.Messages {
				if m, ok := msg.(*tg.Message); ok {
					for _, url := range parse.URLsFromString(m.Message) {
						fmt.Fprintf(&sb, "TG %d %d %s\n", m.ID, m.Date, url)
					}

					if minID == 0 || m.ID < minID {
						minID = m.ID
					}
				}

				if minID == 0 {
					break
				}

				req.OffsetID = minID
			}

			time.Sleep(time.Duration(*BatchDelay) * time.Second)
		}
	})

	if err != nil {
		panic(err)
	}

	if _, err := file.WriteString(sb.String()); err != nil {
		panic(err)
	}

	fmt.Println("Done!")
}
