package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
	"github.com/mattn/go-runewidth"
)

func TestTextCLIOutput(t *testing.T) {
	for _, test := range []struct {
		name      string
		args      []string
		code      int
		output    string
		errorText string
	}{
		{"help", []string{"--output", "text", "--help"}, 0, "命令", ""},
		{"command help", []string{"quiz", "submit", "--help", "--output=text"}, 0, "--answers", ""},
		{"paper", []string{"exam", "read", "--file", "../../examples/exam.json", "--output", "text"}, 0, "[key=", ""},
		{"confirmation", []string{"study", "run", "--course", "1", "--output", "text"}, 5, "", "错误 [CONFIRM_REQUIRED]"},
		{"invalid output", []string{"study", "run", "--output", "csv"}, 2, `"code":"INVALID_ARGUMENT"`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, events bytes.Buffer
			code := Run(context.Background(), test.args, strings.NewReader(""), &out, &events)
			if code != test.code || !strings.Contains(out.String(), test.output) || !strings.Contains(events.String(), test.errorText) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), events.String())
			}
			if test.errorText != "" && out.Len() != 0 {
				t.Fatalf("文本错误应输出到 stderr: %s", out.String())
			}
		})
	}
}

func TestChineseTableAlignmentAndStates(t *testing.T) {
	result := textResult([]chaoxing.Job{
		{ID: "1", Type: "workid", Name: "实验室安全", ChapterID: "100", Passed: true},
		{ID: "2", Type: "video", Name: "AI 基础", ChapterID: "200"},
	})
	if !strings.Contains(result, "测验") || !strings.Contains(result, "已完成") || !strings.Contains(result, "未完成") {
		t.Fatal(result)
	}
	columns := []int{}
	for _, line := range strings.Split(result, "\n") {
		for _, id := range []string{"100", "200"} {
			if i := strings.Index(line, id); i >= 0 {
				columns = append(columns, runewidth.StringWidth(line[:i]))
			}
		}
	}
	if len(columns) != 2 || columns[0] != columns[1] {
		t.Fatalf("中英文名称的列未对齐: %v\n%s", columns, result)
	}
	if empty := textResult([]chaoxing.Job{}); !strings.Contains(empty, "暂无记录") {
		t.Fatal(empty)
	}
}

func TestTextProgress(t *testing.T) {
	var out bytes.Buffer
	events := textEvents{&out}
	for _, data := range []string{
		`{"event":"auth.password_required","message":"请输入密码"}`,
		`{"event":"video.progress","job_id":"v1","seconds":58,"duration":120}`,
		`{"event":"study.completed","job_id":"v1"}`,
	} {
		if n, err := events.Write([]byte(data)); err != nil || n != len(data) {
			t.Fatalf("n=%d err=%v", n, err)
		}
	}
	if out.String() != "请输入密码\n视频 v1：58 / 120 秒\n任务已完成：v1\n" {
		t.Fatal(out.String())
	}
}

func TestOutputModePreservesCommandFlags(t *testing.T) {
	args, text, err := outputMode([]string{"--output", "json", "exam", "read", "--format", "md", "--file", "exam.json"}, &bytes.Buffer{})
	if err != nil || text || strings.Join(args, " ") != "exam read --format md --file exam.json" {
		t.Fatalf("args=%v text=%v err=%v", args, text, err)
	}
}
