package structs

type Action struct {
	Action string `json:"action"`
	Params any    `json:"params"`
	Echo   string `json:"echo"`
}

type GetMsg struct {
	Id int64 `json:"message_id"`
}
