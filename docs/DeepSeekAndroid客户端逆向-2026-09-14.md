# DeepSeek Android 2.5.1 客户端逆向（2026-09-14）

> 第三方观察笔记，不是官方协议。对象是 `tmp/deepseek.apk`，包名 `com.deepseek.chat` **2.5.1**（`versionCode` 271）。对照上一份 [2.4.5 笔记](./DeepSeekAndroid客户端逆向-2026-09-10.md) 和仓库里 ds2api 的上游模拟。
> 文档导航：[文档总索引](./README.md) / [prompt 兼容主链路](./prompt-compatibility.md) / [SSE 行为结构](./DeepSeekSSE行为结构说明-2026-04-05.md)

SMID 规则、`model_type` 绑定、空 POST `{}` 这些 2.4.5 已经核实过的行为，这次 DEX 里没有改。本文只记相对 2.4.5 的差。

## 1. 先看结论

1. 编译期身份变了：`x-client-version` / UA 从 `2.4.5` 升到 `2.5.1`。`versionCode` 265 → 271。`compileSdk` / `targetSdk` 35 → 36（Android 16）。`minSdk` 仍是 23。
2. 文字 completion 线协议没改。`ChatFullCompletionRequest` 仍是那 10 个字段，`model_type` 仍是 `default` / `expert` / `vision`。
3. 登录 body 仍是 `email` + `password` + `device_id` + `os=android`。数美组织串仍是 `P9usCUBauxft8eAmUXaZ`，`should_use_sm_device_id`、`libsmsdk.so` 都在。PoW 仍是 `DeepSeekHashV1`。
4. Ktor 拦截器在原有 `x-client-*` / `x-rangers-id` 后面，多写了 `x-device-model`、`x-device-id`、`x-hif-dliq`、`x-hif-leim`。取值这次没抓包，ds2api 先不发这四个头。
5. `ModelConfig` 在 `search_feature` 和 `file_feature` 之间插入了 `tts_feature`。TTS / ASR / `check_device` 是新接口，文字 Chat 主链路用不到。
6. 首页文案仍是「深度思考」「智能搜索」，另有「快速模式」「专家模式」「识图模式」「自动朗读」。公开 `/v1/models` 不用跟着砍。

## 2. APK

| 项 | 2.4.5 | 2.5.1 |
| --- | --- | --- |
| `versionName` | 2.4.5 | 2.5.1 |
| `versionCode` | 265 | 271 |
| `compileSdk` / `targetSdk` | 35 | 36 |
| DEX | `classes.dex` + `classes2.dex` | 多了 `classes3.dex`（slf4j / 日志，无业务协议） |
| 网络栈 | Ktor + okhttp `4.12.0` | 同 |
| 数美 | `lib/arm64-v8a/libsmsdk.so` | 同 |

本地 APK 仍放 `tmp/`（gitignore），不要提交。

## 3. 客户端身份

拦截器 const-string 顺序（`chat.deepseek.com` 之后）：

```text
x-client-platform = android
x-client-version = 2.5.1
x-client-locale
x-client-bundle-id = com.deepseek.chat
x-rangers-id
x-client-timezone-offset
x-device-model
x-device-id
x-hif-dliq
x-hif-leim
```

UA 拼接仍是 `DeepSeek/2.5.1 Android/` + `Build.VERSION.SDK_INT`。仓库编译期默认改成 `Android/36`，跟这个包的 target 对齐。真机在 Android 15 上会发 `Android/35`，两种都合法。

SOCKS `127.0.0.1:10808` 实号（出口 `129.159.254.122`，本机直连不是这个 IP）打过一次：

| 操作 | 结果 |
| --- | --- |
| UA `DeepSeek/2.5.1 Android/36`，不带 `x-device-*` / HIF | 登录 token 64 字节 |
| 账号原 `device_id` 是 UUID | 登录前换成数美 `B...`，长度 89 |
| `GET /api/v0/check_client_update` | HTTP 200，`biz_data=null` |
| `POST /api/v0/users/auth_token/check_device` | HTTP 200，`rotate=null` |
| 新会话第一条 `model_type=default` | ready `default` |
| **同一会话**下一条改 `expert` | HTTP 200，ready 仍是 `default` |
| 再新建会话，第一条就 `expert` | ready `expert` |

当前这条文字 Chat 主链路不依赖新的四个头。`check_device` 也不会把 token 转掉。

没有 `accept-charset`。空 POST 仍应发 `{}`。

`x-device-id` / `x-device-model` / HIF 的值没有从 DEX 直接读出来。HIF 域名还是：

- `https://hif-dliq.deepseek.com/query`
- `https://hif-leim.deepseek.com/query`

另外还有字符串 `x-hif-ttl`、`x-model-type`，不在这段拦截器序列里，可能是别的路径或响应头。

`/api/v0/check_client_update` 对 2.5.1 返回空 `biz_data`，服务端把这个版本当成当前版。

## 4. 登录

`UsersLoginWithEmailAndPwdRequest` 序列化字段：

