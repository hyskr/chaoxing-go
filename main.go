package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/term"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

type FlagSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description"`
}
type CommandSpec struct {
	Path    string     `json:"path"`
	Summary string     `json:"summary"`
	Writes  bool       `json:"writes"`
	Flags   []FlagSpec `json:"flags"`
}

var courseFlag = FlagSpec{"course", "string", "", true, "课程ID或课程ID:班级ID"}
var outFlag = FlagSpec{"out", "string", "", false, "输出文件路径"}
var jobFlag = FlagSpec{"job", "string", "", true, "任务jobid或workid"}
var fileFlag = FlagSpec{"file", "string", "", true, "试卷或测验JSON路径"}
var commands = []CommandSpec{
	{"auth login", "账号密码登录；密码从标准输入读取", true, []FlagSpec{{"phone", "string", "", true, "学习通手机号"}}},
	{"auth status", "检查已保存的登录会话", false, nil},
	{"course list", "列出账号课程和班级", false, nil},
	{"course inspect", "读取全部章节、任务点进度和卡片任务", false, []FlagSpec{courseFlag}},
	{"study run", "执行视频、音频和文档学习任务", true, []FlagSpec{courseFlag, {"job", "string", "", false, "只处理指定任务"}, {"speed", "number", "1", false, "播放速度，范围1到2"}, {"dry-run", "boolean", "false", false, "只返回执行计划"}, {"confirm", "boolean", "false", false, "确认上报学习进度"}}},
	{"quiz list", "列出课程内全部章节与随堂测验任务", false, []FlagSpec{courseFlag}},
	{"quiz export", "获取指定测验的完整题目和内部选项键", false, []FlagSpec{courseFlag, jobFlag, outFlag}},
	{"quiz submit", "按题目ID提交全部指定答案并读取批阅结果", true, []FlagSpec{courseFlag, jobFlag, {"answers", "string", "", true, "题目ID到答案的JSON文件，或填好answer的测验JSON"}, {"confirm", "boolean", "false", false, "确认提交测验"}}},
	{"exam read", "读取CxStudy或本工具导出的考试JSON", false, []FlagSpec{fileFlag, outFlag, {"format", "string", "json", false, "输出文件格式：json或md"}}},
	{"exam export", "从已进入的考试会话读取整卷并导出JSON", false, []FlagSpec{{"url", "string", "", true, "当前答题页或整卷预览URL"}, {"out", "string", "", true, "试卷JSON输出路径"}}},
	{"exam fill", "把按题目ID给出的答案写入新试卷JSON", true, []FlagSpec{fileFlag, {"answers", "string", "", true, "题目ID到答案的JSON文件"}, {"out", "string", "", true, "填写后试卷路径"}}},
}

func init() {
	for i := range commands {
		commands[i].Flags = append(commands[i].Flags, FlagSpec{"data-dir", "string", ".session", false, "本工具登录会话保存目录"})
	}
}

type parsed struct {
	s map[string]*string
	b map[string]*bool
	n map[string]*float64
}

