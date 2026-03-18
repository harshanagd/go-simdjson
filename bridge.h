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

// Element type codes matching simdjson::dom::element_type.
enum simdjson_element_type {
    SIMDJSON_TYPE_ARRAY   = '[',
    SIMDJSON_TYPE_OBJECT  = '{',
    SIMDJSON_TYPE_INT64   = 'l',
    SIMDJSON_TYPE_UINT64  = 'u',
    SIMDJSON_TYPE_DOUBLE  = 'd',
    SIMDJSON_TYPE_STRING  = '"',
    SIMDJSON_TYPE_BOOL    = 't',
    SIMDJSON_TYPE_NULL    = 'n',
};

// Parser lifecycle.
simdjson_parser simdjson_parser_new(void);
void simdjson_parser_free(simdjson_parser p);

// Parse JSON. Parser retains ownership of internal data until next parse.
simdjson_result simdjson_parse(simdjson_parser p, const char* buf, size_t len);

// Root element type.
int simdjson_root_type(simdjson_parser p);

// Find a string value by key in the root object.
// Returns 0 on success, non-zero on error.
// On success, *out_str and *out_len point into parser-owned memory.
int simdjson_find_string(simdjson_parser p, const char* key, size_t key_len,
                         const char** out_str, size_t* out_len);

// Get the root element as a string (for string root documents).
int simdjson_get_root_string(simdjson_parser p, const char** out_str, size_t* out_len);

// Get the root element as int64.
int simdjson_get_root_int64(simdjson_parser p, int64_t* out);

// Get the root element as uint64.
int simdjson_get_root_uint64(simdjson_parser p, uint64_t* out);

// Get the root element as double.
int simdjson_get_root_double(simdjson_parser p, double* out);

// Get the root element as bool.
int simdjson_get_root_bool(simdjson_parser p, int* out);

// Count elements in root array or keys in root object.
int simdjson_root_count(simdjson_parser p, size_t* out);

#ifdef __cplusplus
}
#endif

#endif // GO_SIMDJSON_BRIDGE_H
