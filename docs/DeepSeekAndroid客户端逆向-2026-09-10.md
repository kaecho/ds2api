# DeepSeek Android 2.4.5 客户端逆向（2026-09-10）

> 第三方观察笔记，不是官方协议。对象是 Google Play 包 `com.deepseek.chat` **2.4.5**（`versionCode` 265），对照仓库里 ds2api 的上游模拟。
> 文档导航：[文档总索引](./README.md) / [prompt 兼容主链路](./prompt-compatibility.md) / [SSE 行为结构](./DeepSeekSSE行为结构说明-2026-04-05.md)

后续若官方再发版，优先核对三件事：登录 `device_id`、请求头身份、completion 的 `model_type` 是否还是 `default` / `expert` / `vision`。

## 1. 先看结论

1. 风控 ban 账号的主因不是模型名改了，而是 **chat 登录开始强制要数美 B 前缀 SMID**。空 `device_id` 或 UUID 会拿到 `biz_code=11 RISK_DEVICE_DETECTED`。
2. App 首页已经没有 Flash / Pro / Vision 选择器，只剩「深度思考」「智能搜索」。这是 UI 融合（DEX 里叫 `model_merge_*`），**线协议没有合成一个模型**。
3. 发给 `/api/v0/chat/completion` 的仍是 `ChatFullCompletionRequest`：`model_type` + `thinking_enabled` + `search_enabled` 分开写。
4. `model_type` 以**该会话第一条 completion** 为准。建会话 `POST {}` 只换 `chat_session_id`。之后在同一会话里改档，服务器会忽略，ready 事件仍回已经绑定的值。
5. ds2api 每次请求新建远端会话，所以公开名 `deepseek-v4-pro` 映射成第一条 `expert` 仍然有效，不必等远程配置把 expert 设成默认。

## 2. 证据来源

| 来源 | 用处 |
| --- | --- |
| APK `classes.dex` / `classes2.dex` 字符串与 `const-string` | 请求体字段、枚举、埋点、远程配置 key |
| `resources.arsc` 中文文案 | 确认首页芯片和「切换模式请开新对话」 |
| Ktor 拦截器（DEX 里 `bx` case 25 / `bo2.<clinit>`） | 实际发出的 HTTP 头 |
| SOCKS5 实账号登录 + completion | 验证 SMID、空 body、`model_type` 绑定 |
| ds2api 实现 | `internal/deepseek/protocol/constants_shared.json`、`internal/deepseek/smid`、`internal/promptcompat/standard_request.go` |

APK 本地放在 `tmp/`（已 gitignore），不要提交。

## 3. 客户端身份

官方 Android 现在固定类似下面这组头。登录也走同一套 identity，**登录请求带 `x-rangers-id`**。没有 `accept-charset`。空 POST 要发 `{}`，不要 `null`。

| 头 | 2.4.5 观察到的值 |
| --- | --- |
| `User-Agent` | `DeepSeek/2.4.5 Android/35` |
| `x-client-platform` | `android` |
| `x-client-version` | `2.4.5` |
| `x-client-locale` | `zh_CN` |
| `x-client-bundle-id` | `com.deepseek.chat` |
| `x-client-timezone-offset` | 秒。中国时区 `28800` |
| `x-rangers-id` | 全请求，含 login |

仓库编译期默认在 `internal/deepseek/protocol/constants_shared.json`。TLS 指纹仍用 `utls.HelloAndroid_11_OkHttp`，UA 已经是 Android 35，两边不完全一致。没有已知的 Android 15 指纹可换时先不动。

`/api/v0/check_client_update` 对 2.4.5 返回空 `biz_data`，服务端把这个版本当成当前版。

DEX 里还有 `x-hif-dliq` / `x-hif-leim` 和 `https://hif-dliq.deepseek.com/query`。本次未接，登录和文字 completion 不依赖它们。

## 4. 登录与数美 SMID

chat 登录：`POST https://chat.deepseek.com/api/v0/users/login`

```json
{
  "email": "...",
  "password": "...",
  "device_id": "B...",
  "os": "android"
}
```

