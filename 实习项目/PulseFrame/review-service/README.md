# PulseFrame Review Service

短视频内容审核服务，独立部署，通过接口与内容模块交互。设计见 `内容审核/流程设计.md`，实现计划见 `内容审核/实现计划.md`，测试结果见 [reports/测试报告-2026-11-21.md](reports/测试报告-2026-11-21.md)。

## 流程

```
内容模块投递任务 → 读取原文件并校验（1 路视频流、≤20 秒）→ 挑帧 → 语音转写 + 画面文字 → 文字规则
  → 小 VLM 每批一次初筛 → 有可疑线索时整片交强 VLM 复核 → 定级并查规则表 → 回调内容模块
```

- 模型只输出"看到了什么"（观察项），等级由程序算，处置查规则表；换表不重跑模型。
- 同一文件、同一识别版本只跑一次模型，结果存在文件审核记录里复用。
- 转人审期间不回调，内容模块查询得到"审核中"；人审结论提交后再回调。

## 目录

```
review-service/
├── cmd/pulseframe-review/   # 服务入口
├── cmd/judge-replay/        # 用线上判定代码离线回放样本（诊断实验用）
├── internal/
│   ├── review/              # 任务队列、流水线、回调、人审与规则表接口、插帧规则
│   ├── judge/               # 两阶段 VLM 客户端、证据及回答校验
│   ├── grading/             # 观察项 → 低俗等级
│   ├── policy/              # 平时表、严查表
│   ├── platform/            # 配置、日志、数据库、HTTP 响应
│   ├── migration/           # 建表
│   └── model.go             # task、file_record、limit_seq
├── worker/                  # Python worker（HTTP，只监听 127.0.0.1）
│   ├── server.py
│   ├── framesel/            # 挑帧、M3 合成与实验
│   └── media_text/          # ASR（FunASR）、OCR（RapidOCR）、文字规则
├── policies/
│   ├── observation/         # 线上观察项提示词
│   └── kuaimod_paper/       # KuaiMod 论文提示词（评测用）
├── scripts/                 # 服务器上的启动脚本：VLM、worker、审核服务
├── test/                    # 内容模块桩、契约测试、端到端测试与统计、插帧诊断、人审、演示
├── tools/                   # 数据准备与 KuaiMod 评测
└── reports/                 # 实验与测试报告（archive/ 为作废的旧报告）
```

## 接口

所有接口都校验 `X-Service-Key`（两个方向各用各的密钥）。

内容模块 → 审核服务：

| 接口 | 说明 |
|---|---|
| `POST /internal/review/tasks` | 投递任务 `{task_id, video_id, kind: publish\|rereview, media_key, title, description}`，task_id 为整数；重复 task_id 返回 200，不新建任务 |
| `GET /internal/review/tasks/{task_id}` | 有结论返回 `{status: pass\|limit\|reject, result, reason}`，机审中、人审中都返回 `status: "审核中"` |
| `GET /internal/review/tasks/{task_id}/detail` | 人审页面用：内部状态、转人审原因、完整机审记录（选中的帧、转写、画面文字、观察项、等级、规则表版本） |
| `POST /internal/review/tasks/{task_id}/decision` | 人审结论 `{result, reason, reviewer, observation?}`，只接受等人审的任务，否则 409 |
| `POST /internal/review/policy` | 按配置中的表名切换；临时表可在配置或请求的 `expire_at` 中指定到期时间 |

发布后由内容模块向 `POST /internal/review/published-candidates` 投递已发布视频，提供 `video_id`、`media_key`、标题和独立的复审 `task_id`。同一次抽查机会重投使用相同编号。响应含 `selected`，入选时返回任务并沿用审核回调。随机模式对全部候选按比例抽取。曝光模式额外提交 `window_seconds`、`previous_exposures`、`current_exposures`，计数来自连续两个等长窗口。抽中后绕过初筛与机审缓存。

审核服务通过 `POST /internal/review/callback` 返回处置，失败时从 1 秒起退避重试，最长间隔 5 分钟。

