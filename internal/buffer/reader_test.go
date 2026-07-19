package buffer

import (
	"errors"
	"io"
	"testing"
	"unicode/utf8"
)

func TestRuneOffset(t *testing.T) {
	src := NewTextSource()
	src.Replace(0, 0, "hello,world.")
	if src.RuneOffset(12) != 12 {
		t.Fail()
	}

	src.Replace(12, 12, "你好，世界")

	_, size := utf8.DecodeRuneInString("你好，世界")
	t.Log(size)
	if src.RuneOffset(13) != 15 {
		t.Fail()
	}
}

func TestReadAt(t *testing.T) {
	src := NewTextSource()
	reader := NewReader(src)
	src.Replace(0, 0, "hello,world.")

	if src.Len() != 12 {
		t.Fail()
	}

	buf := make([]byte, 5)
	n, err := src.ReadAt(buf, 0)
	if err != nil {
		t.Fail()
	}

	if n != 5 || string(buf) != "hello" {
		t.Fail()
	}

	content := reader.ReadAll(buf)
	if string(content) != "hello,world." {
		t.Fail()
	}
}

func TestReadRuneAt(t *testing.T) {
	src := NewTextSource()
	src.SetText([]byte("hello,world.你好，世界"))

	r, err := src.ReadRuneAt(6)
	if err != nil {
		t.Logf("ReadRuneAt error: %v", err)
		t.Fail()
	}
	if r != 'w' {
		t.Fail()
	}

	r, err = src.ReadRuneAt(12)
	if err != nil {
		t.Logf("ReadRuneAt error: %v", err)
		t.Fail()
	}
	if r != '你' {
		t.Fail()
	}

	for _, off := range []int{-1, src.Len()} {
		if _, err := src.ReadRuneAt(off); !errors.Is(err, io.EOF) {
			t.Errorf("ReadRuneAt(%d) error = %v, want io.EOF", off, err)
		}
	}
}
