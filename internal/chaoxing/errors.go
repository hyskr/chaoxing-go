package chaoxing

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Exit    int    `json:"-"`
}

func (e *AppError) Error() string                  { return e.Message }
func Failure(code, message string, exit int) error { return &AppError{code, message, exit} }
