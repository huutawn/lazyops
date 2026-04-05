# Ma trận Hợp đồng

Ma trận này là danh mục trạng thái hiện tại chính thức trên backend, agent, CLI và docs.

## Từ vựng Trạng thái

| Trạng thái | Ý nghĩa | Nhãn kiểm thử thủ công |
| --- | --- | --- |
| `implemented` | Tồn tại hợp đồng công khai thực. | `live` |
| `adapter/composed` | Hành vi thực tồn tại thông qua nhiều hợp đồng hoặc kết nối runtime một phần. | `adapter/composed` |
| `mock-only` | Chỉ tồn tại mock transport hoặc harness cục bộ. | `mock-only` |
| `missing` | Chưa tồn tại bề mặt công khai thực nào. | `spec-only` |

## API HTTP Công khai

| Hợp đồng | Chủ sở hữu | Trạng thái | Tài liệu nguồn | Bằng chứng mã | Ghi chú |
| --- | --- | --- | --- | --- | --- |
| `POST /api/v1/auth/register` | backend, frontend | `implemented` | `guide/lazyops-implementation-master-plan.md`, `guide/backend-guide.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/auth_controller.go` | Bootstrap session cho người dùng web. |
| `POST /api/v1/auth/login` | backend, frontend | `implemented` | như trên | cùng các file route và controller | Đăng nhập bằng mật khẩu. |
| `POST /api/v1/auth/cli-login` | backend, CLI | `implemented` | `guide/cli-guide.md`, `guide/backend-guide.md` | `backend/internal/api/v1/routes.go`, `cli/internal/command/login.go` | Đường dẫn tạo PAT cho CLI. |
| `POST /api/v1/auth/pat/revoke` | backend, CLI | `implemented` | `guide/cli-guide.md` | `backend/internal/api/v1/routes.go`, `cli/internal/command/logout.go` | Thu hồi PAT cho CLI. |
| `GET /api/v1/auth/oauth/google/start` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go` | Route bắt đầu OAuth. |
| `GET /api/v1/auth/oauth/google/callback` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go` | Route callback OAuth. |
| `GET /api/v1/auth/oauth/github/start` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go` | Route bắt đầu OAuth. |
| `GET /api/v1/auth/oauth/github/callback` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go` | Route callback OAuth. |
| `POST /api/v1/github/app/installations/sync` | backend, CLI, frontend | `implemented` | `guide/cli-guide.md`, `guide/backend-guide.md` | `backend/internal/api/v1/routes.go`, `cli/internal/command/link.go` | Đồng bộ danh mục cài đặt GitHub App. |
| `GET /api/v1/github/repos` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go`, `cli/internal/command/link.go` | Khám phá repo sau khi đồng bộ cài đặt. |
| `POST /api/v1/projects` | backend, CLI, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/project_controller.go` | Tạo dự án. |
| `GET /api/v1/projects` | backend, CLI, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `cli/internal/command/init.go` | Danh sách dự án. |
| `POST /api/v1/projects/:id/repo-link` | backend, CLI, frontend | `implemented` | `guide/cli-guide.md`, `guide/backend-guide.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/project_controller.go`, `cli/internal/command/link.go` | Liên kết dự án với repo. |
| `POST /api/v1/integrations/github/webhook` | backend, GitHub | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/integration_controller.go` | Tiếp nhận webhook build và preview. |
| `POST /api/v1/builds/callback` | backend, hệ thống build | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/build_controller.go` | Callback đối chiếu artifact. |
| `POST /api/v1/instances` | backend, CLI, frontend | `implemented` | `guide/cli-guide.md`, `guide/backend-guide.md` | `backend/internal/api/v1/routes.go` | Tạo target standalone hoặc có khả năng mesh. |
| `GET /api/v1/instances` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go`, `cli/internal/command/init.go` | Khám phá target. |
| `POST /api/v1/mesh-networks` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go` | Tạo target mesh. |
| `GET /api/v1/mesh-networks` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go`, `cli/internal/command/init.go` | Khám phá target mesh. |
| `POST /api/v1/clusters` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go` | Tạo target cluster. |
| `GET /api/v1/clusters` | backend, CLI, frontend | `implemented` | như trên | `backend/internal/api/v1/routes.go`, `cli/internal/command/init.go` | Khám phá target cluster. |
| `GET /api/v1/projects/:id/deployment-bindings` | backend, CLI, frontend | `implemented` | master plan, backend guide, `backend-day16-init-facing-contracts.md` | `backend/internal/api/v1/routes.go`, `cli/internal/command/bindings.go` | Cần thiết cho `init`, `link`, `status`, và UI bindings. |
| `POST /api/v1/projects/:id/deployment-bindings` | backend, CLI, frontend | `implemented` | master plan, backend guide, `backend-day15-deployment-binding.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/deployment_binding_controller.go` | Tạo binding từ `target_ref` logic. |
| `POST /api/v1/projects/:id/init/validate-lazyops-yaml` | backend, CLI, frontend | `implemented` | master plan, backend guide, `backend-day16-init-facing-contracts.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/init_contract_controller.go` | Hợp đồng xác thực cho ý định deploy repo. |
| `PUT /api/v1/projects/:id/blueprint` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/blueprint_controller.go` | Biên dịch blueprint. |
| `POST /api/v1/projects/:id/deployments` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/deployment_controller.go`, `backend/internal/service/rollout_execution_service.go` | Tạo bản ghi revision và deployment, rồi thử kickoff rollout `standalone` theo kiểu best-effort khi artifact và agent đã sẵn sàng. |
| `GET /api/v1/projects/:id/topology` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/observability_controller.go` | Mô hình đọc biểu đồ topology với guard truy cập theo project. |
| `GET /api/v1/traces/:correlation_id` | backend, CLI, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/observability_controller.go`, `cli/internal/command/traces.go` | Đường dẫn đọc tóm tắt trace với guard truy cập theo project và envelope mang `correlation_id` của request. |
| `POST /api/v1/agents/enroll` | backend, agent | `implemented` | master plan, `backend-day13-agent-enrollment.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/agent_runtime_controller.go`, `agent/internal/enroll` | Trao đổi bootstrap token. |
| `POST /api/v1/agents/heartbeat` | backend, agent | `implemented` | master plan, `backend-day13-agent-enrollment.md` | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/agent_runtime_controller.go` | Heartbeat đã xác thực chỉ dành cho agent. |
| `POST /api/v1/tunnels/db/sessions` | backend, CLI | `implemented` | master plan, backend guide, CLI guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/tunnel_controller.go`, `cli/internal/command/tunnel.go` | Hỗ trợ backend hiện tại giới hạn ở các binding giải quyết đến target `instance`. |
| `POST /api/v1/tunnels/tcp/sessions` | backend, CLI | `implemented` | CLI guide, CLI task, contracts matrix | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/tunnel_controller.go`, `cli/internal/command/tunnel.go` | Hợp đồng tunnel chính thức hiện hỗ trợ `db` và `tcp`. |
| `DELETE /api/v1/tunnels/sessions/:id` | backend, CLI | `implemented` | CLI guide, CLI task, contracts matrix | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/tunnel_controller.go`, `cli/internal/command/tunnel.go` | Dừng phiên tunnel. |

