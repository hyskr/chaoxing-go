package paper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExamImportPreservesCxStudyOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exam.json")
	os.WriteFile(file, []byte(`{"id":7,"title":"考试","questions":[{"id":9,"type":0,"value":"题干","options":{"C":"显示A","A":"显示B"},"answer":null},{"id":10,"type":2,"value":"填空","options":["第一空"],"answer":null}]}`), 0600)
	p, e := Read(file)
	if e != nil {
		t.Fatal(e)
	}
	if p.Questions[0].Options[0].Key != "C" || p.Questions[0].Options[0].Label != "A" {
		t.Fatal("丢失输入对象顺序")
	}
	p, e = Fill(p, map[string]any{"9": "C", "10": []any{"内容"}})
	if e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "filled.json")
	if e = WriteJSON(target, p); e != nil {
		t.Fatal(e)
	}
	again, e := Read(target)
	if e != nil || again.Questions[0].Answer != "C" || again.Questions[1].Blanks != 1 {
		t.Fatalf("试卷往返读写失败: %v %+v", e, again)
	}
}
