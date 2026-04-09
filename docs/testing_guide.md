# Hướng Dẫn Kịch Bản Test LazyOps Chuyên Sâu

Để test thủ công dự án này với PR bạn đang có, quy trình sẽ được chia làm 3 giai đoạn. Phần khó nhất là kết nối GitHub App, nhưng bạn **không bắt buộc phải thuê server/deploy lên cloud ngay** nếu biết dùng các công cụ Tunnel.

---

## 1. Vấn đề GitHub App & Webhooks (Giải pháp Local)

GitHub App yêu cầu URL thật để gửi payload (khi người dùng tạo PR/push code). 
**Có 2 cách giải quyết:**

| Cách tiếp cận | Giải pháp | Ưu điểm | Nhược điểm |
|---|---|---|---|
| **Cách 1: Local + Tunnel (Khuyên dùng cho Dev)** | Dùng **Ngrok** hoặc **Cloudflare Tunnel (cloudflared)** để expose cổng `8080` (Backend) của máy local ra internet. | Miễn phí, debug trực tiếp trên máy bằng VSCode, log hiển thị ngay lặp tức. | Phải đổi lại Webhook URL trên GitHub mỗi khi restart Ngrok (trừ khi dùng bản trả phí). |
| **Cách 2: Deploy VPS thật** | Thuê 1 VPS rẻ (DigitalOcean/Hetzner ~5$), cài Docker, chạy Backend trên đó kèm 1 domain. | Môi trường y như thật, URL cố định không phải đổi lại. | Tốn tiền, debug khó hơn (phải xem log qua SSH), mỗi lần sửa code phải redeploy. |

