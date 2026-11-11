package cards_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/transmit-bug/remit/internal/cards"
	"github.com/transmit-bug/remit/internal/core"
	"github.com/transmit-bug/remit/internal/ext"
)

// testProducer is the card plugin a test derives with: the stamp its cards carry
// and the half of a cache key that says who answered.
var testProducer = core.Producer{ID: "test.cards", Version: "1.0.0"}

// countingDeriver is a card plugin with no engine behind it: the test watches how
// often it is asked and for which text. Prior art: #268/#269's fakes.
type countingDeriver struct {
	mu     sync.Mutex
	asked  []string
	answer func(text string) ([]ext.DerivedCard, error)
}

func (d *countingDeriver) DeriveBlock(in ext.DeriveIn) ([]ext.DerivedCard, error) {
	d.mu.Lock()
	d.asked = append(d.asked, in.Block.Text)
	answer := d.answer
	d.mu.Unlock()
	if answer != nil {
		return answer(in.Block.Text)
	}
	return nil, nil
}

// times is how often text was asked.
func (d *countingDeriver) times(text string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, got := range d.asked {
		if got == text {
			n++
		}
	}
	return n
}

// total is how often the plugin was asked at all.
func (d *countingDeriver) total() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.asked)
}

// One cache, two resolvers built for the same plugin and type table: the text is
// asked once. The regression this guards: the cache living in the resolver, so a
// write — or an agent listener that restarts — asks every paragraph on screen
// again (docs/specs/plugin-system.md P4).
func TestAResolverReusesTheAnswersInItsCache(t *testing.T) {
	d := &countingDeriver{}
	cache := cards.NewCache(16)
	cache.Resolver(d, testProducer, core.BuiltinTypes()).Derive("a :: b")

	again := cache.Resolver(d, testProducer, core.BuiltinTypes())
	if _, ok := again.Derive("a :: b"); !ok {
		t.Fatal("the plugin's answer did not survive the second resolver")
	}
	if got := d.times("a :: b"); got != 1 {
		t.Errorf("the plugin was asked %d times for one text, want once", got)
	}
}

// An answer belongs to the plugin that gave it and the type table it was asked
// with: editing a type's fields or switching the card plugin asks again rather
// than serving the old answer. The regression this guards: a key of the text
// alone — right only while every write threw the cache away.
func TestAnswersBelongToTheirPluginAndTypeTable(t *testing.T) {
	d := &countingDeriver{}
	cache := cards.NewCache(16)
	text := "a :: b"
	cache.Resolver(d, testProducer, core.BuiltinTypes()).Derive(text)

	edited := core.BuiltinTypes()
	edited[0].Fields = append(edited[0].Fields, "Extra")
	if _, ok := cache.Resolver(d, testProducer, edited).Derive(text); !ok {
		t.Fatal("the plugin answered nothing")
	}
	if got := d.times(text); got != 2 {
		t.Errorf("an edited type asked the plugin %d times, want 2: the old answer was served", got)
	}

	other := core.Producer{ID: "test.other", Version: "1.0.0"}
	cache.Resolver(d, other, core.BuiltinTypes()).Derive(text)
	if got := d.times(text); got != 3 {
		t.Errorf("a switched plugin asked %d times, want 3: the old answer was served", got)
	}
}

// A failure is an answer: the plugin that threw is asked once, not once a frame,
// and the failure belongs to that plugin — a switched plugin asks the new one
// rather than inheriting it. The regression this guards: a failure cached without
// its plugin, so a switched card plugin shows the old one's blank card for good.
func TestAFailureIsCachedAndBelongsToItsPlugin(t *testing.T) {
	d := &countingDeriver{answer: func(string) ([]ext.DerivedCard, error) {
		return nil, errors.New("the card plugin threw")
	}}
	cache := cards.NewCache(16)
	text := "a :: b"
	if _, ok := cache.Resolver(d, testProducer, core.BuiltinTypes()).Derive(text); ok {
		t.Fatal("a failing plugin answered ok")
	}
	cache.Resolver(d, testProducer, core.BuiltinTypes()).Derive(text)
	if got := d.times(text); got != 1 {
		t.Errorf("a failure was asked %d times, want one", got)
	}

	other := core.Producer{ID: "test.other", Version: "1.0.0"}
	if _, ok := cache.Resolver(d, other, core.BuiltinTypes()).Derive(text); ok {
		t.Fatal("the switched plugin was not asked")
	}
	if got := d.times(text); got != 2 {
		t.Errorf("a switched plugin asked %d times, want 2: the old failure was served", got)
	}
}

