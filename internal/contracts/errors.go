package contracts

type GenericResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
}

type SimpleSuccessResponse struct {
	Success bool `json:"success"`
}

func SuccessResponse() GenericResponse {
	return GenericResponse{Success: true, Error: nil}
}

func SimpleSuccess() SimpleSuccessResponse {
	return SimpleSuccessResponse{Success: true}
}

func ErrorResponse(message string) GenericResponse {
	return GenericResponse{Success: false, Error: &message}
}
