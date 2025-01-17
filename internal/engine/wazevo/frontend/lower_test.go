package frontend

import (
	"testing"
	"unsafe"

	"github.com/tetratelabs/wazero/internal/testing/require"
	"github.com/tetratelabs/wazero/internal/wasm"
)

func TestGlobalInstanceValueOffset(t *testing.T) {
	var globalInstance wasm.GlobalInstance
	require.Equal(t, int(unsafe.Offsetof(globalInstance.Val)), globalInstanceValueOffset)
	var memInstance wasm.MemoryInstance
	require.Equal(t, int(unsafe.Offsetof(memInstance.Buffer)), memoryInstanceBufOffset)
}