```text
email
password
device_id
os          // android
```

和 2.4.5 一样。Google / 手机号登录额外带 `shumei_verification`、`hcaptcha_token`。远程 KV 有 `hcaptcha_enabled`、`enable_google_sign_in_captcha`。邮箱密码登录 DEX 里仍然不强制 hCaptcha。

新接口：

```text
POST /api/v0/users/auth_token/check_device
```

`CheckDeviceRequest` 字段是 `device_model`、`device_id`。配套类有 `CheckDeviceRotateData(token=)`，像是 token 轮换，不是登录替代。ds2api 登录成功后暂不调用。

## 5. completion 与档位

`ChatFullCompletionRequest` 字段顺序未变：

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

`const-string` 仍能看到 `default` / `expert` / `vision` 三连。`model_merge_*` 埋点还在。

旁边有一个更瘦的 `ChatCompletionRequest`：

```text
chat_session_id
prompt
ref_file_ids
thinking_enabled
search_enabled
action
```

没有 `parent_message_id` / `model_type` / `preempt` / `audio_id`。主发送路径仍是 Full 那个。

`preempt` 的中文注释还是「随停随发」。远程 KV 新增 `interrupt_and_send_enabled`、`allow_parallel_streams`。ds2api 继续不发 preempt。

`model_type` 绑定 2.5.1 实号重测过：同一会话先 `default` 再改 `expert`，ready 仍是 `default`；新会话第一条 `expert` 才变成 `expert`。

## 6. ModelConfig

kotlinx 字段顺序：

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
tts_feature          // 2.5.1 新增
file_feature
prompt_feature
regenerate_options
tips
edit_quota
regenerate_quota
```

`FileFeature` 未变：`token_limit`、`token_limit_with_thinking`、`max_input_file_count`、`max_upload_file_size`、`support_file_exts`、`conflict_with_search`、`vision`、`enable_thumbnail`。

`TtsFeature` 目前能看到的序列化名是 `auto_tts_enable_by_default`。

远程 KV 仍有 `search_state_on_manually_created_chat` 等。新增值得记的：

| key | 用途（从名字判断） |
| --- | --- |
| `conversation_search_enabled` | 会话级搜索总开关 |
| `allow_file_with_search` | 可能放松「搜索不能带文件」；文案仍写冲突 |
| `interrupt_and_send_enabled` | 随停随发 |
| `thinking_auto_fold_enabled` | 思考结束自动折叠 |
| `pow_header_paths` / `pow_prefetch*` | PoW 覆盖路径和预取 |
| `voice_input_enabled` / `input_default_voice` / `tts_*` | 语音输入和朗读 |
| `hcaptcha_enabled` | 验证码 |

搜索默认状态那一组 key 还在，没有被 `conversation_search_enabled` 换掉。

## 7. 新接口（文字 Chat 不必接）

```text
/api/v0/asr/ws
/api/v0/chat/tts/
/api/v0/chat/tts/voice
/api/v0/chat/tts/voices
/api/v0/index/prepare
/api/v0/index/query
/api/v0/users/auth_token/check_device
```

会话 / PoW / completion 路径没换：

```text
POST /api/v0/chat_session/create
POST /api/v0/chat/create_pow_challenge
POST /api/v0/chat/completion
```

## 8. UI 文案

`resources.arsc` 仍有：

- 深度思考 / 智能搜索
- 专家模式 / 识图模式
- 如需切换模式，请开启新对话
- 搜索开启时无法发送文件，请关闭后重试

2.5.1 多出来的：

- 快速模式 / 快速模式下（和「专家模式下」并列，像是 default 的产品名，不是新的 `model_type`）
- 自动朗读 / 朗读音色 / 切换至语音 / 语音输入
- 思考内容自动折叠

深链 `dsaction://send_to_new_session?model=` 还在。

## 9. ds2api 落地

| 主题 | 做法 |
| --- | --- |
| 默认头 / 版本 | `internal/deepseek/protocol/constants_shared.json` 改为 `2.5.1` + `android_api_level=36` |
| 登录 SMID | 不改。非法 `device_id` 仍走 `internal/deepseek/smid` |
| completion 字段 / 公开模型表 | 不改 |
| `x-device-*` / HIF | 先不发。没有稳定取值之前乱填比缺头更危险 |

留空的 `config.json` `client.*` 会跟新的编译期默认走。如果有人把 `version` 写死成 `2.4.5`，不会自动升。

## 10. 还没做、下次优先看

1. 拦截器里 `x-device-id` / `x-device-model` 的实际值（是不是 SMID / `Build.MODEL`）。
2. HIF 两个头何时带、payload 是什么。登录和文字 completion 现在不依赖它们，只是写进了同一段拦截器。
3. App 登录后会不会自己打 `check_device`。本次主动打过，`rotate=null`。
4. `allow_file_with_search` 线上是不是真允许搜索+文件。
5. native `libsmsdk.so`，以及 TLS 是否要从 `HelloAndroid_11_OkHttp` 再往上提。
