package cli

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
	"github.com/hyskr/chaoxing-go/internal/paper"
	"golang.org/x/term"
)

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
		return nil, chaoxing.Failure("UNKNOWN_COMMAND", "找不到命令 "+path, 2)
	}
	if len(args) < 2 {
		return nil, chaoxing.Failure("UNKNOWN_COMMAND", "请使用 schema 查看命令", 2)
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
		return nil, chaoxing.Failure("UNKNOWN_COMMAND", "找不到命令 "+path, 2)
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
		document, e := paper.Read(p.str("file"))
		if e != nil {
			return nil, e
		}
		if path == "exam fill" {
			answers, e := paper.ReadAnswers(p.str("answers"))
			if e != nil {
				return nil, e
			}
			document, e = paper.Fill(document, answers)
			if e != nil {
				return nil, e
			}
			if e = paper.WriteJSON(p.str("out"), document); e != nil {
				return nil, e
			}
			return map[string]any{"path": p.str("out"), "questions": len(document.Questions), "submitted": false}, nil
		}
		format := p.str("format")
		if format != "json" && format != "md" {
			return nil, chaoxing.Failure("INVALID_ARGUMENT", "--format 只能为 json 或 md", 2)
		}
		if output := p.str("out"); output != "" {
			if format == "md" {
				err = os.WriteFile(output, []byte(paper.Markdown(document)), 0600)
			} else {
				err = paper.WriteJSON(output, document)
			}
			if err != nil {
				return nil, err
			}
			return map[string]any{"path": output, "questions": len(document.Questions)}, nil
		}
		if format == "md" {
			return map[string]any{"markdown": paper.Markdown(document)}, nil
		}
		return document, nil
	}
	if (path == "quiz submit" || path == "study run" && !p.boolean("dry-run")) && !p.boolean("confirm") {
		return nil, chaoxing.Failure("CONFIRM_REQUIRED", "写操作需要 --confirm；study run 可使用 --dry-run", 5)
	}
	c, err := chaoxing.NewClient(filepath.Join(p.str("data-dir"), "session.txt"))
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
			return nil, chaoxing.Failure("INVALID_ARGUMENT", "请通过标准输入传入密码", 2)
		}
		return c.Login(ctx, p.str("phone"), secret)
	}
	if _, err = c.Auth(ctx); err != nil {
		return nil, err
	}
	if path == "auth status" {
		return map[string]any{"authenticated": true}, nil
	}
	if path == "course list" {
		return c.Courses(ctx)
	}
	if path == "exam export" {
		document, e := c.ExportExam(ctx, p.str("url"))
		if e != nil {
			return nil, e
		}
		if e = paper.WriteJSON(p.str("out"), document); e != nil {
			return nil, e
		}
		return map[string]any{"path": p.str("out"), "questions": len(document.Questions), "exam_id": document.ID}, nil
	}
	course, chapters, jobs, err := c.LoadJobs(ctx, p.str("course"))
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
		result := []chaoxing.Job{}
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
			return nil, chaoxing.Failure("INVALID_ARGUMENT", "--speed 必须在1到2之间", 2)
		}
		if id := p.str("job"); id != "" {
			j, e := chaoxing.ChooseJob(jobs, id)
			if e != nil {
				return nil, e
			}
			jobs = []chaoxing.Job{j}
		}
		emit := func(v any) { _ = json.NewEncoder(events).Encode(v) }
		return c.Study(ctx, course, jobs, speed, p.boolean("dry-run"), emit)
	}
	job, err := chaoxing.ChooseJob(jobs, p.str("job"))
	if err != nil {
		return nil, err
	}
	if job.Type != "workid" {
		return nil, chaoxing.Failure("INVALID_ARGUMENT", "指定任务不是章节或随堂测验", 2)
	}
	document, err := c.Quiz(ctx, course, job)
	if err != nil {
		return nil, err
	}
	if path == "quiz export" {
		if output := p.str("out"); output != "" {
			if err = paper.WriteJSON(output, document); err != nil {
				return nil, err
			}
			return map[string]any{"path": output, "questions": len(document.Questions), "completed": document.Completed, "score": document.Score}, nil
		}
		return document, nil
	}
	answers, err := paper.ReadAnswers(p.str("answers"))
	if err != nil {
		return nil, err
	}
	return c.SubmitQuiz(ctx, course, document, answers)
}
