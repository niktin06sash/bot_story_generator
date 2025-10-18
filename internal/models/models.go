package models

type Message struct {
	Command string
	Arguments []string
	ChatID  int64
}

type OutboundMessage struct {
    ChatID int64
    Text   string
}
