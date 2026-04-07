# Mô Tả Cách Vẽ Use Case Cho PR LazyOps

Tài liệu này dùng để vẽ lại sơ đồ use case bằng UML use case chuẩn trong StarUML, draw.io, Visio hoặc Word.

Nguyên tắc chung khi vẽ:

- Vẽ một khung hệ thống tên là `LazyOps Platform`.
- Actor đặt ở ngoài khung hệ thống.
- Use case đặt trong khung hệ thống, dùng hình oval.
- Đường nối giữa actor và use case là đường thẳng association.
- Nếu sơ đồ quá lớn, tách thành nhiều sơ đồ nhỏ theo nhóm chức năng.

## 1. Sơ đồ use case tổng quát

### Tên sơ đồ

`Use Case Tổng Quát Hệ Thống LazyOps`

### Actor cần vẽ

- `Operator Web`
- `Operator CLI`
- `GitHub / GitHub App`
- `Build Worker`
- `Runtime Agent`

### Use case cần vẽ

- `Đăng nhập và quản lý phiên`
- `Tạo project và chọn runtime`
- `Onboard target và đăng ký agent`
- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `Liên kết repo với GitHub App`
- `Xử lý webhook, build callback và đối soát artifact`
- `Tạo deployment và rollout standalone`
- `Quan sát, topology, logs, traces và tunnel debug`

### Actor nối với use case nào

`Operator Web` nối với:

- `Đăng nhập và quản lý phiên`
- `Tạo project và chọn runtime`
- `Onboard target và đăng ký agent`
- `Liên kết repo với GitHub App`
- `Tạo deployment và rollout standalone`
- `Quan sát, topology, logs, traces và tunnel debug`

`Operator CLI` nối với:

- `Đăng nhập và quản lý phiên`
- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `Liên kết repo với GitHub App`
- `Quan sát, topology, logs, traces và tunnel debug`

`GitHub / GitHub App` nối với:

- `Liên kết repo với GitHub App`
- `Xử lý webhook, build callback và đối soát artifact`

`Build Worker` nối với:

- `Xử lý webhook, build callback và đối soát artifact`

`Runtime Agent` nối với:

- `Onboard target và đăng ký agent`
- `Tạo deployment và rollout standalone`
- `Quan sát, topology, logs, traces và tunnel debug`

### Gợi ý bố cục

- Đặt `Operator Web` và `Operator CLI` bên trái.
- Đặt `GitHub / GitHub App`, `Build Worker`, `Runtime Agent` bên phải.
- Các use case đặt trong khung hệ thống theo 3 hàng:
  - hàng 1: khởi tạo
  - hàng 2: tích hợp và triển khai
  - hàng 3: quan sát và debug

## 2. Sơ đồ use case nhóm khởi tạo và tích hợp

### Tên sơ đồ

`Use Case Khởi Tạo Nền Tảng Và Tích Hợp Repo`

### Actor cần vẽ

- `Operator Web`
- `Operator CLI`
- `GitHub / GitHub App`
- `Runtime Agent`

### Use case cần vẽ

- `Tạo project và chọn runtime`
- `Onboard target và đăng ký agent`
- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `Liên kết repo với GitHub App`

### Actor nối với use case nào

`Operator Web` nối với:

- `Tạo project và chọn runtime`
- `Onboard target và đăng ký agent`
- `Liên kết repo với GitHub App`

`Operator CLI` nối với:

- `Khởi tạo lazyops.yaml và DeploymentBinding`
- `Liên kết repo với GitHub App`

`GitHub / GitHub App` nối với:

- `Liên kết repo với GitHub App`

`Runtime Agent` nối với:

- `Onboard target và đăng ký agent`

### Quan hệ logic nên mô tả trong thuyết minh

- `Tạo project và chọn runtime` là bước trước của `Onboard target và đăng ký agent`.
- `Onboard target và đăng ký agent` là bước trước của `Khởi tạo lazyops.yaml và DeploymentBinding`.
- `Khởi tạo lazyops.yaml và DeploymentBinding` là bước trước của `Liên kết repo với GitHub App`.

Lưu ý:

- Ba quan hệ trên thường chỉ mô tả trong phần diễn giải.
- Nếu giáo viên muốn, có thể dùng mũi tên phụ chú thay vì include/extend vì đây là quan hệ quy trình, không phải tái sử dụng use case điển hình.

## 3. Sơ đồ use case nhóm triển khai và vận hành

