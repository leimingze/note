# Rust/codex-rs

在存放 Rust 代码的 codex-rs 目录中：

- Crate 名称统一使用 `codex-` 前缀。例如，`core` 目录中的 crate 命名为 `codex-core`
- 使用 format! 时，如果变量可以内联到 {} 中，务必内联。
- 在执行本仓库的指令前，如果仓库依赖的命令（例如 `just`、`rg` 或 `cargo-insta`）尚未安装，请先安装它们。
- 不要新增或修改任何与 `CODEX_SANDBOX_NETWORK_DISABLED_ENV_VAR` 或 `CODEX_SANDBOX_ENV_VAR` 相关的代码。
  - 你在沙箱中操作，使用 `shell` 工具时始终会设置 `CODEX_SANDBOX_NETWORK_DISABLED=1`。任何使用 `CODEX_SANDBOX_NETWORK_DISABLED_ENV_VAR` 的现有代码都是基于这一事实编写的。它通常用于在作者已知你因沙箱限制而无法运行的测试中提前退出。
  - 同样，当你使用 Seatbelt（`/usr/bin/sandbox-exec`）启动进程时，子进程会设置 `CODEX_SANDBOX=seatbelt`。需要自行运行 Seatbelt 的集成测试无法在 Seatbelt 下运行，因此对 `CODEX_SANDBOX=seatbelt` 的检查也常用于在合适的测试中提前退出。
- 始终按照 https://rust-lang.github.io/rust-clippy/master/index.html#collapsible_if 折叠 if 语句
- 尽可能按照 https://rust-lang.github.io/rust-clippy/master/index.html#uninlined_format_args 内联 format! 参数
- 尽可能使用方法引用而不是闭包，参见 https://rust-lang.github.io/rust-clippy/master/index.html#redundant_closure_for_method_calls
- 避免使用布尔值或含义模糊的 `Option` 参数，它们会迫使调用方写出 `foo(false)` 或 `bar(None)` 这类难以阅读的代码。在能保持调用点自解释时，优先使用枚举、命名方法、newtype 或其他惯用的 Rust API 形态。
- 如果无法做出上述 API 修改，且仍需要少量按位置传字面量的调用点，请遵循 `argument_comment_lint` 约定：
  - 在按位置传递不透明字面量参数（如 `None`、布尔值和数字字面量）之前，使用精确的 `/*param_name*/` 注释。
  - 当方法名与参数名一致时，方法唯一非 self 参数可豁免，例如 `fn enabled(&self, enabled: bool)` 的调用 `.enabled(false)`。
  - 不要为字符串或字符字面量添加此类注释，除非注释确实能增加清晰度；这些字面量有意豁免于该 lint。
  - 注释中的参数名必须与调用函数签名完全一致。
  - 可以在本地运行 `just argument-comment-lint` 执行该 lint 检查。它由 Bazel 驱动，因此首次运行如果 Bazel 尚未预热可能会较慢，但增量调用通常应少于 15 秒。大多数情况下，最好更新 PR 并让 CI 负责检查（或在提交 PR 后在后台异步运行）。注意 CI 会检查全部三个平台，而本地运行不会。
- 尽可能让 `match` 语句穷尽，避免使用通配符分支。
- 新增 trait 应包含说明其角色以及实现方预期使用方式的文档注释。
- 在 Rust trait 中，不鼓励使用 `#[async_trait]` 和 `#[allow(async_fn_in_trait)]`。
  - 优先使用原生 RPITIT trait 方法，并对返回的 future 显式添加 `Send` 约束，参见 `3c7f013f9735` / `#16630`。
  - 推荐的 trait 形态：
    `fn foo(&self, ...) -> impl std::future::Future<Output = T> + Send;`
  - 实现仍可在满足该契约时使用 `async fn foo(&self, ...) -> T`。
  - 不要使用 `#[allow(async_fn_in_trait)]` 作为绕过显式书写 future 契约的捷径。