func (p parsed) str(k string) string {
	if v := p.s[k]; v != nil {
		return *v
	}
	return ""
}
func (p parsed) boolean(k string) bool {
	if v := p.b[k]; v != nil {
		return *v
	}
	return false
}
func parse(spec CommandSpec, args []string) (parsed, error) {
	p := parsed{map[string]*string{}, map[string]*bool{}, map[string]*float64{}}
	fs := flag.NewFlagSet(spec.Path, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flags := append([]FlagSpec{}, spec.Flags...)
	for _, f := range flags {
		switch f.Type {
		case "boolean":
			p.b[f.Name] = fs.Bool(f.Name, f.Default == "true", f.Description)
		case "number":
			p.n[f.Name] = fs.Float64(f.Name, 1, f.Description)
		default:
			p.s[f.Name] = fs.String(f.Name, f.Default, f.Description)
		}
	}
	if err := fs.Parse(args); err != nil {
		return p, failure("INVALID_ARGUMENT", err.Error(), 2)
	}
	if fs.NArg() != 0 {
		return p, failure("INVALID_ARGUMENT", "不支持位置参数："+strings.Join(fs.Args(), " "), 2)
	}
	for _, f := range flags {
		if f.Required && strings.TrimSpace(p.str(f.Name)) == "" {
			return p, failure("INVALID_ARGUMENT", "缺少 --"+f.Name, 2)
		}
	}
	return p, nil
}
func execute(ctx context.Context, args []string, stdin io.Reader, events io.Writer) (any, error) {
	if len(args) == 0 || args[0] == "schema" || args[0] == "--help" {
		if len(args) <= 1 {
			return map[string]any{"commands": commands, "authentication": "password", "answer_encoding": "option_key", "exit_codes": map[string]int{"success": 0, "invalid_argument": 2, "auth_required": 3, "permission_or_target": 4, "precondition": 5, "upstream": 6, "partial": 7}}, nil
		}
		path := strings.Join(args[1:], " ")
		for _, s := range commands {
			if s.Path == path {
				return s, nil
			}
		}
		return nil, failure("UNKNOWN_COMMAND", "找不到命令 "+path, 2)
	}
	if len(args) < 2 {
		return nil, failure("UNKNOWN_COMMAND", "请使用 schema 查看命令", 2)
	}
	path := args[0] + " " + args[1]
	var spec *CommandSpec
	for i := range commands {
		if commands[i].Path == path {
			spec = &commands[i]
			break
		}
	}
	if spec == nil {
		return nil, failure("UNKNOWN_COMMAND", "找不到命令 "+path, 2)
	}
	for _, a := range args[2:] {
		if a == "--help" || a == "-h" {
			return spec, nil
		}
	}
	p, err := parse(*spec, args[2:])
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(path, "exam ") && path != "exam export" {
		paper, e := readPaper(p.str("file"))
		if e != nil {
			return nil, e
		}
		if path == "exam fill" {
			answers, e := readAnswers(p.str("answers"))
			if e != nil {
				return nil, e
			}
			paper, e = fillPaper(paper, answers)
			if e != nil {
				return nil, e
			}
			if e = writeJSON(p.str("out"), paper); e != nil {
				return nil, e
			}
			return map[string]any{"path": p.str("out"), "questions": len(paper.Questions), "submitted": false}, nil
		}
		format := p.str("format")
		if format != "json" && format != "md" {
			return nil, failure("INVALID_ARGUMENT", "--format 只能为 json 或 md", 2)
		}
		if output := p.str("out"); output != "" {
			if format == "md" {
				err = os.WriteFile(output, []byte(markdown(paper)), 0600)
			} else {
				err = writeJSON(output, paper)
			}
			if err != nil {
				return nil, err
			}
			return map[string]any{"path": output, "questions": len(paper.Questions)}, nil
		}
		if format == "md" {
			return map[string]any{"markdown": markdown(paper)}, nil
		}
		return paper, nil
	}
	if (path == "quiz submit" || path == "study run" && !p.boolean("dry-run")) && !p.boolean("confirm") {
		return nil, failure("CONFIRM_REQUIRED", "写操作需要 --confirm；study run 可使用 --dry-run", 5)
	}
	c, err := newClient(filepath.Join(p.str("data-dir"), "session.txt"))
	if err != nil {
		return nil, err
	}
	if path == "auth login" {
		var password []byte
		var e error
		if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			_ = json.NewEncoder(events).Encode(map[string]any{"event": "auth.password_required", "message": "请输入学习通密码（输入不回显），回车登录"})
			password, e = term.ReadPassword(int(file.Fd()))
		} else {
			password, e = io.ReadAll(stdin)
		}
		if e != nil {
			return nil, e
		}
		secret := strings.TrimRight(string(password), "\r\n")
		if secret == "" {
			return nil, failure("INVALID_ARGUMENT", "请通过标准输入传入密码", 2)
		}
		return c.login(ctx, p.str("phone"), secret)
	}
	if _, err = c.auth(ctx); err != nil {
		return nil, err
	}
	if path == "auth status" {
		return map[string]any{"authenticated": true}, nil
	}
	if path == "course list" {
		return c.courses(ctx)
	}
	if path == "exam export" {
		paper, e := c.exportExam(ctx, p.str("url"))
		if e != nil {
			return nil, e
		}
		if e = writeJSON(p.str("out"), paper); e != nil {
			return nil, e
		}
		return map[string]any{"path": p.str("out"), "questions": len(paper.Questions), "exam_id": paper.ID}, nil
	}
	course, chapters, jobs, err := c.loadJobs(ctx, p.str("course"))
	if err != nil {
		return nil, err
	}
	if path == "course inspect" {
		total, finished := 0, 0
		for _, ch := range chapters {
			total += ch.Total
			finished += ch.Finished
		}
		return map[string]any{"course": course, "chapters": chapters, "jobs": jobs, "progress": map[string]int{"total": total, "finished": finished}}, nil
	}
	if path == "quiz list" {
		result := []Job{}
		for _, j := range jobs {
			if j.Type == "workid" {
				result = append(result, j)
			}
		}
		return result, nil
	}
	if path == "study run" {
		speed := *p.n["speed"]
		if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < 1 || speed > 2 {
			return nil, failure("INVALID_ARGUMENT", "--speed 必须在1到2之间", 2)
		}
		if id := p.str("job"); id != "" {
			j, e := chooseJob(jobs, id)
			if e != nil {
				return nil, e
			}
			jobs = []Job{j}
		}
		emit := func(v any) { _ = json.NewEncoder(events).Encode(v) }
		return c.study(ctx, course, jobs, speed, p.boolean("dry-run"), emit)
	}
	job, err := chooseJob(jobs, p.str("job"))
	if err != nil {
		return nil, err
	}
	if job.Type != "workid" {
		return nil, failure("INVALID_ARGUMENT", "指定任务不是章节或随堂测验", 2)
	}
	paper, err := c.quiz(ctx, course, job)
	if err != nil {
		return nil, err
	}
	if path == "quiz export" {
		if output := p.str("out"); output != "" {
			if err = writeJSON(output, paper); err != nil {
				return nil, err
			}
			return map[string]any{"path": output, "questions": len(paper.Questions), "completed": paper.Completed, "score": paper.Score}, nil
		}
		return paper, nil
	}
	answers, err := readAnswers(p.str("answers"))
	if err != nil {
		return nil, err
	}
	return c.submitQuiz(ctx, course, paper, answers)
}
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := execute(ctx, args, stdin, stderr)
	var detail *AppError
	code := 0
	var next []string
	if err != nil {
		if !errors.As(err, &detail) {
			detail = &AppError{"INTERNAL_ERROR", err.Error(), 1}
		}
		code = detail.Exit
		if code == 3 {
			next = []string{"auth login"}
		} else if code == 2 || code == 5 {
			next = []string{"schema"}
		}
	}
	response := struct {
		Schema int            `json:"schema_version"`
		OK     bool           `json:"ok"`
		Data   any            `json:"data"`
		Error  *AppError      `json:"error"`
		Meta   map[string]any `json:"meta"`
		Next   []string       `json:"next_actions"`
	}{1, err == nil, data, detail, map[string]any{"timezone": "Asia/Shanghai", "complete": err == nil}, next}
	if e := json.NewEncoder(stdout).Encode(response); e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	return code
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
