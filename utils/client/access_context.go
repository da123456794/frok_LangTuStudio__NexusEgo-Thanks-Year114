package client

type AccessContext interface {
	TransferData(content string) (string, error)
	TransferCheckNum(data string) (string, error)
	GetBotName() string
}