- 编写测试时，优先对整个对象做相等比较，而不是逐字段比较。
- 不要为静态定义的值添加测试。
- 不要为已删除的逻辑添加负向测试。
- 不要向 `docs/` 文件夹添加面向产品或用户的通用文档。官方 Codex 文档在其他位置维护。例外是 app-server API 文档，其由下文 app-server 指南覆盖。
- 优先使用私有模块和显式导出的公共 crate API。
- 如果修改了 `ConfigToml` 或嵌套配置类型，请运行 `just write-config-schema` 更新 `codex-rs/core/config.schema.json`。
- 处理 MCP 工具调用时，优先使用 `codex-rs/codex-mcp/src/mcp_connection_manager.rs` 处理工具及工具调用的变更。尽量缩小改动范围，利用现有抽象，而不是把代码层层贯穿多个函数调用层级。
- 不要不必要地调用 `reset_client_session`；让增量检查逻辑决定是否复用前一个请求。
- 如果修改了 Rust 依赖（`Cargo.toml` 或 `Cargo.lock`），请在仓库根目录运行 `just bazel-lock-update` 刷新 `MODULE.bazel.lock`，并将该锁文件更新包含在同一次变更中。CI 会验证锁文件是否有漂移。
- Bazel 不会自动将源码树中的文件提供给 Rust 编译期文件访问。如果新增 `include_str!`、`include_bytes!`、`sqlx::migrate!` 或类似的编译期文件/目录读取，请同步更新对应 crate 的 `BUILD.bazel`（`compile_data`、`build_script_data` 或测试数据），否则即使 Cargo 能通过，Bazel 也可能失败。
- 不要创建只被引用一次的辅助小方法。
- 对异步任务进行追踪时，请在函数或方法定义上使用 `#[tracing::instrument(...)]`，而不是在调用点用 `.instrument(...)` 给 future 附加 span。添加插桩前，先检查被调用方（或其直接委托的实现方法）是否已经插桩。
- 避免大模块：
  - 优先新增模块，而不是扩展现有模块。
  - Rust 模块目标控制在 500 行以内（不含测试）。
  - 如果文件超过约 800 行，请在新模块中添加新功能，而不是扩展现有文件，除非有充分的文档化理由。
  - 此规则尤其适用于已经吸引无关改动的高频文件，例如 `codex-rs/tui/src/app.rs`、`codex-rs/tui/src/bottom_pane/chat_composer.rs`、`codex-rs/tui/src/bottom_pane/footer.rs`、`codex-rs/tui/src/chatwidget.rs`、`codex-rs/tui/src/bottom_pane/mod.rs` 以及类似的中心编排模块。
  - 从大模块提取代码时，将相关测试和模块/类型文档一并迁移到新实现中，使不变量保持在拥有它们的代码附近。
  - 除非改动微不足道，否则不要向 `codex-rs/tui/src/chatwidget.rs` 添加新的独立方法；优先使用新模块/文件，让 `chatwidget.rs` 专注于编排。
- 运行 Rust 命令（如 `just fix` 或 `just test`）时要耐心等待，绝不要试图用 PID 终止它们。Rust 锁可能导致执行缓慢，这是预期行为。

完成本仓库任何代码修改后，自动运行 `just fmt`（在 `codex-rs` 目录中）；不要为此请求批准。此外，运行测试：

1. 不要直接运行 `cargo test`。使用 `just test`，以便测试执行遵循仓库默认配置。
2. 运行被修改项目的特定测试。例如，如果修改发生在 `codex-rs/tui`，运行 `just test -p codex-tui`。
3. 这些测试通过后，如果 common、core 或 protocol 有任何修改，请运行完整测试套件 `just test`。常规本地运行避免使用 `--all-features`，因为它会扩大构建矩阵并显著增加 `target/` 磁盘占用；仅在确实需要完整功能覆盖时使用。项目级或单个测试可以在不询问用户的情况下运行，但运行完整测试套件前请先征得用户同意。

在最终确定 `codex-rs` 的大型改动之前，运行 `just fix -p <project>`（在 `codex-rs` 目录中）修复代码中的 lint 问题。优先使用 `-p` 限定范围，避免整个 workspace 的 Clippy 构建变慢；只有在修改了共享 crate 时才运行不带 `-p` 的 `just fix`。运行 `fix` 或 `fmt` 后不要重新运行测试。

## `codex-core` crate

随着时间推移，`codex-core` crate（定义于 `codex-rs/core/`）由于是最大的 crate 而变得臃肿，因此人们常常更愿意把新东西直接加到 `codex-core`，而不是重构出所需的库代码，从而既不让新代码依赖 `codex-core`，也不增加其体积。

为此：**抵制向 codex-core 添加代码！**

特别是在引入新概念/功能/API 时，在添加到 `codex-core` 之前，请考虑：

- 是否存在一个除 `codex-core` 之外合适的现有 crate 可以承载新代码。
- 是否为你的新功能引入 Cargo workspace 新 crate 的时机。必要时重构现有代码以实现这一点。

同样，审查代码时，不要犹豫地对不必要向 `codex-core` 添加代码的 PR 提出反对意见。

## 代码审查规则

### Crate API 表面

尽量保持 crate API 表面最小化。避免扩散仅供测试使用的辅助工具。

### 模型可见上下文

