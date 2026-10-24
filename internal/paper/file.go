// 文件格式参考：CxStudy cxapi/schema.py: QuestionsExportSchema；cxapi/exam.py: ExamDto.export。见 UPSTREAM.md。
package paper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
)

// CxStudy 的对象键是提交用的内部键；对象中的出现顺序才对应显示字母。
func readOptions(raw json.RawMessage) ([]chaoxing.Option, error) {
	result := []chaoxing.Option{}
	s := bytes.TrimSpace(raw)
	if len(s) == 0 || string(s) == "null" {
		return result, nil
	}
	if s[0] == '[' {
		var blanks []string
		if err := json.Unmarshal(s, &blanks); err == nil {
			for i, label := range blanks {
				result = append(result, chaoxing.Option{Key: strconv.Itoa(i + 1), Label: label, Text: ""})
			}
			return result, nil
		}
		if err := json.Unmarshal(s, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	d := json.NewDecoder(bytes.NewReader(s))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("options 必须是对象或选项数组")
	}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value string
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		label := string(rune('A' + len(result)))
		result = append(result, chaoxing.Option{fmt.Sprint(key), label, value})
	}
	_, err = d.Token()
	return result, err
}
func Read(path string) (chaoxing.Paper, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return chaoxing.Paper{}, err
	}
	var raw struct {
		ID        json.RawMessage   `json:"id"`
		Title     string            `json:"title"`
		Kind      string            `json:"kind"`
		Questions []json.RawMessage `json:"questions"`
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return chaoxing.Paper{}, chaoxing.Failure("INVALID_PAPER", "试卷 JSON 解析失败", 2)
	}
	var id any
	_ = decodeJSON(raw.ID, &id)
	p := chaoxing.Paper{ID: str(id), Title: raw.Title, Kind: raw.Kind, AnswerEncoding: "option_key", Questions: []chaoxing.Question{}}
	if p.Kind == "" {
		p.Kind = "exam"
	}
	if len(raw.Questions) == 0 {
		return p, chaoxing.Failure("INVALID_PAPER", "试卷没有 questions", 2)
	}
	seen := map[string]bool{}
	for _, r := range raw.Questions {
		var row struct {
			ID      any             `json:"id"`
			Value   string          `json:"value"`
			Text    string          `json:"text"`
			Type    any             `json:"type"`
			Options json.RawMessage `json:"options"`
			Answer  any             `json:"answer"`
			Blanks  int             `json:"blanks"`
		}
		if err = decodeJSON(r, &row); err != nil {
			return p, err
		}
		typ := str(row.Type)
		if _, err := strconv.Atoi(typ); err == nil {
			typ = chaoxing.QuestionType(typ)
		}
		options, e := readOptions(row.Options)
		if e != nil {
			return p, chaoxing.Failure("INVALID_PAPER", e.Error(), 2)
		}
		q := chaoxing.Question{ID: str(row.ID), Type: typ, Text: row.Text, Options: options, Answer: row.Answer, Blanks: row.Blanks}
		if q.Text == "" {
			q.Text = row.Value
		}
		if typ == "completion" && q.Blanks == 0 {
			q.Blanks = len(options)
		}
		if q.ID == "" || seen[q.ID] || q.Text == "" {
			return p, chaoxing.Failure("INVALID_PAPER", "题目ID为空、重复或缺少题干", 2)
		}
		seen[q.ID] = true
		p.Questions = append(p.Questions, q)
	}
	return p, nil
}
func ReadAnswers(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(b, &raw); err != nil {
		return nil, chaoxing.Failure("INVALID_ANSWERS", "答案 JSON 解析失败", 2)
	}
	if qs, ok := raw["questions"]; ok {
		var questions []struct {
			ID     any `json:"id"`
			Answer any `json:"answer"`
		}
		if err = decodeJSON(qs, &questions); err != nil {
			return nil, err
		}
		out := map[string]any{}
		for _, q := range questions {
			id := str(q.ID)
			if id == "" {
				return nil, chaoxing.Failure("INVALID_ANSWERS", "答案题目缺少ID", 2)
			}
			out[id] = q.Answer
		}
		return out, nil
	}
	var answers map[string]any
	err = decodeJSON(b, &answers)
	return answers, err
}
func Fill(p chaoxing.Paper, answers map[string]any) (chaoxing.Paper, error) {
	for i, q := range p.Questions {
		value, ok := answers[q.ID]
		if !ok {
			value = q.Answer
		}
		a, err := chaoxing.NormalizeAnswer(q, value)
		if err != nil {
			return p, err
		}
		p.Questions[i].Answer = a
	}
	return p, nil
}
func Markdown(p chaoxing.Paper) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n选项显示字母用于网页查看；提交使用括号中的 key。\n\n", p.Title)
	for i, q := range p.Questions {
		fmt.Fprintf(&b, "## %d. %s\n\n%s\n\n", i+1, q.Type, q.Text)
		for _, o := range q.Options {
			fmt.Fprintf(&b, "- %s [key=%s] %s\n", o.Label, o.Key, o.Text)
		}
		if q.Answer != nil {
			a, _ := json.Marshal(q.Answer)
			fmt.Fprintf(&b, "\n答案（内部键）：%s\n", a)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func WriteJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}

func str(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func decodeJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}
