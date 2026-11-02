//! C ABI only. Morphological analysis remains in the unmodified garu-core crate.
use garu_core::{model::Analyzer, types::AnalyzeResult, AnalyzerOptions};
use serde::Serialize;
use std::{
    ffi::{c_char, c_void, CString},
    panic::{catch_unwind, AssertUnwindSafe},
    slice, str,
};

#[derive(Serialize)]
struct Token {
    text: String,
    pos: &'static str,
    start: usize,
    end: usize,
    #[serde(skip_serializing_if = "Option::is_none")]
    score: Option<f32>,
}
#[derive(Serialize)]
struct Analysis {
    tokens: Vec<Token>,
    score: f32,
    elapsed: f64,
}
fn convert(r: AnalyzeResult) -> Analysis {
    Analysis {
        tokens: r
            .tokens
            .into_iter()
            .map(|t| Token {
                text: t.text,
                pos: t.pos.as_str(),
                start: t.start,
                end: t.end,
                score: t.score,
            })
            .collect(),
        score: r.score,
        elapsed: r.elapsed_ms,
    }
}
fn owned(s: String) -> *mut c_char {
    CString::new(s.replace('\0', "�")).unwrap().into_raw()
}
fn failure(message: &str) -> *mut c_char {
    owned(serde_json::json!({"error": message}).to_string())
}
#[no_mangle]
pub extern "C" fn garu_go_version() -> *const c_char {
    concat!(env!("CARGO_PKG_VERSION"), "\0").as_ptr().cast()
}
/// The caller supplies readable model bytes for this call only. The parsed analyzer owns its data.
#[no_mangle]
pub unsafe extern "C" fn garu_go_new(
    model: *const u8,
    len: usize,
    normalize_jamo: i32,
    error: *mut *mut c_char,
) -> *mut c_void {
    if error.is_null() {
        return std::ptr::null_mut();
    }
    *error = std::ptr::null_mut();
    let result = catch_unwind(AssertUnwindSafe(|| {
        if model.is_null() || len == 0 {
            return Err("model is empty".to_string());
        }
        Analyzer::from_bytes_with_options(
            slice::from_raw_parts(model, len),
            AnalyzerOptions {
                normalize_jamo: normalize_jamo != 0,
            },
        )
    }));
    match result {
        Ok(Ok(analyzer)) => Box::into_raw(Box::new(analyzer)).cast(),
        Ok(Err(message)) => {
            *error = owned(message);
            std::ptr::null_mut()
        }
        Err(_) => {
            *error = owned("garu-core panicked while loading model".into());
            std::ptr::null_mut()
        }
    }
}
/// The Go wrapper serializes all calls and Close; pointers never outlive their owner.
#[no_mangle]
pub unsafe extern "C" fn garu_go_analyze(
    analyzer: *mut c_void,
    text: *const u8,
    len: usize,
    top_n: usize,
) -> *mut c_char {
    let result = catch_unwind(AssertUnwindSafe(|| -> Result<String, String> {
        if analyzer.is_null() {
            return Err("analyzer is closed".into());
        }
        if text.is_null() && len != 0 {
            return Err("text is null".into());
        }
        let text = if len == 0 {
            ""
        } else {
            str::from_utf8(slice::from_raw_parts(text, len)).map_err(|e| e.to_string())?
        };
        let a = &*(analyzer.cast::<Analyzer>());
        let results: Vec<Analysis> = if top_n <= 1 {
            vec![convert(a.analyze(text))]
        } else {
            a.analyze_topn(text, top_n)
                .into_iter()
                .map(convert)
                .collect()
        };
        serde_json::to_string(&serde_json::json!({"results": results})).map_err(|e| e.to_string())
    }));
    match result {
        Ok(Ok(json)) => owned(json),
        Ok(Err(message)) => failure(&message),
        Err(_) => failure("garu-core panicked during analysis"),
    }
}
#[no_mangle]
pub unsafe extern "C" fn garu_go_free(analyzer: *mut c_void) {
    if !analyzer.is_null() {
        drop(Box::from_raw(analyzer.cast::<Analyzer>()));
    }
}
#[no_mangle]
pub unsafe extern "C" fn garu_go_string_free(value: *mut c_char) {
    if !value.is_null() {
        drop(CString::from_raw(value));
    }
}