Codex 维护一个上下文（消息历史），在推理请求中发送给模型。

1. 不重写历史 - 上下文必须增量构建。
2. 避免频繁更改上下文导致缓存未命中。
3. 没有无界内容 - 注入模型上下文的每一项都必须有有界大小和硬上限。
4. 没有超过 10K token 的项。
5. 将可能超过 1k token 的新增独立项标记为 P0。这些需要额外的人工审查。
6. 所有注入片段必须在 `core/context` 中定义为 struct，并实现 ContextualUserFragment trait

### 破坏性变更

搜索外部集成表面上的破坏性变更：

- app-server API
- 原始响应项事件（`rawResponseItem/*`），即使仍处于实验阶段
- CLI 参数
- 配置加载
- 从现有 rollout 恢复会话

### 测试编写指南

对于 agent 相关变更，优先使用集成测试而非单元测试。集成测试位于 `core/suite` 下，使用 `test_codex` 设置一个测试用 codex 实例。

改变 agent 逻辑的功能必须添加集成测试：

- 提供需要测试的主要逻辑变更和面向用户行为列表。

如果需要单元测试，请将它们放在专门的测试文件（\*_tests.rs）中。
避免在主实现中添加仅用于测试的函数。

检查是否有现成的辅助函数可以让测试更简洁、更易读。

### 变更规模指南（800 行）

除非是机械性变更，否则总变更行数不应超过 800 行。
对于复杂逻辑变更，规模应控制在 500 行以内。

如果变更更大，请探索是否可以拆分为可评审的阶段，并确定最先落地的、最小的连贯阶段。
阶段拆分建议应基于实际 diff、依赖关系和受影响的调用点。

## TUI 样式约定

参见 `codex-rs/tui/styles.md`。

## TUI 代码约定

- 使用 ratatui 的 Stylize trait 中简洁的样式辅助。
  - 基本 span：使用 "text".into()
  - 带样式 span：使用 "text".red()、 "text".green()、 "text".magenta()、 "text".dim() 等。
  - 优先使用这些方式，而不是直接用 `Span::styled` 和 `Style` 构造样式。
  - 示例：补丁摘要文件行
    - 期望：vec!["  └ ".into(), "M".red(), " ".dim(), "tui/src/app.rs".dim()]

### TUI 样式（ratatui）

- 优先使用 Stylize 辅助：尽可能使用 "text".dim()、 .bold()、 .cyan()、 .italic()、 .underlined()，而不是手动构造 Style。
- 优先使用简单转换：span 使用 "text".into()，line 使用 vec![…].into()；当类型推断有歧义（例如 Paragraph::new/Cell::from）时，使用 Line::from(spans) 或 Span::from(text)。
- 计算样式：如果 Style 是在运行时计算的，使用 `Span::styled` 是可以的（`Span::from(text).set_style(style)` 也可接受）。
- 避免硬编码白色：不要使用 `.white()`；优先使用默认前景色（不指定颜色）。
- 链式调用：为可读性组合辅助函数（例如 url.cyan().underlined()）。
- 单项：优先使用 "text".into()；仅当目标类型在上下文中不明显，或使用 .into() 需要额外类型注解时，才使用 Line::from(text) 或 Span::from(text)。
- 构建 line：当目标类型明显且无需额外类型注解时，使用 vec![…].into() 构建 Line；否则使用 Line::from(vec![…])。
- 避免无谓改动：不要在等价形式之间重构（Span::styled ↔ set_style、Line::from ↔ .into()），除非有明确的可读性或功能性收益；遵循文件局部约定，不要仅为满足 .into() 引入类型注解。
- 紧凑性：优先选择 rustfmt 后保持单行的形式；如果 Line::from(vec![…]) 和 vec![…].into() 中只有一种能避免换行，选择它。如果两者都会换行，选择换行更少的形式。

### 文本换行

- 始终使用 textwrap::wrap 包装纯字符串。
- 如果有 ratatui Line 并需要换行，使用 tui/src/wrapping.rs 中的辅助函数，例如 word_wrap_lines / word_wrap_line。
- 如果需要缩进换行行，尽可能使用 RtOptions 中的 initial_indent / subsequent_indent 选项，而不是编写自定义逻辑。
- 如果有一组行需要统一添加前缀（首行与后续行可选不同），使用 line_utils 中的 `prefix_lines` 辅助函数。

## 测试

### 测试模块组织

- 添加新测试模块时，将其内容定义在单独的兄弟文件中，而不是内联在实现文件中。
- 使用显式的 `#[path = "..._tests.rs"]` 属性，使测试文件名具有描述性且易于定位：

  ```rust
  #[cfg(test)]
  #[path = "parser_tests.rs"]
  mod tests;
  ```