## Endpoint WebSocket và Streaming

| Hợp đồng | Chủ sở hữu | Trạng thái | Tài liệu nguồn | Bằng chứng mã | Ghi chú |
| --- | --- | --- | --- | --- | --- |
| `GET /ws/agents/control` | backend, agent | `implemented` | master plan, agent guide | `backend/internal/api/v1/routes.go`, `agent/internal/contracts/control.go` | Socket điều khiển agent công khai chính thức. |
| `GET /api/v1/ws/agents/control` | backend, agent | `implemented` | chỉ là bí danh tương thích | `backend/internal/api/v1/routes.go` | Giữ lại để tương thích ngược; không coi là chính thức. |
| `GET /ws/operators/stream` | backend, frontend | `implemented` | master plan, backend guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/operator_stream_controller.go` | Luồng operator công khai chính thức; read-only và phát `correlation_id` khi payload sự kiện có sẵn. |
| `GET /api/v1/ws/operators/stream` | backend, frontend | `implemented` | chỉ là bí danh tương thích | `backend/internal/api/v1/routes.go` | Bí danh tương thích; không sử dụng trong tài liệu mới. |
| `GET /ws/logs/stream` | backend, CLI, frontend | `implemented` | master plan, CLI guide | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/observability_controller.go`, `cli/internal/command/logs.go` | Route backend đã tồn tại, đọc log batch thật do agent gửi qua control channel và trả preview envelope có filter/cursor cùng validation cho malformed query. |
| `GET /api/v1/ws/agents` | backend, frontend | `adapter/composed` | câu chuyện kế thừa README | `backend/internal/api/v1/routes.go`, `backend/internal/api/v1/controller/websocket_controller.go` | Luồng demo trạng thái agent kế thừa; không phải là bề mặt control-plane chính thức. |

