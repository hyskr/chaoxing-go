# chaoxing-go

面向终端和 AI Agent 的超星学习通命令行工具，使用 Go 实现。提供账号密码登录、完整任务卡扫描、视频与文档学习、章节／随堂测验读取与指定答案提交，以及考试 JSON 读取、整理和答案填充。

平台接口参考 [Yu-dongxing/CxStudy](https://github.com/Yu-dongxing/CxStudy) `7ad1f13`：账号登录、课程／章节、全部卡片、移动端章节测验、学习进度上报。命令组织、结构化响应和进度事件参考 [WkHelper](https://github.com/L-1124/WkHelper)。

## 构建

需要 Go 1.26 或更新版本。

```sh
go build -o chaoxing .
go test ./...
./chaoxing schema
```

## 登录

```sh
./chaoxing auth login --phone 你的手机号
```

在终端输入密码后回车，密码不回显。非交互调用可从标准输入传入密码。密码用于登录请求，不写入会话文件，也不出现在响应中。登录会话保存在当前目录的 `.session/session.txt`；在其他目录执行时使用相同的 `--data-dir`。

```sh
./chaoxing auth status
./chaoxing course list
```

登录方式仅包含账号密码。触发平台验证码、二次验证或人脸验证时，返回认证错误，由操作者在学习通处理后重新登录。

## 课程与完整流程

课程引用使用 `课程ID:班级ID`。只有一个班级时，可单独使用课程 ID。同一课程存在多个班级时，命令会要求提供班级 ID。

```sh
./chaoxing course inspect --course '课程ID:班级ID'
```

返回全部章节、每节任务点完成数和全部任务卡。视频与测验位于不同卡片页时，也会同时被发现。

推荐流程：登录 → 查询课程与进度 → 查看学习计划 → 执行学习任务 → 导出测验 → Agent 解题 → 提交指定答案 → 读取批阅结果 → 整理考试 JSON。

## 学习任务

```sh
./chaoxing study run --course '课程ID:班级ID' --dry-run
./chaoxing study run --course '课程ID:班级ID' --speed 1 --confirm
./chaoxing study run --course '课程ID:班级ID' --job '任务jobid' --confirm
```

依次执行视频、音频与文档任务，已完成的任务会跳过。视频从服务器提供的播放进度继续，速度范围为 1～2；进度按时间推进并向平台上报。完成以平台 `isPassed` 为准，达到时长却未确认完成时返回错误。使用 Ctrl+C 停止，之后重新运行会读取服务器上的最新进度。

执行过程中，stderr 输出单行 JSON 事件：

```json
{"event":"video.progress","job_id":"video-job","seconds":58,"duration":300,"passed":false}
```

## 章节／随堂测验

```sh
./chaoxing quiz list --course '课程ID:班级ID'
./chaoxing quiz export --course '课程ID:班级ID' --job '测验jobid' --out quiz.json
```

`quiz export` 保留完整题干、题干 HTML、题目 ID、题型和选项。`quiz` 对应挂在章节卡片中的 `work` 测验。支持单选、多选、判断、填空题；已批阅测验返回完成状态和可读取的分数。

选项由三个字段表示：

```json
{"key":"C","label":"A","text":"选项正文"}
```

`label` 是页面显示字母，`key` 是提交用内部键。平台开启选项乱序时两者可能不同。答案必须按题目 ID 和内部键提供。

Agent 可以读取 `quiz.json`，把每题的 `answer` 填好；也可生成独立答案文件：

```json
{
  "101": "C",
  "102": ["B", "D"],
  "103": true,
  "104": ["第一空内容", "第二空内容"]
}
```

```sh
./chaoxing quiz submit --course '课程ID:班级ID' --job '测验jobid' --answers answers.json --confirm
# 也接受填好 answer 的测验文件
./chaoxing quiz submit --course '课程ID:班级ID' --job '测验jobid' --answers quiz.json --confirm
```

提交前重新获取当前表单和令牌，所有题目都提供有效答案后才发送提交请求。平台确认提交成功后读取批阅结果。提交成功但批阅结果暂不可读时，会保留 `submitted: true` 并以退出码 7 返回。

题目由当前 AI Agent 分析作答；本工具接收确定的答案文件，不内置模型服务或题库凭据。

## 考试 JSON

可读取已进入考场的整卷题目并导出 JSON，也可读取 CxStudy 导出的 `export/exam_*.json` 和本工具的标准试卷 JSON。

在学习通完成考试入场验证并进入答题页后，复制当前答题页或整卷预览 URL：

```sh
./chaoxing exam export --url '当前试卷完整URL' --out exam.json
```

该命令沿用已启动的答题会话读取当前参数，再获取移动端整卷页面。考试计时继续以平台为准；会话已提交、失效或需要验证时返回相应错误。

本地文件读取与答案填充不需要登录：

```sh
./chaoxing exam read --file exam.json
./chaoxing exam read --file exam.json --out paper.md --format md
./chaoxing exam fill --file exam.json --answers answers.json --out answered.json
```

转换时保留 CxStudy 选项对象中的原始顺序，显式区分显示字母与内部键；输出 JSON 用选项数组表达顺序。填充答案会检查题型、选项键和填空数量。`exam fill` 写入新的已作答试卷文件，返回 `submitted: false`。

## Agent 契约

stdout 每次只输出一个 JSON 响应；stderr 用于 NDJSON 进度事件及终端密码输入提示事件。

```json
{"schema_version":1,"ok":true,"data":{},"error":null,"meta":{"timezone":"Asia/Shanghai","complete":true},"next_actions":null}
```

```sh
./chaoxing schema
./chaoxing schema quiz submit
./chaoxing quiz submit --help
```

| 退出码 | 含义 |
|---|---|
| 0 | 成功 |
| 1 | 本地文件或程序错误 |
| 2 | 命令、参数、试卷或答案不合法 |
| 3 | 未登录、会话失效或平台验证 |
| 4 | 无权限或找不到目标 |
| 5 | 缺少确认、已批阅或任务不可执行 |
| 6 | 网络、平台或页面协议错误 |
| 7 | 部分完成、提交结果或完成状态未确认 |

## 已知限制

- 考试功能覆盖已启动会话的整卷导出，以及本地 JSON 的读取和答案填充；自动入场、在线保存答案与交卷尚未实现。
- 章节／随堂测验覆盖本机 CxStudy 对应的移动端题目页面；讨论、独立作业、简答题提交及其他题型不在当前实现中。
- 图片以 Markdown 链接和题干 HTML 保留；本工具不执行 OCR，也没有字体反混淆引擎。
- 登录、卡片、测验和上报接口随平台变化可能需要适配。HTTP 测试覆盖协议行为，真实账号验证范围见 [VALIDATION.md](VALIDATION.md)。

## 许可证

GPL-3.0。平台接口流程移植自 CxStudy/CxKitty；来源与版权说明见 `NOTICE`。
