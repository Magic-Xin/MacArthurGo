package _struct

type Message struct {
	MessageType string `json:"message_type"`
	UserId      int    `json:"user_id"`
	GroupId     int    `json:"group_id"`
	Message     string `json:"message"`
}
