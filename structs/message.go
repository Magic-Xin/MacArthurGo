package structs

type Message struct {
	MessageType string `json:"message_type"`
	UserId      int64  `json:"user_id"`
	GroupId     int64  `json:"group_id"`
	Message     string `json:"message"`
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
		Name    string `json:"name"`
		Uin     int64  `json:"uin"`
		Content string `json:"content"`
	} `json:"data"`
}

func NewForwardNode(name string, uin int64) *ForwardNode {
	node := ForwardNode{Type: "node"}
	node.Data.Name, node.Data.Uin = name, uin
	return &node
}
