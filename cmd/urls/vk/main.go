package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/api/params"

	"github.com/VerS7/mediadump/parse"
)

var (
	Token        = flag.String("token", "", "User access token (you can found it on https://vkhost.github.io/)")
	TargetPeerID = flag.Int("uid", 0, "Peer ID with target chat (usually your own uid)")

	OutputFilePath   = flag.String("output", "links.txt", "Output file with URLs from Vkontakte messages")
	OutputFileAppend = flag.Bool("append", false, "Append parsed URLs to end of output file")

	BatchDelay = flag.Int("delay", 1, "Wait time between requesting messages history in seconds")
	BatchLimit = flag.Int("limit", 200, "Messages limit per one history request. Recommended default")
)

func VerifyArgs() error {
	// Token
	if *Token == "" {
		return fmt.Errorf("token not provided")
	}

	return nil
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Vkontakte URLs dump usage\n")
		flag.PrintDefaults()
	}

	flag.Parse()
}

func main() {
	if err := VerifyArgs(); err != nil {
		panic(err)
	}

	token := *Token
	targetPeerID := *TargetPeerID
	limit := *BatchLimit
	once := sync.Once{}

	var fileFlag int
	if *OutputFileAppend {
		fileFlag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	} else {
		fileFlag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	file, _ := os.OpenFile(*OutputFilePath, fileFlag, 0666)
	defer file.Close()

	sb := strings.Builder{}

	vk := api.NewVK(token)

	offset := 0

	b := params.NewMessagesGetHistoryBuilder()
	b.Count(limit)
	b.PeerID(targetPeerID)
	b.Offset(offset)
	b.Rev(0)

	err := func() error {
		fmt.Println("Begin parsing message history...")
		for {
			response, err := vk.MessagesGetHistory(b.Params)
			if err != nil {
				return fmt.Errorf("failed to get history: %w", err)
			}

			once.Do(func() {
				fmt.Printf("Found %d message(s) in chat with PeerID: %d\n", response.Count, targetPeerID)
			})

			if len(response.Items) == 0 {
				return nil
			}

			fmt.Printf("Found batch of %d message(s). Offset: %d\n", len(response.Items), offset)

			for _, m := range response.Items {
				for _, url := range parse.URLsFromString(m.Text) {
					fmt.Fprintf(&sb, "VK %d %d %s\n", m.ID, m.Date, url)
				}
			}

			offset += limit
			b.Offset(offset)

			time.Sleep(time.Duration(*BatchDelay) * time.Second)
		}
	}()

	if err != nil {
		panic(err)
	}

	if _, err := file.WriteString(sb.String()); err != nil {
		panic(err)
	}

	fmt.Println("Done!")
}
