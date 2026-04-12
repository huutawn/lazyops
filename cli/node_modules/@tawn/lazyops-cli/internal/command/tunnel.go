package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
	"lazyops-cli/internal/tunnel"
)

const tunnelSpinnerThreshold = time.Second

var globalTunnelManager = tunnel.NewManager()

func tunnelCommand() *Command {
	return &Command{
		Name:    "tunnel",
		Summary: "Open an optional debug tunnel.",
		Usage:   "lazyops tunnel <db|tcp|stop|list>",
		Subcommands: []*Command{
			tunnelDBCommand(),
			tunnelTCPCommand(),
			tunnelStopCommand(),
			tunnelListCommand(),
		},
	}
}

func tunnelDBCommand() *Command {
	return &Command{
		Name:    "db",
		Summary: "Open a debug database tunnel.",
		Usage:   "lazyops tunnel db [--port <local-port>] [--remote <remote-addr>] [--timeout <duration>]",
		Run:     withAuth(runTunnelDB),
	}
}

func tunnelTCPCommand() *Command {
	return &Command{
		Name:    "tcp",
		Summary: "Open a debug TCP tunnel.",
		Usage:   "lazyops tunnel tcp --port <local-port> --remote <remote-addr> [--timeout <duration>]",
		Run:     withAuth(runTunnelTCP),
	}
}

func tunnelStopCommand() *Command {
	return &Command{
		Name:    "stop",
		Summary: "Stop an active debug tunnel session.",
		Usage:   "lazyops tunnel stop <session-id>",
		Run:     withAuth(runTunnelStop),
	}
}

func tunnelListCommand() *Command {
	return &Command{
		Name:    "list",
		Summary: "List active debug tunnel sessions.",
		Usage:   "lazyops tunnel list",
		Run:     withAuth(runTunnelList),
	}
}

type tunnelArgs struct {
	Port    int
	Remote  string
	Timeout time.Duration
}

func runTunnelDB(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	return runTunnel(ctx, runtime, args, credential, tunnel.TypeDB)
}

func runTunnelTCP(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	return runTunnel(ctx, runtime, args, credential, tunnel.TypeTCP)
}

func runTunnelStop(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	if len(args) < 1 {
		return fmt.Errorf("session id is required. next: rerun `lazyops tunnel stop <session-id>`")
	}

	sessionID := strings.TrimSpace(args[0])
	if sessionID == "" {
		return fmt.Errorf("session id is required. next: rerun `lazyops tunnel stop <session-id>` with a valid session id")
	}

	request := authorizeRequest(transport.Request{
		Method: "DELETE",
		Path:   "/api/v1/tunnels/sessions/" + sessionID,
	}, credential)

	response, err := doWithDelayedSpinner(ctx, runtime, tunnelSpinnerThreshold, "stopping tunnel session", func(ctx context.Context) (transport.Response, error) {
		return runtime.Transport.Do(ctx, request)
	})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return parseAPIError(response)
	}

	if err := globalTunnelManager.Stop(sessionID); err != nil {
		runtime.Output.Warn("local session tracker: %v", err)
	}

	runtime.Output.Success("tunnel session %s stopped", sessionID)
	runtime.Output.Info("local port released")
	return nil
}

func runTunnelList(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	globalTunnelManager.Cleanup()

	active := globalTunnelManager.ActiveSessions()
	if len(active) == 0 {
		runtime.Output.Info("no active tunnel sessions")
		runtime.Output.Info("start one with `lazyops tunnel db` or `lazyops tunnel tcp`")
		return nil
	}

	runtime.Output.Info("active tunnel sessions (%d)", len(active))
	runtime.Output.Print("")
	for _, session := range active {
		runtime.Output.Print("  %s  type=%s  port=%d  remote=%s  expires=%s",
			session.ID,
			session.Type,
			session.LocalPort,
			session.Remote,
			session.ExpiresAt.Format(time.RFC3339),
		)
	}
	runtime.Output.Print("")
	runtime.Output.Info("stop with: lazyops tunnel stop <session-id>")
	return nil
}

