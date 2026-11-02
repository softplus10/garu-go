// Package garu wraps the upstream Garu Rust morphological analyzer using cgo.
// Build or install the matching native library before building a Go consumer.
package garu

/*
#cgo CFLAGS: -I${SRCDIR}/native
#cgo LDFLAGS: -L${SRCDIR}/native/target/release -lgaru_go
#cgo linux LDFLAGS: -lgcc_s -lutil -lrt -lpthread -lm -ldl
#cgo darwin LDFLAGS: -framework Security -framework CoreFoundation -liconv
#include "garu_go.h"
*/
import "C"

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unicode/utf8"
	"unsafe"
)

//go:embed models/base.gmdl
var models embed.FS

// ErrClosed is returned after Close or on an uninitialized Analyzer.
var ErrClosed = errors.New("garu: analyzer is closed")

// Options are passed to upstream AnalyzerOptions without changing the model.
type Options struct{ NormalizeJamo bool }

// NounsOptions matches garu-ko's nouns options. The default includes NNG and NNP only.
type NounsOptions struct{ IncludeSL bool }

// Token preserves upstream Sejong POS tags and Unicode code point offsets.
// Start and End are rune offsets, not Go byte offsets or JavaScript UTF-16 offsets.
type Token struct {
	Text  string   `json:"text"`
	POS   POS      `json:"pos"`
	Start int      `json:"start"`
	End   int      `json:"end"`
	Score *float32 `json:"score,omitempty"`
}

// Result is the upstream analysis result. Elapsed is in milliseconds.
type Result struct {
	Tokens  []Token `json:"tokens"`
	Score   float32 `json:"score"`
	Elapsed float64 `json:"elapsed"`
}

// Analyzer owns a native model. It is safe to share among goroutines, with calls
// serialized per instance. Do not copy an Analyzer; call Close to free native memory.
type Analyzer struct {
	mu  sync.Mutex
	ptr unsafe.Pointer
}

// New loads the unmodified model bundled with this upstream version.
func New(options ...Options) (*Analyzer, error) {
	data, err := models.ReadFile("models/base.gmdl")
	if err != nil {
		return nil, err
	}
	return NewWithModel(data, options...)
}

// NewWithModel loads caller-supplied model bytes. The native core owns the parsed
// data after this call, so the input slice can be released or reused.
func NewWithModel(model []byte, options ...Options) (*Analyzer, error) {
	if len(options) > 1 {
		return nil, errors.New("garu: at most one Options value is supported")
	}
	if len(model) == 0 {
		return nil, errors.New("garu: model is empty")
	}
	nativeVersion := C.GoString(C.garu_go_version())
	if nativeVersion != Version {
		return nil, fmt.Errorf("garu: native library version %s does not match Go package %s", nativeVersion, Version)
	}
	var normalize C.int
	if len(options) == 1 && options[0].NormalizeJamo {
		normalize = 1
	}
	var message *C.char
	ptr := C.garu_go_new((*C.uchar)(unsafe.Pointer(unsafe.SliceData(model))), C.size_t(len(model)), normalize, &message)
	runtime.KeepAlive(model)
	if message != nil {
		defer C.garu_go_string_free(message)
		return nil, fmt.Errorf("garu: %s", C.GoString(message))
	}
	if ptr == nil {
		return nil, errors.New("garu: native initialization failed")
	}
	a := &Analyzer{ptr: ptr}
	runtime.SetFinalizer(a, (*Analyzer).Close)
	return a, nil
}

// Close frees native model memory. Repeated calls are safe.
func (a *Analyzer) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ptr != nil {
		C.garu_go_free(a.ptr)
		a.ptr = nil
	}
	runtime.SetFinalizer(a, nil)
	return nil
}

// Analyze returns the upstream's best analysis, without additional normalization.
func (a *Analyzer) Analyze(text string) (Result, error) {
	results, err := a.AnalyzeTopN(text, 1)
	if err != nil {
		return Result{}, err
	}
	if len(results) == 0 {
		return Result{}, errors.New("garu: native analyzer returned no result")
	}
	return results[0], nil
}

// AnalyzeTopN returns up to n upstream candidates (which may be fewer than n).
func (a *Analyzer) AnalyzeTopN(text string, n int) ([]Result, error) {
	if a == nil {
		return nil, ErrClosed
	}
	if n < 1 || n > 100 {
		return nil, errors.New("garu: n must be between 1 and 100")
	}
	if !utf8.ValidString(text) {
		return nil, errors.New("garu: text must be valid UTF-8")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ptr == nil {
		return nil, ErrClosed
	}
	if text == "" {
		return []Result{{Tokens: []Token{}}}, nil
	}
	value := C.garu_go_analyze(a.ptr, (*C.uchar)(unsafe.Pointer(unsafe.StringData(text))), C.size_t(len(text)), C.size_t(n))
	runtime.KeepAlive(a)
	runtime.KeepAlive(text)
	if value == nil {
		return nil, errors.New("garu: native analysis returned null")
	}
	defer C.garu_go_string_free(value)
	var response struct {
		Results []Result `json:"results"`
		Error   string   `json:"error"`
	}
	if err := json.Unmarshal([]byte(C.GoString(value)), &response); err != nil {
		return nil, fmt.Errorf("garu: decode native result: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("garu: %s", response.Error)
	}
	return response.Results, nil
}

// Tokenize returns all token surfaces in upstream order, including punctuation.
func (a *Analyzer) Tokenize(text string) ([]string, error) {
	result, err := a.Analyze(text)
	if err != nil {
		return nil, err
	}
	tokens := make([]string, 0, len(result.Tokens))
	for _, t := range result.Tokens {
		tokens = append(tokens, t.Text)
	}
	return tokens, nil
}

// Nouns matches garu-ko's NNG/NNP filter, optionally including SL. It does not
// lowercase, deduplicate, or strip particles independently of the upstream core.
func (a *Analyzer) Nouns(text string, options ...NounsOptions) ([]string, error) {
	if len(options) > 1 {
		return nil, errors.New("garu: at most one NounsOptions value is supported")
	}
	result, err := a.Analyze(text)
	if err != nil {
		return nil, err
	}
	includeSL := len(options) == 1 && options[0].IncludeSL
	nouns := make([]string, 0, len(result.Tokens))
	for _, t := range result.Tokens {
		if t.POS == "NNG" || t.POS == "NNP" || (includeSL && t.POS == "SL") {
			nouns = append(nouns, t.Text)
		}
	}
	return nouns, nil
}