## Ánh xạ Lệnh CLI

| Lệnh CLI | Hợp đồng hỗ trợ | Trạng thái | Bằng chứng mã | Ghi chú |
| --- | --- | --- | --- | --- |
| `lazyops login` | `POST /api/v1/auth/cli-login` | `implemented` | `cli/internal/command/login.go` | Luồng PAT thực. |
| `lazyops logout` | thu hồi thông tin cục bộ cộng với thu hồi PAT | `implemented` | `cli/internal/command/logout.go` | Gọi thu hồi khi có thể. |
| `lazyops init` | danh sách dự án, danh sách target, danh sách/tạo binding, xác thực `lazyops.yaml` | `implemented` | `cli/internal/command/init.go` | Client tiếp nhận mỏng trên các API backend thực. |
| `lazyops link` | danh sách dự án, đồng bộ cài đặt GitHub, danh sách repo, liên kết repo, danh sách binding | `implemented` | `cli/internal/command/link.go` | Luồng liên kết repo thực. |
| `lazyops bindings` | `GET /api/v1/projects/:id/deployment-bindings` | `implemented` | `cli/internal/command/bindings.go` | UX liệt kê/lọc thực. |
| `lazyops doctor` | local validation cộng với `GET /api/v1/projects`, `GET /api/v1/projects/:id/deployment-bindings`, khám phá target, và `POST /api/v1/projects/:id/init/validate-lazyops-yaml` | `adapter/composed` | `cli/internal/command/doctor.go` | Không còn phụ thuộc vào `/mock/v1/doctor`; CLI kết hợp local parse/validation với control-plane validation thật. |
| `lazyops status` | local contract parse cộng với danh sách dự án, danh sách binding, `POST /api/v1/projects/:id/init/validate-lazyops-yaml`, và fallback khám phá target | `adapter/composed` | `cli/internal/command/status.go` | Không có route `/api/v1/status` chuyên dụng; CLI tổng hợp trạng thái từ nhiều contract nhưng nay ưu tiên control-plane validation và fallback rõ ràng hơn khi route validation chưa sẵn sàng. |
| `lazyops logs <service>` | `GET /ws/logs/stream` | `implemented` | `cli/internal/command/logs.go` | CLI giữ parser preview hiện tại nhưng giờ đã đọc từ route backend thật thay vì route còn thiếu. |
| `lazyops traces <correlation-id>` | `GET /api/v1/traces/:correlation_id` | `implemented` | `cli/internal/command/traces.go` | Đọc trace thực. |
| `lazyops tunnel db` | `POST /api/v1/tunnels/db/sessions` | `implemented` | `cli/internal/command/tunnel.go` | Tạo phiên tunnel thực. |
| `lazyops tunnel tcp` | `POST /api/v1/tunnels/tcp/sessions` | `implemented` | `cli/internal/command/tunnel.go` | Tạo phiên tunnel thực. |
| `lazyops tunnel stop` | `DELETE /api/v1/tunnels/sessions/:id` | `implemented` | `cli/internal/command/tunnel.go` | Đóng phiên tunnel thực. |
| `lazyops tunnel list` | chỉ trình quản lý phiên cục bộ | `implemented` | `cli/internal/command/tunnel.go` | Không được hỗ trợ bởi API backend theo thiết kế. |

