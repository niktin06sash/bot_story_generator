package errors

type CustomError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (c *CustomError) Error() string {
	return c.Message
}