`device_id` 必须是数美协议 SMID：`B` 开头，长度 ≥ 40。来自数美 `deviceprofile/v4`，组织串 `P9usCUBauxft8eAmUXaZ`（APK 和 web 脚本同一份）。

对照：

| 做法 | chat 登录结果 |
| --- | --- |
| 空 / UUID | `RISK_DEVICE_DETECTED` |
| 只升 UA 到 2.4.5，device_id 仍是 UUID | 一样被拒 |
| web 协议 SMID（`B...`，node 跑 `sm_device_id.js`） | 登录成功，随后 session + PoW + completion 200 |

注意：

- 旧 ds2api 从来没有走数美，登录一直填 UUID。2.4.5 包里早就有 `libsmsdk.so`、`should_use_sm_device_id`、`shumei_verification`，是服务端现在开始卡 chat 登录。
- dsreg 可以不带 SMID 注册，那是 `platform.deepseek.com/auth-api`，不是 `chat.deepseek.com` 登录。
- 当前用 web 协议 `B...` 能过 Android chat 登录。没有去逆 `libsmsdk.so` 的 native SMID。服务端如果以后只认 native 值，再补。
- 本机要有 `node`。实现：`internal/deepseek/smid`。`Login` 在账号 `device_id` 非法时拉取并写回账号配置。

远程配置里仍有 `should_use_sm_device_id`、`sm_sdk_host`、`sm_pass_code_type`。

## 5. 一次对话实际打哪些接口

官方客户端常见顺序：

1. `GET /api/v0/client/settings`（含 `model_configs_v1` 等远程 KV）
2. `POST /api/v0/chat_session/create`，body `{}`
3. `POST /api/v0/chat/create_pow_challenge`
4. `POST /api/v0/chat/completion`，带 `x-ds-pow-response`

ds2api 与 App 一样需要 session id 和 PoW，所以一次对外 Chat 请求在上游也会走 2～4。第一步 **不是**「向服务器打听本回合该用哪个档」。

`ChatSessionCreateBizData` 只有 `chat_session` 和 `ttl_seconds`。会话对象 `ServerChatSession` 带 `id`、`model_type` 等字段。

## 6. 远程模型配置

`GET /api/v0/client/settings` 下发 `kv_remote_settings_model_configs_v1`。DEX 里 `ModelConfig` 字段：

```text
model_type
name
description
welcome_msg
is_default
enabled
switchable
show_model_name_in_session
input_character_limit
think_feature
search_feature
file_feature
prompt_feature
regenerate_options
tips
edit_quota
regenerate_quota
```

`FileFeature`：

```text
token_limit
token_limit_with_thinking
max_input_file_count
max_upload_file_size
support_file_exts
conflict_with_search
vision
enable_thumbnail
```

搜索还有远程开关：`search_state_on_manually_created_chat` / `on_launch` / `on_login` / `on_automatically_created_chat`，以及 `search_force_on` / `search_force_off`。

这套配置决定 **官方 App 展示什么、新对话搜索默认开不开、某档能不能切**。它不是 completion 的白名单拦截。客户端只要在新会话第一条 completion 里写 `expert`，服务端会按 `expert` 回 ready。

## 7. UI 融合之后，档位怎么发给后端

首页 composer：一个输入框 +「深度思考」+「智能搜索」。文案里还能看到「专家模式」「识图模式」「如需切换模式，请开启新对话」「Vision」。

DEX 里 kotlinx 仍把 `model_type` 序列化成 `default` / `expert` / `vision`（多处 `const-string` 三连）。`ChatFullCompletionRequest` 字段顺序：

```text
chat_session_id
parent_message_id
prompt
ref_file_ids
thinking_enabled
search_enabled
audio_id
preempt
model_type
action
```

对应关系：

| 用户动作 | 真正发出去的字段 |
| --- | --- |
| 新开聊天（无选择器） | 第一条 completion 的 `model_type` 用远程 `is_default`，融合后实际是 `default` |
| 深度思考 | `thinking_enabled` |
| 智能搜索 | `search_enabled`（新会话默认看 `search_state_on_manually_created_chat`） |
| 发图 | 埋点 `switch_to_vision`。当前档 `file_feature.vision` 为假时换新会话，`model_type=vision` |
| 搜索 + 文件 | `conflict_with_search` / 「搜索开启时无法发送文件」 |

