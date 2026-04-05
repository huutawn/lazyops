# Danh mục Tính năng

Mỗi tính năng bên dưới liệt kê tính năng là gì, luồng hoạt động, bề mặt nào sở hữu và mức độ trưởng thành hiện tại.

## Auth & PAT

- Là gì: Xác thực web, các điểm nhập OAuth, cấp phát PAT cho CLI, thu hồi PAT, và middleware xác thực chung.
- Luồng công việc: người dùng đăng nhập bằng mật khẩu hoặc OAuth, CLI yêu cầu `POST /api/v1/auth/cli-login`, backend phát hành PAT, CLI lưu trữ cục bộ, và thu hồi sử dụng `POST /api/v1/auth/pat/revoke`.
- Công nghệ: Gin controllers, phân tích cú pháp JWT session, PAT repository, dịch vụ Google OAuth, dịch vụ GitHub OAuth, kho lưu trữ thông tin CLI.
- Bề mặt sở hữu: backend, CLI, frontend.
- Hợp đồng chính thức: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/cli-login`, `POST /api/v1/auth/pat/revoke`, các route bắt đầu/callback OAuth.
- Trạng thái hiện tại: `implemented`.

## GitHub App & Liên kết Repo

- Là gì: Đồng bộ cài đặt GitHub App, khám phá repo, liên kết dự án với repository, và chuẩn hóa webhook.
- Luồng công việc: operator đồng bộ cài đặt, liệt kê repos, liên kết một dự án với một repo và branch, sau đó GitHub webhooks tạo công việc build.
- Công nghệ: Dịch vụ tích hợp GitHub, bộ điều khiển webhook đã chuẩn hóa, lưu trữ liên kết repo, luồng chọn repo CLI.
- Bề mặt sở hữu: backend, CLI, frontend.
- Hợp đồng chính thức: `POST /api/v1/github/app/installations/sync`, `GET /api/v1/github/repos`, `POST /api/v1/projects/:id/repo-link`, `POST /api/v1/integrations/github/webhook`.
- Trạng thái hiện tại: `implemented`.

## Tiếp nhận Target

- Là gì: Tiếp nhận dạng CRUD cho instances, mesh networks và clusters để sau này trở thành các target triển khai.
- Luồng công việc: operator tạo các target, CLI hoặc frontend liệt kê chúng, và sau đó `DeploymentBinding` giải quyết `target_ref` logic thành một trong các target thực này.
- Công nghệ: Dịch vụ và repository target backend, các controller liệt kê/tạo, khám phá target CLI, form target frontend.
- Bề mặt sở hữu: backend, CLI, frontend.
- Hợp đồng chính thức: `POST/GET /api/v1/instances`, `POST/GET /api/v1/mesh-networks`, `POST/GET /api/v1/clusters`.
- Trạng thái hiện tại: `implemented`.

## Init & DeploymentBinding

- Là gì: Hợp đồng tiếp nhận hướng repo liên kết `lazyops.yaml` với một target do backend quản lý.
- Luồng công việc: CLI tải các lựa chọn dự án và target, liệt kê các binding hiện có, xác thực `lazyops.yaml`, tạo hoặc tái sử dụng một binding, sau đó ghi `target_ref` vào hợp đồng repo.
- Công nghệ: Bộ quét và sinh repo CLI, `DeploymentBindingService` backend, dịch vụ xác thực init, chính sách tương thích target.
- Bề mặt sở hữu: backend, CLI, frontend.
- Hợp đồng chính thức: `GET /api/v1/projects/:id/deployment-bindings`, `POST /api/v1/projects/:id/deployment-bindings`, `POST /api/v1/projects/:id/init/validate-lazyops-yaml`.
- Trạng thái hiện tại: `implemented`.

## Blueprint & DesiredStateRevision

- Là gì: Biên dịch từ hợp đồng dự án cộng với chính sách binding thành một blueprint có thể chạy và một revision desired-state đã xếp hàng.
- Luồng công việc: backend xác thực dự án và binding, biên dịch một blueprint, lưu trữ đầu ra đã biên dịch, tạo một `DesiredStateRevision`, sau đó tạo một bản ghi `Deployment`.
- Công nghệ: Trình biên dịch blueprint, lưu trữ revision, máy trạng thái deployment, các trường chính sách nhận biết chế độ runtime.
- Bề mặt sở hữu: backend, agent.
- Hợp đồng chính thức: `PUT /api/v1/projects/:id/blueprint`, `POST /api/v1/projects/:id/deployments`.
- Trạng thái hiện tại: `implemented` cho việc tạo bản ghi, với thực thi rollout vẫn được xử lý riêng.

## Callback Build

- Là gì: Đối chiếu artifact từ hệ thống build trở lại LazyOps.
- Luồng công việc: webhook hoặc điều phối build tạo công việc build, build runner gọi `POST /api/v1/builds/callback`, backend lưu trữ metadata artifact, và sau đó các bản ghi revision tiêu thụ nó.
- Công nghệ: Kho lưu trữ build job, controller callback, lưu trữ metadata artifact, đầu vào trình biên dịch blueprint.
- Bề mặt sở hữu: backend, pipeline GitHub/build.
- Hợp đồng chính thức: `POST /api/v1/builds/callback`.
- Trạng thái hiện tại: `implemented`.

## Đăng ký & Điều khiển Agent

- Là gì: Trao đổi bootstrap-token, heartbeat, WebSocket điều khiển outbound, và dispatch lệnh operator.
- Luồng công việc: agent đăng ký, nhận agent token, heartbeat trên route HTTP chỉ dành cho agent, quay số `GET /ws/agents/control`, và operators dispatch các hằng lệnh chính xác đến agent đã kết nối.
- Công nghệ: Trao đổi bootstrap token, xác thực agent token, trung tâm điều khiển WebSocket outbound, hình dạng bao lệnh chung.
- Bề mặt sở hữu: backend, agent.
- Hợp đồng chính thức: `POST /api/v1/agents/enroll`, `POST /api/v1/agents/heartbeat`, `GET /ws/agents/control`, `POST /api/v1/agents/:agent_id/dispatch`.
- Trạng thái hiện tại: `implemented`.

## Triển khai Standalone

- Là gì: Đường dẫn triển khai một máy cho việc khởi chạy ứng viên, health gate, thăng chức, rollback và dọn dẹp runtime.
- Luồng công việc: tạo deployment sinh revision, rollout executor của backend tự động kickoff plan `standalone` khi artifact và agent đã sẵn sàng, rollout planner giải quyết chế độ target, dispatch gửi các lệnh chính xác, và instance agent thực hiện các hành động runtime.
- Công nghệ: Registry runtime, rollout planner, rollout executor, bao lệnh, các module runtime instance agent.
- Bề mặt sở hữu: backend, agent.
- Hợp đồng chính thức: Bộ lệnh từ `reconcile_revision` đến `garbage_collect_runtime`.
- Trạng thái hiện tại: `adapter/composed` vì `standalone` đã có auto-kickoff và regression tốt hơn, nhưng orchestration vẫn là best-effort và chưa có vòng phản hồi agent đầy đủ cho mọi bước.

## Mạng Mesh

- Là gì: Khả năng tiếp cận chéo node riêng tư, giải quyết phụ thuộc, trạng thái topology, và lập kế hoạch nhận biết mesh.
- Luồng công việc: operator tạo các mạng mesh, deployment bindings nhắm đến các chế độ tương thích mesh, agent báo cáo topology, và backend giải quyết các đường dẫn riêng tư dịch vụ-đến-dịch vụ.
- Công nghệ: Bản ghi mạng mesh, dịch vụ giải quyết phụ thuộc, thu nhận trạng thái topology, từ vựng lệnh route và peer overlay.
- Bề mặt sở hữu: backend, agent, CLI.
- Hợp đồng chính thức: CRUD target mesh, `report_topology_state`, bản ghi lập kế hoạch mesh, `GET /api/v1/projects/:id/topology`.
- Trạng thái hiện tại: `adapter/composed`. Đã sửa `findInstanceForService` để scope theo project thay vì dùng `ListByUser("")` với naive substring match trên JSON.

## Tương thích Sidecar

- Là gì: Tiêm môi trường, tiêm thông tin xác thực được quản lý, và giải cứu localhost cho các ứng dụng lazy.
- Luồng công việc: blueprint mang chính sách tương thích, backend bảo tồn chính sách trong các đầu ra đã biên dịch, agent render sidecars và cấu hình gateway, sau đó thực thi runtime áp dụng kế hoạch.
- Công nghệ: Các trường chính sách tương thích trong `lazyops.yaml`, lệnh render sidecar, lệnh render gateway, metadata phụ thuộc dịch vụ.
- Bề mặt sở hữu: backend, agent, CLI.
- Hợp đồng chính thức: Payload đã biên dịch blueprint, `render_sidecars`, `render_gateway_config`.
- Trạng thái hiện tại: `adapter/composed`.

## Observability

- Là gì: Tóm tắt trace, biểu đồ topology, luồng sự kiện operator, và logs preview phục vụ từ log batch thật.
- Luồng công việc: gateway hoặc agents truyền bá `X-Correlation-ID`, agents báo cáo tóm tắt trace, trạng thái topology, và log batch, backend lưu trữ rồi phục vụ các API đọc topology, trace, và logs, còn operator stream phát sự kiện read-only và mang `correlation_id` khi payload có sẵn.
- Công nghệ: Repository tóm tắt trace, kho lưu trữ node và edge topology, trung tâm luồng operator, các lệnh observability CLI.
- Bề mặt sở hữu: backend, agent, CLI, frontend.
- Hợp đồng chính thức: `GET /api/v1/traces/:correlation_id`, `GET /api/v1/projects/:id/topology`, `GET /ws/operators/stream`, `GET /ws/logs/stream`.
- Trạng thái hiện tại: `adapter/composed` vì traces, topology, logs preview, và guard observability đã hoạt động ổn hơn, nhưng observability vẫn còn ghép từ nhiều bề mặt thay vì một pipeline thống nhất.

## Tunnels

- Là gì: Tunnel debug tùy chọn cho cơ sở dữ liệu hoặc truy cập TCP chung.
- Luồng công việc: CLI đọc `lazyops.yaml`, giải quyết dự án và binding, tạo một phiên tunnel thông qua backend, operator sử dụng cổng cục bộ, sau đó đóng phiên thông qua backend trong khi `tunnel list` vẫn chỉ cục bộ.
- Công nghệ: Trình quản lý tunnel CLI, `MeshPlanningService` backend, lưu trữ `TunnelSession`, xác thực cổng cục bộ, cleanup session expired tự động.
- Bề mặt sở hữu: backend, CLI.
- Hợp đồng chính thức: `POST /api/v1/tunnels/db/sessions`, `POST /api/v1/tunnels/tcp/sessions`, `DELETE /api/v1/tunnels/sessions/:id`.
- Trạng thái hiện tại: `implemented`, với hỗ trợ backend hiện tại giới hạn ở các binding giải quyết đến target `instance`. Đã thêm port conflict detection và auto-close expired sessions.

## Scale-to-Zero

- Là gì: Hành vi sleep và wake dựa trên chính sách cho các dịch vụ nhàn rỗi.
- Luồng công việc: chính sách được lưu trữ trong `lazyops.yaml` và blueprint đã biên dịch, backend cung cấp từ vựng lệnh, và agents cuối cùng thực hiện chuyển đổi wake và sleep.
- Công nghệ: `scale_to_zero_policy` (với `enabled`, `idle_window`, `gateway_hold_timeout`), từ vựng lệnh autosleep, ngữ nghĩa giữ wake-up gateway, chính sách rollout và finops.
- Bề mặt sở hữu: backend, agent, CLI, frontend.
- Hợp đồng chính thức: `wake_service`, `sleep_service`, chính sách scale-to-zero đã biên dịch trong payload blueprint và revision.
- Trạng thái hiện tại: `adapter/composed`. Đã sửa: policy giờ mang đầy đủ `idle_window` và `gateway_hold_timeout`, `WakeServicePayload` có `ScaleToZeroPolicy`, `CheckColdStartTimeout` dùng configurable threshold thay vì hardcoded, K3s boundary enforced cho cả sleep và wake.

## Ranh giới K3s

- Là gì: Một ranh giới nghiêm ngặt giữ LazyOps ở các lớp desired-state, telemetry và chính sách thay vì trở thành một scheduler.
- Luồng công việc: operator tạo một target cluster, bindings có thể nhắm đến `distributed-k3s`, node agents báo cáo health và topology, và chính sách rollout ở trên mức lập lịch Kubernetes.
- Công nghệ: CRUD cluster, hợp đồng node-agent, kỳ vọng telemetry cluster, guardrails planner.
- Bề mặt sở hữu: backend, agent, frontend.
- Hợp đồng chính thức: `POST/GET /api/v1/clusters`, họ hợp đồng node-agent, chế độ runtime `distributed-k3s`, các lệnh telemetry.
- Trạng thái hiện tại: `adapter/composed`. Đã thêm `sleep_service`, `wake_service`, `scale_to_zero` vào forbidden commands của K3s driver.
