package tgnotify_test

import (
	"context"
	"fmt"
	"log"

	"gitlab.com/lyoneel/tg-notify"
)

// The options-struct style: build the options value, then send.
func Example_botSendMessageOpts() {
	bot := tgnotify.New("123456:ABC-TOKEN")
	id, err := bot.SendMessageOpts(context.Background(), "123456789", "deploy finished", &tgnotify.SendOptions{
		ParseMode: "HTML",
		Silent:    true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}

// The functional style: collect options with NewSendOptions.
func Example_botNewSendOptions() {
	bot := tgnotify.New("123456:ABC-TOKEN")
	id, err := bot.SendMessageOpts(context.Background(), "123456789", "deploy finished",
		tgnotify.NewSendOptions(tgnotify.WithSilent()))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}

// The environment style: resolve TELEGRAM_BOT_TOKEN and
// TELEGRAM_CHAT_ID, then send.
func Example_fromEnv() {
	bot, chatID, err := tgnotify.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	id, err := bot.SendMessageOpts(context.Background(), chatID, "deploy finished", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}

// A file upload with a caption.
func Example_botSendFileOpts() {
	bot := tgnotify.New("123456:ABC-TOKEN")
	id, err := bot.SendFileOpts(context.Background(), "123456789", tgnotify.TypeDocument, "report.pdf",
		tgnotify.NewSendOptions(tgnotify.WithCaption("weekly report")))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(id)
}