发送埋点同时带 `is_think_enable`、`is_search_enable`、`model_type`，说明开关和档位一直是两套东西。

`conversation_mode` 出现在 SSE 初始化 envelope 里。实测 `model_type=expert` 时 `conversation_mode` 仍可以是 `DEFAULT`。不要用它判断档位，看 ready 事件里的 `model_type`。

深链还在：`dsaction://send_to_new_session?model=`。分析事件里仍有 `selected_model_switch`、`previous_model_type`、`model_selector_show`，首页只是不再画选择器。

## 8. `model_type` 绑定规则（实测）

SOCKS 出口实号，2.4.5 头 + 数美 SMID：

| 操作 | ready 里的 `model_type` |
| --- | --- |
| 新建会话，第一条 completion 发 `default` | `default` |
| **同一会话**下一条改发 `expert` | 仍是 `default`（HTTP 200，字段被忽略） |
| 再新建会话，第一条就发 `expert` | `expert` |

因此：

- 用 expert：新 `chat_session_id`，第一条 completion 写 `model_type=expert`。不必等远程把 expert 设成 `is_default`。
- 已经 default 聊过的会话，把后续请求改成 expert 没用。
- 官方 App 看起来像「服务器替我选档」，是因为它自己第一条永远填 `default`。
- ds2api 对外每次新建远端会话（见 prompt 兼容文档），`GetModelType` 把 `deepseek-v4-flash` → `default`、`deepseek-v4-pro` → `expert`、`deepseek-v4-vision` → `vision`。映射仍然对应线协议。

公开 `/v1/models` 的 `deepseek-v4-*` 是 ds2api 自己的名字，App 不会发这些字符串。思考 / 搜索继续用 `-nothinking` / `-search` 后缀，对应 payload 里的两个布尔值，不是第四个 `model_type`。vision + search 目前 ds2api 仍标成不支持，和 App「搜索与文件冲突」一致。

## 9. ds2api 落地位置

| 主题 | 代码 |
| --- | --- |
| 默认头 / 版本 | `internal/deepseek/protocol/constants_shared.json`，Go/JS header builder |
| 登录 SMID | `internal/deepseek/client/client_auth.go` `Login`，`internal/deepseek/smid` |
| 建会话空 body | `Client.CreateSession` → `map[string]any{}` |
| completion `model_type` / 开关 | `internal/config/models.go` `GetModelType` / `GetModelConfig`，`internal/promptcompat/standard_request.go` |
| 公开模型列表和别名 | `internal/config/models.go` `DeepSeekModels`、`DefaultModelAliases` |

登录不再提供「跳过数美」的配置项。非法 `device_id` 一律现拉 SMID。

## 10. 还没做、下次优先看

- native `libsmsdk.so` 的 Android SMID。现在 web `B...` 仍被 chat 登录接受。
- TLS 提到 Android 15 / OkHttp 新指纹。
- `x-hif-dliq` / `x-hif-leim`。
- 远程 `model_configs_v1` 的线上完整 JSON（是否还下发 `enabled: true` 的 expert 条目）。
- 发图时 session 是新建还是原地改 `model_type`（文案要求开新对话，未抓完整上传链）。
- `check_client_update` 以后若开始强制升级，身份常量要跟着 APK 再对一次。

## 11. 复查清单

下次拆包或抓包，按这个顺序最快：

1. `User-Agent` / `x-client-version` / `x-client-bundle-id` 有没有变。
2. 登录 body 的 `device_id` 还是不是 `B...`，失败码还是不是 `RISK_DEVICE_DETECTED`。
3. `ChatFullCompletionRequest` 字段名有没有增减。
4. `const-string` 三连 `default` / `expert` / `vision` 还在不在。
5. 新会话第一条 completion 改写 `model_type`，看 ready 是否跟着变；同一会话第二条改档是否仍被忽略。
