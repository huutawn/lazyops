package command

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
)

const logsSpinnerThreshold = time.Second

var allowedLogLevels = []string{"debug", "info", "warn", "error"}

type logsArgs struct {
	Service  string
	Level    string
	Contains string
	Node     string
	Cursor   string
	Limit    int
}

func logsCommand() *Command {
	return &Command{
		Name:    "logs",
		Summary: "Inspect service logs.",
		Usage:   "lazyops logs <service> [--level <debug|info|warn|error>] [--contains <text>] [--node <node>] [--cursor <cursor>] [--limit <count>]",
		Run:     withAuth(runLogs),
	}
}

func runLogs(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	logArgs, err := parseLogsArgs(args)
	if err != nil {
		return err
	}

	repoRoot, err := repo.FindRepoRoot(".")
	if err != nil {
		if errors.Is(err, repo.ErrRepoRootNotFound) {
			return fmt.Errorf("could not find the repository root. next: run `lazyops logs %s` from inside a git repository", logArgs.Service)
		}
		return fmt.Errorf("could not determine the repository root. next: verify the working tree is readable and retry `lazyops logs %s`: %w", logArgs.Service, err)
	}

	metadata, err := readDoctorMetadata(repoRoot, nil)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lazyops.yaml was not found at the repo root. next: run `lazyops init` before `lazyops logs %s`", logArgs.Service)
		}
		return fmt.Errorf("could not read lazyops.yaml. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}
	if err := metadata.ValidateDoctorContract(); err != nil {
		return fmt.Errorf("lazyops.yaml is incomplete. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}
	if err := validateRequestedLogService(metadata, logArgs.Service); err != nil {
		return err
	}

	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return err
	}
	project, err := selectProjectForLink(projectsResponse.Projects, metadata.ProjectSlug, credential)
	if err != nil {
		return err
	}

	request := authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/ws/logs/stream",
		Query:  buildLogsQuery(project.ID, logArgs),
	}, credential)

	response, err := doWithDelayedSpinner(ctx, runtime, logsSpinnerThreshold, "streaming service logs", func(ctx context.Context) (transport.Response, error) {
		return runtime.Transport.Do(ctx, request)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			runtime.Output.Warn("log stream cancelled for service %s in project %s", logArgs.Service, project.Slug)
			return nil
		}
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return parseAPIError(response)
	}

	preview, err := contracts.DecodeLogsStreamPreview(response.Body)
	if err != nil {
		return fmt.Errorf("could not decode the log stream response. next: verify the `/ws/logs/stream` contract and retry `lazyops logs %s`: %w", logArgs.Service, err)
	}
	if strings.TrimSpace(preview.Service) != logArgs.Service {
		return fmt.Errorf("log stream responded for service %q instead of %q. next: verify the backend logs contract filters logs by service", preview.Service, logArgs.Service)
	}

	preview = filterLogsPreview(preview, logArgs)
	printLogsPreview(runtime, project, logArgs, preview)
	return nil
}

