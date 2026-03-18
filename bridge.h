// bridge.h — C bridge for simdjson C++ library.
// Exposes opaque handles and flat C functions for use from Go via CGo.

#ifndef GO_SIMDJSON_BRIDGE_H
#define GO_SIMDJSON_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Opaque handle to a simdjson parser (reusable, poolable).
typedef void* simdjson_parser;

// Result from simdjson_parse. Contains error info if parsing failed.
typedef struct {
    int ok;           // 1 on success, 0 on error
    int error_code;   // simdjson error code on failure
    const char* error_msg; // static error string on failure
} simdjson_result;

// Parser lifecycle.
simdjson_parser simdjson_parser_new(void);
void simdjson_parser_free(simdjson_parser p);

// Parse JSON. Parser retains ownership of internal data until next parse.
simdjson_result simdjson_parse(simdjson_parser p, const char* buf, size_t len);

// Get raw tape and string buffer pointers (zero-copy).
int simdjson_get_tape(simdjson_parser p,
                      const uint64_t** tape, size_t* tape_len,
                      const uint8_t** sbuf, size_t* sbuf_len);

// Runtime info.
const char* simdjson_active_implementation(void);

#ifdef __cplusplus
}
#endif

#endif // GO_SIMDJSON_BRIDGE_H
