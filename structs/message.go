package structs

//type ArrayMessage struct {
//	Type string         `json:"type"`
//	Data map[string]any `json:"data"`
//}

type IncomingMessageStruct struct {
	PeerID       int64            `json:"peer_id"`
	MessageSeq   int64            `json:"message_seq"`
	SenderID     int64            `json:"sender_id"`
	Time         int64            `json:"time"`
	Segments     []MessageSegment `json:"segments"`
	MessageScene string           `json:"message_scene"`

	Friend struct {
		UserID   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
	} `json:"friend"`

	Group struct {
		GroupID int64  `json:"group_id"`
		Name    string `json:"name"`
	} `json:"group"`

	GroupMember struct {
		UserID   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
		Card     string `json:"card"`
		Role     string `json:"role"`
	} `json:"group_member"`

	Command      string
	CleanMessage *[]MessageSegment
}

type MessageSegment struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type OutgoingMessage struct {
	UserId  int64            `json:"user_id"`
	GroupId int64            `json:"group_id"`
	Message []MessageSegment `json:"message"`
}

type OutgoingForwardedMessage struct {
	UserID   int64            `json:"user_id"`
	Name     string           `json:"name"`
	Segments []MessageSegment `json:"segments"`
}

type EventStruct struct {
	Time      int64  `json:"time"`
	SelfID    int64  `json:"self_id"`
	EventType string `json:"event_type"`
	Data      any    `json:"data"`
}

type FeedbackStruct struct {
	Status  string `json:"status"`
	RetCode int    `json:"retcode"`
	Data    struct {
		MessageSeq int64 `json:"message_seq"`
		Time       int64 `json:"time"`

		// For get_login_info
		Uin      int64  `json:"uin"`
		Nickname string `json:"nickname"`

		// For get_message
		Message IncomingMessageStruct `json:"message"`

		// For get_friend_list
		Friends []struct {
			UserId   int64  `json:"user_id"`
			Nickname string `json:"nickname"`
			Remark   string `json:"remark"`
		} `json:"friends"`

		// For get_group_list
		Groups []struct {
			GroupId        int64  `json:"group_id"`
			Name           string `json:"name"`
			MemberCount    int    `json:"member_count"`
			MaxMemberCount int    `json:"max_member_count"`
		} `json:"groups"`

		// For get_group_member_list
		Members []struct {
			UserId   int64  `json:"user_id"`
			GroupId  int64  `json:"group_id"`
			Nickname string `json:"nickname"`
			Card     string `json:"card"`
			Role     string `json:"role"`
		} `json:"members"`
	} `json:"data"`
	Message string `json:"message"`
}
type GroupNudge struct {
	GroupId int64 `json:"group_id"`
	UserId  int64 `json:"user_id"`
}

//
//type EchoMessageStruct struct {
//	Data struct {
//		// Info only
//		Nickname string `json:"nickname"`
//		UserId   int64  `json:"user_id"`
//
//		// originPic only
//		File string `json:"file"`
//
//		Time        int64  `json:"time"`
//		MessageType string `json:"message_type"`
//		MessageId   int64  `json:"message_id"`
//		Sender      struct {
//			UserId int64 `json:"user_id"`
//		}
//		Message []ArrayMessage `json:"message"`
//	} `json:"data"`
//	DataArray []struct {
//		//friendList
//		UserId   int64  `json:"user_id"`
//		Nickname string `json:"nickname"`
//		Remark   string `json:"remark"`
//
//		//groupList
//		GroupId        int64  `json:"group_id"`
//		GroupName      string `json:"group_name"`
//		MemberCount    int    `json:"member_count"`
//		MaxMemberCount int    `json:"max_member_count"`
//
//		//groupMemberList
//		Card string `json:"card"`
//	}
//	Echo   string `json:"echo"`
//	Status string `json:"status"`
//}
//
//type EchoMessageArrayStruct struct {
//	Data []struct {
//		//friendList only
//		UserId   int64  `json:"user_id"`
//		Nickname string `json:"nickname"`
//		Remark   string `json:"remark"`
//
//		//groupList only
//		GroupId        int64  `json:"group_id"`
//		GroupName      string `json:"group_name"`
//		MemberCount    int    `json:"member_count"`
//		MaxMemberCount int    `json:"max_member_count"`
//
//		//groupMemberList
//		Card string `json:"card"`
//	}
//	Echo   string `json:"echo"`
//	Status string `json:"status"`
//}
//
//type PrivateFile struct {
//	UserId int64  `json:"user_id"`
//	File   string `json:"file"`
//	Name   string `json:"name"`
//}
//
//type GroupFile struct {
//	GroupId int64  `json:"group_id"`
//	File    string `json:"file"`
//	Name    string `json:"name"`
//}
//
//type Message struct {
//	MessageType string         `json:"message_type"`
//	UserId      int64          `json:"user_id"`
//	GroupId     int64          `json:"group_id"`
//	Message     []ArrayMessage `json:"message"`
//}
//
//type PrivateMessage struct {
//	UserId  int64          `json:"user_id"`
//	Message []ArrayMessage `json:"message"`
//}
//
//type GroupMessage struct {
//	GroupId int64          `json:"group_id"`
//	Message []ArrayMessage `json:"message"`
//}
//
//type GroupForward struct {
//	GroupId  int64         `json:"group_id"`
//	Messages []ForwardNode `json:"messages"`
//}
//
//type PrivateForward struct {
//	UserId   int64         `json:"user_id"`
//	Messages []ForwardNode `json:"messages"`
//}
//
//type ForwardNode struct {
//	Type string `json:"type"`
//	Data struct {
//		Uin     string         `json:"uin"`
//		Name    string         `json:"name"`
//		Content []ArrayMessage `json:"content"`
//	} `json:"data"`
//}
//
//func NewForwardNode() *ForwardNode {
//	return &ForwardNode{Type: "node"}
//}
