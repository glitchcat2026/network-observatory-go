# Network Observatory / 网络观测站

[English](README.md) | [简体中文](README.zh-CN.md)

本项目是一个基于 Go 实现的轻量级分布式网络监控系统。它支持从多个探测节点收集延迟、丢包率和可用性数据，并通过简洁的 Web 仪表盘展示结果。

## 项目特性

- 跨平台（Windows/Linux）
- 支持 IPV4/IPV6
- Web 仪表盘支持多语言（中文/英文）

## 贡献

```bash
git clone https://github.com/glitchcat2026/network-observatory-go.git
cd network-observatory-go
go mod download
```

## 快速开始

### 1. 启动 Collector

在 Collector 主机上运行：

```bash
./collector-linux-amd64
```

首次启动时，程序会自动生成 `collector-config.json`，然后退出。填写配置文件，然后再次运行程序。

### 2. 启动 Probe

在每台探测主机上进入可执行文件所在目录并运行：

```powershell
.\probe-windows-amd64.exe
```

```bash
./probe-linux-amd64
```

首次运行时，程序会自动生成 `probe-config.json`，然后退出。将 `collector-config.json` 中的 token 原样复制到 `probe-config.json`。填写配置文件后重新运行程序。

### 3. 打开仪表盘

在浏览器中访问：

```text
http://localhost:8080/
```

## 配置说明

### 1. Collector 配置

Collector 默认配置示例：

```json
{
  "db_path": "observatory.db",
  "addr": ":8080",
  "token": "replace-with-the-same-secret-on-every-node",
  "max_body_size": 1048576,
  "lang": "en"
}
```

参数说明：

- `db_path`：SQLite 数据库文件路径。
- `addr`：HTTP 监听地址。例如，`:8080` 表示监听所有网络接口；`127.0.0.1:8080` 表示仅监听 IPv4 本机，`[::1]:8080` 表示仅监听 IPv6 本机。
- `token`：用于验证 Probe 上报的共享密钥。
- `max_body_size`：单次上报请求体的最大大小，单位为字节。
- `lang`：Web 仪表盘的 HTML 语言标签，例如 `"en"` 或 `"zh-cn"`。

### 2. Probe 配置

Probe 默认配置示例：

```json
{
  "collector_url": "http://localhost:8080/api",
  "hostnames": ["example.com", "8.8.8.8"],
  "ip_version": "ipv4",
  "token": "replace-with-the-same-secret-on-every-node",
  "probe_name": "probe-1",
  "sleep_seconds": 10,
  "ping_timeout_seconds": 5
}
```

参数说明：

- `collector_url`：Collector 接收上报的接口地址，通常为 `http://<Collector地址>:8080/api`。
- `hostnames`：需要 Ping 探测的域名或 IP 地址列表。
- `ip_version`：Ping 和域名解析使用的 IP 版本。可选 `"ipv4"` 或 `"ipv6"`。
- `token`：随上报发送的共享密钥，必须与 Collector 配置中的 `token` 完全一致。
- `probe_name`：用于在上报和仪表盘中标识此 Probe 的名称。建议每个 Probe 使用不同名称。
- `sleep_seconds`：每轮探测结束后等待的秒数。
- `ping_timeout_seconds`：对每个目标执行 Ping 探测的最长时间，单位为秒。

Collector 会将探测结果保存到 SQLite。

## 构建

```bash
make build
```

该命令会在 `bin/` 目录下生成 Windows 和 Linux 平台的可执行文件。

## 许可证

本项目基于 MIT License 开源协议。

详情请查看 [LICENSE](LICENSE) 文件。