- 这仅适用于引入新测试模块时。不要仅仅为了遵循此约定而移动或重写现有的内联 `#[cfg(test)] mod tests { ... }` 模块。

### 快照测试

本仓库使用快照测试（通过 `insta`），尤其是在 `codex-rs/tui` 中，用于验证渲染输出。

**要求：** 任何影响用户可见 UI 的变更（包括新增 UI）都必须包含相应的 `insta` 快照覆盖（如果尚不存在则新增快照测试，或更新现有快照）。将快照更新作为 PR 的一部分进行审查和接受，以便 UI 影响易于审查，未来 diff 保持可视化。

当有意更改 UI 或文本输出时，按以下步骤更新快照：

- 运行测试生成更新的快照：
  - `just test -p codex-tui`
- 检查待处理内容：
  - `cargo insta pending-snapshots -p codex-tui`
- 通过直接读取仓库中生成的 `*.snap.new` 文件来审查更改，或预览特定文件：
  - `cargo insta show -p codex-tui path/to/file.snap.new`
- 只有当你打算接受该 crate 中所有新快照时，才运行：
  - `cargo insta accept -p codex-tui`

如果没有该工具：

- `cargo install --locked cargo-insta`

### 基准测试

cargo benchmark 可以通过 `just bench` 运行，使用 divan crate 编写新的基准测试。

使用 `just bench-smoke` 对基准测试进行单次迭代干跑，确保其可用。

### 测试断言

- 测试应使用 pretty_assertions::assert_eq 以获得更清晰的 diff。如果测试模块顶部尚未导入，请导入。
- 尽可能优先进行深度相等比较。对整个对象执行 `assert_eq!()`，而不是逐字段比较。
- 避免在测试中修改进程环境；优先从上层传入环境派生的标志或依赖。

### 在测试中启动 workspace 二进制（Cargo vs Bazel）

- 测试需要启动第一方二进制时，优先使用 `codex_utils_cargo_bin::cargo_bin("...")`，而不是 `assert_cmd::Command::cargo_bin(...)` 或 `escargot`。
  - 在 Bazel 下，二进制和资源可能位于 runfiles 中；使用 `codex_utils_cargo_bin::cargo_bin` 解析在 `chdir` 后仍保持稳定的绝对路径。
- 在 Bazel 下定位 fixture 文件或测试资源时，避免使用 `env!("CARGO_MANIFEST_DIR")`。优先使用 `codex_utils_cargo_bin::find_resource!`，使路径在 Cargo 和 Bazel runfiles 下都能正确解析。

### 集成测试

#### codex_core 集成测试

- 编写端到端 Codex 测试时，优先使用 `core_test_support::responses` 中的工具。
- 默认使用 `TestCodexBuilder::build_with_auto_env()`，确保新测试适用于外部 app/exec OS。详情参见 $remote-tests。
- 所有 `mount_sse*` 辅助函数都返回 `ResponseMock`；请持有它，以便对出站 `/responses` POST 请求体进行断言。
- 当测试应只发出一次 POST 时使用 `ResponseMock::single_request()`，或使用 `ResponseMock::requests()` 检查每个捕获的 `ResponsesRequest`。
- `ResponsesRequest` 暴露辅助方法（`body_json`、`input`、`function_call_output`、`custom_tool_call_output`、`call_output`、`header`、`path`、`query_param`），使断言可以针对结构化 payload，而不是手动挖掘 JSON。
- 使用提供的 `ev_*` 构造函数和 `sse(...)` 构建 SSE payload。
- 优先使用 `wait_for_event` 而非 `wait_for_event_with_timeout`。
- 优先使用 `mount_sse_once` 而非 `mount_sse_once_match` 或 `mount_sse_sequence`。

- 典型模式：

  ```rust
  let mock = responses::mount_sse_once(&server, responses::sse(vec![
      responses::ev_response_created("resp-1"),
      responses::ev_function_call(call_id, "shell", &serde_json::to_string(&args)?),
      responses::ev_completed("resp-1"),
  ])).await;

  codex.submit(Op::UserTurn { ... }).await?;

  // Assert request body if needed.
  let request = mock.single_request();
  // assert using request.function_call_output(call_id) or request.json_body() or other helpers.
  ```

#### app-server 集成测试

- 测试应调用 app-server 的公共 JSON-RPC API。
- 使用与 core 集成测试类似的服务器 mock。
- 默认使用 `TestAppServer::builder().build()` 和 `TestAppServer::send_thread_start_request_with_auto_env()`，确保新测试适用于外部 app/exec OS。详情参见 $remote-tests。

