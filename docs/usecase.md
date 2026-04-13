# Đặc Tả Use Case Current State Của LazyOps

Tài liệu này mô tả use case của LazyOps theo trạng thái hiện tại của repo. Mục tiêu là để nhóm có thể:

- vẽ lại sơ đồ use case bằng StarUML, draw.io, Visio hoặc Mermaid;
- giữ cùng một vocabulary với `README.md`, `docs/index.md`, `docs/feature-catalog.md`, `docs/contracts-matrix.md`, và `docs/short-flow-spec.md`;
- tránh mô tả quá mức các phần vẫn đang ở mức `adapter/composed`.

## 1. Khung đọc tài liệu

### 1.1. Cách hiểu current state

- `standalone` là luồng triển khai chính hiện tại và là lát cắt nên dùng để mô tả trong báo cáo, demo, hoặc sơ đồ sequence.
- `standalone` đã có rollout planner, dispatch xuống agent, health gate, promote, rollback, và garbage collect, nhưng vẫn nên mô tả là `best-effort` hoặc `adapter/composed`, không phải một PaaS đã hoàn thiện tuyệt đối.
- `distributed-mesh` và `distributed-k3s` đã có contract, inventory, planning vocabulary và phần telemetry liên quan, nhưng chưa nên mô tả như runtime end-to-end hoàn chỉnh.
- UX 3 bước `Connect Code`, `Connect Infra`, `Deploy` là lớp orchestration quan trọng ở hiện tại. Lớp UX này dựa trên các contract backend có thật, không phải một hệ thống tách biệt khỏi deployment APIs.
- Không nên claim rằng multi-service monorepo FE+BE trong cùng một project đã deploy end-to-end hoàn chỉnh ở current state.

### 1.2. Actor chuẩn hóa

Các actor nên dùng thống nhất trong sơ đồ và trong phần thuyết minh:

- `Operator Web`
- `Operator CLI`
- `GitHub / GitHub App`
- `Build Worker`
- `Runtime Agent`

## 2. Nhóm use case tổng quát

Các nhóm use case hiện có thật trong hệ thống:

- `Đăng nhập và quản lý phiên`
- `Tạo project và liên kết repo`
- `Kết nối hạ tầng và đăng ký agent`
- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `One-click deploy / tạo deployment`
- `Rollout standalone: prepare, start candidate, health gate, promote, rollback`
- `Quan sát topology, logs, traces, incidents`
- `Tạo tunnel debug DB/TCP`

### 2.1. Sơ đồ use case tổng quát

Dùng danh sách sau để vẽ use case tổng quát nếu cần:

- Khung hệ thống: `LazyOps Platform`
- Actor bên trái: `Operator Web`, `Operator CLI`
- Actor bên phải: `GitHub / GitHub App`, `Build Worker`, `Runtime Agent`
- Use case bên trong khung:
  - `Đăng nhập và quản lý phiên`
  - `Tạo project và liên kết repo`
  - `Kết nối hạ tầng và đăng ký agent`
  - `Khởi tạo lazyops.yaml và DeploymentBinding`
  - `One-click deploy / tạo deployment`
  - `Rollout standalone: prepare, start candidate, health gate, promote, rollback`
  - `Quan sát topology, logs, traces, incidents`
  - `Tạo tunnel debug DB/TCP`

### 2.2. Actor nối với use case nào

`Operator Web` nối với:

- `Đăng nhập và quản lý phiên`
- `Tạo project và liên kết repo`
- `Kết nối hạ tầng và đăng ký agent`
- `One-click deploy / tạo deployment`
- `Rollout standalone: prepare, start candidate, health gate, promote, rollback`
- `Quan sát topology, logs, traces, incidents`

`Operator CLI` nối với:

- `Đăng nhập và quản lý phiên`
- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `Quan sát topology, logs, traces, incidents`
- `Tạo tunnel debug DB/TCP`

`GitHub / GitHub App` nối với:

- `Tạo project và liên kết repo`
- `One-click deploy / tạo deployment`

`Build Worker` nối với:

- `One-click deploy / tạo deployment`

`Runtime Agent` nối với:

- `Kết nối hạ tầng và đăng ký agent`
- `Rollout standalone: prepare, start candidate, health gate, promote, rollback`
- `Quan sát topology, logs, traces, incidents`

## 3. Ghi chú trạng thái để tránh mô tả quá mức

### 3.1. Luồng chính nên trình bày

- Luồng chính để mô tả trong báo cáo là `standalone` kết hợp với one-click deploy hoặc tạo deployment thông thường.
- Các bước quan trọng nên xuất hiện trong sequence diagram là: repo link, webhook hoặc build callback, artifact-ready revision, deployment record, rollout planner, command dispatch, health gate, promote hoặc rollback.

### 3.2. Các phần chỉ nên xem là capability boundary

- `distributed-mesh` nên được mô tả là lớp planning, topology, dependency resolution và tunnel policy có thật, nhưng chưa nên gọi là fully automated runtime.
- `distributed-k3s` nên được mô tả là ranh giới policy, telemetry và desired state ở mức cluster, không nên mô tả như LazyOps đang trực tiếp thay thế hoàn toàn scheduler Kubernetes.

### 3.3. Các claim nên tránh

- Không mô tả mesh hoặc k3s như một pipeline rollout production-ready tương đương `standalone`.
- Không mô tả observability như một platform thống nhất đã hoàn thiện tuyệt đối; current state vẫn là nhiều bề mặt đọc được ghép lại.
- Không mô tả tunnel như truy cập vận hành lâu dài; đây là debug path tạm thời cho `db` và `tcp`.
- Không mô tả multi-service FE+BE trong cùng một project như use case đã hỗ trợ đầy đủ end-to-end.

## 4. Cách đọc UX 3 bước theo contract hiện tại

UX 3 bước nên được giải thích như sau:

1. `Connect Code`
   - bao phủ tạo project, liên kết GitHub App, kiểm tra repo link, tracked branch, webhook health và build readiness;
   - không phải một module riêng tách khỏi backend contracts.
2. `Connect Infra`
   - bao phủ tạo target, SSH/bootstrap flow, enroll agent và heartbeat online.
3. `Deploy`
   - bao phủ one-click deploy hoặc tạo deployment, từ đó kích hoạt rollout planner và command dispatch xuống runtime agent.

Nói ngắn gọn, 3 bước này là lớp UX đơn giản hóa cho operator, còn bên dưới vẫn dùng các contract thật như `ProjectRepoLink`, `DeploymentBinding`, `BuildJob`, `Revision`, `Deployment`, `ws/agents/control`, `topology`, `traces`, và `tunnel sessions`.

## 5. Use case chi tiết

### UC01. Đăng nhập và quản lý phiên

Mục tiêu:

- cho operator truy cập hệ thống bằng web session hoặc PAT cho CLI;
- giữ ranh giới tách biệt giữa session người dùng và agent token.

Actor chính:

- `Operator Web`
- `Operator CLI`

Luồng chính:

1. Operator đăng nhập bằng email/password hoặc OAuth trên web.
2. Backend tạo web session cho frontend.
3. Operator có thể tạo PAT để CLI dùng cho các lệnh local.
4. Các request protected sau đó đi qua middleware xác thực và phân quyền.

Hậu điều kiện:

- operator có phiên hợp lệ để gọi API hoặc WebSocket phù hợp.

Ghi chú current state:

- PAT của CLI là credential cho operator, khác hoàn toàn với agent token.
- `Runtime Agent` không dùng session của user.

### UC02. Tạo project và liên kết repo

Mục tiêu:

- tạo đơn vị quản lý cho ứng dụng;
- gắn project với GitHub App installation, repo và tracked branch;
- chuẩn bị điều kiện cho webhook và build callback.

Actor chính:

- `Operator Web`
- `GitHub / GitHub App`

Luồng chính:

1. Operator tạo project trên web.
2. Operator chọn hoặc xác nhận runtime mode mục tiêu.
3. Operator sync GitHub App installations.
4. Operator chọn repo và tracked branch cho project.
5. Backend tạo hoặc cập nhật `ProjectRepoLink`.
6. Sau bước này, webhook và build job mới có thể map repo vào đúng project.