func parseLogsArgs(args []string) (logsArgs, error) {
	if len(args) == 0 {
		return logsArgs{}, errors.New("service name is required. next: rerun `lazyops logs <service>` with a declared service name")
	}

	service := strings.TrimSpace(args[0])
	if service == "" || strings.HasPrefix(service, "-") {
		return logsArgs{}, errors.New("service name is required. next: rerun `lazyops logs <service>` with a declared service name")
	}

	flagSet := flag.NewFlagSet("logs", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	level := flagSet.String("level", "", "log level filter: debug, info, warn, or error")
	contains := flagSet.String("contains", "", "substring filter applied to log messages")
	node := flagSet.String("node", "", "node filter")
	cursor := flagSet.String("cursor", "", "resume from a stream cursor")
	limit := flagSet.Int("limit", 50, "maximum number of log lines to print")

	if err := flagSet.Parse(args[1:]); err != nil {
		return logsArgs{}, errors.New("invalid logs flags. next: use `lazyops logs <service> [--level <debug|info|warn|error>] [--contains <text>] [--node <node>] [--cursor <cursor>] [--limit <count>]`")
	}
	if flagSet.NArg() > 0 {
		return logsArgs{}, fmt.Errorf("unexpected logs arguments: %s. next: use `lazyops logs <service> [--level <debug|info|warn|error>] [--contains <text>] [--node <node>] [--cursor <cursor>] [--limit <count>]`", strings.Join(flagSet.Args(), " "))
	}

	normalizedLevel := strings.ToLower(strings.TrimSpace(*level))
	if normalizedLevel != "" && !slices.Contains(allowedLogLevels, normalizedLevel) {
		return logsArgs{}, fmt.Errorf("log level filter %q is invalid. next: use one of debug, info, warn, or error", *level)
	}
	if *limit <= 0 {
		return logsArgs{}, fmt.Errorf("log limit %d is invalid. next: use `--limit <positive-number>`", *limit)
	}

	return logsArgs{
		Service:  service,
		Level:    normalizedLevel,
		Contains: strings.TrimSpace(*contains),
		Node:     strings.TrimSpace(*node),
		Cursor:   strings.TrimSpace(*cursor),
		Limit:    *limit,
	}, nil
}

func validateRequestedLogService(metadata lazyyaml.DoctorMetadata, service string) error {
	knownServices := make([]string, 0, len(metadata.Services))
	for _, candidate := range metadata.Services {
		knownServices = append(knownServices, candidate.Name)
		if candidate.Name == service {
			return nil
		}
	}

	if len(knownServices) == 0 {
		return fmt.Errorf("lazyops.yaml does not declare any services for project %q. next: rerun `lazyops init` before using `lazyops logs`", metadata.ProjectSlug)
	}

	slices.Sort(knownServices)
	return fmt.Errorf("service %q is not declared in lazyops.yaml for project %q. next: choose one of %s or rerun `lazyops init` if the repo layout changed", service, metadata.ProjectSlug, strings.Join(knownServices, ", "))
}

func buildLogsQuery(projectID string, args logsArgs) map[string]string {
	query := map[string]string{
		"project": projectID,
		"service": args.Service,
		"limit":   strconv.Itoa(args.Limit),
	}
	if strings.TrimSpace(args.Level) != "" {
		query["level"] = args.Level
	}
	if strings.TrimSpace(args.Contains) != "" {
		query["contains"] = args.Contains
	}
	if strings.TrimSpace(args.Node) != "" {
		query["node"] = args.Node
	}
	if strings.TrimSpace(args.Cursor) != "" {
		query["cursor"] = args.Cursor
	}
	return query
}

func filterLogsPreview(preview contracts.LogsStreamPreview, args logsArgs) contracts.LogsStreamPreview {
	filtered := make([]contracts.LogLine, 0, len(preview.Lines))
	for _, line := range preview.Lines {
		if !logLineMatches(line, args) {
			continue
		}
		filtered = append(filtered, line)
		if len(filtered) >= args.Limit {
			break
		}
	}

	preview.Lines = filtered
	return preview
}

func logLineMatches(line contracts.LogLine, args logsArgs) bool {
	if strings.TrimSpace(args.Level) != "" && !strings.EqualFold(line.Level, args.Level) {
		return false
	}
	if strings.TrimSpace(args.Node) != "" && !strings.EqualFold(strings.TrimSpace(line.Node), args.Node) {
		return false
	}
	if strings.TrimSpace(args.Contains) != "" && !strings.Contains(strings.ToLower(line.Message), strings.ToLower(args.Contains)) {
		return false
	}
	return true
}

func printLogsPreview(runtime *Runtime, project contracts.Project, args logsArgs, preview contracts.LogsStreamPreview) {
	runtime.Output.Print("logs stream")
	runtime.Output.Info("project: %s (%s)", project.Name, project.Slug)
	runtime.Output.Info("service: %s", args.Service)
	printLogsFilters(runtime, args)
	if strings.TrimSpace(preview.Cursor) != "" {
		runtime.Output.Info("cursor: %s", preview.Cursor)
	}

	if len(preview.Lines) == 0 {
		runtime.Output.Warn("no log lines match the current filters for service %s in project %s", args.Service, project.Slug)
		return
	}

	for _, line := range preview.Lines {
		runtime.Output.Print("%s %-5s [%s] %s", line.Timestamp.Format(time.RFC3339), strings.ToUpper(line.Level), fallbackLogNode(line.Node), line.Message)
	}
}

func printLogsFilters(runtime *Runtime, args logsArgs) {
	applied := []string{}
	if strings.TrimSpace(args.Level) != "" {
		applied = append(applied, "level="+args.Level)
	}
	if strings.TrimSpace(args.Contains) != "" {
		applied = append(applied, "contains="+args.Contains)
	}
	if strings.TrimSpace(args.Node) != "" {
		applied = append(applied, "node="+args.Node)
	}
	if strings.TrimSpace(args.Cursor) != "" {
		applied = append(applied, "cursor="+args.Cursor)
	}
	applied = append(applied, "limit="+strconv.Itoa(args.Limit))

	runtime.Output.Info("filters: %s", strings.Join(applied, ", "))
}

func fallbackLogNode(node string) string {
	if strings.TrimSpace(node) == "" {
		return "unknown-node"
	}
	return node
}
