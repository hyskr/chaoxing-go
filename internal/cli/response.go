package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/hyskr/chaoxing-go/internal/chaoxing"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	args, text, err := outputMode(args, stdout)
	var data any
	if err == nil {
		var events io.Writer = stderr
		if text {
			events = textEvents{stderr}
		}
		data, err = execute(ctx, args, stdin, events)
	}
	var detail *chaoxing.AppError
	code := 0
	var next []string
	if err != nil {
		if !errors.As(err, &detail) {
			detail = &chaoxing.AppError{"INTERNAL_ERROR", err.Error(), 1}
		}
		code = detail.Exit
		if code == 3 {
			next = []string{"auth login"}
		} else if code == 2 || code == 5 {
			next = []string{"schema"}
		}
	}
	if text {
		if _, e := io.WriteString(stdout, textResult(data)); e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		if detail != nil {
			fmt.Fprintf(stderr, "错误 [%s]：%s\n", detail.Code, detail.Message)
			if len(next) > 0 {
				fmt.Fprintln(stderr, "可使用：chaoxing "+next[0])
			}
		}
		return code
	}
	response := struct {
		Schema int                `json:"schema_version"`
		OK     bool               `json:"ok"`
		Data   any                `json:"data"`
		Error  *chaoxing.AppError `json:"error"`
		Meta   map[string]any     `json:"meta"`
		Next   []string           `json:"next_actions"`
	}{1, err == nil, data, detail, map[string]any{"timezone": "Asia/Shanghai", "complete": err == nil}, next}
	if e := json.NewEncoder(stdout).Encode(response); e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	return code
}
