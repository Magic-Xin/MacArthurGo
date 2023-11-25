package _struct

type Action struct {
	Action string `json:"action"`
	Params any    `json:"params"`
}

type EchoAction struct {
	Action string `json:"action"`
	Params any    `json:"params"`
	Echo   string `json:"echo"`
}

type GetMsg struct {
	Id int `json:"message_id"`
}