> **Khuyến nghị:** Hãy cài [Ngrok](https://ngrok.com/) và chạy: `ngrok http 8080`. Copy link ngrok đó dán vào phần **Webhook URL** và **Callback URL** của GitHub App.

---

## 2. Quy Trình Test 3 Chế Độ Triển Khai

Để test PR của bạn, bạn cần đăng nhập vào LazyOps Dashboard, tạo 1 Project, kết nối GitHub Repo chứa PR đó, và bật tính năng "Auto Deploy from PR".

### ✅ Phase 2 (Lazy Mode): Cài Agent Qua SSH Rồi Quên SSH
**Mục tiêu:** User không cần tự SSH vào VPS để chạy lệnh bootstrap bằng tay.

**Flow thực tế sau khi deploy Dashboard/Backend:**
1. User vào trang `Instances` -> tạo instance mới.
2. Trong modal `Bootstrap`, nhập thông tin SSH:
   - `host`, `port`, `username`
   - `password` hoặc `private_key`
   - `control_plane_url` (URL backend của bạn)
3. Nhấn **Install via SSH**.
4. Backend sẽ:
   - phát hành bootstrap token tạm thời cho instance
   - SSH vào VPS user
   - chạy `docker run` để khởi động `lazyops-agent` (auto restart)
5. Agent tự enroll về control plane.
6. Instance chuyển sang trạng thái online -> sẵn sàng nhận deploy.

**Điểm quan trọng cho tiêu chí "lazy":**
- SSH credentials chỉ dùng cho request cài đặt đó (không cần lưu lâu dài).
- Sau khi agent đã online, các lần deploy sau đi qua control plane -> **không cần SSH lại**.
- Chỉ cần SSH lại khi agent bị mất, VPS bị recreate, hoặc bạn muốn cài lại sạch.

**Prerequisites trên VPS user:**
- Có Docker và user SSH có quyền chạy Docker.
- Mở outbound tới backend/control-plane URL.
- Nên chạy Linux server (Ubuntu/Debian/CentOS đều ổn nếu Docker chạy ổn).

**Nếu muốn gọi trực tiếp bằng API (không qua UI):**
```bash
curl -X POST "<BACKEND_URL>/api/v1/instances/<INSTANCE_ID>/install-agent/ssh" \
  -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "203.0.113.10",
    "port": 22,
    "username": "root",
    "password": "<optional>",
    "private_key": "<optional>",
    "host_key_fingerprint": "SHA256:...",
    "control_plane_url": "https://api.your-lazyops.com",
    "runtime_mode": "standalone",
    "agent_kind": "instance_agent",
    "agent_image": "tawn/lazyops-agent:latest"
  }'
```

### 🟢 Giai đoạn 1: Test Standalone (Cùng 1 máy)
**Mục tiêu:** Đảm bảo Backend và Agent giao tiếp được với nhau, code clone về được build, Gateway (Caddy) hoạt động.
1. Khởi động Backend (trên máy local hoặc VPS).
2. Khởi động Agent *trên cùng máy đó*. Copy token từ Backend để enroll Agent.
3. Tạo 1 Deployment (chọn mode `standalone`), chọn server vừa enroll.
4. Mở PR trên GitHub repo -> LazyOps Backend nhận Webhook -> Ra lệnh cho Agent.
5. **Kỳ vọng:** Agent clone code, tạo Caddyfile, routing traffic vào đúng port của app. Truy cập domain/URL được cấp phát sẽ ra app của thư mục PR đó.

### 🟡 Giai đoạn 2: Test Sidecar "Localhost Rescue"
**Mục tiêu:** Đảm bảo "sự kì diệu" của LazyOps hoạt động: App của user vẫn hardcode `localhost:5432`, nhưng LazyOps sẽ bắt traffic đó và điều hướng đến Database thật.
1. Sửa code trong PR của bạn: Hardcode kết nối database/redis vào [localhost](file:///home/tawn/lazyops/agent/internal/runtime/sidecar_manager.go#1260-1271) với 1 port cụ thể (VD: DB URL `postgres://user:pass@localhost:5432/db`).
2. Trên giao diện LazyOps, khi cài đặt môi trường cho PR, thêm 1 "Internal Service" là PostgreSQL.
3. Deploy lại PR.
4. **Kỳ vọng:** Agent kích hoạt sidecar proxy và iptables DNAT. App của bạn cố gắng gọi `localhost:5432` nhưng iptables chuyển hướng sang iptables/proxy của agent, và proxy này forward tiếp đến container PostgreSQL thật do LazyOps đang quản lý trên máy.
5. **Cách verify thủ công:** 
   - Kiểm tra iptables: `sudo iptables -t nat -L LAZYOPS_SIDECAR -n -v`
   - Gọi API của app, nếu lưu được dữ liệu nghĩa là "localhost rescue" thành công mĩ mãn.

### 🔴 Giai đoạn 3: Test Distributed Mesh (Cần 2 máy/VM)
Đây là "trùm cuối" của kiến trúc này. Cần test tính năng WireGuard tự động.
**Quá trình chuẩn bị:**
1. Cần 2 node Linux (có thể dùng 2 máy ảo VirtualBox/Multipass trên máy local, hoặc 2 con VPS). Đảm bảo cởi firewall cho port WireGuard (default `20000+`).
2. **Node 1 (Web Node):** Chạy Agent 1. Dùng làm node hứng traffic và chạy Backend của PR.
3. **Node 2 (Data Node):** Chạy Agent 2. Dùng làm node chuyên chứa Database.

**Thao tác test:**
1. Trên giao diện LazyOps, tạo kiến trúc:
   - Service App -> Đặt (*placement*) ở Node 1.
   - Target DB -> Đặt ở Node 2.
2. Thiết lập chế độ Deploy mode = `distributed_mesh`.
3. Nhấn Deploy.
4. **Kỳ vọng:** 
   - Cả hai Agent nhận lệnh. [mesh_manager.go](file:///home/tawn/lazyops/agent/internal/runtime/mesh_manager.go) trên cả hai máy tạo config WireGuard và gọi `wg-quick up`.
   - Node 1 tạo một Sidecar Proxy trên localhost, nhưng Upstream của nó chỉ thẳng vào IP WireGuard private (VD: `10.0.0.2:5432`) của Node 2.
   - Ứng dụng ở Node 1 gọi `localhost:5432` -> Iptables chặn -> Sidecar proxy nhận -> Chạy qua đường hầm WireGuard -> Sang DB ở Node 2.

**Cách verify thủ công:**
- Chạy `sudo wg show` trên cả 2 máy, kiểm tra xem có peer kết nối, có handshake (bytes received/sent) không.
- Nếu app của bạn hoạt động bình thường như thể DB đang ở local, kiến trúc đã đại thành công.

---

## 3. Lời khuyên chuẩn bị Server/Môi trường để Test
Vì chức năng thao tác sâu vào mạng lưới, LazyOps Agent **RẤT CẦN** quyền root/Admin thao tác (`sudo`).
Nếu test cục bộ trên máy tính cá nhân Linux/Mac của bạn (Local), nó có thể làm rác rules mạng của bạn và gây lỗi sau này.

**Khuyên dùng:** 
Hãy tạo các Linux Container qua **LXD/Incus** hoặc các ảo hoá nhẹ như **Multipass / UTM**. Chạy Agent bên trong đó để an toàn 100% cho máy thật của bạn. Cài sẵn:
- Git
- Docker
- Caddy (`apt install caddy`)
- WireGuard (`apt install wireguard`)
- iptables

Khi làm hỏng có thể xóa máy ảo làm lại (destroy/recreate) chỉ tốn vài chục giây.