### Tên sơ đồ

`Use Case Triển Khai Và Điều Phối Runtime`

### Actor cần vẽ

- `Operator Web`
- `GitHub`
- `Build Worker`
- `Runtime Agent`

### Use case cần vẽ

- `Xử lý webhook, build callback và đối soát artifact`
- `Tạo deployment và rollout standalone`

### Actor nối với use case nào

`GitHub` nối với:

- `Xử lý webhook, build callback và đối soát artifact`

`Build Worker` nối với:

- `Xử lý webhook, build callback và đối soát artifact`

`Operator Web` nối với:

- `Tạo deployment và rollout standalone`

`Runtime Agent` nối với:

- `Tạo deployment và rollout standalone`

### Quan hệ logic nên mô tả trong thuyết minh

- `Xử lý webhook, build callback và đối soát artifact` cung cấp đầu vào artifact cho `Tạo deployment và rollout standalone`.
- `Tạo deployment và rollout standalone` là use case trung tâm của phần vận hành runtime hiện tại.

## 4. Sơ đồ use case nhóm quan sát và debug

### Tên sơ đồ

`Use Case Quan Sát Và Debug Hệ Thống`

### Actor cần vẽ

- `Operator Web`
- `Operator CLI`
- `Runtime Agent`

### Use case cần vẽ

- `Quan sát topology`
- `Xem logs`
- `Xem traces`
- `Theo dõi incidents`
- `Tạo tunnel debug DB/TCP`

### Actor nối với use case nào

`Operator Web` nối với:

- `Quan sát topology`
- `Xem logs`
- `Xem traces`
- `Theo dõi incidents`

`Operator CLI` nối với:

- `Xem logs`
- `Xem traces`
- `Tạo tunnel debug DB/TCP`

`Runtime Agent` nối với:

- `Quan sát topology`
- `Xem logs`
- `Xem traces`
- `Theo dõi incidents`

### Cách hiểu khi vẽ

- Với `Runtime Agent`, đường nối mang nghĩa agent là nguồn phát sinh dữ liệu cho các use case observability.
- Với `Operator Web` và `Operator CLI`, đường nối mang nghĩa người vận hành là người sử dụng các chức năng quan sát/debug.

## 5. Danh sách use case chi tiết để vẽ riêng nếu cần

Nếu cần tách thành nhiều sơ đồ nhỏ hơn nữa, có thể vẽ riêng từng use case chính như sau.

### UC01. Đăng nhập và quản lý phiên

Actor:

- `Operator Web`
- `Operator CLI`

Use case:

- `Đăng nhập bằng email/password`
- `Đăng nhập bằng OAuth`
- `Tạo PAT cho CLI`
- `Thu hồi PAT`

Đường nối:

`Operator Web` nối với:

- `Đăng nhập bằng email/password`
- `Đăng nhập bằng OAuth`

`Operator CLI` nối với:

- `Đăng nhập bằng email/password`
- `Tạo PAT cho CLI`
- `Thu hồi PAT`

### UC02. Tạo project và chọn runtime

Actor:

- `Operator Web`

Use case:

- `Tạo project`
- `Chọn runtime mode`

Đường nối:

`Operator Web` nối với:

- `Tạo project`
- `Chọn runtime mode`

### UC03. Onboard target và đăng ký agent

Actor:

- `Operator Web`
- `Runtime Agent`

Use case:

- `Tạo instance`
- `Tạo mesh network`
- `Tạo cluster`
- `Phát bootstrap token`
- `Agent enroll`
- `Agent heartbeat`
- `Mở control channel`

Đường nối:

`Operator Web` nối với:

- `Tạo instance`
- `Tạo mesh network`
- `Tạo cluster`
- `Phát bootstrap token`

`Runtime Agent` nối với:

- `Agent enroll`
- `Agent heartbeat`
- `Mở control channel`

### UC04. Khởi tạo lazyops.yaml và DeploymentBinding

Actor:

- `Operator CLI`

Use case:

- `Quét repository`
- `Phát hiện services`
- `Lấy danh sách project và target`
- `Chọn hoặc tạo DeploymentBinding`
- `Sinh lazyops.yaml`
- `Kiểm tra an toàn cấu hình`
- `Ghi file lazyops.yaml`

Đường nối:

`Operator CLI` nối với toàn bộ các use case trên.

### UC05. Liên kết repo với GitHub App

Actor:

- `Operator Web`
- `Operator CLI`
- `GitHub / GitHub App`

