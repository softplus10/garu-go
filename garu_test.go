package garu

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func testAnalyzer(t testing.TB) *Analyzer {
	t.Helper()
	a, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestSearchNounsMatchUpstreamFilter(t *testing.T) {
	a := testAnalyzer(t)
	for _, text := range []string{"도서관에서 책을 읽는다", "AI 문서와 BM25 검색", "도서관", "컴퓨터", "연필과 공책", "😀 한국어 문장", "한국\x00문장", "", "   "} {
		t.Run(text, func(t *testing.T) {
			r, err := a.Analyze(text)
			if err != nil {
				t.Fatal(err)
			}
			for _, include := range []bool{false, true} {
				want := []string{}
				for _, token := range r.Tokens {
					if token.POS == "NNG" || token.POS == "NNP" || (include && token.POS == "SL") {
						want = append(want, token.Text)
					}
				}
				got, err := a.Nouns(text, NounsOptions{IncludeSL: include})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("nouns=%v want %v", got, want)
				}
			}
			tokens, err := a.Tokenize(text)
			if err != nil {
				t.Fatal(err)
			}
			if len(tokens) != len(r.Tokens) {
				t.Fatalf("token count %d != %d", len(tokens), len(r.Tokens))
			}
		})
	}
	nouns, err := a.Nouns("AI 문서", NounsOptions{IncludeSL: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, noun := range nouns {
		if noun == "AI" {
			found = true
		}
	}
	if !found {
		t.Fatalf("SL token missing: %v", nouns)
	}
}

func TestLifecycleAndInvalidInput(t *testing.T) {
	if _, err := NewWithModel(nil); err == nil {
		t.Fatal("accepted empty model")
	}
	if _, err := NewWithModel([]byte("invalid model")); err == nil {
		t.Fatal("accepted invalid model")
	}
	a := testAnalyzer(t)
	if _, err := a.Analyze(string([]byte{0xff})); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
	if _, err := a.AnalyzeTopN("경제", 0); err == nil {
		t.Fatal("accepted n=0")
	}
	r, err := a.Analyze("")
	if err != nil || r.Tokens == nil || len(r.Tokens) != 0 {
		t.Fatalf("empty analysis: %+v %v", r, err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Analyze("경제"); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
	var zero Analyzer
	if _, err := zero.Nouns("경제"); !errors.Is(err, ErrClosed) {
		t.Fatalf("zero value: %v", err)
	}
}

func TestConcurrentClose(t *testing.T) {
	a := testAnalyzer(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				_, err := a.Analyze("도서관과 한국어 문장")
				if err != nil && !errors.Is(err, ErrClosed) {
					t.Errorf("analysis: %v", err)
				}
			}
		}()
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

func TestCustomModelOwnsInput(t *testing.T) {
	data, err := models.ReadFile("models/base.gmdl")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewWithModel(data, Options{NormalizeJamo: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	clear(data)
	results, err := a.AnalyzeTopN("한국어 문장을 분석한다", 3)
	if err != nil || len(results) == 0 {
		t.Fatalf("model bytes not owned: %v", err)
	}
}

func BenchmarkNouns(b *testing.B) {
	a := testAnalyzer(b)
	b.ResetTimer()
	for range b.N {
		if _, err := a.Nouns("도서관에서 새로운 책을 빌린다", NounsOptions{IncludeSL: true}); err != nil {
			b.Fatal(err)
		}
	}
}
