package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/transport"
)

type AuthenticatedRunFunc func(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error

func withAuth(run AuthenticatedRunFunc) RunFunc {
	return func(ctx context.Context, runtime *Runtime, args []string) error {
		credential, err := requireAuth(ctx, runtime)
		if err != nil {
			return err
		}

		return run(ctx, runtime, args, credential)
	}
}

func requireAuth(ctx context.Context, runtime *Runtime) (credentials.Record, error) {
	if runtime.Credentials == nil {
		return credentials.Record{}, errors.New("CLI credentials are not configured. next: fix the credential store setup and rerun `lazyops login`")
	}

	record, err := runtime.Credentials.Load(ctx)
	if err != nil {
		if errors.Is(err, credentials.ErrNotFound) {
			return credentials.Record{}, errors.New("CLI is not logged in. next: run `lazyops login --email <email> --password <password>` or `lazyops login --provider <github|google>`")
		}

		return credentials.Record{}, fmt.Errorf("could not load CLI credentials. next: verify local credential storage and rerun `lazyops login`: %w", err)
	}

	if strings.TrimSpace(record.Token) == "" {
		return credentials.Record{}, errors.New("CLI credentials are empty. next: rerun `lazyops login` to issue a new PAT")
	}

	return record, nil
}

func authorizeRequest(request transport.Request, credential credentials.Record) transport.Request {
	authorized := request
	if authorized.Headers == nil {
		authorized.Headers = map[string]string{}
	} else {
		authorized.Headers = cloneStringMap(authorized.Headers)
	}

	authorized.Headers["Authorization"] = "Bearer " + credential.Token
	return authorized
}

func renderAuthorizedRequest(ctx context.Context, runtime *Runtime, title string, credential credentials.Record, request transport.Request) error {
	return renderRequest(ctx, runtime, title, authorizeRequest(request, credential))
}

func runAuthorizedSequence(ctx context.Context, runtime *Runtime, title string, credential credentials.Record, requests ...transport.Request) error {
	authorized := make([]transport.Request, 0, len(requests))
	for _, request := range requests {
		authorized = append(authorized, authorizeRequest(request, credential))
	}

	return runSequence(ctx, runtime, title, authorized...)
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}

	return cloned
}
