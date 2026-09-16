package main

import (
	"context"
	"fmt"

	"gitlab.com/lyoneel/tgnotify"
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

	user, err := retryWithBackoff(func() (*tgnotify.User, error) {
		return bot.GetMe(ctx)
	}, opts.noRetry, opts.retries, opts.baseWait)
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
