package core

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type LaunchResponse struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

func writeLaunchMessage(w io.Writer, value any) error {
	data, e := json.Marshal(value)
	if e != nil {
		return e
	}
	defer clear(data)
	if len(data) == 0 || len(data) > 16384 {
		return fmt.Errorf("启动消息过长")
	}
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	packet := append(header, data...)
	defer clear(packet)
	for len(packet) > 0 {
		n, e := w.Write(packet)
		if e != nil {
			return e
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		packet = packet[n:]
	}
	return nil
}
func readLaunchMessage(r io.Reader, value any) error {
	var header [4]byte
	if _, e := io.ReadFull(r, header[:]); e != nil {
		return e
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > 16384 {
		return fmt.Errorf("启动消息长度无效")
	}
	data := make([]byte, size)
	defer clear(data)
	if _, e := io.ReadFull(r, data); e != nil {
		return e
	}
	return json.Unmarshal(data, value)
}
func (r *ExternalLaunchRequest) UnmarshalJSON(data []byte) error {
	type plain ExternalLaunchRequest
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(data, &fields); e != nil {
		return e
	}
	_, terminal := fields["terminal"]
	_, file := fields["file"]
	if terminal || file {
		var value plain
		if e := json.Unmarshal(data, &value); e != nil {
			return e
		}
		*r = ExternalLaunchRequest(value)
	} else {
		var value TerminalLaunchRequest
		if e := json.Unmarshal(data, &value); e != nil {
			return e
		}
		r.Terminal = &value
		r.File = nil
	}
	return r.Validate()
}
func (r *FileLaunchRequest) UnmarshalJSON(data []byte) error {
	type plain FileLaunchRequest
	var raw struct{ Profile map[string]json.RawMessage }
	if e := json.Unmarshal(data, &raw); e != nil {
		return e
	}
	if encrypted, ok := raw.Profile["encryptedPassword"]; ok && string(encrypted) != "null" {
		return fmt.Errorf("启动入口不接受保存的加密凭据")
	}
	var value plain
	if e := json.Unmarshal(data, &value); e != nil {
		return e
	}
	*r = FileLaunchRequest(value)
	return r.Validate()
}
func (r TerminalLaunchRequest) GoString() string { return r.String() }
func (r FileLaunchRequest) GoString() string     { return r.String() }
func (r ExternalLaunchRequest) GoString() string { return r.String() }
