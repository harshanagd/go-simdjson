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

// simdjson_element is a value type (16 bytes) passed by value — no heap allocation.
typedef struct {
    uint64_t data[2];
} simdjson_element;

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

// Root access.
int simdjson_get_root(simdjson_parser p, simdjson_element* out);
int simdjson_root_type(simdjson_parser p);

// Element type and value extraction.
int simdjson_element_type(simdjson_element e);
int simdjson_element_get_string(simdjson_element e, const char** out, size_t* out_len);
int simdjson_element_get_int64(simdjson_element e, int64_t* out);
int simdjson_element_get_uint64(simdjson_element e, uint64_t* out);
int simdjson_element_get_double(simdjson_element e, double* out);
int simdjson_element_get_bool(simdjson_element e, int* out);

// Object navigation.
int simdjson_object_find_key(simdjson_element obj_elem, const char* key, size_t key_len,
                             simdjson_element* out);
int simdjson_object_get_count(simdjson_element obj_elem, size_t* out);
int simdjson_object_iter(simdjson_element obj_elem, size_t idx,
                         const char** out_key, size_t* out_key_len,
                         simdjson_element* out_val);

// Array navigation.
int simdjson_array_get_count(simdjson_element arr_elem, size_t* out);
int simdjson_array_at(simdjson_element arr_elem, size_t idx, simdjson_element* out);

// Runtime info.
const char* simdjson_active_implementation(void);

#ifdef __cplusplus
}
#endif

#endif // GO_SIMDJSON_BRIDGE_H
