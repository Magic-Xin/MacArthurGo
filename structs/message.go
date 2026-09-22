package structs

import (
	"MacArthurGo/structs/cqcode"
	"bytes"
	"encoding/json"
	"fmt"
)

type MessageStruct struct {
	Time        int64  `json:"time"`
	PostType    string `json:"post_type"`
	MessageType string `json:"message_type"`
	MessageId   int64  `json:"message_id"`
	GroupId     int64  `json:"group_id"`
	UserId      int64  `json:"user_id"`
	Sender      struct {
		UserId   int64  `json:"user_id"`
		Nickname string `json:"nickname"`
	} `json:"sender"`
	Message    []cqcode.ArrayMessage `json:"message"`
	RawMessage string                `json:"raw_message"`
	Echo       string                `json:"echo"`

	Command      string
	CleanMessage []cqcode.ArrayMessage
}

func (m *MessageStruct) UnmarshalJSON(data []byte) error {
	type Alias MessageStruct
	aux := &struct {
		Message json.RawMessage `json:"message"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.Message) == 0 || bytes.Equal(aux.Message, []byte("null")) {
		m.Message = nil
		return nil
	}

	var s string
	if err := json.Unmarshal(aux.Message, &s); err == nil {
		m.Message = []cqcode.ArrayMessage{
			{
				Type: "text",
				Data: map[string]interface{}{"text": s},
			},
		}
		return nil
	}

	var arr []cqcode.ArrayMessage
	if err := json.Unmarshal(aux.Message, &arr); err == nil {
		m.Message = arr
		return nil
	}

	return fmt.Errorf("unsupported message format: %s", string(aux.Message))
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
		Message []cqcode.ArrayMessage `json:"message"`

		// NapCat stream API
		Type           string `json:"type"`
		Status         string `json:"status"`
		StreamID       string `json:"stream_id"`
		ReceivedChunks int    `json:"received_chunks"`
		TotalChunks    int    `json:"total_chunks"`
		FilePath       string `json:"file_path"`
		FileSize       int64  `json:"file_size"`
		SHA256         string `json:"sha256"`
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
	Echo    string `json:"echo"`
	Status  string `json:"status"`
	Retcode int    `json:"retcode"`
	Message string `json:"message"`
	Wording string `json:"wording"`
	Stream  string `json:"stream"`
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
	MessageType string                `json:"message_type"`
	UserId      int64                 `json:"user_id"`
	GroupId     int64                 `json:"group_id"`
	Message     []cqcode.ArrayMessage `json:"message"`
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
		Uin     string                `json:"uin"`
		Name    string                `json:"name"`
		Content []cqcode.ArrayMessage `json:"content"`
	} `json:"data"`
}

func NewForwardNode() *ForwardNode {
	return &ForwardNode{Type: "node"}
}
