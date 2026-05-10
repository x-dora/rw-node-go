package contracts

type UserCommandRequest struct {
	Username string         `json:"username,omitempty"`
	Inbound  string         `json:"inbound,omitempty"`
	Raw      map[string]any `json:"-"`
}

type AddUserRequest struct {
	Data     []UserInboundData `json:"data"`
	HashData UserHashData      `json:"hashData"`
}

type UserInboundData struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	UUID       string `json:"uuid,omitempty"`
	Flow       string `json:"flow,omitempty"`
	CipherType int    `json:"cipherType,omitempty"`
	IVCheck    bool   `json:"ivCheck,omitempty"`
}

type UserHashData struct {
	VlessUUID     string  `json:"vlessUuid"`
	PrevVlessUUID *string `json:"prevVlessUuid,omitempty"`
}

type AddUsersRequest struct {
	AffectedInboundTags []string          `json:"affectedInboundTags"`
	Users               []BulkUserRequest `json:"users"`
}

type BulkUserRequest struct {
	InboundData []BulkUserInboundData `json:"inboundData"`
	UserData    BulkUserData          `json:"userData"`
}

type BulkUserInboundData struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
	Flow string `json:"flow,omitempty"`
}

type BulkUserData struct {
	UserID         string `json:"userId"`
	HashUUID       string `json:"hashUuid"`
	VlessUUID      string `json:"vlessUuid"`
	TrojanPassword string `json:"trojanPassword"`
	SSPassword     string `json:"ssPassword"`
}

type RemoveUserRequest struct {
	Username string       `json:"username"`
	HashData UserHashData `json:"hashData"`
}

type RemoveUsersRequest struct {
	Users []RemoveUsersItem `json:"users"`
}

type RemoveUsersItem struct {
	UserID   string `json:"userId"`
	HashUUID string `json:"hashUuid"`
}

type InboundTagRequest struct {
	Tag string `json:"tag"`
}

type DropUsersConnectionsRequest struct {
	UserIDs []string `json:"userIds"`
}

type DropIPsRequest struct {
	IPs []string `json:"ips"`
}

type InboundUsersResponse struct {
	Users []InboundUser `json:"users"`
}

type InboundUsersCountResponse struct {
	Count int `json:"count"`
}

type InboundUser struct {
	Username string `json:"username"`
	Level    int    `json:"level"`
	Protocol string `json:"protocol"`
}
