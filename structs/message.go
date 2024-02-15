package structs

import "MacArthurGo/structs/cqcode"

type PrivateFile struct {
	UserId int64  `json:"user_id"`
	File   string `json:"file"`
	Name   string `json:"name"`
}

type GroupFile struct {
	GroupId int64  `json:"group_id"`
	File    string `json:"file"`
	Name    string `json:"name"`
}

type PrivateMessage struct {
	UserId  int64                 `json:"user_id"`
	Message []cqcode.ArrayMessage `json:"message"`
}

type GroupMessage struct {
	GroupId int64                 `json:"group_id"`
	Message []cqcode.ArrayMessage `json:"message"`
}

type GroupForward struct {
	GroupId  int64         `json:"group_id"`
	Messages []ForwardNode `json:"messages"`
}

type PrivateForward struct {
	UserId   int64         `json:"user_id"`
	Messages []ForwardNode `json:"messages"`
}

type ForwardNode struct {
	Type string `json:"type"`
	Data struct {
		Content []cqcode.ArrayMessage `json:"content"`
	} `json:"data"`
}

func NewForwardNode() *ForwardNode {
	return &ForwardNode{Type: "node"}
}
