# 岩芯编录复核台

岩芯编录复核台面向钻探现场编录员、地质项目负责人、实验室接样员和质量复核员，提供受控的岩芯孔段编录与样品交接流程。系统以单进程 JSON HTTP API 运行，业务数据保存为带版本、序号和 SHA-256 校验和的本地 JSON 账本。

系统支持建立钻探任务、登记连续孔段、记录和处置现场异常、冻结孔段发起取样、批准或退回取样申请、登记实验结果、提交质量复核结论，以及签发可独立验证且无修改入口的样品交接凭据。每次状态变化都要求 `Idempotency-Key`，聚合修改还要求请求体携带 `expectedVersion`。

## 环境要求

- Go 1.23 或更高版本。
- 无外部数据库或网络服务依赖。
- 默认账本路径为 `data/corelog-ledger.json`。

## 构建、运行和测试

标准构建命令：

```json
{"argv":["go","build","./cmd/corelog-server"],"working_directory":"."}
```

标准运行命令：

```json
{"argv":["go","run","./cmd/corelog-server","-addr=127.0.0.1:19081"],"working_directory":"."}
```

也可设置 `PORT` 为端口号，服务会绑定到 `127.0.0.1:<PORT>`。显式 `-addr` 优先于 `PORT`。服务拒绝非回环监听地址，默认地址固定为 `127.0.0.1:19081`。

标准测试命令：

```json
{"argv":["go","test","./..."],"working_directory":"."}
```

有界启动自检命令会读取并校验账本、执行领域不变量检查、输出 JSON 结果后自行退出：

```json
{"argv":["go","run","./cmd/corelog-server","-selfcheck","-addr=127.0.0.1:19081"],"working_directory":"."}
```

可用 `-data` 指定其他账本文件，例如在隔离环境中使用临时路径。首次写入时，服务会创建父目录；提交采用同目录临时文件、文件 `Sync`、原子 `Rename` 和目录 `Sync`。

## API 约定

请求和响应使用 `application/json`。成功响应统一为 `{"data":...}`，错误响应统一为 `{"error":{"code":"...","message":"...","field":"..."}}`。所有 `POST` 请求必须携带非空 `Idempotency-Key` 请求头，相同键和相同请求会返回原资源；相同键用于不同请求会返回 `409 Conflict`。

主要路由如下：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/v1/health` | 进程存活检查 |
| `GET` | `/v1/selfcheck` | 当前账本业务不变量检查 |
| `POST` | `/v1/campaigns` | 创建钻探任务 |
| `GET` | `/v1/campaigns` | 查询任务列表 |
| `GET` | `/v1/campaigns/{campaignID}` | 查询任务详情 |
| `POST` | `/v1/campaigns/{campaignID}/intervals` | 登记孔段 |
| `POST` | `/v1/campaigns/{campaignID}/intervals/batch` | 批量登记连续孔段并返回进度 |
| `GET` | `/v1/campaigns/{campaignID}/intervals` | 查询任务孔段及各钻孔编录进度 |
| `GET` | `/v1/intervals/{intervalID}` | 查询孔段详情与异常 |
| `POST` | `/v1/intervals/{intervalID}/anomalies` | 登记现场异常 |
| `POST` | `/v1/intervals/{intervalID}/anomalies/{anomalyID}/resolve` | 处置异常 |
| `GET` | `/v1/campaigns/{campaignID}/anomalies` | 查询异常证据清单与处置统计 |
| `POST` | `/v1/campaigns/{campaignID}/sampling-requests` | 发起取样申请并冻结孔段 |
| `GET` | `/v1/campaigns/{campaignID}/sampling-requests` | 查询任务取样申请 |
| `GET` | `/v1/sampling-requests/{requestID}` | 查询取样申请详情 |
| `POST` | `/v1/sampling-requests/{requestID}/review` | 批准或退回取样申请 |
| `POST` | `/v1/sampling-requests/{requestID}/resubmit` | 补正重提取样申请 |
| `POST` | `/v1/sampling-requests/{requestID}/test-results` | 登记检测结果 |
| `GET` | `/v1/sampling-requests/{requestID}/test-results` | 查询申请的检测结果 |
| `GET` | `/v1/test-results/{resultID}` | 查询检测结果详情 |
| `POST` | `/v1/test-results/{resultID}/review` | 提交检测质量复核结论 |
| `POST` | `/v1/sampling-requests/{requestID}/test-results/review-batch` | 批量提交检测质量复核结论 |
| `GET` | `/v1/sampling-requests/{requestID}/handoff-readiness` | 查询交接准备状态 |
| `POST` | `/v1/sampling-requests/{requestID}/certificates` | 签发交接凭据 |
| `GET` | `/v1/certificates/{certificateID}` | 查询不可变交接凭据 |
| `GET` | `/v1/certificates/{certificateID}/verify` | 重新计算并验证凭据哈希 |

取样复核和检测复核请求中的 `decision` 使用 `approve` 或 `return`。孔段一旦被待复核或已批准的取样申请冻结，就不能再登记或处置异常；取样申请退回后孔段自动解冻，补正重提时必须提供孔段最新版本并完成全部异常处置。

## 数据完整性

账本顶层记录 `schemaVersion`、单调递增的 `sequence`、保存时间和针对完整业务状态计算的 `checksum`。启动恢复会校验所有实体索引、跨实体引用、凭据哈希和账本校验和。交接前 selfcheck 还会检查孔段连续性、冻结版本一致性、检测质量复核结论和凭据完整性。交接凭据没有更新或删除路由，任何离线修改都会在恢复校验或凭据验证时被发现。