Hậu điều kiện:

- project có repo link hợp lệ.

Ghi chú current state:

- Đây là phần nền của `Connect Code`.
- Cần mô tả repo link là contract thật, không phải chỉ là metadata UI.

### UC03. Kết nối hạ tầng và đăng ký agent

Mục tiêu:

- đưa target thật vào hệ thống;
- tạo control channel outbound từ target về backend.

Actor chính:

- `Operator Web`
- `Runtime Agent`

Luồng chính:

1. Operator tạo target, đặc biệt là `instance` cho luồng `standalone`.
2. Backend cấp bootstrap token ngắn hạn.
3. Operator chạy agent trên máy đích.
4. Agent gọi enroll API.
5. Backend xác thực bootstrap token, ownership và thông tin target.
6. Backend cấp agent token và lưu quan hệ với target.
7. Agent mở `GET /ws/agents/control` và gửi heartbeat định kỳ.

Hậu điều kiện:

- target online và sẵn sàng nhận rollout command.

Ghi chú current state:

- Đây là nền của `Connect Infra`.
- Nên nhấn mạnh agent dùng outbound control WebSocket, không yêu cầu backend gọi inbound vào server đích.

### UC04. Khởi tạo `lazyops.yaml` và `DeploymentBinding`

Mục tiêu:

- tạo contract triển khai trong repo local mà không ghi secret hoặc thông tin hạ tầng thật vào repo.

Actor chính:

- `Operator CLI`

Luồng chính:

1. Operator chạy `lazyops init` trong git repo.
2. CLI scan repo và detect service candidates.
3. CLI gọi backend để lấy project, target và bindings liên quan.
4. CLI tạo mới hoặc tái sử dụng `DeploymentBinding`.
5. CLI generate `lazyops.yaml` với `project_slug`, `runtime_mode`, `deployment_binding.target_ref`, services và policy logic.
6. CLI validate để tránh ghi SSH key, token, IP raw, hoặc backend IDs vào repo.

Hậu điều kiện:

- repo có `lazyops.yaml` như local contract.

Ghi chú current state:

- `lazyops.yaml` chỉ nên được mô tả là logical deployment contract.
- `DeploymentBinding` mới là nơi backend resolve target thật.

### UC05. One-click deploy / tạo deployment

Mục tiêu:

- tạo đường ngắn nhất để operator đi từ project đã bootstrap sang deployment chạy được;
- kết nối UX 3 bước với build pipeline và deployment records.

Actor chính:

- `Operator Web`
- `GitHub / GitHub App`
- `Build Worker`

Luồng chính:

1. Nếu project đã liên kết repo, GitHub webhook và build callback có thể tạo `BuildJob` và revision `artifact_ready` phù hợp cho lần deploy tiếp theo.
2. Operator dùng one-click deploy hoặc tạo deployment thông thường từ UI.
3. Backend resolve bootstrap context, repo link, binding và runtime mode.
4. Backend tái sử dụng artifact hoặc revision hiện có nếu đã sẵn sàng; nếu không, backend đi theo nhánh autogen hoặc default revision phù hợp với current state.
5. Backend tạo `DesiredStateRevision` và `Deployment` record.

Hậu điều kiện:

- hệ thống có deployment record đủ điều kiện để kích hoạt rollout standalone.

Ghi chú current state:

- One-click deploy là đường UX quan trọng ở hiện tại.
- Cần mô tả rõ đây là orchestration trên contract backend, không phải một đường đi tách biệt khỏi revision/deployment APIs.

### UC06. Rollout standalone: prepare, start candidate, health gate, promote, rollback

Mục tiêu:

- thực thi kế hoạch rollout xuống target `instance` qua runtime agent;
- kiểm soát promote hoặc rollback theo health gate.

Actor chính:

- `Operator Web`
- `Runtime Agent`

Luồng chính:

1. Backend kiểm tra artifact, binding và trạng thái agent.
2. Rollout planner sinh plan cho runtime mode `standalone`.
3. Backend dispatch command xuống agent theo thứ tự logic.
4. Agent chuẩn bị workspace, render sidecars nếu cần, render gateway config, reconcile revision, start candidate workload.
5. Agent chạy health gate.
6. Nếu health gate pass, backend promote release.
7. Nếu health gate fail, backend rollback release và ghi incident.

