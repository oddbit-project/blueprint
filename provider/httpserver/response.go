package httpserver

type JSONResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data, omitempty"`
}

type JSONErrorDetail struct {
	Message   string      `json:"message,omitempty"`
	FormError interface{} `json:"formError,omitempty"`
}

type JSONResponseError struct {
	Success bool            `json:"success"`
	Error   JSONErrorDetail `json:"error"`
}
