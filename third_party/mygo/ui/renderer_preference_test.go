package ui

import (
	"errors"
	"testing"
)

func TestRuntimeRendererPreferencePreservesRuntime(t *testing.T) {
	var renderers []*testGPU
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) { g := &testGPU{}; renderers = append(renderers, g); return g, nil })
	frame()
	rt := h.rt
	h.rt.public.UseSoftwareRenderer(true)
	frame()
	frame()
	if h.rt != rt || len(renderers) != 1 || renderers[0].released != 1 || s.pixels != 2 {
		t.Fatal("software transition recreated runtime or retried GPU")
	}
	h.rt.public.UseSoftwareRenderer(true)
	frame()
	if renderers[0].released != 1 {
		t.Fatal("repeat preference released twice")
	}
	h.rt.public.UseSoftwareRenderer(false)
	frame()
	if h.rt != rt || len(renderers) != 2 || renderers[1].frames != 1 {
		t.Fatal("GPU transition failed")
	}
	h.detach()
	if renderers[1].released != 1 {
		t.Fatal("renderer not released")
	}
}
func TestRuntimeRendererPreferenceFallsBackAfterEnableFailure(t *testing.T) {
	attempts := 0
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) { attempts++; return nil, errors.New("fixture unavailable") })
	h.rt.public.UseSoftwareRenderer(true)
	frame()
	if attempts != 0 || s.pixels != 1 {
		t.Fatal("disabled renderer attempted GPU")
	}
	h.rt.public.UseSoftwareRenderer(false)
	frame()
	if attempts != 1 || s.pixels != 2 {
		t.Fatal("failed GPU did not fall back")
	}
	h.detach()
}