// The cache is bounded by texts per generation: filling it drops the generation
// before it and starts a new one, so the page on screen still answers without
// asking again. The regression this guards: an unbounded cache (a long session
// grows it without limit) and a clear-on-overflow, which would re-ask the screen.
func TestTheCacheIsBoundedAndKeepsTheGenerationOnScreen(t *testing.T) {
	const bound = 4
	text := func(i int) string { return fmt.Sprintf("question %d :: answer", i) }
	d := &countingDeriver{}
	cache := cards.NewCache(bound)
	r := cache.Resolver(d, testProducer, core.BuiltinTypes())

	for i := range bound {
		r.Derive(text(i))
	}
	if got := cache.Len(); got != bound {
		t.Fatalf("the cache holds %d answers after %d texts, want %d", got, bound, bound)
	}

	for i := bound; i < 2*bound; i++ {
		r.Derive(text(i))
	}
	if got := cache.Len(); got > 2*bound {
		t.Errorf("the cache holds %d answers, more than two generations of %d", got, bound)
	}
	// The generation before the overflow still answers, without the plugin.
	for i := range bound {
		if _, ok := r.Derive(text(i)); !ok {
			t.Fatalf("the previous generation lost text %d", i)
		}
	}
	if got := d.total(); got != 2*bound {
		t.Errorf("the previous generation asked the plugin again: %d asks, want %d", got, 2*bound)
	}

	// Filling the current generation drops the one before it, so the first texts
	// are asked once more — and the cache is still two generations big.
	for i := 2 * bound; i < 3*bound; i++ {
		r.Derive(text(i))
	}
	r.Derive(text(0))
	if got := d.times(text(0)); got != 2 {
		t.Errorf("the text before two overflows was asked %d times, want 2", got)
	}
	if got := cache.Len(); got > 2*bound {
		t.Errorf("the cache holds %d answers, more than two generations of %d", got, bound)
	}
}

// The agent API's source is one resolver reached by every HTTP handler at once,
// and the cache is now shared with every other resolver built for the same plugin
// and type table: they must be safe together and still ask once per text. The
// regression this guards: the unlocked map of #271, back through a second owner.
func TestResolversSharingACacheDeriveAtOnce(t *testing.T) {
	texts := []string{"a :: b", "c :: d", "e :: f", "g :: h"}

	// What one thread answers, over a cache of its own.
	reference := cards.NewCache(16).Resolver(&countingDeriver{}, testProducer, core.BuiltinTypes())
	want := map[string]bool{}
	for _, text := range texts {
		_, want[text] = reference.Derive(text)
	}

	d := &countingDeriver{}
	cache := cards.NewCache(16)
	shared := []*cards.Resolver{
		cache.Resolver(d, testProducer, core.BuiltinTypes()),
		cache.Resolver(d, testProducer, core.BuiltinTypes()),
	}
	const goroutines, rounds = 8, 25
	start := make(chan struct{})
	var wg sync.WaitGroup
	failures := make(chan string, goroutines*rounds)
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := range rounds {
				text := texts[(g+i)%len(texts)]
				_, ok := shared[g%len(shared)].Derive(text)
				if ok != want[text] {
					failures <- fmt.Sprintf("goroutine %d derived %q as ok %v, want %v", g, text, ok, want[text])
				}
			}
		}(g)
	}
	close(start)
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	for _, text := range texts {
		if got := d.times(text); got != 1 {
			t.Errorf("%q was asked %d times across one cache, want once", text, got)
		}
	}
}

// The window keeps its resolver across a reload only while it was built for what
// is on screen: the plugin and the whole type table, templates included. The
// regression this guards: a kept resolver rendering a type with the templates of
// the build before the edit.
func TestServesTellsWhatAResolverWasBuiltFor(t *testing.T) {
	r := cards.New(nil, testProducer, core.BuiltinTypes())
	if !r.Serves(testProducer, core.BuiltinTypes()) {
		t.Error("a resolver does not serve what it was built for")
	}
	if r.Serves(core.Producer{ID: "test.other", Version: "1.0.0"}, core.BuiltinTypes()) {
		t.Error("a resolver serves another plugin")
	}
	edited := core.BuiltinTypes()
	edited[0].Fields = append(edited[0].Fields, "Extra")
	if r.Serves(testProducer, edited) {
		t.Error("a resolver serves a type table whose fields changed")
	}
	reversed := core.BuiltinTypes()
	reversed[0].Front = "{{Nope"
	if r.Serves(testProducer, reversed) {
		t.Error("a resolver serves a type table whose template changed")
	}
	var none *cards.Resolver
	if none.Serves(testProducer, core.BuiltinTypes()) {
		t.Error("no resolver serves anything")
	}
}
