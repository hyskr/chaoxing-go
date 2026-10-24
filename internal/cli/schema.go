package cli

import (
	"flag"
	"io"
	"strings"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
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
		commands[i].Flags = append(commands[i].Flags, FlagSpec{"output", "string", "auto", false, "展示格式：auto（终端文本、管道JSON）、text或json"}, FlagSpec{"data-dir", "string", ".session", false, "本工具登录会话保存目录"})
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
		return p, chaoxing.Failure("INVALID_ARGUMENT", err.Error(), 2)
	}
	if fs.NArg() != 0 {
		return p, chaoxing.Failure("INVALID_ARGUMENT", "不支持位置参数："+strings.Join(fs.Args(), " "), 2)
	}
	for _, f := range flags {
		if f.Required && strings.TrimSpace(p.str(f.Name)) == "" {
			return p, chaoxing.Failure("INVALID_ARGUMENT", "缺少 --"+f.Name, 2)
		}
	}
	return p, nil
}
