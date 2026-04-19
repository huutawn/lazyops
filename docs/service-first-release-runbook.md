# Service-First Release Runbook

## 1. Add node vào cluster

- Mở `Clusters` và chọn cluster managed hiện có.
- Bấm `Add node`, nhập `instance_name`, IP public/private, và thông tin SSH.
- Chờ join stages hoàn tất:
  - K3s agent cài xong
  - node xuất hiện trong `Node inventory`
  - node được gắn label `lazyops.io/instance-id`
- Xác nhận node mới hiện trong placement picker của project.

## 2. Tạo internal Postgres

- Vào `Projects -> Services`.
- Chọn `Them service -> Internal Postgres`.
- Đặt tên service, mặc định nên dùng `db` nếu đây là database chính của project.
- Sau khi lưu:
  - service xuất hiện trong unified inventory
  - repo service khác có thể gán `postgres.basic` tới service này
  - trên `distributed-k3s`, host runtime sẽ là DNS nội bộ của service, không phải `localhost`

## 3. Deploy service hoặc deploy project

- Deploy một service:
  - trong service card, bấm `Deploy`
  - LazyOps tạo deployment mới chỉ cho service đó, đồng thời tự kéo theo internal dependency cần thiết
- Deploy toàn project:
  - dùng lane `Deployments` hoặc one-click flow hiện tại
  - backend tự compile internal deploy plan từ service inventory, env bundle, bindings, và connection templates
- Không cần compile blueprint thủ công trong normal flow.

## 4. Kiểm tra env inject

- Vào `Biến môi trường`.
- Xác nhận helper snippets cho Postgres có đủ:
  - `DB_URL`
  - `DB_NAME`
  - `DB_HOST`
  - `DB_PORT`
  - `DB_USERNAME`
  - `DB_PASSWORD`
- Với `distributed-k3s`, `DB_HOST` phải là service DNS như `db` hoặc `<service-name>`, không phải `localhost`.

## 5. Debug runtime, logs, placement

- Vào `Logs / Runtime` để xem:
  - runtime status theo service
  - rollout state
  - recent logs
  - effective node placement
  - internal endpoints
- Nếu service `pinned_node` chưa chạy đúng node:
  - kiểm tra node có `Ready`
  - kiểm tra `placement_node_id` còn tồn tại trong cluster inventory
  - kiểm tra service card có hiển thị node đích đúng hay không
- Nếu chưa có deploy/log/metrics:
  - UI phải hiện empty state rõ ràng thay vì blank
  - ưu tiên xác nhận deployment đã được tạo và rollout đã bắt đầu

## 6. Legacy / debug lanes

- `Blueprint`, `Validate`, `Bindings`, `Routing`, `Integrations` được giữ lại cho expert/debug.
- Không dùng các lane này làm bước bắt buộc trong flow deploy chuẩn.
- Nếu thấy lỗi kiểu `blueprint_not_found` trong flow chuẩn, coi đó là regression của compatibility layer.
