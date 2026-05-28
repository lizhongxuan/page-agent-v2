# aiops-page-ext

aiops-page-ext 是一个基于 Chrome Side Panel 的 AI 浏览器自动化插件。它可以读取当前网页结构，并通过自然语言任务驱动浏览器完成点击、输入、滚动、切换标签页、提取信息等操作，适合 AIOps 控制台、运维后台、内部系统排障和网页流程自动化。

插件内置任务历史、人工接管、项目知识库、Playwright 用例导出，以及面向 MCP/本地应用的 Hub 连接能力。

## 功能特性

- **自然语言控制网页**：在侧边栏输入任务，例如“打开告警列表，筛选 P1 告警并总结最新 5 条”，Agent 会按步骤操作页面。
- **多标签页操作**：支持打开新标签页、切换标签页、关闭标签页，并在任务上下文中理解当前标签页状态。
- **人工接管敏感步骤**：遇到账号、密码、验证码、MFA、CAPTCHA 等敏感输入时，插件会让用户直接在网页中完成，Agent 不读取也不保存这些内容。
- **项目知识库增强**：可配置本地或内部知识库接口，在任务开始前检索 runbook、字段说明、排障文档，并注入到 Agent 上下文中。
- **任务历史与复跑**：完成或失败的任务会保存到本地 IndexedDB，可查看历史步骤、重新执行任务、导出历史 JSON。
- **Playwright 导出**：已记录的 WebOps 会话可导出为可运行的 Playwright 项目，用于回放、回归测试或转人工维护。
- **Hub/MCP 外部调用**：插件提供 Hub 页面，通过 WebSocket 接收本地应用或 MCP Server 的任务请求。
- **网页内调用接口**：配置授权 Token 后，可信网页可通过 `window.PAGE_AGENT_EXT.execute()` 调用插件执行任务。

## 环境要求

- Node.js `^22.13.0` 或 `>=24`
- npm `^11.6.3`
- Chrome 或 Chromium 系浏览器

首次安装依赖：

```bash
npm install
```

## 开发模式

启动插件开发服务：

```bash
npm run dev:ext
```

WXT 会启动 Chrome，并使用 `.wxt/chrome-data` 作为持久化开发浏览器配置目录。插件会以开发模式加载，点击浏览器工具栏中的 **AIOPS Page Ext** 图标即可打开侧边栏。

如果需要只在插件包目录运行：

```bash
npm run dev --workspace=@page-agent/ext
```

## 构建

构建并打包 Chrome 插件：

```bash
npm run build:ext
```

产物位于：

- 未压缩插件目录：`packages/extension/.output/chrome-mv3`
- zip 包：`packages/extension/.output/page-agent-ext-<version>-chrome.zip`

构建整个 monorepo：

```bash
npm run build
```

将构建产物复制到桌面，便于手动安装或分发：

```bash
npm run build:ext:desktop
```

## 手动安装插件

1. 运行 `npm run build:ext`。
2. 打开 Chrome 的 `chrome://extensions/`。
3. 开启右上角 **Developer mode**。
4. 点击 **Load unpacked**。
5. 选择 `packages/extension/.output/chrome-mv3`。

如果要发布到商店或发给其他人，可以使用 `.output` 目录中的 zip 包。

## 基本使用

1. 打开一个需要操作的网页。
2. 点击浏览器工具栏中的 **AIOPS Page Ext** 图标，打开侧边栏。
3. 点击设置按钮，配置模型：
    - **Base URL**：OpenAI-compatible API 地址，例如 `https://api.openai.com/v1`。
    - **Model**：模型名称。
    - **API Key**：模型服务密钥。
    - **Response Language**：选择系统语言、英文或中文。
4. 回到聊天页，输入自然语言任务并发送。
5. Agent 运行时可以点击停止按钮取消任务。
6. 如果 Agent 请求人工输入或敏感信息，请按侧边栏和网页浮层提示完成后继续。

示例任务：

```text
打开告警中心，筛选最近 24 小时的严重告警，按服务名汇总数量。
```

```text
进入订单列表，搜索订单号 A12345，提取当前状态和最后更新时间。
```

```text
在新标签页打开 https://example.com，查找登录按钮，但不要输入账号密码。
```

## 设置项说明

### 模型配置

插件使用 OpenAI-compatible 接口：

- `Base URL`：模型服务地址。
- `Model`：模型名称。
- `API Key`：鉴权密钥，可为空，取决于你的网关配置。

### 高级配置

- `Max Steps`：单次任务允许的最大 Agent 步数。
- `System Instruction`：附加系统提示词，用于固定业务规则、操作偏好或输出格式。
- `Disable named tool_choice`：关闭命名工具选择，适配不支持该能力的模型服务。
- `Experimental llms.txt support`：实验性读取站点 `llms.txt`。
- `Experimental include all tabs`：允许 Agent 读取和操作窗口中的所有未固定标签页，而不是只关注当前任务标签组。

### 项目知识库

设置页的“项目知识库”用于在任务开始前检索业务文档，并将命中的内容注入到 `<project_knowledge>` 中。

