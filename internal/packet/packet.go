package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
)

const (
	defaultSizeBytes   = 4
	defaultHeaderBytes = 1
	defaultSharedSize  = 2 * 1024 * 1024
)

const (
	CompileInfoHeader = 0
)

type CompileCommand struct {
	CGO    bool   `json:"cgo"`    // 是否开启CGO
	GOOS   string `json:"goos"`   // 目标操作系统
	GOARCH string `json:"goarch"` // 目标架构
	Output string `json:"output"` // 输出文件名
	Shell  string `json:"shell"`  // 编译前执行的shell脚本
	Size   int64  `json:"size"`   // 压缩文件大小
}

type ComplieResponse struct {
	Exec string `json:"exec"` // 可执行文件名称
	Size int64  `json:"size"` // 可执行文件大小
	Err  string `json:"err"`  // 错误信息
}

// 读取消息
func ReadMessage(reader io.Reader) (int, []byte, error) {
	buf := make([]byte, defaultSizeBytes)

	if _, err := io.ReadFull(reader, buf); err != nil {
		return 0, nil, err
	}

	size := binary.BigEndian.Uint32(buf)

	if size == 0 {
		return 0, nil, nil
	}

	data := make([]byte, size)

	if _, err := io.ReadFull(reader, data); err != nil {
		return 0, nil, err
	}

	return int(uint8(data[0])), data[defaultHeaderBytes:], nil
}

// 写入编译命令
func WriteCompileCommand(writer io.Writer, cmd *CompileCommand) error {
	v, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	buf.Grow(defaultSizeBytes + defaultHeaderBytes + len(v))

	if err = binary.Write(buf, binary.BigEndian, uint32(defaultHeaderBytes+len(v))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.BigEndian, uint8(CompileInfoHeader)); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.BigEndian, v); err != nil {
		return err
	}

	_, err = writer.Write(buf.Bytes())
	return err
}

// 解析编译命令
func ParseComplieCommand(data []byte) (*CompileCommand, error) {
	cmd := &CompileCommand{}

	if err := json.Unmarshal(data, cmd); err != nil {
		return nil, err
	}

	return cmd, nil
}

// 写入编译响应
func WriteComplieResponse(writer io.Writer, resp *ComplieResponse) error {
	v, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	buf.Grow(defaultSizeBytes + defaultHeaderBytes + len(v))

	if err = binary.Write(buf, binary.BigEndian, uint32(defaultHeaderBytes+len(v))); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.BigEndian, uint8(CompileInfoHeader)); err != nil {
		return err
	}

	if err = binary.Write(buf, binary.BigEndian, v); err != nil {
		return err
	}

	_, err = writer.Write(buf.Bytes())
	return err
}

// 解析编译响应
func ParseComplieResponse(data []byte) (*ComplieResponse, error) {
	resp := &ComplieResponse{}

	if err := json.Unmarshal(data, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// 写入编译错误响应
func WriteComplieError(writer io.Writer, err error) {
	if e := WriteComplieResponse(writer, &ComplieResponse{Err: err.Error()}); e != nil {
		log.Printf("发送响应失败: %s\n", e)
	}
}

// 分片写入编译文件
func WriteCompileFile(writer io.Writer, data []byte, callback ...func(uint8, float64)) error {
	for s, n := 0, len(data); s < n; s += defaultSharedSize {
		i := uint8(s/defaultSharedSize + 1)
		v := data[s:min(s+defaultSharedSize, n)]

		buf := &bytes.Buffer{}
		buf.Grow(defaultSizeBytes + defaultHeaderBytes)

		if err := binary.Write(buf, binary.BigEndian, uint32(defaultHeaderBytes+len(v))); err != nil {
			return err
		}

		if err := binary.Write(buf, binary.BigEndian, i); err != nil {
			return err
		}

		if _, err := writer.Write(buf.Bytes()); err != nil {
			return err
		}

		if _, err := writer.Write(v); err != nil {
			return err
		}

		if len(callback) > 0 && callback[0] != nil {
			callback[0](i, float64(s+len(v))/float64(n))
		}
	}

	return nil
}
