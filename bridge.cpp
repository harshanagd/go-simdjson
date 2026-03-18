// bridge.cpp — C bridge implementation for simdjson.

#include "bridge.h"
#include "simdjson.h"

#include <cstring>

// static_assert that our value-type bridge matches the C++ type.
// static_assert that our value-type bridge matches the C++ type.
static_assert(sizeof(simdjson_element) == sizeof(simdjson::dom::element),
              "simdjson_element size mismatch");
static_assert(alignof(simdjson_element) >= alignof(simdjson::dom::element),
              "simdjson_element alignment mismatch");

struct parser_state {
    simdjson::dom::parser parser;
    simdjson::dom::element root;
    bool has_doc;
};

// Convert between C bridge type and C++ type via memcpy (safe, same size).
static inline simdjson::dom::element to_cpp(simdjson_element e) {
    simdjson::dom::element out;
    memcpy(&out, &e, sizeof(e));
    return out;
}

static inline simdjson_element to_c(simdjson::dom::element e) {
    simdjson_element out;
    memcpy(&out, &e, sizeof(out));
    return out;
}

extern "C" {

// --- Parser lifecycle ---

simdjson_parser simdjson_parser_new(void) {
    return new parser_state{};
}

void simdjson_parser_free(simdjson_parser p) {
    delete static_cast<parser_state*>(p);
}

simdjson_result simdjson_parse(simdjson_parser p, const char* buf, size_t len) {
    auto* state = static_cast<parser_state*>(p);
    state->has_doc = false;
    auto error = state->parser.parse(buf, len).get(state->root);
    if (error) {
        return {0, static_cast<int>(error), simdjson::error_message(error)};
    }
    state->has_doc = true;
    return {1, 0, nullptr};
}

// --- Root access ---

int simdjson_get_root(simdjson_parser p, simdjson_element* out) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    *out = to_c(state->root);
    return 0;
}

int simdjson_root_type(simdjson_parser p) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    return static_cast<int>(state->root.type());
}

// --- Element type and value extraction ---

int simdjson_element_type(simdjson_element e) {
    return static_cast<int>(to_cpp(e).type());
}

int simdjson_element_get_string(simdjson_element e, const char** out, size_t* out_len) {
    std::string_view val;
    auto error = to_cpp(e).get(val);
    if (error) return static_cast<int>(error);
    *out = val.data();
    *out_len = val.size();
    return 0;
}

int simdjson_element_get_int64(simdjson_element e, int64_t* out) {
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_uint64(simdjson_element e, uint64_t* out) {
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_double(simdjson_element e, double* out) {
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_bool(simdjson_element e, int* out) {
    bool val;
    auto error = to_cpp(e).get(val);
    if (error) return static_cast<int>(error);
    *out = val ? 1 : 0;
    return 0;
}

// --- Object navigation ---

int simdjson_object_find_key(simdjson_element obj_elem, const char* key, size_t key_len,
                             simdjson_element* out) {
    simdjson::dom::object obj;
    auto error = to_cpp(obj_elem).get(obj);
    if (error) return static_cast<int>(error);
    simdjson::dom::element val;
    error = obj[std::string_view(key, key_len)].get(val);
    if (error) return static_cast<int>(error);
    *out = to_c(val);
    return 0;
}

int simdjson_object_get_count(simdjson_element obj_elem, size_t* out) {
    simdjson::dom::object obj;
    auto error = to_cpp(obj_elem).get(obj);
    if (error) return static_cast<int>(error);
    *out = obj.size();
    return 0;
}

int simdjson_object_iter(simdjson_element obj_elem, size_t idx,
                         const char** out_key, size_t* out_key_len,
                         simdjson_element* out_val) {
    simdjson::dom::object obj;
    auto error = to_cpp(obj_elem).get(obj);
    if (error) return static_cast<int>(error);
    size_t i = 0;
    for (auto field : obj) {
        if (i == idx) {
            *out_key = field.key.data();
            *out_key_len = field.key.size();
            *out_val = to_c(field.value);
            return 0;
        }
        i++;
    }
    return -1;
}

// --- Array navigation ---

int simdjson_array_get_count(simdjson_element arr_elem, size_t* out) {
    simdjson::dom::array arr;
    auto error = to_cpp(arr_elem).get(arr);
    if (error) return static_cast<int>(error);
    *out = arr.size();
    return 0;
}

int simdjson_array_at(simdjson_element arr_elem, size_t idx, simdjson_element* out) {
    simdjson::dom::array arr;
    auto error = to_cpp(arr_elem).get(arr);
    if (error) return static_cast<int>(error);
    simdjson::dom::element val;
    error = arr.at(idx).get(val);
    if (error) return static_cast<int>(error);
    *out = to_c(val);
    return 0;
}

// --- Runtime info ---

const char* simdjson_active_implementation(void) {
    return simdjson::get_active_implementation()->name().data();
}

} // extern "C"
