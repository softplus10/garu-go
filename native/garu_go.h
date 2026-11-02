#ifndef GARU_GO_H
#define GARU_GO_H
#include <stddef.h>
/* All returned strings are owned by Rust; release with garu_go_string_free. */
const char *garu_go_version(void);
void *garu_go_new(const unsigned char *model, size_t len, int normalize_jamo, char **error);
void garu_go_free(void *analyzer);
char *garu_go_analyze(void *analyzer, const unsigned char *text, size_t len, size_t top_n);
void garu_go_string_free(char *value);
#endif
