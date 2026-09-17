package main

import (
	"context"
	"fmt"

	"gitlab.com/lyoneel/tg-notify"
)

// runWhoami prints the bot's identity, validating the token via getMe.
func runWhoami(ctx context.Context, opts options) error {
	token, err := resolveTokenOnly(opts.token)
	if err != nil {
		return err
	}
	bot := tgnotify.New(token)
	if err := configureBot(bot, opts); err != nil {
		return err
	}

	user, err := bot.GetMe(ctx)
	if err != nil {
		return err
	}

	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"id\":%d,\"is_bot\":%t,\"first_name\":%q,\"username\":%q}\n",
			user.ID, user.IsBot, user.FirstName, user.Username)
		return nil
	}
	fmt.Printf("@%s (id: %d)\n", user.Username, user.ID)
	return nil
}