## App-server API 开发最佳实践

以下指南适用于 `codex-rs` 中的 app-server 协议工作，尤其是：

- `app-server-protocol/src/protocol/common.rs`
- `app-server-protocol/src/protocol/v2.rs`
- `app-server/README.md`

### 核心规则

- 所有活跃的 API 开发都应发生在 app-server v2 中。不要向 v1 添加新的 API 表面。
- 一致地遵循 payload 命名：请求 payload 使用 `*Params`，响应使用 `*Response`，通知使用 `*Notification`。
- 将 RPC 方法暴露为 `<resource>/<method>`，并保持 `<resource>` 为单数（例如 `thread/read`、`app/list`）。
- 除非 tagged union 或明确的兼容性要求需要定向重命名，否则始终在 wire 上以 camelCase 暴露字段，使用 `#[serde(rename_all = "camelCase")]`。
- 除非明确的兼容性要求需要定向重命名，否则始终在 wire 上以 camelCase 暴露字符串枚举值，并配套 serde 和 TS 的 `rename_all = "camelCase"` 注解。
- 例外：config RPC payload 预期使用 snake_case，以对应 config.toml 的键（参见 `app-server-protocol/src/protocol/v2.rs` 中的 config 读写/列表 API）。
- 始终在 v2 请求/响应/通知类型上设置 `#[ts(export_to = "v2/")]`，使生成的 TypeScript 落在正确的命名空间中。
- 永远不要对 v2 API payload 字段使用 `#[serde(skip_serializing_if = "Option::is_none")]`。
  例外：有意不带 params 的 client->server 请求可以使用：
  `params: #[ts(type = "undefined")] #[serde(skip_serializing_if = "Option::is_none")] Option<()>`。
- 保持 Rust 与 TS 的 wire 重命名一致。如果字段或变体使用了 `#[serde(rename = "...")]`，请添加对应的 `#[ts(rename = "...")]`。
- 对于判别联合，在两个序列化器中都使用显式标签：
  `#[serde(tag = "type", ...)]` 和 `#[ts(tag = "type", ...)]`。
- 在 API 边界优先使用普通 `String` ID（如需要，在内部做 UUID 解析/转换）。
- 时间戳应为整数 Unix 秒（`i64`），命名使用 `*_at`（例如 `created_at`、`updated_at`、`resets_at`）。
- 对于实验性 API 表面：
  使用 `#[experimental("method/or/field")]`，当需要字段级门控时派生 `ExperimentalApi`，当只有方法的某些字段是实验性时在 `common.rs` 中使用 `inspect_params: true`。

### client->server 请求 payload（`*Params`）

- 每个可选字段都必须标注 `#[ts(optional = nullable)]`。不要在 client->server 请求 payload（`*Params`）之外使用 `#[ts(optional = nullable)]`。
- 可选集合字段（例如 `Vec`、`HashMap`）必须使用 `Option<...>` + `#[ts(optional = nullable)]`。不要使用 `#[serde(default)]` 模拟可选集合，也不要在 v2 payload 字段上使用 `skip_serializing_if`。
- 当希望省略布尔字段表示 `false` 时，使用 `#[serde(default, skip_serializing_if = "std::ops::Not::not")] pub field: bool` 而不是 `Option<bool>`。
- 对于新的列表方法，默认实现游标分页：
  请求字段 `pub cursor: Option<String>` 和 `pub limit: Option<u32>`，
  响应字段 `pub data: Vec<...>` 和 `pub next_cursor: Option<String>`。

### 开发工作流

- API 行为变化时更新 app-server 文档/示例（至少更新 `app-server/README.md`）。
- API 形状变化时重新生成 schema fixture：
  `just write-app-server-schema`
  （实验性 API fixture 受影响时，还要运行 `just write-app-server-schema --experimental`）。
- 使用 `just test -p codex-app-server-protocol` 进行验证。
- 避免仅断言 `common.rs` 中单个请求字段实验性标记的样板测试；依赖 schema 生成/测试和行为覆盖。

## Python 开发最佳实践

### 忽略 Python 2 兼容性

本项目使用 Python 3+。不应使用 `__future__` 模块。

如果需要关注不同 3.xx 点版本之间的功能兼容性，请查看最近的 `pyproject.toml` 的 `requires-python` 字段，确定支持的最低运行时版本。

## 平台支持

除非功能明确特定于某操作系统，否则测试和功能必须支持 Linux、macOS 和 Windows。

Codex 支持在不同操作系统上运行连接的 app-server 和 exec-server。关于这些配置的集成测试详情，参见 `$remote-tests` 技能。
