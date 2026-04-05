package command

import (
	"context"
	"fmt"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/transport"
)

func fetchValidateLazyopsYAML(
	ctx context.Context,
	runtime *Runtime,
	credential credentials.Record,
	projectID string,
	document lazyyaml.Document,
) (contracts.ValidateLazyopsYAMLResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "POST",
		Path:   fmt.Sprintf("/api/v1/projects/%s/init/validate-lazyops-yaml", projectID),
		Body:   mustMarshalJSON(document),
	}, credential))
	if err != nil {
		return contracts.ValidateLazyopsYAMLResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.ValidateLazyopsYAMLResponse{}, parseAPIError(response)
	}

	validation, err := contracts.DecodeValidateLazyopsYAMLResponse(response.Body)
	if err != nil {
		return contracts.ValidateLazyopsYAMLResponse{}, fmt.Errorf("could not decode the lazyops.yaml validation response. next: verify `POST /api/v1/projects/:id/init/validate-lazyops-yaml` returns the documented schema: %w", err)
	}
	return validation, nil
}
