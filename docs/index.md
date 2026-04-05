# Trung tâm Tài liệu LazyOps

Thư mục này là trung tâm tài liệu trạng thái hiện tại cho LazyOps.

Mô hình tài liệu được thiết kế theo dạng lai:

- `guide/lazyops-implementation-master-plan.md` vẫn là hợp đồng mục tiêu.
- `docs/contracts-matrix.md` là ma trận hợp đồng trạng thái hiện tại và là nơi chính thức để kiểm tra xem một bề mặt có `implemented`, `adapter/composed`, `mock-only`, hay `missing`.
- `docs/feature-catalog.md` giải thích sản phẩm theo tính năng, luồng công việc và công nghệ.
- `/docs.md` là sổ tay kiểm thử thủ công, được tổ chức cả theo slice giao hàng và theo tính năng.

## Từ vựng Trạng thái

| Trạng thái | Ý nghĩa |
| --- | --- |
| `implemented` | Tồn tại một đường dẫn mã thực sự và được kết nối với bề mặt công khai. |
| `adapter/composed` | Hành vi hướng người dùng tồn tại, nhưng được xây dựng bằng cách kết hợp nhiều hợp đồng hoặc các phần runtime một phần. |
| `mock-only` | Hành vi chỉ tồn tại đối với CLI hoặc mock cục bộ. |
| `missing` | Hợp đồng đã được lên kế hoạch hoặc tham chiếu, nhưng chưa có bề mặt thực nào. |

## Ma trận Chế độ Runtime

| Chế độ runtime | Loại target chính | Dạng agent chính | Trách nhiệm control-plane | Trạng thái hiện tại |
| --- | --- | --- | --- | --- |
| `standalone` | `instance` | `instance_agent` | Liên kết target, biên dịch blueprint, tạo revisions, xếp hàng deployment, dispatch lệnh điều khiển, cung cấp debug tunnels. | `adapter/composed` |
| `distributed-mesh` | `mesh_network` cộng với placement instance | `instance_agent` | Liên kết target logic, giải quyết phụ thuộc mesh, phát ra topology, lập kế hoạch tunnel và chính sách overlay. | `adapter/composed` |
| `distributed-k3s` | `cluster` | `node_agent` | Giữ LazyOps ở ranh giới chính sách, telemetry và desired-state trong khi K3s lên lịch workload. | `adapter/composed` |

## Luồng công việc End-to-End

1. `login`
   - Web UI sử dụng xác thực session.
   - CLI sử dụng `POST /api/v1/auth/cli-login` để tạo PAT.
2. `target enroll`
   - Operators tạo các target `Instance`, `MeshNetwork`, hoặc `Cluster`.
   - Agents đăng ký với bootstrap tokens và tiếp tục với agent tokens.
3. `init`
   - CLI quét repo, tải hoặc tạo một `DeploymentBinding`, và ghi `lazyops.yaml`.
4. `repo link`
   - Backend đồng bộ cài đặt GitHub App, liệt kê repos, và lưu trữ `ProjectRepoLink`.
5. `webhook/build`
   - GitHub webhook tạo công việc build.
   - Hệ thống build gọi `POST /api/v1/builds/callback`.
6. `revision`
   - Backend biên dịch một `Blueprint`, tạo một `DesiredStateRevision`, và tạo một bản ghi `Deployment`.
7. `rollout`
   - Từ vựng lệnh rollout đã được khóa.
   - `standalone` giờ có đường kickoff best-effort từ `POST /api/v1/projects/:id/deployments` sang plan, dispatch lệnh, health gate, promote hoặc rollback, và garbage collect.
   - Các chế độ runtime khác vẫn còn được kết hợp một phần thay vì hoàn toàn tự động.
8. `observe`
   - Các API đọc topology, trace, và logs preview đang hoạt động.
   - Logs hiện được lưu từ `agent.log_batch` và đọc qua `GET /ws/logs/stream`.

## Hướng dẫn Bề mặt

- Backend chịu trách nhiệm lưu trữ, API công khai, tiếp nhận callback build, bản ghi revision, bản ghi target, lưu trữ trace, đọc topology, và bản ghi phiên tunnel.
- Agent chịu trách nhiệm đăng ký outbound, heartbeat, tiêu thụ kênh điều khiển, thực thi runtime, phát telemetry, và hành vi runtime hướng sidecar.
- CLI chịu trách nhiệm quét repo cục bộ, sinh `lazyops.yaml`, luồng công việc operator, và các lệnh observability/debug mỏng.
- Frontend tiêu thụ cùng các hợp đồng công khai nhưng nên sử dụng ma trận trong thư mục này thay vì đảo ngược lịch sử trình theo dõi.