func runTunnel(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record, tunnelType tunnel.TunnelType) error {
	tArgs, err := parseTunnelArgs(args, tunnelType)
	if err != nil {
		return err
	}

	repoRoot, err := repo.FindRepoRoot(".")
	if err != nil {
		return fmt.Errorf("could not find the repository root. next: run `lazyops tunnel %s` from inside a git repository: %w", tunnelType, err)
	}

	metadata, err := lazyyaml.ReadDoctorMetadata(repoRoot)
	if err != nil {
		return fmt.Errorf("could not read lazyops.yaml. next: run `lazyops init` before opening a debug tunnel: %w", err)
	}
	if err := metadata.ValidateDoctorContract(); err != nil {
		return fmt.Errorf("lazyops.yaml is incomplete. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}

	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return err
	}
	project, err := selectProjectForLink(projectsResponse.Projects, metadata.ProjectSlug, credential)
	if err != nil {
		return err
	}

	cfg := tunnel.DefaultConfig(tunnelType)
	if tArgs.Port > 0 {
		cfg.LocalPort = tArgs.Port
	}
	if strings.TrimSpace(tArgs.Remote) != "" {
		cfg.Remote = tArgs.Remote
	}
	if tArgs.Timeout > 0 {
		cfg.Timeout = tArgs.Timeout
	}
	cfg.ProjectID = project.ID
	cfg.TargetRef = metadata.TargetRef

	if err := cfg.Validate(); err != nil {
		return err
	}

	portChecker := tunnel.DefaultPortChecker{}
	if err := portChecker.IsPortAvailable(cfg.LocalPort); err != nil {
		return err
	}

	request := authorizeRequest(transport.Request{
		Method: "POST",
		Path:   tunnelSessionPath(tunnelType),
		Body:   buildTunnelSessionBody(cfg),
	}, credential)

	response, err := doWithDelayedSpinner(ctx, runtime, tunnelSpinnerThreshold, "creating tunnel session", func(ctx context.Context) (transport.Response, error) {
		return runtime.Transport.Do(ctx, request)
	})
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return parseAPIError(response)
	}

	session, err := contracts.DecodeTunnelSession(response.Body)
	if err != nil {
		return fmt.Errorf("could not decode the tunnel session response. next: verify the tunnel session contract and retry `lazyops tunnel %s`: %w", tunnelType, err)
	}

	localSession := tunnel.NewSessionFromContract(session.SessionID, tunnelType, session.LocalPort, session.Remote, session.Status, cfg.Timeout)
	globalTunnelManager.Register(localSession)

	printTunnelSession(runtime, tunnelType, cfg, session, project)
	runtime.Output.Warn("%s", tunnel.DebugWarningMessage(tunnelType))
	return nil
}

func parseTunnelArgs(args []string, tunnelType tunnel.TunnelType) (tunnelArgs, error) {
	flagSet := flag.NewFlagSet("tunnel-"+tunnelType.String(), flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	port := flagSet.Int("port", 0, "local port to bind the tunnel to")
	remote := flagSet.String("remote", "", "remote address to forward to")
	timeout := flagSet.Duration("timeout", 0, "tunnel session timeout (e.g. 30m, 1h)")

	if err := flagSet.Parse(args); err != nil {
		return tunnelArgs{}, fmt.Errorf("invalid tunnel flags. next: use `lazyops tunnel %s [--port <port>] [--remote <addr>] [--timeout <duration>]`", tunnelType)
	}
	if flagSet.NArg() > 0 {
		return tunnelArgs{}, fmt.Errorf("unexpected tunnel arguments: %s. next: use `lazyops tunnel %s [--port <port>] [--remote <addr>] [--timeout <duration>]`", strings.Join(flagSet.Args(), " "), tunnelType)
	}

	if tunnelType == tunnel.TypeTCP && *port == 0 {
		return tunnelArgs{}, fmt.Errorf("--port is required for tcp tunnels. next: rerun `lazyops tunnel tcp --port <local-port> --remote <remote-addr>`")
	}

	return tunnelArgs{
		Port:    *port,
		Remote:  strings.TrimSpace(*remote),
		Timeout: *timeout,
	}, nil
}

func tunnelSessionPath(tunnelType tunnel.TunnelType) string {
	switch tunnelType {
	case tunnel.TypeDB:
		return "/api/v1/tunnels/db/sessions"
	case tunnel.TypeTCP:
		return "/api/v1/tunnels/tcp/sessions"
	default:
		return "/api/v1/tunnels/unknown/sessions"
	}
}

func buildTunnelSessionBody(cfg tunnel.Config) []byte {
	body := map[string]any{
		"project_id": cfg.ProjectID,
		"local_port": cfg.LocalPort,
		"remote":     cfg.Remote,
		"timeout":    cfg.Timeout.String(),
	}
	if cfg.TargetRef != "" {
		body["target_ref"] = cfg.TargetRef
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	return data
}

func printTunnelSession(runtime *Runtime, tunnelType tunnel.TunnelType, cfg tunnel.Config, session contracts.TunnelSession, project contracts.Project) {
	runtime.Output.Success("tunnel session created")
	runtime.Output.Info("type: %s", tunnelType)
	runtime.Output.Info("project: %s (%s)", project.Name, project.Slug)
	runtime.Output.Info("local port: %d", session.LocalPort)
	if strings.TrimSpace(session.Remote) != "" {
		runtime.Output.Info("remote: %s", session.Remote)
	}
	runtime.Output.Info("session id: %s", session.SessionID)
	runtime.Output.Info("status: %s", session.Status)
	if strings.TrimSpace(session.ExpiresAt) != "" {
		runtime.Output.Info("expires at: %s", session.ExpiresAt)
	}
	runtime.Output.Print("")
	runtime.Output.Info("connect: %s://127.0.0.1:%d", tunnelType, session.LocalPort)
	runtime.Output.Info("stop with: lazyops tunnel stop %s", session.SessionID)
}
