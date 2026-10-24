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
	data, err := execute(ctx, args, stdin, stderr)
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
