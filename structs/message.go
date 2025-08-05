package structs

type ArrayMessage struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type MessageStruct struct {
	Time        int64  `json:"time"`
	MessageType string `json:"message_type"`
	MessageId   int64  `json:"message_id"`
	GroupId     int64  `json:"group_id"`
	UserId      int64  `json:"user_id"`
	Sender      struct {
		UserId   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
	} `json:"sender"`
	Message    []ArrayMessage `json:"message"`
	RawMessage string         `json:"raw_message"`
	Echo       string         `json:"echo"`

	Command      string
	CleanMessage *[]ArrayMessage
}

type EchoMessageStruct struct {
	Data struct {
		// Info only
		Nickname string `json:"nickname"`
		UserId   int64  `json:"user_id"`

		// originPic only
		File string `json:"file"`

		Time        int64  `json:"time"`
		MessageType string `json:"message_type"`
		MessageId   int64  `json:"message_id"`
		Sender      struct {
			UserId int64 `json:"user_id"`
		}
		Message []ArrayMessage `json:"message"`
	} `json:"data"`
	DataArray []struct {
		//friendList
		UserId   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
		Remark   string `json:"remark"`

		//groupList
		GroupId        int64  `json:"group_id"`
		GroupName      string `json:"group_name"`
		MemberCount    int    `json:"member_count"`
		MaxMemberCount int    `json:"max_member_count"`

		//groupMemberList
		Card string `json:"card"`
	}
	Echo   string `json:"echo"`
	Status string `json:"status"`
}

type EchoMessageArrayStruct struct {
	Data []struct {
		//friendList only
		UserId   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
		Remark   string `json:"remark"`

		//groupList only
		GroupId        int64  `json:"group_id"`
		GroupName      string `json:"group_name"`
		MemberCount    int    `json:"member_count"`
		MaxMemberCount int    `json:"max_member_count"`

		//groupMemberList
		Card string `json:"card"`
	}
	Echo   string `json:"echo"`
	Status string `json:"status"`
}

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

type Message struct {
	MessageType string         `json:"message_type"`
	UserId      int64          `json:"user_id"`
	GroupId     int64          `json:"group_id"`
	Message     []ArrayMessage `json:"message"`
}

type PrivateMessage struct {
	UserId  int64          `json:"user_id"`
	Message []ArrayMessage `json:"message"`
}

type GroupMessage struct {
	GroupId int64          `json:"group_id"`
	Message []ArrayMessage `json:"message"`
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
		Uin     string         `json:"uin"`
		Name    string         `json:"name"`
		Content []ArrayMessage `json:"content"`
	} `json:"data"`
}

func NewForwardNode() *ForwardNode {
	return &ForwardNode{Type: "node"}
}