Use case:

- `Đồng bộ GitHub installations`
- `Liệt kê repositories khả dụng`
- `Liên kết repo với project`
- `Chọn tracked branch`

Đường nối:

`Operator Web` nối với:

- `Đồng bộ GitHub installations`
- `Liên kết repo với project`
- `Chọn tracked branch`

`Operator CLI` nối với:

- `Đồng bộ GitHub installations`
- `Liên kết repo với project`
- `Chọn tracked branch`

`GitHub / GitHub App` nối với:

- `Đồng bộ GitHub installations`
- `Liệt kê repositories khả dụng`

### UC06. Xử lý webhook, build callback và đối soát artifact

Actor:

- `GitHub`
- `Build Worker`

Use case:

- `Nhận GitHub webhook`
- `Kiểm tra chữ ký webhook`
- `Chuẩn hóa sự kiện`
- `Tạo build job`
- `Nhận build callback`
- `Đối soát artifact`
- `Tạo artifact-ready revision`

Đường nối:

`GitHub` nối với:

- `Nhận GitHub webhook`

`Build Worker` nối với:

- `Nhận build callback`

### UC07. Tạo deployment và rollout standalone

Actor:

- `Operator Web`
- `Runtime Agent`

Use case:

- `Tạo deployment`
- `Lập kế hoạch rollout`
- `Dispatch command`
- `Chạy health gate`
- `Promote release`
- `Rollback release`
- `Ghi incident`

Đường nối:

`Operator Web` nối với:

- `Tạo deployment`

`Runtime Agent` nối với:

- `Dispatch command`
- `Chạy health gate`
- `Promote release`
- `Rollback release`

Gợi ý mô tả quan hệ:

- `Tạo deployment` dẫn tới `Lập kế hoạch rollout`
- `Lập kế hoạch rollout` dẫn tới `Dispatch command`
- `Dispatch command` dẫn tới `Chạy health gate`
- `Chạy health gate` rẽ nhánh sang `Promote release` hoặc `Rollback release`
- `Rollback release` thường đi kèm `Ghi incident`

### UC08. Quan sát, topology, logs, traces và tunnel debug

Actor:

- `Operator Web`
- `Operator CLI`
- `Runtime Agent`

Use case:

- `Gửi topology state`
- `Gửi log batch`
- `Gửi trace summary`
- `Xem topology`
- `Xem logs`
- `Xem trace`
- `Xem correlated observability`
- `Tạo tunnel debug`
- `Đóng tunnel debug`

Đường nối:

`Runtime Agent` nối với:

- `Gửi topology state`
- `Gửi log batch`
- `Gửi trace summary`

`Operator Web` nối với:

- `Xem topology`
- `Xem logs`
- `Xem trace`
- `Xem correlated observability`

`Operator CLI` nối với:

- `Xem logs`
- `Xem trace`
- `Tạo tunnel debug`
- `Đóng tunnel debug`

## 6. Mẫu câu mô tả để ghi dưới sơ đồ

Bạn có thể dùng đoạn mô tả sau trong báo cáo:

> Sơ đồ use case của hệ thống LazyOps được xây dựng với các actor chính gồm Operator Web, Operator CLI, GitHub/GitHub App, Build Worker và Runtime Agent. Các actor này tương tác với các nhóm chức năng chính của hệ thống như xác thực, khởi tạo project và target, tạo `lazyops.yaml`, liên kết repository, xử lý webhook/build callback, triển khai ứng dụng và quan sát hệ thống. Đường nối giữa actor và use case thể hiện actor nào trực tiếp sử dụng hoặc tham gia vào chức năng đó.

## 7. Gợi ý vẽ nhanh

Nếu bạn cần vẽ thật nhanh bằng tay hoặc bằng công cụ UML:

1. Vẽ khung `LazyOps Platform`.
2. Đặt actor người dùng bên trái: `Operator Web`, `Operator CLI`.
3. Đặt actor hệ thống ngoài bên phải: `GitHub / GitHub App`, `Build Worker`, `Runtime Agent`.
4. Đặt các use case trong khung theo cụm:
   - cụm khởi tạo
   - cụm tích hợp
   - cụm triển khai
   - cụm quan sát
5. Nối actor với đúng các use case đã liệt kê ở trên.
6. Nếu sơ đồ tổng quát quá rối, tách thành 3 sơ đồ nhỏ:
   - khởi tạo và tích hợp
   - triển khai
   - observability và debug