## Từ vựng Lệnh Agent

Trường `type` của lệnh phải sử dụng các hằng lệnh chính xác. Các bí danh có dấu chấm như `deploy.start_candidate` không phải là chính thức.

| Loại lệnh | Agent | Backend | Trạng thái | Bằng chứng | Ghi chú |
| --- | --- | --- | --- | --- | --- |
| `reconcile_revision` | có | có | `implemented` | `agent/internal/contracts/commands.go`, `backend/internal/runtime/command_envelope.go` | Khớp hằng chính xác. |
| `prepare_release_workspace` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `ensure_mesh_peer` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `sync_overlay_routes` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `render_sidecars` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `render_gateway_config` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `start_release_candidate` | có | có | `implemented` | cùng các file | Thay thế chính thức cho ví dụ có dấu chấm cũ. |
| `run_health_gate` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `promote_release` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `rollback_release` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `wake_service` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `sleep_service` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `report_topology_state` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `report_trace_summary` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `report_metric_rollup` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |
| `report_log_batch` | có | có | `implemented` | `agent/internal/contracts/commands.go`, `backend/internal/runtime/command_envelope.go` | Từ vựng đã được điều chỉnh dù luồng log thực vẫn còn thiếu. |
| `garbage_collect_runtime` | có | có | `implemented` | cùng các file | Khớp hằng chính xác. |

## Mô hình và Từ vựng Cốt lõi

| Mô hình hoặc từ vựng | Chủ sở hữu | Trạng thái | Tài liệu nguồn | Bằng chứng mã | Ghi chú |
| --- | --- | --- | --- | --- | --- |
| `DeploymentBinding` | backend, CLI, frontend | `implemented` | backend guide, CLI guide | `backend/internal/models/deployment_binding.go`, `cli/internal/contracts/models.go` | `target_ref` logic vẫn là nguồn sự thật hướng repo. |
| `Blueprint` | backend, agent, frontend | `implemented` | backend guide, master plan | `backend/internal/models/blueprint.go`, `backend/internal/service/blueprint_service.go` | Mang chính sách tương thích, domain và scale. |
| `DesiredStateRevision` | backend, agent | `implemented` | backend guide, master plan | `backend/internal/models/desired_state_revision.go`, `backend/internal/service/deployment_service.go` | Đường dẫn tạo revision đang hoạt động. |
| `TraceSummary` | backend, agent, CLI, frontend | `implemented` | backend guide, master plan | `backend/internal/models/trace_summary.go`, `backend/internal/service/observability_service.go` | API đọc trace đang hoạt động. |
| `TopologyGraph` | backend, agent, frontend | `implemented` | backend guide, master plan | `backend/internal/service/observability_service.go`, `backend/internal/api/v1/controller/observability_controller.go` | Được phục vụ bởi `GET /api/v1/projects/:id/topology`. |
| `TunnelSession` | backend, CLI | `implemented` | CLI guide, backend guide | `backend/internal/models/mesh.go`, `backend/internal/api/v1/controller/tunnel_controller.go`, `cli/internal/contracts/models.go` | Các loại tunnel công khai là `db` và `tcp`. | | Các chế độ runtime `standalone`, `distributed-mesh`, `distributed-k3s` | backend, CLI, agent, frontend | `adapter/composed` | master plan, tất cả guide | `cli/internal/lazyyaml`, `agent/internal/contracts`, các dịch vụ và planner backend | Các hợp đồng được chia sẻ, nhưng mức độ hoàn thiện rollout khác nhau theo chế độ. |
| Command envelope | backend, agent | `implemented` | master plan, agent guide | `agent/internal/contracts/control.go`, `backend/internal/runtime/command_envelope.go` | `type` phải sử dụng các hằng chính xác. |
