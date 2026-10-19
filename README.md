# FlyDog

局域网即时通讯与文件传输工具，**兼容飞鸽传书 / 飞秋（IPMSG）协议**，支持文件夹传输。

基于 **Go + Wails v3 + React + TypeScript** 构建，单文件 exe，无需服务器。

## 功能

- **协议兼容**：实现 IPMSG（飞鸽传书）Draft-9 协议，可与飞秋 / 飞鸽传书互通
  - UDP 2425 广播发现（`BR_ENTRY` / `ANSENTRY` / `BR_EXIT` / `BR_ABSENCE`）
  - TCP 2425 文件传输（`GETFILEDATA` 断点续传 / `GETDIRFILES` 文件夹）
  - GB18030 ⇄ UTF-8 自动编解码，兼容飞秋扩展版本号 `1_lbt6_0#128#MAC#...#9`
- **文件夹传输**：递归流式传输目录树（DIR / 文件 / `RETPARENT` 层级游标）
- **跨网段**：UDP 单播扫描 + TCP 对等通道（端口 2426），应对只放行 TCP 的企业网络
- **消息提示**：应用内弹窗 + 提示音 + 系统通知 + 未读角标 + 标题计数
- **飞秋扩展**：窗口抖动、正在输入、打开会话通知、文本格式解析
- **界面**：浅色/深色主题、头像（24 款内置经典头像 + 自定义上传）、消息分组、传输进度
- **常驻**：关闭最小化到托盘、托盘右键退出、单实例（重复启动聚焦已有窗口）
- **本地存储**：配置与聊天记录持久化（`%APPDATA%\flydog\`）

## 运行

```bat
build.bat release     :: 构建 → bin\flydog.exe
build.bat debug       :: 调试构建
build.bat dev         :: 开发模式（前端热重载）
build.bat package     :: 打包安装程序（需 NSIS）
build.bat clean       :: 清理产物
```

首次运行需允许防火墙访问 **UDP/TCP 2425、TCP 2426**；也可右键管理员运行 `firewall.bat` 自动添加规则。

> 旧版 Windows 10 需先安装 [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)。

## 开发

环境要求：Go 1.25+、Node.js 20+、[Wails v3 CLI](https://v3.wails.io)

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
npm --prefix frontend install
wails3 dev -config ./build/config.yml -port 9245
```

```bash
go test ./internal/...   # 协议层单元测试
```

## 项目结构

```
main.go                    Wails 应用入口（窗口 / 托盘 / 单实例 / 关闭到托盘）
appservice.go              前端绑定的服务（引擎控制、配置、历史记录、对话框）
internal/ipmsg/            协议层（纯 Go，不依赖 Wails，可独立测试）
  const.go                 命令字与选项常量
  packet.go                报文编解码
  codec.go                 GB18030 ⇄ UTF-8、文件名转义
  fileinfo.go              附件条目与目录头格式
  engine.go                UDP 发现 / 消息 / 成员表
  transfer.go              TCP 文件与文件夹收发
  tcp.go                   TCP 对等通道（跨网段）
  scan.go                  网段扫描
  member.go                成员表
frontend/src/              React 界面
build/                     Wails v3 构建资源
```

## 协议参考

- [IP Messenger Protocol (Draft-10)](https://github.com/shirouzu/ipmsg) — H.Shirouzu，官方实现与协议文档
- [compilelife/feiq](https://github.com/compilelife/feiq) — 飞秋（Mac）源码，飞秋扩展命令参考

## 已知限制

- 与飞秋跨 VLAN 时：IPMSG 依赖 UDP 广播，若网络策略丢弃跨网段 UDP，只能扫描/直连或改用 flydog ↔ flydog 的 TCP 通道
- 同一台机器上无法同时运行两个监听 2425 的客户端
- 聊天记录仅本机保存，不做同步（协议本身不支持）

## License

MIT