配置项：

- `Knowledge Base URL`：知识库服务地址。
- `Project Key`：项目标识，例如 `aiops`。
- `Knowledge API Key`：知识库接口鉴权密钥，可为空。
- `允许发送页面摘要`：预留开关，用于控制知识库检索是否可以携带页面摘要。

仓库中提供了一个本地知识库示例：

```bash
cd tools/local-knowledge-base
docker compose up -d --build
curl http://127.0.0.1:8787/health
```

然后在插件中配置：

- Knowledge Base URL: `http://127.0.0.1:8787`
- Project Key: `aiops`
- API Key: 默认留空

## 历史记录和导出

侧边栏顶部的历史按钮可以查看本地任务历史。每条历史支持：

- 查看任务执行事件流。
- 重新执行同一任务。
- 删除单条历史或清空全部历史。
- 导出历史 JSON。
- 对带 WebOps 记录的任务导出 Playwright 回放项目。

导出的 Playwright zip 包包含：

- `package.json`
- `playwright.config.ts`
- `tests/replay.spec.ts`
- `fixtures/session.json`
- `README.md`

敏感值会在导出前脱敏；人工接管步骤会以注释形式保留，需要人工补齐或重新设计自动化流程。

## Hub 和 MCP 使用

插件包含 Hub 页面，用于让本地应用通过 WebSocket 调用浏览器 Agent。可以在设置页点击 **Manage Page Agent Hub** 打开。

Hub 协议：

```text
Caller -> Hub
{ type: "execute", task: string, config?: object }
{ type: "stop" }

Hub -> Caller
{ type: "ready" }
{ type: "result", success: boolean, data: string }
{ type: "error", message: string }
```

配合仓库内 MCP Server 调试：

```bash
npm run dev:ext
npx @modelcontextprotocol/inspector node packages/mcp/src/index.js
```

MCP Server 会启动本地 HTTP/WebSocket 服务，打开 launcher 页面，再由插件打开 Hub 标签页并建立连接。默认每个外部连接都需要用户确认；如果在 Hub 页面开启 **Auto-approve connections**，后续连接会自动通过，请只在可信环境中使用。

## 网页内调用

插件可以把调用能力暴露给可信网页。流程如下：

1. 在插件设置中复制 `User Auth Token`。
2. 在目标网页的 `localStorage` 写入同名 Token：

```js
localStorage.setItem('PageAgentExtUserAuthToken', '<copied-token>')
```

3. 刷新页面。Token 匹配后，页面中会出现 `window.PAGE_AGENT_EXT`。
4. 在网页脚本中调用：

```js
await window.PAGE_AGENT_EXT.execute('点击搜索按钮并总结结果', {
    baseURL: 'https://api.openai.com/v1',
    model: 'gpt-5.1',
    apiKey: '<your-api-key>',
    onStatusChange(status) {
        console.log('status', status)
    },
    onActivity(activity) {
        console.log('activity', activity)
    },
    onHistoryUpdate(history) {
        console.log('history', history)
    },
})
```

停止当前任务：

```js
window.PAGE_AGENT_EXT.stop()
```

## 常用命令

```bash
npm run dev:ext              # 启动插件开发模式
npm run build:ext            # 构建并打包插件 zip
npm run build:ext:desktop    # 构建插件并复制到桌面
npm run build                # 构建全部包
npm run typecheck            # 类型检查
npm run lint                 # ESLint
npm test                     # 运行测试
npm run cleanup              # 清理 dist 和 .output
```

## 权限和安全说明

插件需要以下浏览器权限：

- `tabs`、`tabGroups`：读取和管理任务相关标签页。
- `sidePanel`：提供侧边栏交互界面。
- `storage`：保存模型配置、历史记录、Hub 授权状态等本地数据。
- `<all_urls>`：在用户打开的网页中注入内容脚本，以读取可交互元素并执行网页操作。

安全建议：

- 不要让 Agent 读取或保存密码、验证码、MFA、API Key 等敏感值。
- 遇到登录、验证码、支付、权限变更等高风险步骤时，优先使用人工接管。
- 只在可信网页中配置 `PageAgentExtUserAuthToken`。
- 只在可信本地环境中开启 Hub 的自动批准连接。

## 项目结构

```text
packages/extension/                 # aiops-page-ext 插件主体
  src/entrypoints/background.ts      # 扩展后台脚本
  src/entrypoints/content.ts         # 内容脚本和页面注入逻辑
  src/entrypoints/sidepanel/         # Chrome Side Panel UI
  src/entrypoints/hub/               # Hub 页面和 WebSocket 协议
  src/agent/                         # MultiPageAgent、标签页控制、人工接管
  src/webops/                        # 知识库、交互浮层、录制、Playwright 导出
packages/core/                      # PageAgentCore
packages/page-controller/           # DOM 提取与网页操作
packages/llms/                      # OpenAI-compatible LLM 客户端
packages/mcp/                       # MCP Server
tools/local-knowledge-base/          # 本地知识库示例服务
```
