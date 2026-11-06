package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
)

type host struct {
	mod *moejs.Module
	js  string
}

func must(e error) {
	if e != nil {
		fmt.Println("FATAL:", e)
		os.Exit(1)
	}
}

func (h *host) newRuntime(opts moejs.Options, globals map[string]any) *moejs.Runtime {
	rt := moejs.NewRuntime(opts)
	for k, v := range globals {
		must(rt.SetGlobal(k, v))
	}
	must(rt.Load(h.mod))
	return rt
}

func main() {
	src, err := os.ReadFile("main.js")
	must(err)
	manifest, err := os.ReadFile("extension.json")
	must(err)

	// ---- 1. cold start
	t0 := time.Now()
	mod, err := moejs.Compile("main.js", string(src))
	must(err)
	compileCost := time.Since(t0)

	echo := moejs.NativeFunc(func(r *engine.Realm, this engine.Value, args []engine.Value) (engine.Value, error) {
		if len(args) == 0 {
			return engine.Undefined(), nil
		}
		return args[0], nil
	})

	h := &host{mod: mod}
	t1 := time.Now()
	rt := h.newRuntime(moejs.Options{}, map[string]any{"remit": map[string]any{"echo": echo}})
	coldCost := time.Since(t1)

	describeHook, err := mod.Hook("describe")
	must(err)
	cardsHook, err := mod.Hook("cards")
	must(err)
	eventHook, err := mod.Hook("on_event")
	must(err)

	res, err := rt.Call(describeHook)
	must(err)
	out, err := rt.AppendJSON(nil, res)
	must(err)
	fmt.Printf("1. cold start: compile=%v runtime+load=%v\n   describe()=%s\n   manifest=%s\n", compileCost, coldCost, out, string(manifest[:40]))

	// ---- 2. per-call cost
	body, _ := json.Marshal(map[string]any{
		"id":    "^b1",
		"text":  "What is the Bohr effect?::Hb binds O2 more weakly at low pH, shifting the curve right.",
		"title": "Biology",
	})
	blockArg, err := rt.ParseJSON(body)
	must(err)
	const n = 10000
	t2 := time.Now()
	var last []byte
	for i := 0; i < n; i++ {
		res, err := rt.Call(cardsHook, blockArg)
		must(err)
		last, err = rt.AppendJSON(last[:0], res)
		must(err)
	}
	perCall := time.Since(t2) / n
	fmt.Printf("2. per call (cards, %d B in): %v  → %s\n", len(body), perCall, last)

	// ---- 3. interrupt an endless loop
	spinHook, err := mod.Hook("spin")
	must(err)
	spinRT := h.newRuntime(moejs.Options{}, map[string]any{"remit": map[string]any{"echo": echo}})
	var interrupted bool
	t3 := time.Now()
	done := make(chan error, 1)
	go func() { done <- spinAndReturn(spinRT, spinHook) }()
	time.Sleep(50 * time.Millisecond)
	stopAt := time.Now()
	spinRT.Interrupt(errors.New("1s budget spent"))
	err = <-done
	var ie *moejs.InterruptedError
	interrupted = errors.As(err, &ie)
	fmt.Printf("3. endless loop: interrupt→return=%v (total %v), InterruptedError=%v\n", time.Since(stopAt), time.Since(t3), interrupted)

	// ---- 4. memory limit
	hungryHook, err := mod.Hook("hungry")
	must(err)
	memRT := h.newRuntime(moejs.Options{MemoryLimit: 8 << 20}, nil)
	err = func() (e error) { _, e = memRT.Call(hungryHook); return }()
	var mle *moejs.MemoryLimitError
	_ = mle
	fmt.Printf("4. memory limit 8MiB: ErrMemoryLimit=%v err=%v\n", errors.Is(err, moejs.ErrMemoryLimit), err)

	// ---- 5. panic inside a Go host function
	boom := moejs.NativeFunc(func(r *engine.Realm, this engine.Value, args []engine.Value) (engine.Value, error) {
		panic("host bug")
	})
	panicRT := h.newRuntime(moejs.Options{}, map[string]any{"remit": map[string]any{"echo": boom}})
	ev, err := panicRT.ParseJSON([]byte(`{"name":"note.saved"}`))
	must(err)
	_, err = panicRT.Call(eventHook, ev)
	var internal *moejs.InternalError
	fmt.Printf("5. host panic: survived=%v InternalError=%v err=%v\n", true, errors.As(err, &internal), err)

	// ---- 6. dynamic code off
	dynHook, err := mod.Hook("dynamic")
	must(err)
	dynRT := h.newRuntime(moejs.Options{DisableDynamicCode: true}, nil)
	_, err = dynRT.Call(dynHook)
	fmt.Printf("6. DisableDynamicCode: eval() error=%v\n", err != nil)

	// ---- 7. pool sizing: cost of one more instance, and memory each
	const pool = 50
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	t7 := time.Now()
	rts := make([]*moejs.Runtime, pool)
	for i := range rts {
		rts[i] = h.newRuntime(moejs.Options{}, map[string]any{"remit": map[string]any{"echo": echo}})
	}
	perInstance := time.Since(t7) / pool
	runtime.GC()
	runtime.ReadMemStats(&after)
	fmt.Printf("7. new runtime+load = %v each; heap before=%d KiB after=%d KiB for %d loaded runtimes\n", perInstance, before.HeapAlloc/1024, after.HeapAlloc/1024, pool)
	_ = rts

	// ---- 8. cancellation through the documented pattern
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	ctxRT := h.newRuntime(moejs.Options{}, nil)
	go func() {
		<-ctx.Done()
		ctxRT.Interrupt(context.Cause(ctx))
	}()
	_, err = ctxRT.Call(spinHook)
	fmt.Printf("8. context deadline → %v\n", err)
}
