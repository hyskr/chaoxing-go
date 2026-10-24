// 协议来源：CxStudy cxapi/schema.py: 课程、题型及导出字段；Go 数据模型和序列化约定见 UPSTREAM.md。
package chaoxing

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Course struct {
	ID      string `json:"id"`
	ClassID string `json:"class_id"`
	CPI     string `json:"cpi"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}
type Chapter struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Name     string `json:"name"`
	Total    int    `json:"total"`
	Finished int    `json:"finished"`
}
type Job struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	ChapterID string `json:"chapter_id"`
	Card      int    `json:"card"`
	Passed    bool   `json:"passed"`
	ObjectID  string `json:"object_id,omitempty"`
	WorkID    string `json:"work_id,omitempty"`
	SchoolID  string `json:"school_id,omitempty"`
	PlayTime  int    `json:"play_time_ms,omitempty"`
	raw       map[string]any
	defaults  map[string]any
}
type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Text  string `json:"text"`
}
type Question struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Text    string   `json:"text"`
	Options []Option `json:"options"`
	Answer  any      `json:"answer"`
	HTML    string   `json:"html,omitempty"`
	Blanks  int      `json:"blanks,omitempty"`
}
type Paper struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Kind           string     `json:"kind"`
	Completed      bool       `json:"completed"`
	Score          string     `json:"score,omitempty"`
	AnswerEncoding string     `json:"answer_encoding"`
	Questions      []Question `json:"questions"`
	form           map[string][]string
	job            Job
}

func str(x any) string {
	if x == nil {
		return ""
	}
	return fmt.Sprint(x)
}
func integer(x any) int { n, _ := strconv.Atoi(str(x)); return n }
func object(x any) map[string]any {
	m, _ := x.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func array(x any) []any { a, _ := x.([]any); return a }
func truth(x any) bool  { return x == true || str(x) == "1" || str(x) == "true" }
func first(xs ...any) string {
	for _, x := range xs {
		if s := str(x); s != "" {
			return s
		}
	}
	return ""
}
func decodeJSON(b []byte, v any) error {
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	return d.Decode(v)
}