Hậu điều kiện:

- deployment ở trạng thái `promoted`, `failed`, hoặc `rolled_back`.

Ghi chú current state:

- Đây là use case trung tâm nên demo trong đồ án.
- Nên mô tả `standalone` là luồng rõ ràng nhất hiện tại nhưng vẫn ở mức `best-effort` hoặc `adapter/composed`.
- Nếu cần nhắc sidecar hoặc internal services, chỉ mô tả chúng là runtime helper của rollout standalone.

### UC07. Quan sát topology, logs, traces, incidents

Mục tiêu:

- giúp operator theo dõi hệ thống sau triển khai;
- giữ `correlation_id` xuyên suốt giữa request, rollout và telemetry.

Actor chính:

- `Operator Web`
- `Operator CLI`
- `Runtime Agent`

Luồng chính:

1. Agent gửi log batches, trace summaries và topology snapshots về backend.
2. Backend lưu trữ và phục vụ các API đọc cho operator.
3. Frontend và CLI gọi API để xem topology, traces, logs preview và incidents.
4. Operator dùng dữ liệu này để xác định nguyên nhân sự cố hoặc kiểm tra trạng thái sau rollout.

Hậu điều kiện:

- operator có bề mặt quan sát thực để hỗ trợ vận hành.

Ghi chú current state:

- Đây là nhóm use case có thật, nhưng vẫn là tập hợp các bề mặt observability được ghép lại.
- Không nên mô tả như một observability platform hợp nhất đã hoàn thiện tuyệt đối.

### UC08. Tạo tunnel debug DB/TCP

Mục tiêu:

- tạo đường debug tạm thời cho operator khi cần kiểm tra dịch vụ nội bộ.

Actor chính:

- `Operator CLI`
- `Runtime Agent`

Luồng chính:

1. Operator chạy `lazyops tunnel db` hoặc `lazyops tunnel tcp`.
2. CLI đọc `lazyops.yaml`, resolve project và binding.
3. Backend tạo `TunnelSession` với kiểm tra local-port conflict và TTL.
4. Operator dùng cổng cục bộ đã cấp để debug.
5. Operator đóng tunnel khi hoàn tất.

Hậu điều kiện:

- có một debug path tạm thời phục vụ xử lý sự cố.

Ghi chú current state:

- Tunnel công khai hiện hỗ trợ `db` và `tcp`.
- Đây là debug path, không phải production access pattern.

## 6. Gợi ý cấu trúc sơ đồ khi mang đi báo cáo

Nếu cần chia sơ đồ use case thành nhiều hình nhỏ, có thể tách theo ba cụm:

- `Khởi tạo và liên kết`: UC01, UC02, UC03, UC04
- `Triển khai`: UC05, UC06
- `Vận hành và debug`: UC07, UC08

Bố cục nên ưu tiên:

- actor thao tác ở bên trái: `Operator Web`, `Operator CLI`;
- actor hệ thống ngoài ở bên phải: `GitHub / GitHub App`, `Build Worker`, `Runtime Agent`;
- các use case chính xếp theo chiều từ trái sang phải: bootstrap, deploy, observe.

## 7. Một đoạn mô tả ngắn có thể dùng ngay trong báo cáo

LazyOps ở current state là một deployment control plane đa bề mặt gồm frontend, backend, CLI và outbound runtime agent. Luồng nổi bật nhất hiện tại là `standalone`: operator liên kết repo và hạ tầng qua UX 3 bước `Connect Code`, `Connect Infra`, `Deploy`; backend xử lý GitHub webhook, build callback, revision và deployment records; sau đó runtime agent nhận command rollout qua WebSocket outbound để chuẩn bị workspace, khởi chạy candidate, chạy health gate, rồi promote hoặc rollback. Các bề mặt observability như topology, traces, logs preview, incidents và debug tunnels đã có thật, nhưng vẫn nên mô tả là các bề mặt `adapter/composed` thay vì một hệ thống observability hoàn toàn thống nhất.
