# ClipLink

局域网剪贴板同步工具，支持 macOS 和 Windows 设备之间自动共享剪贴板内容。

## 功能

- 同一局域网内的设备剪贴板实时同步
- 通过 mDNS（Bonjour/零配置）自动发现对等设备，无需手动配置 IP
- 支持 macOS 系统托盘图标与开机自启
- 支持 Windows（开机自启通过注册表实现）

## 安装

### 直接下载

从 [Releases](../../releases) 页面下载对应平台的二进制文件。

### 从源码构建

需要 Go 1.21+

```bash
# macOS
make build

# Windows (交叉编译)
make build-windows

# 全平台
make build-all
```

## 使用

直接运行可执行文件，程序会在系统托盘显示图标。

```bash
./cliplink
# 或指定端口（默认 56789）
./cliplink -port 56789
```

在同一局域网内的多台设备上运行 ClipLink 后，复制的内容会自动同步到所有设备。

## 工作原理

1. 每个节点启动时通过 mDNS 广播自身服务
2. 节点间通过 TCP 长连接传输剪贴板内容
3. 使用消息 ID 去重，防止内容在节点间循环转发
4. 轮询本地剪贴板变化（300ms 间隔），有变化时广播给所有已连接节点

## 依赖

- [fyne.io/systray](https://github.com/fyne-io/systray) — 系统托盘
- [grandcat/zeroconf](https://github.com/grandcat/zeroconf) — mDNS 服务发现

## License

[MIT](LICENSE)