## 配置（环境变量）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `HTTP_PORT` | 8081 | |
| `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` | 127.0.0.1 / 3306 / 必填 / 必填 / 必填 | 启动时自动建表 |
| `SERVICE_KEY` | 必填 | 内容模块调用审核服务的密钥 |
| `CONTENT_MODULE_BASE_URL` / `CONTENT_MODULE_SERVICE_KEY` | http://127.0.0.1:8080 / 必填 | 回调内容模块 |
| `VLM_BASE_URL` / `VLM_MODEL` | 必填 | 小 VLM 初筛服务地址与模型名称 |
| `STRONG_VLM_BASE_URL` / `STRONG_VLM_MODEL` | 必填 | 整片复核服务，须支持提交的全部复核图片（含局部放大图）。可用同前缀设置 API_KEY、TIMEOUT、MAX_RETRIES、MAX_TOKENS、DISABLE_THINKING |
| `SAMPLING_FILE` | `policies/published_sampling.json` | 发布后抽查模式、抽取百分比、曝光窗口及两个增长门槛 |
| `VLM_TIMEOUT` / `VLM_MAX_RETRIES` / `VLM_MAX_TOKENS` / `VLM_DISABLE_THINKING` | 60（秒）/ 3 / 1024 / true | |
| `IDENT_VERSION` | 必填 | 识别版本（模型 + 提示词 + 观察项定义），文件审核记录按它复用，改了任何一项都要换 |
| `MEDIA_ROOT` | /data/media | 原文件根目录（对象存储挂载点），media_key 相对于它解析，不能越界 |
| `WORK_DIR` | /tmp/pulseframe-review | 帧图片和音频的工作目录，人审时回看 |
| `WORKER_URL` | http://127.0.0.1:8090 | Python worker |
| `PROMPT_PATH` | policies/observation/system_prompt.txt | |
| `REVIEW_WORKERS` / `REVIEW_RETRY_BASE` / `REVIEW_RETRY_MAX` / `REVIEW_LEASE` | 2 / 1m / 4m / 10m | 故障任务留在机审队列。重试间隔翻倍至上限；领取进程中断后，租约到期即可接手 |
| `CALLBACK_RETRY_BASE` / `CALLBACK_RETRY_MAX` | 1s / 5m | |
| `POLICY_FILE` | `policies/moderation.json` | 规则文件。编辑标签处置或增加策略后，重启服务加载 |
| `POLICY_TABLE` / `POLICY_EXPIRE_AT` | 空 / 空 | 启动表名和到期时间；空表名使用文件的 `default_table`。兼容 `POLICY_STRICT_EXPIRE_AT` |

## 运行

显存服务器上（脚本里的路径按服务器布局写，环境变量文件只放在服务器上，不进仓库）：

```bash
scripts/serve_vlm.sh                       # vLLM + Qwen3.5-9B，端口 8000
scripts/serve_worker.sh                    # worker，端口 8090（首次启动会下载 ASR/OCR 模型）
scripts/serve_review.sh /path/to/review.env  # 审核服务
```

## 测试

```bash
go test ./internal/...                                     # Go 单元测试
python -m pytest worker/framesel worker/media_text         # 挑帧、文字规则（需要 numpy、opencv、ffmpeg）
```

端到端、契约、插帧诊断、人审和演示脚本的用法见 [reports/end-to-end.md](reports/end-to-end.md) 的"复现"一节。

## 已知限制

- backend 内容模块只有表结构，投递 worker、回调接口还没写；目前用 `test/fake_content_module.py` 按契约模拟。
- 原文件从 `MEDIA_ROOT` 读取，没有直接接对象存储 SDK。
- 转人审率偏高（第一轮 52%），主要来自格式异常和 2–3 票的项；渐变叠加的插帧约一半拦不住。详见测试报告的"问题与建议"。
- 按实现计划只做设计、不写代码：用户举报、降级、监控、违规库检索、音频分类、二维码。
