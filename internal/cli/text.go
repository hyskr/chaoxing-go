package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// outputMode follows stdout so a pipe receives JSON even when stderr is a terminal.
func outputMode(args []string, stdout io.Writer) ([]string, bool, error) {
	terminal := false
	if file, ok := stdout.(*os.File); ok {
		terminal = term.IsTerminal(int(file.Fd()))
	}
	mode := "auto"
	clean := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--output":
			if i+1 == len(args) {
				return clean, terminal, chaoxing.Failure("INVALID_ARGUMENT", "缺少 --output 的值：auto、text 或 json", 2)
			}
			i++
			mode = args[i]
		case strings.HasPrefix(args[i], "--output="):
			mode = strings.TrimPrefix(args[i], "--output=")
		default:
			clean = append(clean, args[i])
		}
	}
	switch mode {
	case "text":
		return clean, true, nil
	case "json":
		return clean, false, nil
	case "auto":
		return clean, terminal, nil
	default:
		return clean, terminal, chaoxing.Failure("INVALID_ARGUMENT", "--output 只能为 auto、text 或 json", 2)
	}
}

func table(b *strings.Builder, title string, headers []string, rows [][]string) {
	fmt.Fprintln(b, title)
	b.WriteByte('\n')
	widths := make([]int, len(headers))
	all := append([][]string{headers}, rows...)
	for _, row := range all {
		for i, cell := range row {
			row[i] = strings.Join(strings.Fields(cell), " ")
			widths[i] = max(widths[i], runewidth.StringWidth(row[i]))
		}
	}
	writeRow := func(row []string) {
		for i, cell := range row {
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-runewidth.StringWidth(cell)+2))
			}
		}
		b.WriteByte('\n')
	}
	writeRow(headers)
	line := make([]string, len(headers))
	for i, width := range widths {
		line[i] = strings.Repeat("─", width)
	}
	writeRow(line)
	for _, row := range rows {
		writeRow(row)
	}
	if len(rows) == 0 {
		fmt.Fprintln(b, "暂无记录")
	}
}

func jobType(kind string) string {
	switch kind {
	case "video":
		return "视频"
	case "audio":
		return "音频"
	case "document":
		return "文档"
	case "workid":
		return "测验"
	default:
		return kind
	}
}
func jobRows(jobs []chaoxing.Job) [][]string {
	rows := [][]string{}
	for i, job := range jobs {
		state := "未完成"
		if job.Passed {
			state = "已完成"
		}
		rows = append(rows, []string{strconv.Itoa(i + 1), job.ID, jobType(job.Type), job.Name, job.ChapterID, state})
	}
	return rows
}

