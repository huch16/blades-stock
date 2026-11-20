package models

import (
	"testing"
)

// TestFloat32ConversionsEmptyInput 固化 Float32 转换的空输入语义（R-10）。
//
// 统一后的语义以本包实现为准：空输入（nil 或空切片）返回长度为 0 的
// 非 nil 切片。调用方核查：agent 的唯一生产调用点
// （semantic_distiller.go:447）有 len(vec) > 0 守卫，永不传入空输入；
// GetVector 的调用方（semantic_distiller.go:616）只用 len(vec) == 0
// 判断，nil 与空切片无差别。因此 nil 语义不被任何调用方依赖。
func TestBytesToFloat32sLEEmptyInput(t *testing.T) {
	for _, in := range [][]byte{nil, {}} {
		got := BytesToFloat32sLE(in)
		if got == nil {
			t.Errorf("BytesToFloat32sLE(%v) = nil，统一语义要求空非 nil 切片", in)
		}
		if len(got) != 0 {
			t.Errorf("BytesToFloat32sLE(%v) 长度 = %d，要求 0", in, len(got))
		}
	}
	// 非 4 倍数长度仍返回 nil（非法输入守卫，不随空输入语义改变）
	if got := BytesToFloat32sLE([]byte{1, 2, 3}); got != nil {
		t.Errorf("BytesToFloat32sLE(3 bytes) = %v，要求 nil", got)
	}
}

func TestFloat32sToBytesLEEmptyInput(t *testing.T) {
	for _, in := range [][]float32{nil, {}} {
		got := Float32sToBytesLE(in)
		if got == nil {
			t.Errorf("Float32sToBytesLE(%v) = nil，统一语义要求空非 nil 切片", in)
		}
		if len(got) != 0 {
			t.Errorf("Float32sToBytesLE(%v) 长度 = %d，要求 0", in, len(got))
		}
	}
}

func TestFloat32ConversionsRoundTrip(t *testing.T) {
	orig := []float32{1.5, -2.25, 0, 3.14159}
	packed := Float32sToBytesLE(orig)
	if len(packed) != 4*len(orig) {
		t.Fatalf("打包后长度 = %d，要求 %d", len(packed), 4*len(orig))
	}
	unpacked := BytesToFloat32sLE(packed)
	if len(unpacked) != len(orig) {
		t.Fatalf("解包后长度 = %d，要求 %d", len(unpacked), len(orig))
	}
	for i, v := range orig {
		if unpacked[i] != v {
			t.Errorf("往返第 %d 个元素 = %v，要求 %v", i, unpacked[i], v)
		}
	}
}
