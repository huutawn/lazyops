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
		Usage:   "lazyops logout [--yes]",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			confirmed, err := parseLogoutArgs(args)
			if err != nil {
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

			if !confirmed {
				return errors.New("logout will revoke the remote PAT and clear local credentials. next: confirm with `lazyops logout --yes`")
			}

			revoked, warning, err := revokePATIfAvailable(ctx, runtime, record)
			if err != nil {
				return err
			}

			if err := runtime.Credentials.Clear(ctx); err != nil {
				return fmt.Errorf("CLI logout could not clear local credentials. next: remove the local credential entry and retry `lazyops logout`: %w", err)
			}

			if warning != "" {
				runtime.Output.Warn("%s", warning)
			}

			if revoked {
				runtime.Output.Success("logged out and revoked the remote CLI session")
				return nil
			}

			runtime.Output.Success("logged out and cleared the local CLI session")
			return nil
		},
	}
}

func parseLogoutArgs(args []string) (bool, error) {
	flagSet := flag.NewFlagSet("logout", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	yes := flagSet.Bool("yes", false, "confirm logout without interactive prompt")

	if err := flagSet.Parse(args); err != nil {
		return false, errors.New("invalid logout flags. next: run `lazyops logout --yes`")
	}

	if flagSet.NArg() > 0 {
		return false, fmt.Errorf("unexpected logout arguments: %s. next: run `lazyops logout --yes`", strings.Join(flagSet.Args(), " "))
	}

	return *yes, nil
}

func revokePATIfAvailable(ctx context.Context, runtime *Runtime, record credentials.Record) (bool, string, error) {
	if strings.TrimSpace(record.Token) == "" {
		return false, "stored CLI session had no PAT; cleared local session only", nil
	}

	if runtime.Transport == nil {
		return false, "remote PAT revoke transport is unavailable; cleared local session only", nil
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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false, "", err
		}

		return false, "remote PAT revoke could not be reached; cleared local session only", nil
	}

	return parsePATRevokeResponse(response)
}

func parsePATRevokeResponse(response transport.Response) (bool, string, error) {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return true, "", nil
	}

	switch response.StatusCode {
	case 401:
		return false, "remote CLI PAT was already invalid or revoked; cleared local session only", nil
	case 404, 405, 501:
		return false, "remote PAT revoke endpoint is unavailable; cleared local session only", nil
	default:
		return false, fmt.Sprintf("remote PAT revoke did not complete (status %d); local CLI session was cleared but the remote PAT may still be active", response.StatusCode), nil
	}
}
