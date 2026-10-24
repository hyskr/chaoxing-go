# 上游来源与更新

本项目用 Go 重写 CxStudy 的部分超星移动端协议流程。接口路径、请求参数、签名公式和页面字段来自下列 Python 实现；Go 的包组织、CLI、文件读写和执行控制在本项目中实现。

## 版本基线

| 来源 | 固定版本 | 用途 |
|---|---|---|
| [Yu-dongxing/CxStudy](https://github.com/Yu-dongxing/CxStudy) | [`7ad1f13f627aca55622afef9f22c50a4951b6e61`](https://github.com/Yu-dongxing/CxStudy/tree/7ad1f13f627aca55622afef9f22c50a4951b6e61) | 实际参考的本机 CxStudy checkout 的 HEAD；超星协议与导出格式基线 |
| [L-1124/WkHelper](https://github.com/L-1124/WkHelper) | [`714859e1d6269f0fe6b024b4ae1c3959ab75bc2b`](https://github.com/L-1124/WkHelper/tree/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b) | 2026-10-24 对照的文档基线；TTY 自动选择可读展示、JSON 响应、NDJSON 事件、schema 自省的设计参考 |

CxStudy 延续 CxKitty 项目，原项目作者及贡献者包括 SocialSisterYi、CxKitty 与 CxStudy 贡献者，许可证为 GPL-3.0，见 [上游 LICENSE](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/LICENSE) 和本仓库 [NOTICE](NOTICE)。WkHelper 提供命令接口设计参考；超星协议来自 CxStudy，WkHelper 的 Python 模块未移植进本项目。

## 实现对应表

本表的 CxStudy 链接全部固定到上述提交。函数名用于定位协议，Go 实现按职责重新组织。

| 本项目文件／入口 | 上游文件与符号 | 对应内容 |
|---|---|---|
| [`internal/chaoxing/auth.go`](internal/chaoxing/auth.go)：`Login`、`Auth`、`encryptPassword` | [cxapi/api.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/api.py)：`ChaoXingAPI.login_passwd`、`accinfo` | 密码登录、AES-CBC 编码、SSO 会话检查 |
| [`internal/chaoxing/client.go`](internal/chaoxing/client.go)：`NewClient`、`request` | [cxapi/session.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/session.py)：`SessionWraper`；[cxapi/utils.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/utils.py)：`mobile_ua_sign`、`get_ua` | Cookie 会话、移动端请求头、6.3.9 移动端 UA 签名；Go HTTP 与会话文件由本项目实现 |
| [`internal/chaoxing/course.go`](internal/chaoxing/course.go)：`Courses`、`chapters` | [cxapi/api.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/api.py)：`fetch_classes`；[cxapi/classes.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/classes.py)：`get_chapters_by_index`；[cxapi/chapters.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/chapters.py)：`fetch_point_status` | 课程、班级、章节及任务点进度 |
| [`internal/chaoxing/task.go`](internal/chaoxing/task.go)：`jobs`、`attachment`、`refreshJob`、`signParams` | [cxapi/chapters.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/chapters.py)：`fetch_points_by_index`；[cxapi/base.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/base.py)：`TaskPointBase.fetch_attachment`；[cxapi/utils.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/utils.py)：`inf_enc_sign` | 枚举卡片、读取 AttachmentSetting、刷新任务令牌、inf_enc 签名 |
| [`internal/chaoxing/quiz.go`](internal/chaoxing/quiz.go)：`parseQuiz`、`Quiz`、`SubmitQuiz` | [cxapi/task_point/work.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/task_point/work.py)：`parse_question`、`construct_questions_form`、`PointWorkDto.fetch_all`、`final_submit` | 章节／随堂测验 HTML、题型、内部选项键、表单与提交接口 |
| [`internal/chaoxing/study.go`](internal/chaoxing/study.go)：`videoSignature`、`videoReport`、`learnVideo` | [cxapi/task_point/video.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/task_point/video.py)：`PointVideoDto.fetch`、`play_report`；[resolver/media.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/resolver/media.py)：`MediaPlayResolver.execute` | 音视频资源状态、进度上报、签名及上报节奏 |
| [`internal/chaoxing/study.go`](internal/chaoxing/study.go)：`Study` 文档分支 | [cxapi/task_point/document.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/task_point/document.py)：`parse_attachment`、`report` | 文档任务标记与完成上报 |
| [`internal/chaoxing/exam.go`](internal/chaoxing/exam.go)：`parseExamHTML`、`ExportExam` | [cxapi/exam.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/exam.py)：`parse_question`、`ExamDto.fetch_all` | 整卷预览接口、试卷 HTML 与选项字段 |
| [`internal/chaoxing/model.go`](internal/chaoxing/model.go)、`quiz.go` 的 `QuestionType` | [cxapi/schema.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/schema.py)：`ClassModule`、`ChapterModel`、`QuestionType`、`QuestionModel` | 课程与题目字段、平台数字题型；Go 输出模型在本项目定义 |
| [`internal/paper/file.go`](internal/paper/file.go) | [cxapi/schema.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/schema.py)：`QuestionsExportSchema`；[cxapi/exam.py](https://github.com/Yu-dongxing/CxStudy/blob/7ad1f13f627aca55622afef9f22c50a4951b6e61/cxapi/exam.py)：`ExamDto.export` | 兼容 CxStudy 导出 JSON；读取、归一化、答案填充和 Markdown 转换由本项目实现 |
| [`internal/cli/`](internal/cli) | [WkHelper README：核心特性与架构](https://github.com/L-1124/WkHelper/blob/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b/README.md) | 终端可读展示／管道 JSON、结构化输出与命令自省设计；Go 表格、命令定义、解析、退出码和调度由本项目实现 |

终端展示另对照 WkHelper 的 [cli/context.py](https://github.com/L-1124/WkHelper/blob/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b/wkhelper/cli/context.py)、[cli/output.py](https://github.com/L-1124/WkHelper/blob/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b/wkhelper/cli/output.py) 和 [cli/markdown.py](https://github.com/L-1124/WkHelper/blob/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b/wkhelper/cli/markdown.py) 的 TTY 选择与可读呈现方式。Go 实现在 `internal/cli/text.go`，展示选择参数为 `--output`；`exam read --format` 用于试卷文件格式。

`internal/chaoxing/html.go` 使用 `golang.org/x/net/html` 自行实现 DOM 遍历与 JS 对象提取；`answer.go` 实现统一答案校验；`errors.go` 实现本工具错误约定。`cmd/chaoxing`、测试夹具与示例文件由本项目维护。

## 本项目适配

更新上游时需要保留这些行为：

| 本地行为 | 对应代码 | 回归测试 |
|---|---|---|
| 从整份响应定位 AttachmentSetting，支持前置无关脚本和 JSON 字符串内大括号；延续实际使用版本的本地适配 | `html.go: assignedJSON` | `html_test.go: TestAssignedJSONHandlesScriptOrderAndQuotedBraces` |
| 枚举章节所有任务卡，以章节完成计数辅助判定任务完成 | `task.go: jobs` | `task_test.go: TestAllCardsAreScanned` |
| 显示字母 `label` 与内部提交键 `key` 分开，导入 CxStudy 对象时保留出现顺序 | `quiz.go`、`exam.go`、`paper/file.go` | `quiz_test.go`、`exam_test.go`、`paper/file_test.go` |
| 测验提交查询参数将上游的 `cpi:` 修正为 `cpi`；提交前刷新卡片令牌，提交后读取批阅结果 | `quiz.go: Quiz、SubmitQuiz` | `quiz_test.go: TestQuizSubmissionAndReview` |
| 视频从服务器断点继续，按实际经过时间推进；以服务器 isPassed 确认完成，到达时长后最多进行三次末端上报 | `study.go: learnVideo` | `study_test.go: TestVideoCompletionRequiresPlatformAck` 覆盖末端确认 |
| 文档上报后刷新卡片，以 job 标记消失识别完成 | `task.go: refreshJob`、`study.go: Study` | `study_test.go: TestDocumentCompletionUsesJobMarker` |
| 考试从已进入的会话刷新 enc 和剩余时间参数，读取整卷预览 | `exam.go: ExportExam` | `exam_test.go: TestExamExportUsesCurrentSessionAndOptionKeys` |
| 本地试卷命令直接处理文件；CLI 输出 JSON 响应并保持确认参数约定 | `cli/execute.go`、`cli/response.go` | `cli/cli_test.go` |

## 跟进更新

查看 [CxStudy 基线到 main 的变化](https://github.com/Yu-dongxing/CxStudy/compare/7ad1f13f627aca55622afef9f22c50a4951b6e61...main) 与 [WkHelper 文档基线到 main 的变化](https://github.com/L-1124/WkHelper/compare/714859e1d6269f0fe6b024b4ae1c3959ab75bc2b...main)。新提交只是候选更新，表中的基线表示已经用于本项目实现或文档对照的版本。

在单独的上游 checkout 查看协议文件差异，保留本项目独立 Git 历史：

```sh
# 首次获取；已有 checkout 时直接进入相应目录
git clone https://github.com/Yu-dongxing/CxStudy.git ../CxStudy
git -C ../CxStudy fetch origin
git -C ../CxStudy log --oneline 7ad1f13f627aca55622afef9f22c50a4951b6e61..origin/main
git -C ../CxStudy diff 7ad1f13f627aca55622afef9f22c50a4951b6e61 origin/main -- \
  cxapi/api.py cxapi/session.py cxapi/utils.py cxapi/classes.py \
  cxapi/chapters.py cxapi/base.py cxapi/task_point cxapi/exam.py \
  cxapi/schema.py resolver/media.py
```

按对应表将需要的协议变更移植到 Go，更新相关响应／HTML 测试夹具，并运行对应测试。例如测验协议变化：

```sh
go test ./internal/chaoxing -run 'TestQuiz|TestMissingAnswer'
go test ./...
go build -o chaoxing ./cmd/chaoxing
```

合入更新时同步修改本文件的完整提交号、固定链接和本地差异表，在 [VALIDATION.md](VALIDATION.md) 记录验证范围。WkHelper 更新主要对照 CLI 契约，独立评估是否采用。

## 已知限制

Go 实现覆盖账号密码登录、课程／任务卡、学习上报、章节测验和考试读取；CxStudy 的二维码登录、题库后端、自动验证码、人脸验证与考试在线保存／交卷没有移植。本机 CxStudy 的本地修改不等于上游提交，已纳入的适配以上表和 Go 代码为准。真实账号与 HTTP 测试的验证边界见 [VALIDATION.md](VALIDATION.md)。
