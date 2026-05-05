package contracts

type UserCommandRequest struct {
	Username string         `json:"username,omitempty"`
	Inbound  string         `json:"inbound,omitempty"`
	Raw      map[string]any `json:"-"`
}

type InboundUsersResponse struct {
	Users []InboundUser `json:"users"`
}

type InboundUsersCountResponse struct {
	Count int `json:"count"`
}

type InboundUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Level    int    `json:"level"`
}
