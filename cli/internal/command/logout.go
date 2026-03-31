package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/transport"
)

const logoutSpinnerThreshold = time.Second

func logoutCommand() *Command {
	return &Command{
		Name:    "logout",
		Summary: "Revoke or clear the local CLI session.",
		Usage:   "lazyops logout",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			if err := parseLogoutArgs(args); err != nil {
				return err
			}

			if runtime.Credentials == nil {
				return errors.New("CLI credentials are not configured. next: fix the credential store setup before using `lazyops logout`")
			}

			record, err := runtime.Credentials.Load(ctx)
			switch {
			case errors.Is(err, credentials.ErrNotFound):
				runtime.Output.Info("no local CLI session found")
				return nil
			case err != nil:
				return fmt.Errorf("could not load local CLI session. next: verify local credential storage and retry `lazyops logout`: %w", err)
			}

			response, err := doWithDelayedSpinner(ctx, runtime, logoutSpinnerThreshold, "revoking CLI PAT", func(ctx context.Context) (transport.Response, error) {
				body, marshalErr := json.Marshal(map[string]string{"token": record.Token})
				if marshalErr != nil {
					return transport.Response{}, marshalErr
				}

				return runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
					Method: "POST",
					Path:   "/api/v1/auth/pat/revoke",
					Body:   body,
				}, record))
			})
			if err != nil {
				return err
			}

			if err := parsePATRevokeResponse(response); err != nil {
				return err
			}

			if err := runtime.Credentials.Clear(ctx); err != nil {
				return fmt.Errorf("remote session was revoked but local credentials cleanup failed. next: remove the local credential entry and retry: %w", err)
			}

			runtime.Output.Success("logged out and revoked the remote CLI session")
			return nil
		},
	}
}

func parseLogoutArgs(args []string) error {
	flagSet := flag.NewFlagSet("logout", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	if err := flagSet.Parse(args); err != nil {
		return errors.New("invalid logout flags. next: run `lazyops logout` without extra arguments")
	}

	if flagSet.NArg() > 0 {
		return fmt.Errorf("unexpected logout arguments: %s. next: run `lazyops logout` without extra arguments", strings.Join(flagSet.Args(), " "))
	}

	return nil
}

func parsePATRevokeResponse(response transport.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}

	return parseAPIError(response)
}
