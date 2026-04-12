package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/transport"
)

const tracesSpinnerThreshold = time.Second

type tracesArgs struct {
	CorrelationID string
}

func tracesCommand() *Command {
	return &Command{
		Name:    "traces",
		Summary: "Inspect distributed request flow by correlation id.",
		Usage:   "lazyops traces <correlation-id>",
		Run:     withAuth(runTraces),
	}
}

func runTraces(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	traceArgs, err := parseTracesArgs(args)
	if err != nil {
		return err
	}

	request := authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/api/v1/traces/" + traceArgs.CorrelationID,
	}, credential)

	response, err := doWithDelayedSpinner(ctx, runtime, tracesSpinnerThreshold, "fetching trace data", func(ctx context.Context) (transport.Response, error) {
		return runtime.Transport.Do(ctx, request)
	})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return parseAPIError(response)
	}

	summary, err := contracts.DecodeTraceSummary(response.Body)
	if err != nil {
		return fmt.Errorf("could not decode the trace response. next: verify the `/api/v1/traces` contract and retry `lazyops traces %s`: %w", traceArgs.CorrelationID, err)
	}

	printTraceSummary(runtime, traceArgs, summary)
	return nil
}

func parseTracesArgs(args []string) (tracesArgs, error) {
	if len(args) == 0 {
		return tracesArgs{}, errors.New("correlation id is required. next: rerun `lazyops traces <correlation-id>`")
	}

	correlationID := strings.TrimSpace(args[0])
	if correlationID == "" || strings.HasPrefix(correlationID, "-") {
		return tracesArgs{}, errors.New("correlation id is required. next: rerun `lazyops traces <correlation-id>` with a valid correlation id")
	}

	flagSet := flag.NewFlagSet("traces", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	if err := flagSet.Parse(args[1:]); err != nil {
		return tracesArgs{}, errors.New("invalid traces flags. next: use `lazyops traces <correlation-id>`")
	}
	if flagSet.NArg() > 0 {
		return tracesArgs{}, fmt.Errorf("unexpected traces arguments: %s. next: use `lazyops traces <correlation-id>`", strings.Join(flagSet.Args(), " "))
	}

	return tracesArgs{
		CorrelationID: correlationID,
	}, nil
}

func printTraceSummary(runtime *Runtime, args tracesArgs, summary contracts.TraceSummary) {
	runtime.Output.Print("trace")
	runtime.Output.Info("correlation id: %s", summary.CorrelationID)

	if len(summary.ServicePath) > 0 {
		runtime.Output.Print("")
		runtime.Output.Info("service path:")
		for i, svc := range summary.ServicePath {
			prefix := "  "
			if i == 0 {
				prefix = "-> "
			}
			if i < len(summary.ServicePath)-1 {
				runtime.Output.Print("%s%s", prefix, svc)
			} else {
				runtime.Output.Print("%s%s (final)", prefix, svc)
			}
		}
	}

	if len(summary.NodeHops) > 0 {
		runtime.Output.Print("")
		runtime.Output.Info("node hops:")
		for _, hop := range summary.NodeHops {
			runtime.Output.Print("  %s", hop)
		}
	}

	if summary.TotalLatencyMS > 0 {
		runtime.Output.Print("")
		runtime.Output.Info("total latency: %dms", summary.TotalLatencyMS)
	}

	if strings.TrimSpace(summary.LatencyHotspot) != "" {
		runtime.Output.Warn("latency hotspot: %s", summary.LatencyHotspot)
	}

	if len(summary.ServicePath) == 0 {
		runtime.Output.Warn("trace %s returned an empty service path. next: verify the correlation id is correct and that tracing is enabled for this request", args.CorrelationID)
	}
}
