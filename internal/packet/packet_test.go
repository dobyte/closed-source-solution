package packet_test

import (
	"bytes"
	"testing"

	"github.com/dobyte/closed-source-solution/internal/packet"
)

func TestWriteCompileFile(t *testing.T) {
	buf := &bytes.Buffer{}
	data := make([]byte, 32666624)

	packet.WriteCompileFile(buf, data, func(i uint8, f float64) {
		t.Logf("index: %d, progress: %f", i, f)
	})
}
