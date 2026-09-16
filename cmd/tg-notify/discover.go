package main

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/lyoneel/tgnotify"
)

func runDiscover(ctx context.Context, opts options) error {
	token, err := resolveTokenOnly(opts.token)
	if err != nil {
		return err
	}

	bot := tgnotify.New(token)
	if err := configureBot(bot, opts); err != nil {
		return err
	}
	updates, err := retryWithBackoff(func() ([]tgnotify.Update, error) {
		return bot.GetUpdates(ctx, opts.offset)
	}, opts.noRetry, opts.retries, opts.baseWait)
	if err != nil {
		return err
	}
	if len(updates) == 0 {
		return errors.New("no updates found; open your bot in Telegram, send /start and any message, then retry with --discover-chat-id")
	}
	latest := updates[len(updates)-1]
	id, ok := latest.ChatID()
	if !ok {
		return errors.New("no message found in latest update")
	}
	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"chat_id\":%d}\n", id)
		return nil
	}
	fmt.Println(id)
	return nil
}
