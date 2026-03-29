package protocol

import "fmt"

type MessageID string

type ThreadID string

type AgentName string

func NewMessageID(seq int64) MessageID {
	return MessageID(fmt.Sprintf("msg_%012d", seq))
}

func NewThreadID(seq int64) ThreadID {
	return ThreadID(fmt.Sprintf("thr_%012d", seq))
}
