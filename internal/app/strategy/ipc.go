package strategy

import "time"

// IpcMessage IPC 消息
type IpcMessage struct {
	Version   string                 `json:"version"`
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

const IpcMessageVersion = "1.0"

const (
	IpcMessageTypeInit        = "init"
	IpcMessageTypeTick        = "tick"
	IpcMessageTypeKline       = "kline"
	IpcMessageTypeHistory     = "history"
	IpcMessageTypeStop        = "stop"
	IpcMessageTypeRPCResponse = "rpc_response"
)

// NewIpcMessage 构造出站 IPC 消息（供 pythonipc 适配器使用）。
func NewIpcMessage(msgType string, data map[string]interface{}) *IpcMessage {
	return &IpcMessage{
		Version:   IpcMessageVersion,
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
}
