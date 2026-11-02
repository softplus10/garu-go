//! Independent upstream reference: no Go wrapper or C ABI calls.
use garu_core::model::Analyzer;
use serde_json::json;
use std::io::{self, BufRead};

fn main() {
    let analyzer =
        Analyzer::from_bytes(include_bytes!("../../upstream/js/models/base.gmdl")).unwrap();
    for line in io::stdin().lock().lines() {
        let text = line.unwrap();
        let r = analyzer.analyze(&text);
        let tokens: Vec<_> = r
            .tokens
            .iter()
            .map(|t| {
                json!({
                    "text": t.text, "pos": t.pos.as_str(), "start": t.start, "end": t.end,
                })
            })
            .collect();
        let nouns: Vec<_> = r
            .tokens
            .iter()
            .filter(|t| matches!(t.pos.as_str(), "NNG" | "NNP" | "SL"))
            .map(|t| &t.text)
            .collect();
        println!(
            "{}",
            json!({"text":text,"result":{"tokens":tokens,"score":r.score},"nouns":nouns})
        );
    }
}
