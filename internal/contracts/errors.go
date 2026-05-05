package contracts

type GenericResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
}

func SuccessResponse() GenericResponse {
	return GenericResponse{Success: true, Error: nil}
}

func ErrorResponse(message string) GenericResponse {
	return GenericResponse{Success: false, Error: &message}
}