func textResult(data any) string {
	var b strings.Builder
	switch value := data.(type) {
	case nil:
		return ""
	case []chaoxing.Course:
		rows := [][]string{}
		for i, course := range value {
			state := "进行中"
			if course.Status == "ended" {
				state = "已结束"
			}
			rows = append(rows, []string{strconv.Itoa(i + 1), course.ID + ":" + course.ClassID, course.Name, state})
		}
		table(&b, fmt.Sprintf("课程列表（共 %d 门）", len(value)), []string{"序号", "课程:班级", "课程名称", "状态"}, rows)
	case []chaoxing.Job:
		table(&b, fmt.Sprintf("任务列表（共 %d 项）", len(value)), []string{"序号", "任务 ID", "类型", "任务名称", "章节 ID", "状态"}, jobRows(value))
	case []chaoxing.StudyResult:
		rows := [][]string{}
		for i, result := range value {
			state := map[string]string{"planned": "待执行", "already_completed": "已完成（跳过）", "completed": "已完成"}[result.State]
			rows = append(rows, []string{strconv.Itoa(i + 1), result.JobID, result.Name, state})
		}
		table(&b, fmt.Sprintf("学习任务（共 %d 项）", len(value)), []string{"序号", "任务 ID", "任务名称", "状态"}, rows)
	case chaoxing.Paper:
		fmt.Fprintf(&b, "%s（共 %d 题）\n", value.Title, len(value.Questions))
		if value.Completed {
			fmt.Fprintln(&b, "状态：已完成")
		}
		if value.Score != "" {
			fmt.Fprintf(&b, "得分：%s\n", value.Score)
		}
		for i, q := range value.Questions {
			kind := map[string]string{"single": "单选题", "multiple": "多选题", "judgement": "判断题", "completion": "填空题", "shortanswer": "简答题"}[q.Type]
			fmt.Fprintf(&b, "\n%d. [%s] %s（题目 ID：%s）\n", i+1, kind, q.Text, q.ID)
			for _, option := range q.Options {
				fmt.Fprintf(&b, "   %s. %s [key=%s]\n", option.Label, option.Text, option.Key)
			}
			if q.Answer != nil {
				fmt.Fprintf(&b, "   答案（内部键）：%v\n", q.Answer)
			}
		}
	case CommandSpec:
		fmt.Fprintf(&b, "用法：chaoxing %s [参数]\n%s\n\n", value.Path, value.Summary)
		rows := [][]string{}
		for _, flag := range value.Flags {
			required := ""
			if flag.Required {
				required = "必填"
			}
			rows = append(rows, []string{"--" + flag.Name, flag.Description, required, flag.Default})
		}
		table(&b, "参数", []string{"参数", "说明", "要求", "默认值"}, rows)
	case *CommandSpec:
		return textResult(*value)
	case map[string]any:
		if specs, ok := value["commands"].([]CommandSpec); ok {
			rows := [][]string{}
			for _, spec := range specs {
				rows = append(rows, []string{spec.Path, spec.Summary})
			}
			table(&b, "chaoxing — 超星学习通命令行工具", []string{"命令", "说明"}, rows)
			fmt.Fprintln(&b, "\n使用 chaoxing <命令> --help 查看参数。\n终端默认显示表格；--output json 输出 JSON，--output text 强制文本。")
		} else if course, ok := value["course"].(chaoxing.Course); ok {
			progress := value["progress"].(map[string]int)
			fmt.Fprintf(&b, "%s（%s:%s）\n任务点：%d/%d\n\n", course.Name, course.ID, course.ClassID, progress["finished"], progress["total"])
			rows := [][]string{}
			for _, chapter := range value["chapters"].([]chaoxing.Chapter) {
				rows = append(rows, []string{chapter.Label, chapter.ID, chapter.Name, fmt.Sprintf("%d/%d", chapter.Finished, chapter.Total)})
			}
			table(&b, "章节", []string{"序号", "章节 ID", "章节名称", "任务点"}, rows)
			b.WriteByte('\n')
			b.WriteString(textResult(value["jobs"]))
		} else if markdown, ok := value["markdown"].(string); ok {
			b.WriteString(markdown)
		} else {
			if value["authenticated"] == true {
				fmt.Fprintln(&b, "已登录")
			}
			if value["submitted"] == true {
				fmt.Fprintln(&b, "提交成功")
				if value["verified"] == false {
					fmt.Fprintln(&b, "批阅状态尚未确认")
				}
			} else if value["submitted"] == false {
				fmt.Fprintln(&b, "本地答案已填写（未提交）")
			}
			for _, field := range []struct{ key, label string }{{"path", "输出文件"}, {"questions", "题目数量"}, {"exam_id", "考试 ID"}, {"work_id", "测验 ID"}, {"cookie_file", "会话文件"}, {"score", "得分"}} {
				if v, ok := value[field.key]; ok && v != "" {
					fmt.Fprintf(&b, "%s：%v\n", field.label, v)
				}
			}
			if value["completed"] == true {
				fmt.Fprintln(&b, "状态：已完成")
			}
			if result, ok := value["paper"]; ok {
				b.WriteString(textResult(result))
			}
		}
	}
	return b.String()
}

type textEvents struct{ writer io.Writer }

func (events textEvents) Write(data []byte) (int, error) {
	var event struct {
		Event    string `json:"event"`
		Message  string `json:"message"`
		JobID    string `json:"job_id"`
		Seconds  int    `json:"seconds"`
		Duration int    `json:"duration"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return 0, err
	}
	var line string
	switch event.Event {
	case "auth.password_required":
		line = event.Message
	case "video.progress":
		line = fmt.Sprintf("视频 %s：%d / %d 秒", event.JobID, event.Seconds, event.Duration)
	case "study.completed":
		line = "任务已完成：" + event.JobID
	}
	_, err := fmt.Fprintln(events.writer, line)
	return len(data), err
}
