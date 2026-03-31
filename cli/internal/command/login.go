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

const loginSpinnerThreshold = time.Second

type cliLoginRequest struct {
	Method   string `json:"method"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Provider string `json:"provider,omitempty"`
}

type cliLoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"user"`
	Meta struct {
		StorageHint string `json:"storage_hint"`
		Provider    string `json:"provider,omitempty"`
		AuthMethod  string `json:"auth_method,omitempty"`
	} `json:"meta"`
}

type apiErrorResponse struct {
	Error    string `json:"error"`
	Message  string `json:"message"`
	NextStep string `json:"next_step"`
}

func loginCommand() *Command {
	return &Command{
		Name:    "login",
		Summary: "Authenticate and store CLI identity.",
		Usage:   "lazyops login [--email <email> --password <password>] | [--provider <github|google>]",
		Run: func(ctx context.Context, runtime *Runtime, args []string) error {
			loginRequest, err := parseLoginArgs(args)
			if err != nil {
				return err
			}

			body, err := json.Marshal(loginRequest)
			if err != nil {
				return err
			}

			response, err := doWithDelayedSpinner(ctx, runtime, loginSpinnerThreshold, "authenticating with LazyOps", func(ctx context.Context) (transport.Response, error) {
				return runtime.Transport.Do(ctx, transport.Request{
					Method: "POST",
					Path:   "/api/v1/auth/cli-login",
					Body:   body,
				})
			})
			if err != nil {
				return err
			}

			loginResponse, err := parseLoginResponse(response)
			if err != nil {
				return err
			}

			if runtime.Credentials == nil {
				return errors.New("credential store is not configured")
			}

			saveResult, err := runtime.Credentials.Save(ctx, credentials.Record{
				Token:       loginResponse.Token,
				UserID:      loginResponse.User.ID,
				DisplayName: loginResponse.User.DisplayName,
			})
			if err != nil {
				return fmt.Errorf("login succeeded but storing credentials failed: %w", err)
			}

			displayName := strings.TrimSpace(loginResponse.User.DisplayName)
			if displayName == "" {
				displayName = "unknown user"
			}

			modeDescription := "email/password"
			if strings.EqualFold(loginRequest.Method, "browser") {
				modeDescription = fmt.Sprintf("%s browser OAuth", loginRequest.Provider)
			}

			runtime.Output.Success("logged in as %s via %s", displayName, modeDescription)
			runtime.Output.Info("credentials stored in %s", saveResult.Backend)

			if strings.TrimSpace(loginResponse.Meta.StorageHint) != "" && saveResult.Backend != loginResponse.Meta.StorageHint {
				runtime.Output.Warn("backend suggested %s storage, using %s instead", loginResponse.Meta.StorageHint, saveResult.Backend)
			}

			return nil
		},
	}
}

func parseLoginArgs(args []string) (cliLoginRequest, error) {
	flagSet := flag.NewFlagSet("login", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	email := flagSet.String("email", "", "email address for direct login")
	password := flagSet.String("password", "", "password for direct login")
	provider := flagSet.String("provider", "", "browser provider: github or google")

	if err := flagSet.Parse(args); err != nil {
		return cliLoginRequest{}, fmt.Errorf("invalid login flags. next: use `lazyops login --email <email> --password <password>` or `lazyops login --provider <github|google>`")
	}

	if flagSet.NArg() > 0 {
		return cliLoginRequest{}, fmt.Errorf("unexpected login arguments: %s. next: use `lazyops login --email <email> --password <password>` or `lazyops login --provider <github|google>`", strings.Join(flagSet.Args(), " "))
	}

	request := cliLoginRequest{
		Email:    strings.TrimSpace(*email),
		Password: strings.TrimSpace(*password),
		Provider: strings.ToLower(strings.TrimSpace(*provider)),
	}

	switch {
	case request.Provider != "" && (request.Email != "" || request.Password != ""):
		return cliLoginRequest{}, errors.New("choose one login mode only. next: either use email/password or `--provider <github|google>`")
	case request.Provider != "":
		if request.Provider != "github" && request.Provider != "google" {
			return cliLoginRequest{}, fmt.Errorf("unsupported provider %q. next: use `--provider github` or `--provider google`", request.Provider)
		}
		request.Method = "browser"
		return request, nil
	case request.Email == "" && request.Password == "":
		return cliLoginRequest{}, errors.New("login input is missing. next: use `lazyops login --email <email> --password <password>` or `lazyops login --provider <github|google>`")
	case request.Email == "" || request.Password == "":
		return cliLoginRequest{}, errors.New("email/password login requires both values. next: provide `--email` and `--password`, or switch to `--provider github` or `--provider google`")
	default:
		request.Method = "password"
		return request, nil
	}
}

func parseLoginResponse(response transport.Response) (cliLoginResponse, error) {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return cliLoginResponse{}, parseAPIError(response)
	}

	var payload cliLoginResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return cliLoginResponse{}, fmt.Errorf("could not decode login response: %w", err)
	}

	if strings.TrimSpace(payload.Token) == "" {
		return cliLoginResponse{}, errors.New("login response is missing a PAT. next: verify the backend returns `token` for `POST /api/v1/auth/cli-login`")
	}

	return payload, nil
}

func parseAPIError(response transport.Response) error {
	var payload apiErrorResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return fmt.Errorf("request failed with status %d. next: verify the backend contract and try again", response.StatusCode)
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = fmt.Sprintf("request failed with status %d", response.StatusCode)
	}

	nextStep := strings.TrimSpace(payload.NextStep)
	if nextStep == "" {
		nextStep = "check your credentials or provider selection and retry `lazyops login`"
	}

	if strings.HasSuffix(message, ".") || strings.HasSuffix(message, "!") || strings.HasSuffix(message, "?") {
		return fmt.Errorf("%s next: %s", message, nextStep)
	}

	return fmt.Errorf("%s. next: %s", message, nextStep)
}

func doWithDelayedSpinner(
	ctx context.Context,
	runtime *Runtime,
	threshold time.Duration,
	message string,
	run func(ctx context.Context) (transport.Response, error),
) (transport.Response, error) {
	type result struct {
		response transport.Response
		err      error
	}

	resultCh := make(chan result, 1)
	go func() {
		response, err := run(ctx)
		resultCh <- result{response: response, err: err}
	}()

	timer := time.NewTimer(threshold)
	defer timer.Stop()

	spinner := runtime.SpinnerFactory.New()
	started := false

	for {
		select {
		case <-ctx.Done():
			if started {
				spinner.Stop("")
			}
			return transport.Response{}, ctx.Err()
		case outcome := <-resultCh:
			if started {
				spinner.Stop("")
			}
			return outcome.response, outcome.err
		case <-timer.C:
			if !started {
				spinner.Start(message)
				started = true
			}
		}
	}
}
