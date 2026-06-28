package binutil

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

const (
	testStrHello   = "hello"
	testStrChinese = "你好世界"
)

func TestReadNString(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		maxLength int
		want      string
		wantErr   bool
	}{
		{"正常字符串", append([]byte(testStrHello), 0), -1, testStrHello, false},
		{"空字符串", []byte{0}, -1, "", false},
		{"含特殊字符", append([]byte("test\x00"), 0), -1, "test", false}, // 嵌入的 null 字节会截断
		{"限制最大长度", append([]byte("abcdefghij"), 0), 5, "abcde", false},
		{"无null终止符", []byte("no-null"), -1, "", true}, // EOF前面有数据应报UnexpectedEOF
		{"空数据", []byte{}, -1, "", true},               // 空数据返回EOF错误
		{"空数据", []byte{}, -1, "", true},
		{"超长字符串", bytes.Repeat([]byte("a"), 1000), 10, strings.Repeat("a", 10), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			got, err := ReadNString(r, tt.maxLength)
			if tt.wantErr {
				if err == nil {
					t.Error("期望错误但未获取到")
				}
				return
			}
			if err != nil {
				t.Errorf("ReadNString() 错误: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ReadNString() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

func TestReadStringI32Size(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		maxLength int
		want      string
		wantErr   bool
	}{
		{"正常字符串", makeStringI32(testStrHello), -1, testStrHello, false},
		{"空字符串", makeStringI32(""), -1, "", false},
		{"含空格", makeStringI32("hello world"), -1, "hello world", false},
		{"限制最大长度", makeStringI32("abcdefghij"), 5, "abcde", false},
		{"零长度", []byte{0, 0, 0, 0}, -1, "", false},
		{"中文", makeStringI32(testStrChinese), -1, testStrChinese, false},
		{"极长字符串(16KB)", makeStringI32(strings.Repeat("x", 16384)), -1, strings.Repeat("x", 16384), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			got, err := ReadStringI32Size(r, tt.maxLength)
			if tt.wantErr {
				if err == nil {
					t.Error("期望错误但未获取到")
				}
				return
			}
			if err != nil {
				t.Errorf("ReadStringI32Size() 错误: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ReadStringI32Size() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

func TestReadStringI32Size_限制最大长度后流未损坏(t *testing.T) {
	// 构造: size=100, 100字节'A', 然后一个 int32=42
	var sizePrefix [4]byte
	binary.LittleEndian.PutUint32(sizePrefix[:], 100)
	payload := bytes.Repeat([]byte("A"), 100)
	var suffix [4]byte
	binary.LittleEndian.PutUint32(suffix[:], 42)

	data := make([]byte, 0, len(sizePrefix)+len(payload)+len(suffix))
	data = append(data, sizePrefix[:]...)
	data = append(data, payload...)
	data = append(data, suffix[:]...)

	r := bytes.NewReader(data)

	// 读取被限制长度的字符串
	s, err := ReadStringI32Size(r, 10)
	if err != nil {
		t.Fatalf("ReadStringI32Size() 错误: %v", err)
	}
	if s != strings.Repeat("A", 10) {
		t.Errorf("ReadStringI32Size() = %q, 期望 10个A", s)
	}

	// 验证流中剩余数据正确：应读到一个 int32=42
	var v int32
	err = binary.Read(r, binary.LittleEndian, &v)
	if err != nil {
		t.Fatalf("后续读取 int32 失败（流被损坏）: %v", err)
	}
	if v != 42 {
		t.Errorf("后续读取 int32 = %d, 期望 42（流被损坏）", v)
	}
}

func TestWriteThenReadNString(t *testing.T) {
	tests := []string{
		testStrHello,
		"",
		"hello world",
		testStrChinese,
		strings.Repeat("x", 10000),
	}

	for _, s := range tests {
		t.Run(s[:minStrLen(s, 20)], func(t *testing.T) {
			var buf bytes.Buffer
			err := WriteNString(&buf, s)
			if err != nil {
				t.Fatalf("WriteNString() 错误: %v", err)
			}
			got, err := ReadNString(&buf, -1)
			if err != nil {
				t.Fatalf("ReadNString() 错误: %v", err)
			}
			if got != s {
				t.Errorf("往返值不匹配: %q != %q", got, s)
			}
		})
	}
}

func TestWriteThenReadStringI32Size(t *testing.T) {
	tests := []string{
		testStrHello,
		"",
		testStrChinese,
		strings.Repeat("x", 10000),
	}

	for _, s := range tests {
		var buf bytes.Buffer
		err := WriteStringI32Size(&buf, s)
		if err != nil {
			t.Fatalf("WriteStringI32Size() 错误: %v", err)
		}
		got, err := ReadStringI32Size(&buf, -1)
		if err != nil {
			t.Fatalf("ReadStringI32Size() 错误: %v", err)
		}
		if got != s {
			t.Errorf("往返值不匹配: %q != %q", got, s)
		}
	}
}

func TestReadInt32_小端序(t *testing.T) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, 0x01020304)
	r := bytes.NewReader(buf)
	v, err := ReadInt32(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0x01020304 {
		t.Errorf("ReadInt32() = 0x%X, 期望 0x01020304", v)
	}
}

func TestReadUInt32_小端序(t *testing.T) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, 0xFFFFFFFF)
	r := bytes.NewReader(buf)
	v, err := ReadUInt32(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0xFFFFFFFF {
		t.Errorf("ReadUInt32() = 0x%X, 期望 0xFFFFFFFF", v)
	}
}

func TestReadFloat32_小端序(t *testing.T) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, 0x3F800000) // IEEE 754 float32 1.0
	r := bytes.NewReader(buf)
	v, err := ReadFloat32(r)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1.0 {
		t.Errorf("ReadFloat32() = %f, 期望 1.0", v)
	}
}

func TestReadStringI32Size_非法输入(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"负数长度", []byte{0xFF, 0xFF, 0xFF, 0xFF}},
		{"截断的长度前缀", []byte{0x01}},
		{"截断的字符串数据", []byte{0x05, 0x00, 0x00, 0x00, 0x41, 0x42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			_, err := ReadStringI32Size(r, -1)
			if err == nil {
				t.Error("期望错误但未获取到")
			}
		})
	}
}

// 辅助函数
func makeStringI32(s string) []byte {
	if len(s) > 1<<30 {
		panic("测试字符串过长")
	}
	b := make([]byte, 4+len(s))
	binary.LittleEndian.PutUint32(b, uint32(len(s))) //nolint:gosec // 测试辅助函数，s 长度受控
	copy(b[4:], s)
	return b
}

func minStrLen(s string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
