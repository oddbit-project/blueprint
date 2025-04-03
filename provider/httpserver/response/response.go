package response

type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorDetail struct {
	Message      string      `json:"message,omitempty"`
	RequestError interface{} `json:"requestError,omitempty"`
}

type JSONResponseError struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
}
