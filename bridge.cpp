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
    size_t string_buf_len; // tracked after parse
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

static inline bool is_null_element(simdjson_element e) {
    return e.data[0] == 0 && e.data[1] == 0;
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

// --- Element type and value extraction ---

int simdjson_element_type(simdjson_element e) {
    if (is_null_element(e)) return -1;
    return static_cast<int>(to_cpp(e).type());
}

int simdjson_element_get_string(simdjson_element e, const char** out, size_t* out_len) {
    if (is_null_element(e)) return -1;
    std::string_view val;
    auto error = to_cpp(e).get(val);
    if (error) return static_cast<int>(error);
    *out = val.data();
    *out_len = val.size();
    return 0;
}

int simdjson_element_get_int64(simdjson_element e, int64_t* out) {
    if (is_null_element(e)) return -1;
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_uint64(simdjson_element e, uint64_t* out) {
    if (is_null_element(e)) return -1;
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_double(simdjson_element e, double* out) {
    if (is_null_element(e)) return -1;
    auto error = to_cpp(e).get(*out);
    if (error) return static_cast<int>(error);
    return 0;
}

int simdjson_element_get_bool(simdjson_element e, int* out) {
    if (is_null_element(e)) return -1;
    bool val;
    auto error = to_cpp(e).get(val);
    if (error) return static_cast<int>(error);
    *out = val ? 1 : 0;
    return 0;
}

// --- Object navigation ---

int simdjson_object_find_key(simdjson_element obj_elem, const char* key, size_t key_len,
                             simdjson_element* out) {
    if (is_null_element(obj_elem)) return -1;
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
    if (is_null_element(obj_elem)) return -1;
    simdjson::dom::object obj;
    auto error = to_cpp(obj_elem).get(obj);
    if (error) return static_cast<int>(error);
    *out = obj.size();
    return 0;
}

// Object iterator: stores {current, end} as two 16-byte C++ iterators.
struct obj_iter_state {
    simdjson::dom::object::iterator cur;
    simdjson::dom::object::iterator end;
};
static_assert(sizeof(obj_iter_state) == sizeof(simdjson_obj_iter), "obj_iter size mismatch");

static inline obj_iter_state obj_to_cpp(simdjson_obj_iter it) {
    obj_iter_state s;
    memcpy(&s, &it, sizeof(s));
    return s;
}

static inline simdjson_obj_iter obj_to_c(obj_iter_state s) {
    simdjson_obj_iter out;
    memcpy(&out, &s, sizeof(out));
    return out;
}

int simdjson_object_iter_begin(simdjson_element obj_elem, simdjson_obj_iter* out) {
    if (is_null_element(obj_elem)) return -1;
    simdjson::dom::object obj;
    auto error = to_cpp(obj_elem).get(obj);
    if (error) return static_cast<int>(error);
    *out = obj_to_c({obj.begin(), obj.end()});
    return 0;
}

int simdjson_object_iter_next(simdjson_obj_iter* it,
                              const char** out_key, size_t* out_key_len,
                              simdjson_element* out_val) {
    auto state = obj_to_cpp(*it);
    if (state.cur == state.end) return 1; // done
    auto field = *state.cur;
    *out_key = field.key.data();
    *out_key_len = field.key.size();
    *out_val = to_c(field.value);
    ++state.cur;
    *it = obj_to_c(state);
    return 0;
}

// Array iterator: stores {current, end} as two 16-byte C++ iterators.
struct arr_iter_state {
    simdjson::dom::array::iterator cur;
    simdjson::dom::array::iterator end;
};
static_assert(sizeof(arr_iter_state) == sizeof(simdjson_arr_iter), "arr_iter size mismatch");

static inline arr_iter_state arr_to_cpp(simdjson_arr_iter it) {
    arr_iter_state s;
    memcpy(&s, &it, sizeof(s));
    return s;
}

static inline simdjson_arr_iter arr_to_c(arr_iter_state s) {
    simdjson_arr_iter out;
    memcpy(&out, &s, sizeof(out));
    return out;
}

int simdjson_array_get_count(simdjson_element arr_elem, size_t* out) {
    if (is_null_element(arr_elem)) return -1;
    simdjson::dom::array arr;
    auto error = to_cpp(arr_elem).get(arr);
    if (error) return static_cast<int>(error);
    *out = arr.size();
    return 0;
}

int simdjson_array_iter_begin(simdjson_element arr_elem, simdjson_arr_iter* out) {
    if (is_null_element(arr_elem)) return -1;
    simdjson::dom::array arr;
    auto error = to_cpp(arr_elem).get(arr);
    if (error) return static_cast<int>(error);
    *out = arr_to_c({arr.begin(), arr.end()});
    return 0;
}

int simdjson_array_iter_next(simdjson_arr_iter* it, simdjson_element* out_val) {
    auto state = arr_to_cpp(*it);
    if (state.cur == state.end) return 1; // done
    *out_val = to_c(*state.cur);
    ++state.cur;
    *it = arr_to_c(state);
    return 0;
}

// --- Tape access ---

int simdjson_get_tape(simdjson_parser p,
                      const uint64_t** tape, size_t* tape_len,
                      const uint8_t** sbuf, size_t* sbuf_len) {
    auto* state = static_cast<parser_state*>(p);
    if (!state->has_doc) return -1;
    auto& doc = state->parser.doc;
    // Tape length: first root entry payload points past the last entry
    uint64_t first = doc.tape[0];
    size_t len = (first & 0x00ffffffffffffff) + 1;
    *tape = doc.tape.get();
    *tape_len = len;
    *sbuf = doc.string_buf.get();
    // Compute actual string buffer usage by finding max string end in tape
    size_t max_end = 0;
    for (size_t i = 0; i < len; i++) {
        uint8_t tag = doc.tape[i] >> 56;
        if (tag == '"') {
            uint64_t offset = doc.tape[i] & 0x00ffffffffffffff;
            uint32_t slen;
            memcpy(&slen, doc.string_buf.get() + offset, sizeof(uint32_t));
            size_t end = offset + 4 + slen + 1;
            if (end > max_end) max_end = end;
        }
    }
    *sbuf_len = max_end;
    return 0;
}

// --- Serialization ---

// Thread-local buffer for element serialization.
static thread_local std::string serialize_buf;

int simdjson_element_to_string(simdjson_element e, const char** out, size_t* out_len) {
    if (is_null_element(e)) return -1;
    serialize_buf = simdjson::minify(to_cpp(e));
    *out = serialize_buf.data();
    *out_len = serialize_buf.size();
    return 0;
}

// --- Runtime info ---

const char* simdjson_active_implementation(void) {
    return simdjson::get_active_implementation()->name().data();
}

} // extern "C"
