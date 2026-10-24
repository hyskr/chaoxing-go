package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIJSONAndConfirmContract(t *testing.T) {
	var out, events bytes.Buffer
	code := Run(context.Background(), []string{"study", "run", "--course", "1"}, strings.NewReader(""), &out, &events)
	var envelope map[string]any
	if e := json.Unmarshal(out.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	if code != 5 || envelope["ok"] != false || envelope["error"].(map[string]any)["code"] != "CONFIRM_REQUIRED" {
		t.Fatalf("不一致的退出码: %d %s", code, out.String())
	}
	out.Reset()
	code = Run(context.Background(), []string{"schema", "quiz", "submit"}, strings.NewReader(""), &out, &events)
	if code != 0 || !strings.Contains(out.String(), "answers") {
		t.Fatal("schema 与命令脱节")
	}
}

func TestExamFileCommands(t *testing.T) {
	dir := t.TempDir()
	filled := filepath.Join(dir, "answered.json")
	var out, events bytes.Buffer
	code := Run(context.Background(), []string{"exam", "fill", "--file", "../../examples/exam.json", "--answers", "../../examples/answers.json", "--out", filled}, strings.NewReader(""), &out, &events)
	if code != 0 || !strings.Contains(out.String(), `"submitted":false`) {
		t.Fatalf("本地填充失败: code=%d %s", code, out.String())
	}
	out.Reset()
	code = Run(context.Background(), []string{"exam", "read", "--file", filled, "--format", "md"}, strings.NewReader(""), &out, &events)
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Markdown string `json:"markdown"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if code != 0 || !response.OK || !strings.Contains(response.Data.Markdown, "[key=") || !strings.Contains(response.Data.Markdown, "答案（内部键）") {
		t.Fatalf("本地读取输出不完整: code=%d %s", code, out.String())
	}
	out.Reset()
	markdown := filepath.Join(dir, "paper.md")
	code = Run(context.Background(), []string{"exam", "read", "--file", filled, "--format", "md", "--out", markdown}, strings.NewReader(""), &out, &events)
	data, err := os.ReadFile(markdown)
	if code != 0 || err != nil || string(data) != response.Data.Markdown {
		t.Fatalf("Markdown 文件输出与响应不一致: code=%d err=%v", code, err)
	}
}
